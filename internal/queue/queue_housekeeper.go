package queue

import (
	"context"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/takenet/deckard/internal/audit"
	"github.com/takenet/deckard/internal/dtime"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/metrics"
	"github.com/takenet/deckard/internal/queue/cache"
	"github.com/takenet/deckard/internal/queue/configuration"
	"github.com/takenet/deckard/internal/queue/message"
	"github.com/takenet/deckard/internal/queue/pool"
	"github.com/takenet/deckard/internal/queue/score"
	"github.com/takenet/deckard/internal/queue/storage"
	"github.com/takenet/deckard/internal/shutdown"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ProcessTimeoutMessages process messages timeout that have not been acked (or nacked) for more
// than 5 minutes returning them back to the active queue with the maximum score.
// TODO: allow each queue to have its own deadline for timeout.
// TODO: change the behavior of this so it doesn't need to load all queue names in memory, we could use the storage to list queues with a cursor
// TODO: we could even change the timeout mechanism to be not based on the queue name
func ProcessTimeoutMessages(ctx context.Context, queue *Queue) error {
	t := dtime.Now()

	queues, err := queue.cache.ListQueues(ctx, "*", pool.PROCESSING_POOL)

	if err != nil {
		logger.S(ctx).Error("Error list processing queues: ", err)
		return err
	}

	logger.S(ctx).Debugf("Processing queues timeout for: %v", queues)

	total := int64(0)
	for _, queueName := range queues {
		if shutdown.Ongoing() {
			logger.S(ctx).Info("Shutdown started. Stopping timeout process.")

			break
		}

		if ctx.Err() == context.Canceled {
			logger.S(ctx).Info("Context canceled. Stoping timeout elements removal.")
			break
		}

		result, err := queue.TimeoutMessages(ctx, queueName)

		if err != nil {
			logger.S(ctx).Errorf("Error processing timeouts for queue %s: %v", queueName, err)

			continue
		}

		count := int64(len(result))
		if count > 0 {
			logger.S(ctx).Warnf("Queue %s had %d timeouts.", queueName, count)
		}

		total += count
	}

	if total > 0 {
		logger.S(ctx).Warnf("%d elements got timeout. Processed in %s.", total, time.Since(t))
	}

	return nil
}

// processLockPool moves messages from the lock message pool to the message pool.
func ProcessLockPool(ctx context.Context, queue *Queue) {
	lockAckQueues, err := queue.cache.ListQueues(ctx, "*", pool.LOCK_ACK_POOL)

	if err != nil {
		logger.S(ctx).Error("Error getting lock_ack queue names: ", err)
		return
	}

	unlockMessages(ctx, queue, lockAckQueues, cache.LOCK_ACK)

	if shutdown.Ongoing() {
		logger.S(ctx).Info("Shutdown started. Stopping unlock process.")

		return
	}

	lockNackQueues, err := queue.cache.ListQueues(ctx, "*", pool.LOCK_NACK_POOL)

	if err != nil {
		logger.S(ctx).Error("Error getting lock_nack queue names: ", err)
		return
	}

	unlockMessages(ctx, queue, lockNackQueues, cache.LOCK_NACK)
}

func unlockMessages(ctx context.Context, pool *Queue, queues []string, lockType cache.LockType) {
	for i := range queues {
		if shutdown.Ongoing() {
			logger.S(ctx).Info("Shutdown started. Stopping unlock process.")

			break
		}

		ids, err := pool.cache.UnlockMessages(ctx, queues[i], lockType)

		if err != nil {
			logger.S(ctx).Errorf("Error processing locks for queue '%s': %v", queues[i], err.Error())

			continue
		}

		for index := range ids {
			pool.auditor.Store(ctx, audit.Entry{
				ID:     ids[index],
				Queue:  queues[i],
				Signal: audit.UNLOCK,
				Reason: string(lockType),
			})
		}

		metrics.HousekeeperUnlock.Add(ctx, int64(len(ids)), metric.WithAttributes(attribute.String("queue", message.GetQueuePrefix(queues[i])), attribute.String("lock_type", string(lockType))))
	}
}

func isRecovering(ctx context.Context, pool *Queue) (bool, error) {
	recovery, err := pool.cache.Get(ctx, cache.RECOVERY_RUNNING)
	if err != nil {
		logger.S(ctx).Error("Error to get full recovery status: ", err)

		return false, err
	}

	return recovery == "true", nil
}

// RecoveryMessagesPool recover messages pool sending all storage data to cache
func RecoveryMessagesPool(ctx context.Context, pool *Queue) (metrify bool) {
	t := dtime.Now()

	breakpoint, err := pool.cache.Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY)
	if err != nil {
		logger.S(ctx).Error("Error to get storage breakpoint: ", err)

		return true
	}

	if breakpoint == cache.RECOVERY_FINISHED {
		return false
	}

	isRecovering, err := isRecovering(ctx, pool)
	if err != nil {
		return true
	}

	if breakpoint == "" && !isRecovering {
		if !tryToStartRecovery(ctx, pool) {
			return true
		}
	}

	recoveryBreakpoint, err := pool.cache.Get(ctx, cache.RECOVERY_BREAKPOINT_KEY)
	if err != nil {
		logger.S(ctx).Error("Error to get recovery breakpoint: ", err)

		return true
	}

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("_id", 1)

	messages, err := pool.storage.Find(ctx,
		&storage.FindOptions{
			InternalFilter: &storage.InternalFilter{
				InternalIdBreakpointGt:  breakpoint,
				InternalIdBreakpointLte: recoveryBreakpoint,
			},
			Projection: &map[string]int{
				"id":         1,
				"score":      1,
				"queue":      1,
				"last_score": 1,
				"last_usage": 1,
				"lock_ms":    1,
			},
			Sort:  sort,
			Limit: int64(4000),
		})

	if err != nil {
		logger.S(ctx).Error("Error to get storage elements: ", err)
		return true
	}

	if len(messages) > 0 {
		addMessages := make([]*message.Message, len(messages))
		for i := range messages {
			addMessages[i] = &messages[i]

			// TODO:
			if messages[i].Score < score.Min {
				messages[i].Score = score.Min
			} else if messages[i].Score > score.Max {
				messages[i].Score = score.Max
			}
		}

		_, err := pool.AddMessagesToCacheWithAuditReason(ctx, "recovery", addMessages...)

		if err != nil {
			logger.S(ctx).Error("Error adding element: ", err)
			return true
		}

		_ = pool.cache.Set(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY, pool.storage.GetStringInternalId(ctx, &messages[len(messages)-1]))
	}

	if len(messages) < 4000 {
		logger.S(ctx).Info("Full recovery finished.")

		_ = pool.cache.Set(ctx, cache.RECOVERY_RUNNING, "false")
		_ = pool.cache.Set(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY, cache.RECOVERY_FINISHED)
	}

	logger.S(ctx).Debugf("%d messages updated in %s with %s breakpoint.", len(messages), time.Since(t), breakpoint)

	return true
}

func tryToStartRecovery(ctx context.Context, pool *Queue) bool {
	logger.S(ctx).Info("Starting full cache recovery.")
	err := pool.cache.Set(ctx, cache.RECOVERY_RUNNING, "true")

	if err != nil {
		logger.S(ctx).Error("Error to set full recovery status: ", err)

		return false
	}

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("_id", -1)

	messages, err := pool.storage.Find(ctx,
		&storage.FindOptions{
			Projection: &map[string]int{
				"_id": 1,
			},
			Sort:  sort,
			Limit: int64(1),
		})

	if err != nil {
		logger.S(ctx).Error("Error to get storage last element: ", err)
		return false
	}

	if len(messages) == 0 {
		logger.S(ctx).Info("Storage is empty. Finishing recovery.")

		_ = pool.cache.Set(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY, cache.RECOVERY_FINISHED)
		_ = pool.cache.Set(ctx, cache.RECOVERY_RUNNING, "false")

		return false
	}

	recoveryBreakpointKey := pool.storage.GetStringInternalId(ctx, &messages[0])

	logger.S(ctx).Info("Storage last element key breakpoint: ", recoveryBreakpointKey)

	_ = pool.cache.Set(ctx, cache.RECOVERY_BREAKPOINT_KEY, recoveryBreakpointKey)

	return true
}

// Remove 10000 expired elements ordered by expiration date asc for each queue.
func RemoveTTLMessages(ctx context.Context, pool *Queue, filterDate *time.Time) (bool, error) {
	if isRecovering, err := isRecovering(ctx, pool); isRecovering || err != nil {
		return false, err
	}

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("expiry_date", 1)

	messages, err := pool.storage.Find(ctx, &storage.FindOptions{
		Limit: 10000,
		InternalFilter: &storage.InternalFilter{
			ExpiryDate: filterDate,
		},
		Projection: &map[string]int{
			"id":    1,
			"queue": 1,
			"_id":   0,
		},
		Sort: sort,
	})

	if err != nil {
		logger.S(ctx).Error("Error getting elements from queue: ", err)

		return true, err
	}

	messageMap := make(map[string][]string)
	for i := range messages {
		if messageMap[messages[i].Queue] == nil {
			messageMap[messages[i].Queue] = make([]string, 0)
		}

		messageMap[messages[i].Queue] = append(messageMap[messages[i].Queue], messages[i].ID)
	}

	for queue := range messageMap {
		if shutdown.Ongoing() {
			logger.S(ctx).Info("Shutdown started. Stoping ttl elements removal.")

			break
		}

		ids := messageMap[queue]

		cacheRemoved, storageRemoved, err := pool.Remove(ctx, queue, "TTL", ids...)

		if err != nil {
			logger.S(ctx).Errorf("Error removing %d elements from %s: %v", len(ids), queue, err)

			return true, err
		}

		metrics.HousekeeperTTLCacheRemoved.Add(ctx, cacheRemoved, metric.WithAttributes(attribute.String("queue", message.GetQueuePrefix(queue))))
		metrics.HousekeeperTTLStorageRemoved.Add(ctx, storageRemoved, metric.WithAttributes(attribute.String("queue", message.GetQueuePrefix(queue))))
	}

	return true, nil
}

// Checks if there is any queue with max_elements configuration and
// then remove every exceeding messages using expiry_date to sort which elements will be removed
// TODO manage message pool update individually by queue to avoid future bottlenecks
func RemoveExceedingMessages(ctx context.Context, pool *Queue) (bool, error) {
	if isRecovering, err := isRecovering(ctx, pool); isRecovering || err != nil {
		return false, err
	}

	configurations, err := pool.storage.ListQueueConfigurations(ctx)

	if err != nil {
		logger.S(ctx).Error("Error listing queue names: ", err)

		return true, nil
	}

	for _, configuration := range configurations {
		if shutdown.Ongoing() {
			logger.S(ctx).Info("Context canceled. Stoping exceeding elements removal.")

			break
		}

		_ = pool.removeExceedingMessagesFromQueue(ctx, configuration)
	}

	return true, nil
}

func (pool *Queue) removeExceedingMessagesFromQueue(ctx context.Context, queueConfiguration *configuration.QueueConfiguration) error {
	if queueConfiguration == nil || queueConfiguration.MaxElements <= 0 || queueConfiguration.Queue == "" {
		return nil
	}

	queue := queueConfiguration.Queue

	comment := "housekeeper.removeExceedingMessagesFromQueue_1"
	total, err := pool.storage.Count(ctx, &storage.FindOptions{InternalFilter: &storage.InternalFilter{Queue: queue}, Comment: comment},
		&storage.CountOptions{Comment: comment})

	if err != nil {
		logger.S(ctx).Errorf("Error counting queue %s: %v", queue, err)

		return err
	}

	if total <= queueConfiguration.MaxElements {
		return nil
	}

	diff := total - queueConfiguration.MaxElements

	logger.S(ctx).Debugf("Removing %d elements from queue %s.", diff, queue)

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("expiry_date", 1)

	comment = "housekeeper.removeExceedingMessagesFromQueue_2"
	messages, err := pool.storage.Find(ctx, &storage.FindOptions{
		Limit: diff,
		InternalFilter: &storage.InternalFilter{
			Queue: queue,
		},
		Projection: &map[string]int{
			"id":  1,
			"_id": 0,
		},
		Sort:    sort,
		Comment: comment,
	})

	if err != nil {
		logger.S(ctx).Errorf("Error getting %d elements from queue %s: %v", diff, queue, err)

		return err
	}

	ids := make([]string, len(messages))
	for i := range messages {
		ids[i] = messages[i].ID
	}

	cacheRemoved, storageRemoved, err := pool.Remove(ctx, queue, "MAX_ELEMENTS", ids...)
	if err != nil {
		logger.S(ctx).Errorf("Error removing %d elements from %s: %v", diff, queue, err)

		return err
	}

	metrics.HousekeeperExceedingCacheRemoved.Add(ctx, cacheRemoved, metric.WithAttributes(attribute.String("queue", message.GetQueuePrefix(queue))))
	metrics.HousekeeperExceedingStorageRemoved.Add(ctx, storageRemoved, metric.WithAttributes(attribute.String("queue", message.GetQueuePrefix(queue))))

	return nil
}

func ComputeMetrics(ctx context.Context, pool *Queue) {
	queues, err := pool.storage.ListQueuePrefixes(ctx)

	if err != nil {
		logger.S(ctx).Error("Error getting queue names: ", err)

		return
	}

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("last_usage", 1)

	oldestElement := make(map[string]int64, 0)
	totalElements := make(map[string]int64, 0)
	for _, queue := range queues {
		if shutdown.Ongoing() {
			logger.S(ctx).Info("Context canceled. Stopping metrics computation.")

			break
		}

		message, err := pool.storage.Find(ctx, &storage.FindOptions{
			Projection: &map[string]int{"last_usage": 1, "_id": 0},
			Sort:       sort,
			Limit:      1,
			InternalFilter: &storage.InternalFilter{
				QueuePrefix: queue,
			},
		})

		if err != nil {
			logger.S(ctx).Errorf("Error getting queue %s oldest element: %v", queue, err)

			continue
		}

		if len(message) == 1 && message[0].LastUsage != nil {
			oldestElement[queue] = dtime.ElapsedTime(*message[0].LastUsage)
		}

		total, err := pool.Count(ctx, &storage.FindOptions{
			InternalFilter: &storage.InternalFilter{QueuePrefix: queue},
		})

		if err != nil {
			logger.S(ctx).Errorf("Error counting queue %s elements: %v", queue, err)

			continue
		}

		totalElements[queue] = total
	}

	metrics.MetricsMap.UpdateOldestElementMap(oldestElement)
	metrics.MetricsMap.UpdateTotalElementsMap(totalElements)
}
