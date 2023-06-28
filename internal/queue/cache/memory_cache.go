package cache

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/takenet/deckard/internal/queue/entities"
	"github.com/takenet/deckard/internal/queue/utils"
)

// MemoryStorage is an implementation of the Storage Interface using memory.
// Currently only insert and pull functions are implemented.
type MemoryCache struct {
	queues           map[string]*list.List
	processingQueues map[string]*list.List
	lockAckQueues    map[string]*list.List
	lockNackQueues   map[string]*list.List
	lock             *sync.Mutex
	keys             map[string]string
}

type MemoryMessageEntry struct {
	score float64
	id    string
}

var _ Cache = &MemoryCache{}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		queues:           make(map[string]*list.List),
		processingQueues: make(map[string]*list.List),
		lockAckQueues:    make(map[string]*list.List),
		lockNackQueues:   make(map[string]*list.List),
		keys:             make(map[string]string),
		lock:             &sync.Mutex{},
	}
}

func (cache *MemoryCache) Flush(_ context.Context) {
	cache.lock.Lock()
	cache.queues = make(map[string]*list.List)
	cache.processingQueues = make(map[string]*list.List)
	cache.lockAckQueues = make(map[string]*list.List)
	cache.lockNackQueues = make(map[string]*list.List)
	cache.keys = make(map[string]string)
	cache.lock.Unlock()
}

func (cache *MemoryCache) Remove(ctx context.Context, queue string, ids ...string) (removed int64, err error) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	removedElements := int64(0)

	result, dataResult, err := cache.removeFromSlice(ctx, cache.queues[queue], ids...)
	if err != nil {
		return result, err
	}
	cache.queues[queue] = dataResult
	removedElements += result

	result, dataResult, err = cache.removeFromSlice(ctx, cache.processingQueues[queue], ids...)
	if err != nil {
		return result, err
	}
	cache.processingQueues[queue] = dataResult
	removedElements += result

	result, dataResult, err = cache.removeFromSlice(ctx, cache.lockAckQueues[queue], ids...)
	if err != nil {
		return result, err
	}
	cache.lockAckQueues[queue] = dataResult
	removedElements += result

	result, dataResult, err = cache.removeFromSlice(ctx, cache.lockNackQueues[queue], ids...)
	if err != nil {
		return result, err
	}
	cache.lockNackQueues[queue] = dataResult
	removedElements += result

	return removedElements, nil
}

func (cache *MemoryCache) removeFromSlice(_ context.Context, data *list.List, ids ...string) (removed int64, dataResult *list.List, err error) {
	if data == nil || data.Len() == 0 {
		return 0, data, nil
	}

	count := int64(0)

	for _, id := range ids {
		for q := data.Front(); q != nil; q = q.Next() {
			value := q.Value.(*MemoryMessageEntry)

			if value.id == id {
				count++

				data.Remove(q)

				break
			}
		}
	}

	return count, data, nil
}

func (cache *MemoryCache) MakeAvailable(_ context.Context, message *entities.Message) (bool, error) {
	if message.Queue == "" {
		return false, errors.New("invalid message queue")
	}

	entry := createEntry(message)

	cache.lock.Lock()
	defer cache.lock.Unlock()

	var result bool

	cache.processingQueues[message.Queue], result = removeEntry(entry, cache.processingQueues[message.Queue])
	if result {
		cache.queues[message.Queue], _ = insertEntry(entry, cache.queues[message.Queue])
	}

	return result, nil
}

func (cache *MemoryCache) ListQueues(ctx context.Context, pattern string, poolType entities.PoolType) (queues []string, err error) {
	result := make([]string, 0)

	cache.lock.Lock()
	defer cache.lock.Unlock()

	var queueMap *map[string]*list.List
	switch poolType {
	case entities.PRIMARY_POOL:
		queueMap = &cache.queues
	case entities.PROCESSING_POOL:
		queueMap = &cache.processingQueues
	case entities.LOCK_ACK_POOL:
		queueMap = &cache.lockAckQueues
	case entities.LOCK_NACK_POOL:
		queueMap = &cache.lockNackQueues
	}

	for queue := range *queueMap {
		if (*queueMap)[queue] == nil {
			continue
		}

		if utils.MatchGlob(queue, pattern) && (*queueMap)[queue].Len() > 0 {
			result = append(result, queue)
		}
	}

	return result, nil
}

func (cache *MemoryCache) LockMessage(_ context.Context, message *entities.Message, lockType LockType) (bool, error) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	if message.Queue == "" {
		return false, errors.New("invalid queue to lock")
	}

	if message.LockMs <= 0 {
		return false, errors.New("invalid lock seconds")
	}

	if _, ok := cache.processingQueues[message.Queue]; !ok {
		return false, nil
	}

	entry := createEntry(message)

	var result bool
	cache.processingQueues[message.Queue], result = removeEntry(entry, cache.processingQueues[message.Queue])

	if result {
		entry.score = float64(utils.NowMs() + message.LockMs)

		switch lockType {
		case LOCK_ACK:
			cache.lockAckQueues[message.Queue], _ = insertEntry(entry, cache.lockAckQueues[message.Queue])
		case LOCK_NACK:
			cache.lockNackQueues[message.Queue], _ = insertEntry(entry, cache.lockNackQueues[message.Queue])
		}
	}

	return result, nil
}

func (cache *MemoryCache) UnlockMessages(ctx context.Context, queue string, lockType LockType) (messages []string, err error) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	var lockScore float64
	var data map[string]*list.List
	if lockType == LOCK_ACK {
		data = cache.lockAckQueues
		lockScore = float64(utils.NowMs())

	} else {
		data = cache.lockNackQueues
		lockScore = 0
	}

	nowMs := utils.NowMs()

	unlockedMessages := make([]string, 0)
	for queue := range data {
		list := data[queue]

		if list == nil {
			continue
		}

		for e := list.Front(); e != nil; e = e.Next() {
			value := e.Value.(*MemoryMessageEntry)

			if nowMs >= int64(value.score) {
				list.Remove(e)

				value.score = lockScore

				cache.queues[queue], _ = insertEntry(value, cache.queues[queue])

				unlockedMessages = append(unlockedMessages, value.id)
			}
		}
	}

	return unlockedMessages, nil
}

func (cache *MemoryCache) PullMessages(ctx context.Context, queue string, n int64, minScore *float64, maxScore *float64) (ids []string, err error) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	if cache.queues[queue] == nil || cache.queues[queue].Len() == 0 {
		return nil, nil
	}

	total := int64(cache.queues[queue].Len())

	result := make([]string, 0, utils.MinInt64(n, total))

	filteredOut := make([]*MemoryMessageEntry, 0)
	for i := int64(0); i < n && i < total && cache.queues[queue].Len() != 0; i++ {
		element := cache.queues[queue].Front()

		entry := element.Value.(*MemoryMessageEntry)

		cache.queues[queue], _ = removeEntry(entry, cache.queues[queue])

		if minScore != nil && entry.score < *minScore {
			filteredOut = append(filteredOut, entry)

			continue
		}

		if maxScore != nil && entry.score > *maxScore {
			filteredOut = append(filteredOut, entry)

			continue
		}

		processingEntry := &MemoryMessageEntry{
			id:    entry.id,
			score: float64(time.Now().Unix()),
		}

		cache.processingQueues[queue], _ = insertEntry(processingEntry, cache.processingQueues[queue])

		result = append(result, entry.id)
	}

	for _, entry := range filteredOut {
		cache.queues[queue], _ = insertEntry(entry, cache.queues[queue])
	}

	return result, nil
}

func (cache *MemoryCache) TimeoutMessages(_ context.Context, queue string, timeout time.Duration) (ids []string, err error) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	if cache.processingQueues[queue] == nil {
		return nil, nil
	}

	timeoutTime := time.Now().Add(-1 * timeout).Unix()

	count := int64(0)

	result := make([]string, 0)
	for e := cache.processingQueues[queue].Front(); e != nil; e = e.Next() {
		value := e.Value.(*MemoryMessageEntry)

		if value.score <= float64(timeoutTime) {
			value.score = entities.MaxScore()

			result = append(result, value.id)

			count++
			cache.processingQueues[queue], _ = removeEntry(value, cache.processingQueues[queue])
			cache.queues[queue], _ = insertEntry(value, cache.queues[queue])
		}
	}

	return result, nil
}

func (cache *MemoryCache) Insert(ctx context.Context, queue string, messages ...*entities.Message) ([]string, error) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	for _, message := range messages {
		if message.Queue != queue {
			return nil, errors.New("invalid queue to insert data")
		}
	}

	insertIds := make([]string, 0)
	insertions := make([]*entities.Message, 0)
	for _, message := range messages {
		isPresent := cache.isPresentOnAnyPool(ctx, queue, message.ID)

		if isPresent {
			continue
		}

		insertions = append(insertions, message)
		insertIds = append(insertIds, message.ID)
	}

	for _, message := range insertions {
		cache.queues[message.Queue], _ = insertEntry(createEntry(message), cache.queues[message.Queue])
	}

	return insertIds, nil
}

func createEntry(message *entities.Message) *MemoryMessageEntry {
	return &MemoryMessageEntry{
		id:    message.ID,
		score: message.Score,
	}
}

func insertEntry(data *MemoryMessageEntry, listData *list.List) (*list.List, bool) {
	if listData == nil {
		listData = list.New()
	}

	_, removed := removeEntry(data, listData)

	if listData.Len() == 0 {
		listData.PushBack(data)

		return listData, !removed
	}

	for q := listData.Front(); q != nil; q = q.Next() {
		value := q.Value.(*MemoryMessageEntry)

		if data.score <= value.score {
			listData.InsertBefore(data, q)

			return listData, !removed
		}
	}

	listData.PushBack(data)

	return listData, !removed
}

func removeEntry(data *MemoryMessageEntry, listToRemove *list.List) (*list.List, bool) {
	if listToRemove == nil || listToRemove.Len() == 0 {
		return listToRemove, false
	}

	for q := listToRemove.Front(); q != nil; q = q.Next() {
		value := q.Value.(*MemoryMessageEntry)

		if value.id != data.id {
			continue
		}

		listToRemove.Remove(q)

		return listToRemove, true
	}

	return listToRemove, false
}

func (cache *MemoryCache) IsProcessing(ctx context.Context, queue string, id string) (bool, error) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	return cache.isPresentOnPool(ctx, &cache.processingQueues, queue, id), nil
}

func (cache *MemoryCache) isPresentOnAnyPool(ctx context.Context, queue string, id string) bool {
	isActive := cache.isPresentOnPool(ctx, &cache.queues, queue, id)
	isProcessing := cache.isPresentOnPool(ctx, &cache.processingQueues, queue, id)
	isLockedAck := cache.isPresentOnPool(ctx, &cache.lockAckQueues, queue, id)
	isLockedNack := cache.isPresentOnPool(ctx, &cache.lockNackQueues, queue, id)

	return isActive || isProcessing || isLockedAck || isLockedNack
}

func (cache *MemoryCache) isPresentOnPool(_ context.Context, pool *map[string]*list.List, queue string, id string) bool {
	if pool == nil {
		return false
	}

	data := (*pool)[queue]

	if data == nil || data.Len() == 0 {
		return false
	}

	for q := data.Front(); q != nil; q = q.Next() {
		value := q.Value.(*MemoryMessageEntry)

		if value.id == id {
			return true
		}
	}

	return false
}
func (cache *MemoryCache) Get(_ context.Context, key string) (value string, err error) {
	cache.lock.Lock()
	defer cache.lock.Unlock()
	return cache.keys[key], nil
}

func (cache *MemoryCache) Set(_ context.Context, key string, value string) error {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	cache.keys[key] = value

	return nil
}

func (cache *MemoryCache) Close(ctx context.Context) error {
	// do nothing

	return nil
}
