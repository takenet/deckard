package messagepool

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/audit"
	"github.com/takenet/deckard/internal/messagepool/cache"
	"github.com/takenet/deckard/internal/messagepool/entities"
	"github.com/takenet/deckard/internal/messagepool/queue"
	"github.com/takenet/deckard/internal/messagepool/storage"
	"github.com/takenet/deckard/internal/metrics"
	"github.com/takenet/deckard/internal/mocks"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestUpdateOldestMessagePoolMap(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx = context.Background()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().ListQueuePrefixes(ctx).Return([]string{"a", "b"}, nil)

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("last_usage", 1)

	now := time.Now()

	mockStorage.EXPECT().Find(ctx, &storage.FindOptions{
		Projection: &map[string]int{"last_usage": 1, "_id": 0},
		Sort:       sort,
		Limit:      1,
		InternalFilter: &storage.InternalFilter{
			QueuePrefix: "a",
		},
	}).Return([]entities.Message{
		{LastUsage: &now},
	}, nil)

	nowMinusTenSeconds := time.Now().Add(-10 * time.Second)
	mockStorage.EXPECT().Find(ctx, &storage.FindOptions{
		Projection: &map[string]int{"last_usage": 1, "_id": 0},
		Sort:       sort,
		Limit:      1,
		InternalFilter: &storage.InternalFilter{
			QueuePrefix: "b",
		},
	}).Return([]entities.Message{
		{LastUsage: &nowMinusTenSeconds},
	}, nil)

	mockStorage.EXPECT().Count(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{QueuePrefix: "a"},
	}).Return(int64(25), nil)

	mockStorage.EXPECT().Count(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{QueuePrefix: "b"},
	}).Return(int64(11), nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)
	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	ComputeMetrics(ctx, q)

	require.Equal(t, 2, len(metrics.MetricsMap.OldestElement))
	require.LessOrEqual(t, metrics.MetricsMap.OldestElement["a"], int64(100))
	require.GreaterOrEqual(t, metrics.MetricsMap.OldestElement["b"], int64(10000))

	require.Equal(t, 2, len(metrics.MetricsMap.TotalElements))
	require.Equal(t, int64(25), metrics.MetricsMap.TotalElements["a"])
	require.Equal(t, int64(11), metrics.MetricsMap.TotalElements["b"])
}

func TestProcessLockPool(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx = context.Background()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().ListQueues(ctx, "*", entities.LOCK_ACK_POOL).Return([]string{"a", "b"}, nil)
	mockCache.EXPECT().ListQueues(ctx, "*", entities.LOCK_NACK_POOL).Return([]string{"c", "d"}, nil)

	mockCache.EXPECT().UnlockMessages(ctx, "a", cache.LOCK_ACK)
	mockCache.EXPECT().UnlockMessages(ctx, "b", cache.LOCK_ACK)
	mockCache.EXPECT().UnlockMessages(ctx, "c", cache.LOCK_NACK)
	mockCache.EXPECT().UnlockMessages(ctx, "d", cache.LOCK_NACK)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	ProcessLockPool(ctx, q)
}

func TestProcessLockPoolAckListErrorShouldDoNothing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx = context.Background()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().ListQueues(ctx, "*", entities.LOCK_ACK_POOL).Return(nil, fmt.Errorf("error"))

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	ProcessLockPool(ctx, q)
}

func TestProcessLockPoolNackAckListErrorShouldDoNothing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx = context.Background()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().ListQueues(ctx, "*", entities.LOCK_ACK_POOL).Return([]string{}, nil)
	mockCache.EXPECT().ListQueues(ctx, "*", entities.LOCK_NACK_POOL).Return(nil, fmt.Errorf("error"))

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	ProcessLockPool(ctx, q)
}

func TestProcessUnlockErrorShouldUnlockOthers(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx = context.Background()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().ListQueues(ctx, "*", entities.LOCK_ACK_POOL).Return([]string{"a", "b"}, nil)
	mockCache.EXPECT().ListQueues(ctx, "*", entities.LOCK_NACK_POOL).Return([]string{"c", "d"}, nil)

	mockCache.EXPECT().UnlockMessages(ctx, "a", cache.LOCK_ACK).Return(nil, fmt.Errorf("error"))
	mockCache.EXPECT().UnlockMessages(ctx, "b", cache.LOCK_ACK)
	mockCache.EXPECT().UnlockMessages(ctx, "c", cache.LOCK_NACK)
	mockCache.EXPECT().UnlockMessages(ctx, "d", cache.LOCK_NACK)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	ProcessLockPool(ctx, q)
}

func TestRecoveryMessagesCacheError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	cacheMessages := []*entities.Message{{
		ID:                "id",
		Queue:             "queue",
		InternalId:        4321,
		ExpiryDate:        time.Time{},
		LastUsage:         &now,
		Score:             entities.GetScore(&now, 54321),
		LastScoreSubtract: 54321,
	}, {
		ID:                "id2",
		InternalId:        65456,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 23457),
		LastUsage:         &now,
		LastScoreSubtract: 23457,
	}}

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY).Return("12345", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_RUNNING).Return("false", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_BREAKPOINT_KEY).Return("65456", nil)
	mockCache.EXPECT().Insert(ctx, "queue", cacheMessages).Return(nil, errors.New("cache error"))

	storageMessages := []entities.Message{{
		ID:                "id",
		InternalId:        4321,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 54321),
		LastUsage:         &now,
		LastScoreSubtract: 54321,
	}, {
		ID:                "id2",
		InternalId:        65456,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 23457),
		LastUsage:         &now,
		LastScoreSubtract: 23457,
	}}

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("_id", 1)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			InternalIdBreakpointGt:  "12345",
			InternalIdBreakpointLte: "65456",
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
	}).Return(storageMessages, nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RecoveryMessagesPool(ctx, q)
}

func TestCheckTimeoutMessagesListQueueErrorShouldDoNothing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().ListQueues(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error list queues"))

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	ProcessTimeoutMessages(ctx, q)
}

func TestCheckTimeoutMessagesErrorForQueueShouldContinueOtherQueues(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().ListQueues(gomock.Any(), gomock.Any(), gomock.Any()).Return([]string{"q1", "q2"}, nil)
	mockCache.EXPECT().TimeoutMessages(gomock.Any(), "q1", cache.DefaultCacheTimeout).Return(nil, errors.New("error timeout messages"))
	mockCache.EXPECT().TimeoutMessages(gomock.Any(), "q2", cache.DefaultCacheTimeout).Return([]string{"id2"}, nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	ProcessTimeoutMessages(ctx, q)
}

func TestRecoveryMessagesPoolShouldAddMessagesAfterBreakpoint(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	cacheMessages := []*entities.Message{{
		ID:                "id",
		Queue:             "queue",
		InternalId:        45456,
		ExpiryDate:        time.Time{},
		LastUsage:         &now,
		Score:             entities.GetScore(&now, 54321),
		LastScoreSubtract: 54321,
	}, {
		ID:                "id2",
		InternalId:        65456,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 23457),
		LastUsage:         &now,
		LastScoreSubtract: 23457,
	}}

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_RUNNING).Return("true", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY).Return("12345", nil)
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY, "65456")
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY, cache.RECOVERY_FINISHED)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_BREAKPOINT_KEY).Return("65456", nil)
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_RUNNING, "false")
	mockCache.EXPECT().Insert(ctx, "queue", cacheMessages).Return([]string{"id", "id2"}, nil)

	storageMessages := []entities.Message{{
		ID:                "id",
		InternalId:        45456,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 54321),
		LastUsage:         &now,
		LastScoreSubtract: 54321,
	}, {
		ID:                "id2",
		InternalId:        65456,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 23457),
		LastUsage:         &now,
		LastScoreSubtract: 23457,
	}}

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("_id", 1)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			InternalIdBreakpointGt:  "12345",
			InternalIdBreakpointLte: "65456",
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
	}).Return(storageMessages, nil)
	mockStorage.EXPECT().GetStringInternalId(ctx, &entities.Message{
		ID:                "id2",
		InternalId:        65456,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 23457),
		LastUsage:         &now,
		LastScoreSubtract: 23457,
	}).Return("65456")

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RecoveryMessagesPool(ctx, q)
}

func TestRecoveryMessagesPoolInitWithEmptyStorageShouldNotStartRecovery(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_RUNNING).Return("false", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY).Return("", nil)
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_RUNNING, "true")

	mockCache.EXPECT().Set(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY, cache.RECOVERY_FINISHED)
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_RUNNING, "false")

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("_id", -1)
	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(ctx, &storage.FindOptions{
		Projection: &map[string]int{
			"_id": 1,
		},
		Sort:  sort,
		Limit: int64(1),
	}).Return([]entities.Message{}, nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RecoveryMessagesPool(ctx, q)
}

func TestRecoveryMessagesPoolInitNonEmptyStorageShouldStartRecovery(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	storageNotLast := primitive.NewObjectID()
	storageLast := primitive.NewObjectID()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_RUNNING).Return("false", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY).Return("", nil)
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_RUNNING, "true")
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_BREAKPOINT_KEY, storageLast.Hex())
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY, storageNotLast.Hex())
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_BREAKPOINT_KEY).Return(storageLast.Hex(), nil)

	mockStorage := mocks.NewMockStorage(mockCtrl)

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("_id", -1)
	mockStorage.EXPECT().Find(ctx, &storage.FindOptions{
		Projection: &map[string]int{
			"_id": 1,
		},
		Sort:  sort,
		Limit: int64(1),
	}).Return([]entities.Message{
		{InternalId: storageLast},
	}, nil)

	mockStorage.EXPECT().GetStringInternalId(ctx, &entities.Message{
		InternalId: storageLast,
	}).Return(storageLast.Hex())

	storageMessages := make([]entities.Message, 4000)
	cacheMessages := make([]*entities.Message, 4000)
	insertedIds := make([]string, 4000)
	for i := 0; i < len(storageMessages); i++ {
		storageMessages[i] = entities.Message{
			Queue:      "queue",
			ID:         strconv.Itoa(i),
			InternalId: primitive.NewObjectID(),
		}

		cacheMessages[i] = &storageMessages[i]
		insertedIds[i] = storageMessages[i].ID
	}
	storageMessages[len(storageMessages)-1].InternalId = storageNotLast

	mockCache.EXPECT().Insert(ctx, "queue", cacheMessages).Return(insertedIds, nil)

	sort = orderedmap.NewOrderedMap[string, int]()
	sort.Set("_id", 1)
	mockStorage.EXPECT().Find(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			InternalIdBreakpointGt:  "",
			InternalIdBreakpointLte: storageLast.Hex(),
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
	}).Return(storageMessages, nil)
	mockStorage.EXPECT().GetStringInternalId(ctx, &storageMessages[len(storageMessages)-1]).Return(storageNotLast.Hex())

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RecoveryMessagesPool(ctx, q)
}

func TestRecoveryMessagesPoolAlreadyRunning(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	cacheMessages := []*entities.Message{{
		ID:                "id",
		Queue:             "queue",
		InternalId:        4321,
		ExpiryDate:        time.Time{},
		LastUsage:         &now,
		Score:             entities.GetScore(&now, 54321),
		LastScoreSubtract: 54321,
	}, {
		ID:                "id2",
		InternalId:        65456,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 23457),
		LastUsage:         &now,
		LastScoreSubtract: 23457,
	}}

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_BREAKPOINT_KEY).Return("65456", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY).Return("", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_RUNNING).Return("true", nil)
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY, "65456")

	mockCache.EXPECT().Set(ctx, cache.RECOVERY_RUNNING, "false")
	mockCache.EXPECT().Set(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY, cache.RECOVERY_FINISHED)

	mockCache.EXPECT().Insert(ctx, "queue", cacheMessages).Return([]string{"id", "id2"}, nil)

	storageMessages := []entities.Message{{
		ID:                "id",
		InternalId:        4321,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 54321),
		LastUsage:         &now,
		LastScoreSubtract: 54321,
	}, {
		ID:                "id2",
		InternalId:        65456,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 23457),
		LastUsage:         &now,
		LastScoreSubtract: 23457,
	}}

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("_id", 1)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			InternalIdBreakpointGt:  "",
			InternalIdBreakpointLte: "65456",
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
	}).Return(storageMessages, nil)
	mockStorage.EXPECT().GetStringInternalId(ctx, &entities.Message{
		ID:                "id2",
		InternalId:        65456,
		Queue:             "queue",
		ExpiryDate:        time.Time{},
		Score:             entities.GetScore(&now, 23457),
		LastUsage:         &now,
		LastScoreSubtract: 23457,
	}).Return("65456")

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RecoveryMessagesPool(ctx, q)
}

func TestRecoveryMessagesPoolStorageBreakpointError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY).Return("", errors.New("storage breakpoint error"))
	mockStorage := mocks.NewMockStorage(mockCtrl)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RecoveryMessagesPool(ctx, q)
}

func TestRecoveryMessagesPoolRecoveryRunningError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY).Return("1234", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_RUNNING).Return("", errors.New("recovery running error"))
	mockStorage := mocks.NewMockStorage(mockCtrl)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RecoveryMessagesPool(ctx, q)
}

func TestRecoveryMessagesPoolRecoveryBreakpointRunningError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY).Return("1234", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_RUNNING).Return("true", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_BREAKPOINT_KEY).Return("", errors.New("recovery running error"))
	mockStorage := mocks.NewMockStorage(mockCtrl)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RecoveryMessagesPool(ctx, q)
}

func TestRecoveryMessagesPoolStorageError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("_id", 1)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_STORAGE_BREAKPOINT_KEY).Return("12345", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_BREAKPOINT_KEY).Return("654987", nil)
	mockCache.EXPECT().Get(ctx, cache.RECOVERY_RUNNING).Return("true", nil)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			InternalIdBreakpointGt:  "12345",
			InternalIdBreakpointLte: "654987",
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
	}).Return(nil, errors.New("storage errors"))

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RecoveryMessagesPool(ctx, q)
}

func TestRemoveTTLMessagesShouldRemoveExpiredElements(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("expiry_date", 1)

	now := time.Now()

	var expiryDate time.Time
	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Limit: 10000,
		InternalFilter: &storage.InternalFilter{
			ExpiryDate: &now,
		},
		Projection: &map[string]int{
			"id":    1,
			"queue": 1,
			"_id":   0,
		},
		Sort: sort,
	}).DoAndReturn(
		func(ctx context.Context, opt *storage.FindOptions) ([]entities.Message, error) {
			expiryDate = *opt.InternalFilter.ExpiryDate

			return []entities.Message{{ID: "1", Queue: "q1"}, {ID: "2", Queue: "q2"}}, nil
		},
	)

	mockStorage.EXPECT().Remove(gomock.Any(), "q1", []string{"1"}).Return(int64(1), nil)
	mockStorage.EXPECT().Remove(gomock.Any(), "q2", []string{"2"}).Return(int64(1), nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Remove(gomock.Any(), "q1", []string{"1"}).Return(int64(1), nil)
	mockCache.EXPECT().Remove(gomock.Any(), "q2", []string{"2"}).Return(int64(1), nil)
	mockCache.EXPECT().Get(gomock.Any(), cache.RECOVERY_RUNNING).Return("", nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	result, err := RemoveTTLMessages(ctx, q, &now)
	require.NoError(t, err)
	require.True(t, result)

	require.True(t, expiryDate.After(now) || expiryDate.Equal(now))
	require.True(t, expiryDate.Before(now.Add(1*time.Second)))
}

func TestCheckTimeoutMessagesShouldExecuteTimeoutToAllQueues(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().ListQueues(gomock.Any(), gomock.Any(), gomock.Any()).Return([]string{"q1", "q2"}, nil)
	mockCache.EXPECT().TimeoutMessages(gomock.Any(), "q1", cache.DefaultCacheTimeout).Return([]string{"id1"}, nil)
	mockCache.EXPECT().TimeoutMessages(gomock.Any(), "q2", cache.DefaultCacheTimeout).Return([]string{"id2"}, nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	ProcessTimeoutMessages(ctx, q)
}

func TestRemoveExceedingMessagesNoQueuesShouldDoNothing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().ListQueueConfigurations(gomock.Any()).Return([]*entities.QueueConfiguration{}, nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(gomock.Any(), cache.RECOVERY_RUNNING).Return("", nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RemoveExceedingMessages(ctx, q)
}

func TestRemoveExceedingMessagesListErrorShouldDoNothing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().ListQueueConfigurations(gomock.Any()).Return(nil, fmt.Errorf("anyerror"))

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(gomock.Any(), cache.RECOVERY_RUNNING).Return("", nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, nil, mockCache)

	RemoveExceedingMessages(ctx, q)
}

func TestRemoveExceedingMessagesNoQueuesShouldCallRemoveMethodToEachQueue(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().ListQueueConfigurations(gomock.Any()).Return([]*entities.QueueConfiguration{{Queue: "q1"}, {Queue: "q2"}}, nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(gomock.Any(), cache.RECOVERY_RUNNING).Return("", nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	RemoveExceedingMessages(ctx, q)
}

func TestRemoveExceedingMessagesRecoveryRunning(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(gomock.Any(), cache.RECOVERY_RUNNING).Return("true", nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	RemoveExceedingMessages(ctx, q)
}

func TestRemoveTTLMessagesRecoveryRunning(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Get(gomock.Any(), cache.RECOVERY_RUNNING).Return("true", nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	RemoveTTLMessages(ctx, q, &time.Time{})
}
