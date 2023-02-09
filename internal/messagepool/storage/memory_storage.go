package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/takenet/deckard/internal/messagepool/entities"
	"github.com/takenet/deckard/internal/messagepool/utils"
)

// MemoryStorage is an implementation of the Storage Interface using memory.
// Currently only insert and pull functions are implemented.
type MemoryStorage struct {
	docs            map[string]*entities.Message
	configurations  map[string]*entities.QueueConfiguration
	lock            *sync.RWMutex
	internalCounter int64
}

var _ Storage = &MemoryStorage{}

func NewMemoryStorage(ctx context.Context) *MemoryStorage {
	storage := &MemoryStorage{
		docs:           make(map[string]*entities.Message),
		configurations: make(map[string]*entities.QueueConfiguration),

		lock:            &sync.RWMutex{},
		internalCounter: int64(0),
	}

	return storage
}

func (storage *MemoryStorage) ListQueueConfigurations(ctx context.Context) ([]*entities.QueueConfiguration, error) {
	configurations := make([]*entities.QueueConfiguration, len(storage.configurations))

	configurationIndex := 0
	for i := range storage.configurations {
		configurations[configurationIndex] = storage.configurations[i]

		configurationIndex += 1
	}

	return configurations, nil
}

func (storage *MemoryStorage) EditQueueConfiguration(_ context.Context, configuration *entities.QueueConfiguration) error {
	if configuration.MaxElements == 0 {
		return nil
	}

	if configuration.MaxElements < 0 {
		configuration.MaxElements = 0
	}

	storage.configurations[configuration.Queue] = configuration

	return nil
}

func (storage *MemoryStorage) GetQueueConfiguration(_ context.Context, queue string) (*entities.QueueConfiguration, error) {
	return storage.configurations[queue], nil
}

func (storage *MemoryStorage) Flush(_ context.Context) (deletedCount int64, err error) {
	storage.lock.Lock()
	count := int64(len(storage.docs))
	count += int64(len(storage.configurations))

	storage.docs = make(map[string]*entities.Message)
	storage.configurations = make(map[string]*entities.QueueConfiguration)

	storage.lock.Unlock()

	return count, nil
}

func (storage *MemoryStorage) Insert(_ context.Context, messages ...*entities.Message) (int64, int64, error) {
	inserted := int64(0)
	modified := int64(0)

	for i := range messages {
		if messages[i].Queue == "" {
			return 0, 0, fmt.Errorf("message has a invalid queue")
		}

		if messages[i].ID == "" {
			return 0, 0, fmt.Errorf("message has a invalid ID")
		}

		key := getKey(messages[i])

		storage.lock.RLock()
		_, present := storage.docs[key]
		storage.lock.RUnlock()

		storage.lock.Lock()
		if present {
			modified++

			storage.docs[key].Description = messages[i].Description
			storage.docs[key].ExpiryDate = messages[i].ExpiryDate
			storage.docs[key].Metadata = messages[i].Metadata
			storage.docs[key].Payload = messages[i].Payload
			storage.docs[key].StringPayload = messages[i].StringPayload
		} else {
			inserted++

			storage.internalCounter += 1
			messages[i].InternalId = storage.internalCounter
			now := time.Now()
			messages[i].LastUsage = &now
			storage.docs[key] = messages[i]
		}

		storage.lock.Unlock()
	}

	return inserted, modified, nil
}

func (storage *MemoryStorage) GetStringInternalId(_ context.Context, message *entities.Message) string {
	if message.InternalId == nil {
		return ""
	}

	return strconv.FormatInt(message.InternalId.(int64), 10)
}

func getKey(message *entities.Message) string {
	return message.Queue + ":" + message.ID
}

func (storage *MemoryStorage) Count(_ context.Context, opts *FindOptions) (int64, error) {
	count := int64(0)

	storage.lock.RLock()
	for _, value := range storage.docs {
		matches, err := messageMatchesFilter(value, opts)

		if err != nil {
			return 0, err
		}

		if matches {
			count++
		}
	}
	storage.lock.RUnlock()

	return count, nil
}

func messageMatchesFilter(q *entities.Message, opts *FindOptions) (bool, error) {
	if opts == nil {
		return true, nil
	}

	matchesInternal, err := matchesInternalFilter(q, opts.InternalFilter)

	return matchesInternal, err
}

func matchesInternalFilter(message *entities.Message, filter *InternalFilter) (bool, error) {
	if filter == nil {
		return true, nil
	}

	if filter.Queue != "" && message.Queue != filter.Queue {
		return false, nil
	}

	if filter.QueuePrefix != "" && message.QueuePrefix != filter.QueuePrefix {
		return false, nil
	}

	if filter.Ids != nil && len(*filter.Ids) > 0 {
		contains := false
		for _, id := range *filter.Ids {
			if id == message.ID {
				contains = true
				break
			}
		}

		if !contains {
			return false, nil
		}
	}

	if filter.InternalIdBreakpointGt != "" {
		internalId := message.InternalId.(int64)
		breakpoint, err := utils.StrToInt64(filter.InternalIdBreakpointGt)

		if err != nil {
			return false, errors.New("breakpoint for memory storage need to be a valid int64 message")
		}

		if internalId <= breakpoint {
			return false, nil
		}
	}

	if filter.InternalIdBreakpointLte != "" {
		internalId := message.InternalId.(int64)
		breakpoint, err := utils.StrToInt64(filter.InternalIdBreakpointLte)

		if err != nil {
			return false, errors.New("breakpoint for memory storage need to be a valid int64 message")
		}

		if internalId > breakpoint {
			return false, nil
		}
	}

	if filter.ExpiryDate != nil && message.ExpiryDate.After(*filter.ExpiryDate) {
		return false, nil
	}

	return true, nil
}

func (storage *MemoryStorage) Find(_ context.Context, opt *FindOptions) ([]entities.Message, error) {
	var messages []entities.Message

	storage.lock.RLock()
	defer storage.lock.RUnlock()

	for k := range storage.docs {
		if (opt == nil || opt.InternalFilter == nil || opt.InternalFilter.ExpiryDate == nil) && isExpired(storage.docs[k]) {
			continue
		}

		matches, err := messageMatchesFilter(storage.docs[k], opt)

		if err != nil {
			return nil, err
		}

		if matches {
			messages = append(messages, *storage.docs[k])
		}
	}

	if opt != nil && opt.Sort != nil {
		sort.SliceStable(messages, func(i, j int) bool {
			for _, key := range (*opt.Sort).Keys() {
				value, _ := (*opt.Sort).Get(key)

				switch key {
				case "expiry_date":
					if (messages[i].ExpiryDate != time.Time{} || messages[j].ExpiryDate != time.Time{}) {
						if value == -1 {
							return messages[i].ExpiryDate.After(messages[j].ExpiryDate)
						}

						return messages[i].ExpiryDate.Before(messages[j].ExpiryDate)
					}
				case "score":
					if value == -1 {
						return messages[i].Score > messages[j].Score
					}

					return messages[i].Score < messages[j].Score
				case "_id":
					if value == -1 {
						return messages[i].InternalId.(int64) > messages[j].InternalId.(int64)
					}

					return messages[i].InternalId.(int64) < messages[j].InternalId.(int64)
				}
			}

			return messages[i].InternalId.(int64) < messages[j].InternalId.(int64)
		})
	}

	if opt != nil && opt.Limit > 0 && len(messages) > int(opt.Limit) {
		messages = messages[:opt.Limit]
	}

	return messages, nil
}

func isExpired(message *entities.Message) bool {
	return !message.Timeless && (message.ExpiryDate.Before(time.Now()) || message.ExpiryDate.Equal(time.Now()))
}

func (storage *MemoryStorage) Remove(_ context.Context, queue string, ids ...string) (deleted int64, err error) {
	count := int64(0)

	storage.lock.Lock()
	for key, message := range storage.docs {
		if message.Queue != queue {
			continue
		}

		for _, id := range ids {
			if message.ID == id {
				delete(storage.docs, key)

				count++
			}
		}
	}
	storage.lock.Unlock()

	return count, nil
}

func (storage *MemoryStorage) Ack(_ context.Context, message *entities.Message) (modifiedCount int64, err error) {
	storage.lock.RLock()
	value, contains := storage.docs[getKey(message)]
	storage.lock.RUnlock()

	if !contains {
		return 0, nil
	}

	storage.lock.Lock()
	value.TotalScoreSubtract += message.LastScoreSubtract
	value.UsageCount += 1
	value.LastUsage = message.LastUsage
	value.LastScoreSubtract = message.LastScoreSubtract
	value.Score = message.Score
	value.Breakpoint = message.Breakpoint
	storage.lock.Unlock()

	return 1, nil
}

func (storage *MemoryStorage) ListQueueNames(_ context.Context) (queues []string, err error) {
	temporaryMap := make(map[string]bool)

	storage.lock.RLock()
	for _, message := range storage.docs {
		temporaryMap[message.Queue] = true
	}
	storage.lock.RUnlock()

	result := make([]string, 0, len(temporaryMap))
	for key := range temporaryMap {
		result = append(result, key)
	}

	return result, nil
}

func (storage *MemoryStorage) ListQueuePrefixes(_ context.Context) (queues []string, err error) {
	temporaryMap := make(map[string]bool)

	storage.lock.RLock()
	for _, message := range storage.docs {
		temporaryMap[message.QueuePrefix] = true
	}
	storage.lock.RUnlock()

	result := make([]string, 0, len(temporaryMap))
	for key := range temporaryMap {
		result = append(result, key)
	}

	return result, nil
}

func (storage *MemoryStorage) Close(ctx context.Context) error {
	// do nothing

	return nil
}
