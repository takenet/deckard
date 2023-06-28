package queue

//go:generate mockgen -destination=../mocks/mock_queue.go -package=mocks -source=queue.go

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/takenet/deckard/internal/audit"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/metrics"
	"github.com/takenet/deckard/internal/queue/cache"
	"github.com/takenet/deckard/internal/queue/entities"
	"github.com/takenet/deckard/internal/queue/storage"
	"go.opentelemetry.io/otel/attribute"
)

type DeckardQueue interface {
	AddMessagesToCache(ctx context.Context, messages ...*entities.Message) (int64, error)
	AddMessagesToStorage(ctx context.Context, messages ...*entities.Message) (inserted int64, updated int64, err error)
	Nack(ctx context.Context, message *entities.Message, timestamp time.Time, reason string) (bool, error)
	Ack(ctx context.Context, message *entities.Message, timestamp time.Time, reason string) (bool, error)
	TimeoutMessages(ctx context.Context, queue string) ([]string, error)
	Pull(ctx context.Context, queue string, n int64, scoreFilter int64) (*[]entities.Message, error)
	Remove(ctx context.Context, queue string, reason string, ids ...string) (cacheRemoved int64, storageRemoved int64, err error)
	Count(ctx context.Context, opts *storage.FindOptions) (int64, error)
	GetStorageMessages(ctx context.Context, opt *storage.FindOptions) ([]entities.Message, error)

	// Flushes all deckard content from cache and storage.
	// Used only for memory instance.
	Flush(ctx context.Context) (bool, error)
}

type Queue struct {
	storage                   storage.Storage
	cache                     cache.Cache
	auditor                   audit.Auditor
	QueueConfigurationService QueueConfigurationService
}

var _ DeckardQueue = &Queue{}

func NewQueue(auditor audit.Auditor, storageImpl storage.Storage, queueService QueueConfigurationService, cache cache.Cache) *Queue {
	queue := Queue{
		cache:                     cache,
		storage:                   storageImpl,
		auditor:                   auditor,
		QueueConfigurationService: queueService,
	}

	return &queue
}

func (pool *Queue) Count(ctx context.Context, opts *storage.FindOptions) (int64, error) {
	if opts == nil {
		opts = &storage.FindOptions{}
	}

	result, err := pool.storage.Count(ctx, &storage.FindOptions{
		InternalFilter: opts.InternalFilter,
	})

	if err != nil {
		logger.S(ctx).Error("Error counting elements: ", err)

		return int64(0), errors.New("internal error counting elements")
	}

	return result, nil
}

func (pool *Queue) GetStorageMessages(ctx context.Context, opt *storage.FindOptions) ([]entities.Message, error) {
	result, err := pool.storage.Find(ctx, opt)

	if err != nil {
		logger.S(ctx).Error("Error getting storage elements: ", err)

		return nil, err
	}

	return result, nil
}

func (pool *Queue) AddMessagesToStorage(ctx context.Context, messages ...*entities.Message) (inserted int64, updated int64, err error) {
	queues := make(map[string]bool)

	for i := range messages {
		message := messages[i]

		queueName := message.Queue

		if queues[queueName] {
			continue
		}

		queues[queueName] = true
	}

	insertions, updates, err := pool.storage.Insert(ctx, messages...)

	if err != nil {
		logger.S(ctx).Error("Error inserting storage data: ", err)
	}

	return insertions, updates, err
}

func (pool *Queue) AddMessagesToCache(ctx context.Context, messages ...*entities.Message) (int64, error) {
	return pool.AddMessagesToCacheWithAuditReason(ctx, "", messages...)
}

func (pool *Queue) AddMessagesToCacheWithAuditReason(ctx context.Context, reason string, messages ...*entities.Message) (int64, error) {
	membersByQueue := make(map[string][]*entities.Message)
	for i := range messages {
		queueMessages, ok := membersByQueue[messages[i].Queue]
		if !ok {
			queueMessages = make([]*entities.Message, 0)
		}

		queueMessages = append(queueMessages, messages[i])
		membersByQueue[messages[i].Queue] = queueMessages
	}

	count := int64(0)
	for queueName := range membersByQueue {
		elements := membersByQueue[queueName]

		insertions, err := pool.cache.Insert(ctx, queueName, elements...)

		if err != nil {
			logger.S(ctx).Error("Error inserting cache data: ", err)

			return 0, fmt.Errorf("error inserting cache data: %w", err)
		}

		for _, id := range insertions {
			pool.auditor.Store(ctx, audit.Entry{
				ID:     id,
				Queue:  queueName,
				Signal: audit.INSERT_CACHE,
				Reason: reason,
			})
		}

		count += int64(len(insertions))
	}

	return count, nil
}

func (pool *Queue) Nack(ctx context.Context, message *entities.Message, timestamp time.Time, reason string) (bool, error) {
	if message == nil {
		return false, nil
	}

	if message.Queue == "" {
		return false, fmt.Errorf("message has a invalid queue")
	}

	if message.ID == "" {
		return false, fmt.Errorf("message has a invalid ID")
	}

	defer func() {
		metrics.QueueNack.Add(ctx, 1, attribute.String("queue", entities.GetQueuePrefix(message.Queue)), attribute.String("reason", reason))
	}()

	if message.LockMs > 0 {
		result, err := pool.cache.LockMessage(ctx, message, cache.LOCK_NACK)

		if err != nil {
			logger.S(ctx).Error("Error locking message: ", err)

			return false, err
		}

		pool.auditor.Store(ctx, audit.Entry{
			ID:                message.ID,
			Queue:             message.Queue,
			LastScoreSubtract: message.LastScoreSubtract,
			Breakpoint:        message.Breakpoint,
			Signal:            audit.NACK,
			Reason:            reason,
			LockMs:            message.LockMs,
		})

		return result, nil
	}

	message.Score = entities.MaxScore()

	result, err := pool.cache.MakeAvailable(ctx, message)

	if err != nil {
		logger.S(ctx).Error("Error making element available: ", err)

		return false, err
	}

	pool.auditor.Store(ctx, audit.Entry{
		ID:                message.ID,
		Queue:             message.Queue,
		LastScoreSubtract: message.LastScoreSubtract,
		Breakpoint:        message.Breakpoint,
		Signal:            audit.NACK,
		Reason:            reason,
	})

	return result, nil
}

func (pool *Queue) Ack(ctx context.Context, message *entities.Message, timestamp time.Time, reason string) (bool, error) {
	if message == nil {
		return false, nil
	}

	if message.Queue == "" {
		return false, fmt.Errorf("message has a invalid queue")
	}

	if message.ID == "" {
		return false, fmt.Errorf("message has a invalid ID")
	}

	message.LastUsage = &timestamp
	message.UpdateScore()

	_, err := pool.storage.Ack(ctx, message)

	if err != nil {
		logger.S(ctx).Error("Error acking element on storage: ", err)

		return false, err
	}

	metrics.QueueAck.Add(ctx, 1, attribute.String("queue", entities.GetQueuePrefix(message.Queue)), attribute.String("reason", reason))

	if message.LockMs > 0 {
		result, err := pool.cache.LockMessage(ctx, message, cache.LOCK_ACK)

		if err != nil {
			logger.S(ctx).Error("Error locking element: ", err)

			return false, err
		}

		pool.auditor.Store(ctx, audit.Entry{
			ID:                message.ID,
			Queue:             message.Queue,
			LastScoreSubtract: message.LastScoreSubtract,
			Breakpoint:        message.Breakpoint,
			Signal:            audit.ACK,
			Reason:            reason,
			LockMs:            message.LockMs,
		})

		return result, nil
	}

	result, availableErr := pool.cache.MakeAvailable(ctx, message)
	if availableErr != nil {
		logger.S(ctx).Error("Error making element available: ", availableErr)

		return false, availableErr
	}

	pool.auditor.Store(ctx, audit.Entry{
		ID:                message.ID,
		Queue:             message.Queue,
		LastScoreSubtract: message.LastScoreSubtract,
		Breakpoint:        message.Breakpoint,
		Signal:            audit.ACK,
		Reason:            reason,
	})

	return result, nil
}

func (pool *Queue) TimeoutMessages(ctx context.Context, queue string) ([]string, error) {
	ids, err := pool.cache.TimeoutMessages(context.Background(), queue, cache.DefaultCacheTimeout)

	if err != nil {
		logger.S(ctx).Error("Error on queue timeouts: ", err)

		return nil, err
	}

	if len(ids) > 0 {
		metrics.QueueTimeout.Add(ctx, int64(len(ids)), attribute.String("queue", entities.GetQueuePrefix(queue)))

		for _, id := range ids {
			pool.auditor.Store(ctx, audit.Entry{
				ID:     id,
				Queue:  queue,
				Signal: audit.TIMEOUT,
			})
		}
	}

	return ids, nil
}

func (pool *Queue) Pull(ctx context.Context, queue string, n int64, scoreFilter int64) (*[]entities.Message, error) {
	ids, err := pool.cache.PullMessages(ctx, queue, n, scoreFilter)
	if err != nil {
		logger.S(ctx).Error("Error pulling cache elements: ", err)

		return nil, err
	}

	if len(ids) == 0 {
		metrics.QueueEmptyQueue.Add(ctx, 1, attribute.String("queue", entities.GetQueuePrefix(queue)))

		return nil, nil
	}

	orderedSort := orderedmap.NewOrderedMap[string, int]()
	orderedSort.Set("score", 1)

	messages, notFound, err := pool.getFromStorage(ctx, ids, queue, orderedSort, false)
	if err != nil {
		return nil, err
	}

	if len(notFound) == 0 {
		return &messages, nil
	}

	// Retry not found elements
	retryMessages, retryNotFound, err := pool.getFromStorage(ctx, notFound, queue, orderedSort, true)
	if err != nil {
		return nil, err
	}

	if len(retryNotFound) > 0 {
		metrics.QueueNotFoundInStorage.Add(ctx, int64(len(notFound)), attribute.String("queue", entities.GetQueuePrefix(queue)))

		for _, id := range retryNotFound {
			pool.auditor.Store(ctx, audit.Entry{
				ID:     id,
				Signal: audit.MISSING_STORAGE,
				Queue:  queue,
			})
		}

		// Remove inconsistent elements from cache.
		// They was probably removed from storage but not from the cache.
		_, _ = pool.cache.Remove(ctx, queue, retryNotFound...)
	}

	if len(retryMessages) > 0 {
		messages = append(messages, retryMessages...)

		// Sort by score
		sort.Slice(messages, func(i, j int) bool {
			return messages[i].Score < messages[j].Score
		})
	}

	if len(messages) == 0 {
		metrics.QueueEmptyQueueStorage.Add(ctx, 1, attribute.String("queue", entities.GetQueuePrefix(queue)))

		return nil, nil
	}

	return &messages, nil
}

func (pool *Queue) getFromStorage(ctx context.Context, ids []string, queue string, sort *orderedmap.OrderedMap[string, int], retry bool) ([]entities.Message, []string, error) {
	messages, err := pool.storage.Find(ctx, &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &ids,
			Queue: queue,
		},
		Limit: int64(len(ids)),
		Retry: retry,
	})

	if err != nil {
		logger.S(ctx).Error("Error getting data from storage: ", err)

		return nil, nil, fmt.Errorf("error getting storage data: %w", err)
	}

	notFound := notFoundIds(ids, messages)

	return messages, notFound, nil
}

func (pool *Queue) Remove(ctx context.Context, queue string, reason string, ids ...string) (cacheRemoved int64, storageRemoved int64, err error) {
	cacheCount, err := pool.cache.Remove(ctx, queue, ids...)

	if err != nil {
		logger.S(ctx).Error("Error removing elements from cache: ", err)

		return 0, 0, err
	}

	storageCount, err := pool.storage.Remove(ctx, queue, ids...)

	if err != nil {
		logger.S(ctx).Error("Error removing elements from storage: ", err)

		return cacheCount, 0, err
	}

	for i := range ids {
		pool.auditor.Store(ctx, audit.Entry{
			ID:     ids[i],
			Queue:  queue,
			Signal: audit.REMOVE,
			Reason: reason,
		})
	}

	return cacheCount, storageCount, nil
}

func (pool *Queue) Flush(ctx context.Context) (bool, error) {
	pool.cache.Flush(ctx)
	_, err := pool.storage.Flush(ctx)

	if err != nil {
		logger.S(ctx).Error("Error flushing deckard: ", err)

		return false, err
	}

	return true, err
}

// notFoundIds calculate the difference between the ids and the found messages.
func notFoundIds(ids []string, messages []entities.Message) []string {
	notFound := make([]string, 0)
	found := make(map[string]struct{})

	for _, q := range messages {
		found[q.ID] = struct{}{}
	}

	for _, id := range ids {
		if _, ok := found[id]; !ok {
			notFound = append(notFound, id)
		}
	}

	return notFound
}
