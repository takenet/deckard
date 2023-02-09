package messagepool

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/takenet/deckard/internal/mocks"
)

var ctx = context.Background()

func TestPull(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("score", 1)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"123"},
			Queue: "test",
		},
		Limit: int64(1),
	}).Return([]entities.Message{
		{
			ID:    "123",
			Queue: "test",
		},
	}, nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().PullMessages(gomock.Any(), "test", int64(1), int64(0)).Return([]string{"123"}, nil)

	q := NewMessagePool(nil, mockStorage, nil, mockCache)

	messages, err := q.Pull(ctx, "test", 1, 0)

	require.NoError(t, err)
	require.Len(t, *messages, 1, "expected one message")
	require.Equal(t, (*messages)[0], entities.Message{ID: "123", Queue: "test"})
}

func TestAckStorageErrorShouldResultError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Ack(gomock.Any(), &entities.Message{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
		Score:     entities.GetScore(&now, 0),
	}).Return(int64(0), errors.New("ack_error"))
	mockCache := mocks.NewMockCache(mockCtrl)

	q := NewMessagePool(nil, mockStorage, nil, mockCache)

	result, err := q.Ack(ctx, &entities.Message{
		ID:    "id",
		Queue: "queue",
	}, now, "")

	require.Error(t, err)
	require.False(t, result)
}

func TestAckMakeAvailableErrorShouldResultError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	message := &entities.Message{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
		Score:     entities.GetScore(&now, 0),
	}
	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Ack(gomock.Any(), message).Return(int64(1), nil)
	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().MakeAvailable(gomock.Any(), message).Return(false, errors.New("make available error"))

	q := NewMessagePool(nil, mockStorage, nil, mockCache)

	result, err := q.Ack(ctx, &entities.Message{
		ID:    "id",
		Queue: "queue",
	}, now, "")

	require.Error(t, err)
	require.False(t, result)
}

func TestAckSuccessfulShouldAudit(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	message := &entities.Message{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
		Score:     entities.GetScore(&now, 0),
	}
	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Ack(gomock.Any(), message).Return(int64(1), nil)
	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().MakeAvailable(gomock.Any(), message).Return(true, nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:                message.ID,
		Queue:             message.Queue,
		LastScoreSubtract: message.LastScoreSubtract,
		Breakpoint:        message.Breakpoint,
		Signal:            audit.ACK,
		Reason:            "reason",
	})

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	result, err := q.Ack(ctx, &entities.Message{
		ID:    "id",
		Queue: "queue",
	}, now, "reason")

	require.NoError(t, err)
	require.True(t, result)
}

func TestAckNilMessage(t *testing.T) {
	q := NewMessagePool(nil, nil, nil, nil)

	result, err := q.Ack(ctx, nil, time.Time{}, "")

	require.NoError(t, err)
	require.False(t, result)
}

func TestAckWithoutQueue(t *testing.T) {
	q := NewMessagePool(nil, nil, nil, nil)

	result, err := q.Ack(ctx, &entities.Message{ID: "1"}, time.Time{}, "")

	require.Error(t, err)
	require.False(t, result)
}

func TestAckWithoutId(t *testing.T) {
	q := NewMessagePool(nil, nil, nil, nil)

	result, err := q.Ack(ctx, &entities.Message{Queue: "queue"}, time.Time{}, "")

	require.Error(t, err)
	require.False(t, result)
}

func TestNackMakeAvailableErrorShouldResultError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	message := &entities.Message{
		ID:    "id",
		Queue: "queue",
		Score: entities.MaxScore(),
	}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().MakeAvailable(gomock.Any(), message).Return(false, errors.New("make available error"))

	q := NewMessagePool(nil, mockStorage, nil, mockCache)

	result, err := q.Nack(ctx, &entities.Message{
		ID:    "id",
		Queue: "queue",
	}, now, "")

	require.Error(t, err)
	require.False(t, result)
}

func TestNackSuccessfulShouldAudit(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	expectCall := &entities.Message{
		ID:    "id",
		Queue: "queue",
		Score: entities.MaxScore(),
	}
	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().MakeAvailable(gomock.Any(), expectCall).Return(true, nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:                expectCall.ID,
		Queue:             expectCall.Queue,
		LastScoreSubtract: expectCall.LastScoreSubtract,
		Breakpoint:        expectCall.Breakpoint,
		Signal:            audit.NACK,
		Reason:            "reason",
	})

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	result, err := q.Nack(ctx, &entities.Message{
		ID:    "id",
		Queue: "queue",
	}, now, "reason")

	require.NoError(t, err)
	require.True(t, result)
}

func TestNackNilMessage(t *testing.T) {
	q := NewMessagePool(nil, nil, nil, nil)

	result, err := q.Nack(ctx, nil, time.Now(), "")

	require.NoError(t, err)
	require.False(t, result)
}

func TestNackWithoutQueue(t *testing.T) {
	q := NewMessagePool(nil, nil, nil, nil)

	result, err := q.Nack(ctx, &entities.Message{ID: "1"}, time.Now(), "")

	require.Error(t, err)
	require.False(t, result)
}

func TestNackWithoutId(t *testing.T) {
	q := NewMessagePool(nil, nil, nil, nil)

	result, err := q.Nack(ctx, &entities.Message{Queue: "queue"}, time.Now(), "")

	require.Error(t, err)
	require.False(t, result)
}

func TestPullShouldDeleteNotFoundInStorageAndReturnRemaining(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("score", 1)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"1", "2", "3"},
			Queue: "test",
		},
		Limit: int64(3),
	}).Return([]entities.Message{
		{
			ID:    "1",
			Queue: "test",
		},
	}, nil)
	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"2", "3"},
			Queue: "test",
		},
		Limit: int64(2),
		Retry: true,
	}).Return([]entities.Message{}, nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().PullMessages(gomock.Any(), "test", int64(3), int64(0)).Return([]string{"1", "2", "3"}, nil)
	mockCache.EXPECT().Remove(gomock.Any(), "test", "2", "3").Return(int64(2), nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:     "2",
		Queue:  "test",
		Signal: audit.MISSING_STORAGE,
	})
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:     "3",
		Queue:  "test",
		Signal: audit.MISSING_STORAGE,
	})
	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	messages, err := q.Pull(ctx, "test", 3, 0)

	require.NoError(t, err)
	require.Len(t, *messages, 1, "expected one message")
	require.Equal(t, (*messages)[0], entities.Message{ID: "1", Queue: "test"})
}

func TestPullElementsFromRetryShouldNotAuditMissingElements(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("score", 1)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"1", "2", "3"},
			Queue: "test",
		},
		Limit: int64(3),
	}).Return(nil, nil)

	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"1", "2", "3"},
			Queue: "test",
		},
		Limit: int64(3),
		Retry: true,
	}).Return([]entities.Message{
		{
			ID:    "1",
			Queue: "test",
		}, {
			ID:    "2",
			Queue: "test",
		}, {
			ID:    "3",
			Queue: "test",
		},
	}, nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().PullMessages(gomock.Any(), "test", int64(3), int64(0)).Return([]string{"1", "2", "3"}, nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	messages, err := q.Pull(ctx, "test", 3, 0)

	require.NoError(t, err)
	require.Len(t, *messages, 3)
}

func TestPullElementsFromBothFirstTryAndRetryShouldMergeElementsAndKeepScoreOrder(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("score", 1)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"1", "2", "3"},
			Queue: "test",
		},
		Limit: int64(3),
	}).Return([]entities.Message{
		{
			ID:    "2",
			Queue: "test",
			Score: 2,
		},
	}, nil)

	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"1", "3"},
			Queue: "test",
		},
		Limit: int64(2),
		Retry: true,
	}).Return([]entities.Message{
		{
			ID:    "1",
			Queue: "test",
			Score: 1,
		}, {
			ID:    "3",
			Queue: "test",
			Score: 3,
		},
	}, nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().PullMessages(gomock.Any(), "test", int64(3), int64(0)).Return([]string{"1", "2", "3"}, nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	messages, err := q.Pull(ctx, "test", 3, 0)

	require.NoError(t, err)
	require.Len(t, *messages, 3)

	require.Equal(t, "1", (*messages)[0].ID)
	require.Equal(t, "2", (*messages)[1].ID)
	require.Equal(t, "3", (*messages)[2].ID)
}

func TestPullNothingFoundOnStorage(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("score", 1)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"1", "2", "3"},
			Queue: "test",
		},
		Limit: int64(3),
	}).Return(nil, nil)

	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Sort: sort,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"1", "2", "3"},
			Queue: "test",
		},
		Limit: int64(3),
		Retry: true,
	}).Return(nil, nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().PullMessages(gomock.Any(), "test", int64(3), int64(0)).Return([]string{"1", "2", "3"}, nil)
	mockCache.EXPECT().Remove(gomock.Any(), "test", "1", "2", "3").Return(int64(3), nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:     "1",
		Queue:  "test",
		Signal: audit.MISSING_STORAGE,
	})
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:     "2",
		Queue:  "test",
		Signal: audit.MISSING_STORAGE,
	})
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:     "3",
		Queue:  "test",
		Signal: audit.MISSING_STORAGE,
	})
	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	messages, err := q.Pull(ctx, "test", 3, 0)

	require.NoError(t, err)
	require.Nil(t, messages)
}

func TestPullCacheError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().PullMessages(gomock.Any(), "test", int64(1), int64(0)).Return(nil, errors.New("cache_error"))

	q := NewMessagePool(nil, nil, nil, mockCache)

	messages, err := q.Pull(ctx, "test", 1, 0)

	require.Error(t, err)
	require.Nil(t, messages)
}

func TestPullCacheNoResults(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().PullMessages(gomock.Any(), "test", int64(1), int64(0)).Return(nil, nil)

	q := NewMessagePool(nil, nil, nil, mockCache)

	messages, err := q.Pull(ctx, "test", 1, 0)

	require.NoError(t, err)
	require.Nil(t, messages)
}

type isSameEntry struct {
	value audit.Entry
}

func (m isSameEntry) Matches(arg interface{}) bool {
	entry := arg.(audit.Entry)

	if entry.Queue != m.value.Queue {
		return false
	}

	if entry.Signal != m.value.Signal {
		return false
	}

	if entry.ID != m.value.ID {
		return false
	}

	return true
}

func (m isSameEntry) String() string {
	return fmt.Sprintf("%v", m.value)
}

func TestMessagePoolTimeoutError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().TimeoutMessages(gomock.Any(), "queue_test", cache.DefaultCacheTimeout).Return(nil, errors.New("test_error"))

	q := NewMessagePool(nil, nil, nil, mockCache)

	_, err := q.TimeoutMessages(ctx, "queue_test")

	require.Error(t, err)
}

func TestMessagePoolRemoveShouldRemoveFromCacheAndStorage(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Remove(gomock.Any(), "queue_test", "1", "2").Return(int64(1), nil)
	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Remove(gomock.Any(), "queue_test", "1", "2").Return(int64(2), nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:     "2",
		Queue:  "queue_test",
		Signal: audit.REMOVE,
		Reason: "",
	})
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:     "1",
		Queue:  "queue_test",
		Signal: audit.REMOVE,
		Reason: "",
	})

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	cacheRemoved, storageRemoved, err := q.Remove(ctx, "queue_test", "", "1", "2")
	require.NoError(t, err)
	require.Equal(t, int64(1), cacheRemoved)
	require.Equal(t, int64(2), storageRemoved)
}

func TestMessagePoolRemoveCacheError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Remove(gomock.Any(), "queue_test", "1").Return(int64(0), errors.New("cache_error"))

	q := NewMessagePool(nil, nil, nil, mockCache)

	cacheRemoved, storageRemoved, err := q.Remove(ctx, "queue_test", "", "1")
	require.Error(t, err)
	require.Equal(t, int64(0), cacheRemoved)
	require.Equal(t, int64(0), storageRemoved)
}

func TestMessagePoolRemoveStorageError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Remove(gomock.Any(), "queue_test", "1").Return(int64(1), nil)

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Remove(gomock.Any(), "queue_test", "1").Return(int64(0), errors.New("storage_error"))

	q := NewMessagePool(nil, mockStorage, nil, mockCache)

	cacheRemoved, storageRemoved, err := q.Remove(ctx, "queue_test", "", "1")
	require.Error(t, err)
	require.Equal(t, int64(1), cacheRemoved)
	require.Equal(t, int64(0), storageRemoved)
}

func TestMessagePoolTimeout(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().TimeoutMessages(gomock.Any(), "queue_test", cache.DefaultCacheTimeout).Return([]string{"1", "2"}, nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	mockAuditor.EXPECT().Store(gomock.Any(), isSameEntry{audit.Entry{
		ID:     "1",
		Signal: audit.TIMEOUT,
		Queue:  "queue_test",
	}}).Times(1)
	mockAuditor.EXPECT().Store(gomock.Any(), isSameEntry{audit.Entry{
		ID:     "2",
		Signal: audit.TIMEOUT,
		Queue:  "queue_test",
	}}).Times(1)

	q := NewMessagePool(mockAuditor, nil, nil, mockCache)

	ids, err := q.TimeoutMessages(ctx, "queue_test")

	require.NoError(t, err)
	require.Equal(t, []string{"1", "2"}, ids)
}

func TestAddMessagesToCacheSameIdInSameRequestShouldSetLastElementScore(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Insert(gomock.Any(), "queue", []*entities.Message{
		{
			ID:        "id",
			Queue:     "queue",
			LastUsage: &now,
			Score:     entities.GetScore(&now, 0),
		}, {
			// No last usage
			ID:    "id",
			Queue: "queue",
			Score: entities.MaxScore(),
		},
	}).Return([]string{"id", "id"}, nil)

	mockCache.EXPECT().Insert(gomock.Any(), "queue2", []*entities.Message{
		{
			ID:        "id",
			Queue:     "queue2",
			LastUsage: &now,
			Score:     entities.GetScore(&now, 0),
		},
	}).Return([]string{"id"}, nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:     "id",
		Queue:  "queue2",
		Signal: audit.INSERT_CACHE,
	})
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:     "id",
		Queue:  "queue",
		Signal: audit.INSERT_CACHE,
	}).MinTimes(2)

	q := NewMessagePool(mockAuditor, nil, nil, mockCache)

	messages := []*entities.Message{{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
		Score:     entities.GetScore(&now, 0),
	}, {
		// No last usage
		ID:    "id",
		Queue: "queue",
		Score: entities.MaxScore(),
	}, {
		// Different queue with score
		ID:        "id",
		Queue:     "queue2",
		LastUsage: &now,
		Score:     entities.GetScore(&now, 0),
	}}
	count, err := q.AddMessagesToCache(ctx, messages...)

	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestAddMessagesToStorageWithoutEditingQueueConfiguration(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	messages := []*entities.Message{{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
	}, {
		// No last usage
		ID:    "id",
		Queue: "queue",
	}, {
		// Different queue with score (should be ignored)
		ID:        "id",
		Queue:     "queue2",
		LastUsage: &now,
		Score:     1234,
	}}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Insert(gomock.Any(), messages).Return(int64(2), int64(0), nil)

	q := NewMessagePool(nil, mockStorage, nil, nil)

	inserted, updated, err := q.AddMessagesToStorage(ctx, messages...)

	require.NoError(t, err)
	require.Equal(t, int64(2), inserted)
	require.Equal(t, int64(0), updated)
}

func TestAddMessagesError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Insert(gomock.Any(), "queue", []*entities.Message{
		{
			ID:        "id",
			Queue:     "queue",
			LastUsage: &now,
			Score:     entities.GetScore(&now, 0),
		},
	}).Return(nil, errors.New("insert error"))

	q := NewMessagePool(nil, nil, nil, mockCache)

	messages := []*entities.Message{{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
		Score:     entities.GetScore(&now, 0),
	}}
	_, err := q.AddMessagesToCache(ctx, messages...)

	require.Error(t, err)
}

func TestRemoveExceedingMessagesQueueZeroMaxElementsShouldDoNothing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockCache := mocks.NewMockCache(mockCtrl)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	require.NoError(t, q.removeExceedingMessagesFromQueue(ctx, &entities.QueueConfiguration{MaxElements: 0, Queue: "q1"}))
}

func TestRemoveExceedingMessagesEmptyQueueShouldDoNothing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	configuration := &entities.QueueConfiguration{MaxElements: 2, Queue: "q1"}
	mockStorage.EXPECT().Count(gomock.Any(), &storage.FindOptions{InternalFilter: &storage.InternalFilter{Queue: "q1"}}).Return(int64(0), nil)

	mockCache := mocks.NewMockCache(mockCtrl)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	require.NoError(t, q.removeExceedingMessagesFromQueue(ctx, configuration))
}

func TestRemoveExceedingMessagesErrorCountingShouldReturnError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	configuration := &entities.QueueConfiguration{MaxElements: 2, Queue: "q1"}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Count(gomock.Any(), &storage.FindOptions{InternalFilter: &storage.InternalFilter{Queue: "q1"}}).Return(int64(0), fmt.Errorf("anyerr"))

	mockCache := mocks.NewMockCache(mockCtrl)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	require.Error(t, q.removeExceedingMessagesFromQueue(ctx, configuration))
}

func TestRemoveExceedingMessagesShouldRemoveExceedingElements(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	maxElements := int64(2)
	count := int64(5)

	configuration := &entities.QueueConfiguration{MaxElements: maxElements, Queue: "q1"}

	mockStorage.EXPECT().Count(gomock.Any(), &storage.FindOptions{InternalFilter: &storage.InternalFilter{Queue: "q1"}}).Return(count, nil)

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("expiry_date", 1)

	mockStorage.EXPECT().Find(gomock.Any(), &storage.FindOptions{
		Limit: count - maxElements,
		InternalFilter: &storage.InternalFilter{
			Queue: "q1",
		},
		Projection: &map[string]int{
			"id":  1,
			"_id": 0,
		},
		Sort: sort,
	}).Return([]entities.Message{{ID: "1"}, {ID: "2"}}, nil)

	mockStorage.EXPECT().Remove(gomock.Any(), "q1", []string{"1", "2"}).Return(int64(2), nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Remove(gomock.Any(), "q1", []string{"1", "2"}).Return(int64(2), nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	require.NoError(t, q.removeExceedingMessagesFromQueue(ctx, configuration))
}

func TestRemoveExceedingMessagesFindErrorShouldRemoveResultError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	maxElements := int64(2)
	count := int64(5)

	configuration := &entities.QueueConfiguration{MaxElements: maxElements, Queue: "q1"}

	mockStorage.EXPECT().Count(gomock.Any(), &storage.FindOptions{InternalFilter: &storage.InternalFilter{Queue: "q1"}}).Return(count, nil)

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("expiry_date", 1)

	mockStorage.EXPECT().Find(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("anyerror"))

	mockCache := mocks.NewMockCache(mockCtrl)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	require.Error(t, q.removeExceedingMessagesFromQueue(ctx, configuration))
}

func TestRemoveExceedingMessagesRemoveErrorShouldResultError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	maxElements := int64(2)
	count := int64(5)

	configuration := &entities.QueueConfiguration{MaxElements: maxElements, Queue: "q1"}

	mockStorage.EXPECT().Count(gomock.Any(), &storage.FindOptions{InternalFilter: &storage.InternalFilter{Queue: "q1"}}).Return(count, nil)

	sort := orderedmap.NewOrderedMap[string, int]()
	sort.Set("expiry_date", 1)

	mockStorage.EXPECT().Find(gomock.Any(), gomock.Any()).Return([]entities.Message{{ID: "1"}, {ID: "2"}}, nil)
	mockStorage.EXPECT().Remove(gomock.Any(), "q1", []string{"1", "2"}).Return(int64(0), fmt.Errorf("anyerror"))

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().Remove(gomock.Any(), "q1", []string{"1", "2"}).Return(int64(2), nil)

	q := NewMessagePool(&audit.AuditorImpl{}, mockStorage, queue.NewConfigurationService(ctx, mockStorage), mockCache)

	require.Error(t, q.removeExceedingMessagesFromQueue(ctx, configuration))
}

func TestCountShouldCallStorage(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	opts := &storage.FindOptions{}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Count(ctx, opts).Return(int64(12), nil)
	mockCache := mocks.NewMockCache(mockCtrl)

	q := NewMessagePool(nil, mockStorage, nil, mockCache)

	result, err := q.Count(ctx, opts)

	require.NoError(t, err)
	require.Equal(t, int64(12), result)
}

func TestNilOptsShouldCreateEmptyOpts(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	opts := &storage.FindOptions{}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Count(ctx, opts).Return(int64(12), nil)
	mockCache := mocks.NewMockCache(mockCtrl)

	q := NewMessagePool(nil, mockStorage, nil, mockCache)

	result, err := q.Count(ctx, nil)

	require.NoError(t, err)
	require.Equal(t, int64(12), result)
}

func TestCountStorageErrorShouldResultError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	opts := &storage.FindOptions{}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Count(ctx, opts).Return(int64(0), fmt.Errorf("error"))
	mockCache := mocks.NewMockCache(mockCtrl)

	q := NewMessagePool(nil, mockStorage, nil, mockCache)

	result, err := q.Count(ctx, nil)

	require.Error(t, err)
	require.Equal(t, int64(0), result)
}

func TestAckWithLockShouldLock(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	message := &entities.Message{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
		Score:     entities.GetScore(&now, 0),
		LockMs:    10,
	}
	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Ack(gomock.Any(), message).Return(int64(1), nil)
	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().LockMessage(gomock.Any(), message, cache.LOCK_ACK).Return(true, nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:                message.ID,
		Queue:             message.Queue,
		LastScoreSubtract: message.LastScoreSubtract,
		Breakpoint:        message.Breakpoint,
		Signal:            audit.ACK,
		Reason:            "reason",
		LockMs:            10,
	})

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	result, err := q.Ack(ctx, &entities.Message{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
		LockMs:    10,
	}, now, "reason")

	require.NoError(t, err)
	require.True(t, result)
}

func TestAckWithLockErrorShouldResultError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	message := &entities.Message{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
		Score:     entities.GetScore(&now, 0),
		LockMs:    10,
	}
	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().Ack(gomock.Any(), message).Return(int64(1), nil)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().LockMessage(gomock.Any(), message, cache.LOCK_ACK).Return(false, fmt.Errorf("error"))

	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	result, err := q.Ack(ctx, &entities.Message{
		ID:        "id",
		Queue:     "queue",
		LastUsage: &now,
		LockMs:    10,
	}, now, "reason")

	require.Error(t, err)
	require.False(t, result)
}

func TestNackWithLockShouldLock(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	message := &entities.Message{
		ID:     "id",
		Queue:  "queue",
		LockMs: 10,
	}

	mockStorage := mocks.NewMockStorage(mockCtrl)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().LockMessage(gomock.Any(), message, cache.LOCK_NACK).Return(true, nil)

	mockAuditor := mocks.NewMockAuditor(mockCtrl)
	mockAuditor.EXPECT().Store(gomock.Any(), audit.Entry{
		ID:                message.ID,
		Queue:             message.Queue,
		LastScoreSubtract: message.LastScoreSubtract,
		Breakpoint:        message.Breakpoint,
		Signal:            audit.NACK,
		Reason:            "reason",
		LockMs:            10,
	})

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	result, err := q.Nack(ctx, &entities.Message{
		ID:     "id",
		Queue:  "queue",
		LockMs: 10,
	}, now, "reason")

	require.NoError(t, err)
	require.True(t, result)
}

func TestNackWithLockErrorShouldResultError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	now := time.Now()

	message := &entities.Message{
		ID:     "id",
		Queue:  "queue",
		LockMs: 10,
	}

	mockStorage := mocks.NewMockStorage(mockCtrl)

	mockCache := mocks.NewMockCache(mockCtrl)
	mockCache.EXPECT().LockMessage(gomock.Any(), message, cache.LOCK_NACK).Return(false, fmt.Errorf("error"))

	mockAuditor := mocks.NewMockAuditor(mockCtrl)

	q := NewMessagePool(mockAuditor, mockStorage, nil, mockCache)

	result, err := q.Nack(ctx, &entities.Message{
		ID:     "id",
		Queue:  "queue",
		LockMs: 10,
	}, now, "reason")

	require.Error(t, err)
	require.False(t, result)
}
