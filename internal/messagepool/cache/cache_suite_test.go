package cache

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/messagepool/entities"
	"github.com/takenet/deckard/internal/messagepool/utils"
)

var ctx, cancel = context.WithCancel(context.Background())

type CacheIntegrationTestSuite struct {
	suite.Suite
	cache Cache
}

func (suite *CacheIntegrationTestSuite) AfterTest(_, _ string) {
	ctx, cancel = context.WithCancel(context.Background())

	suite.cache.Flush(ctx)
}

func (suite *CacheIntegrationTestSuite) BeforeTest(_, _ string) {
	ctx, cancel = context.WithCancel(context.Background())

	suite.cache.Flush(ctx)
}

func (suite *CacheIntegrationTestSuite) TestTimeoutMessagesShouldMakeAvailableWithMaxScore() {
	_, insertError := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
	})
	require.NoError(suite.T(), insertError)

	_, pullError := suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), pullError)

	select {
	case <-time.After(time.Millisecond * 50):
		// giving time to redis

	case <-time.After(300 * time.Millisecond):
		result, err := suite.cache.TimeoutMessages(ctx, "queue", 50*time.Millisecond)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), 1, len(result))
		require.Equal(suite.T(), []string{"id1"}, result)

		// Insert lower score
		now := time.Now()
		_, insertError := suite.cache.Insert(ctx, "queue", &entities.Message{
			ID:          "id2",
			Description: "desc",
			Queue:       "queue",
			LastUsage:   &now,
		})
		require.NoError(suite.T(), insertError)

		// Guarantee timeout message has the maximum score
		messages, pullAfterTimeout := suite.cache.PullMessages(ctx, "queue", 1, time.Now().Unix())
		require.NoError(suite.T(), pullAfterTimeout)
		require.Len(suite.T(), messages, 1)
		require.Equal(suite.T(), "id1", messages[0])
	}
}

func (suite *CacheIntegrationTestSuite) TestFlush() {
	_ = suite.cache.Set(ctx, RECOVERY_STORAGE_BREAKPOINT_KEY, "asdf")

	_, insertError := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
	})
	require.NoError(suite.T(), insertError)

	suite.cache.Flush(ctx)

	breakpoint, err := suite.cache.Get(ctx, RECOVERY_STORAGE_BREAKPOINT_KEY)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), "", breakpoint)

	messages, pullError := suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), pullError)
	require.Len(suite.T(), messages, 0)
}

func (suite *CacheIntegrationTestSuite) TestTimeoutMessagesShouldNotMakeAvailable() {
	threeMinutesTime := time.Now().Add(-3 * time.Minute)

	_, insertError := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(utils.TimeToMs(&threeMinutesTime)),
	})
	require.NoError(suite.T(), insertError)

	_, insertError2 := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "queue",
	})
	require.NoError(suite.T(), insertError2)

	messages, pullError := suite.cache.PullMessages(ctx, "queue", 2, 0)
	require.NoError(suite.T(), pullError)
	require.Len(suite.T(), messages, 2)

	noMessages, pullError := suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), pullError)
	require.Len(suite.T(), noMessages, 0)

	result, err := suite.cache.TimeoutMessages(ctx, "queue", DefaultCacheTimeout)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), 0, len(result))

	messages, pullAfterTimeout := suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), pullAfterTimeout)
	require.Len(suite.T(), messages, 0)
}

func (suite *CacheIntegrationTestSuite) TestGetStorageBreakpointShouldReturnEmpty() {
	result, err := suite.cache.Get(ctx, RECOVERY_STORAGE_BREAKPOINT_KEY)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), "", result)
}

func (suite *CacheIntegrationTestSuite) TestSetStorageBreakpointShouldReturnValue() {
	_ = suite.cache.Set(ctx, RECOVERY_STORAGE_BREAKPOINT_KEY, "asdfg")

	time.Sleep(time.Millisecond * 100)

	result, err := suite.cache.Get(ctx, RECOVERY_STORAGE_BREAKPOINT_KEY)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), "asdfg", result)
}

func (suite *CacheIntegrationTestSuite) TestInsertOneOkShouldNotBeAvailableAgain() {
	inserts, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1"}, inserts)

	messages, err := suite.cache.PullMessages(ctx, "queue", 1, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 1)
	require.Equal(suite.T(), []string{"id1"}, messages)

	messagesAgain, err := suite.cache.PullMessages(ctx, "queue", 1, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messagesAgain, 0)
}

func (suite *CacheIntegrationTestSuite) TestPullShouldResultMaxScore() {
	// Older last usage = bigger score
	inserts, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(50),
	}, &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(5),
	}, &entities.Message{
		ID:          "id3",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(10),
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1", "id2", "id3"}, inserts)

	// Result should be id 2, id 3 and then id 1
	messages, err := suite.cache.PullMessages(ctx, "queue", 100, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 3)
	require.Equal(suite.T(), []string{"id2", "id3", "id1"}, messages)
}

func (suite *CacheIntegrationTestSuite) TestInsertOrderedPullShouldResultMaxScore() {
	inserts, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(50),
	}, &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(10),
	}, &entities.Message{
		ID:          "id3",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(5),
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1", "id2", "id3"}, inserts)

	messages, err := suite.cache.PullMessages(ctx, "queue", 100, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 3)
	require.Equal(suite.T(), []string{"id3", "id2", "id1"}, messages)
}

func (suite *CacheIntegrationTestSuite) TestInsertSameObjectTwiceShouldNotUpdateScore() {
	first, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(50),
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1"}, first)

	second, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(20),
	})
	require.NoError(suite.T(), err)

	// count 0 -> element already exists
	require.Equal(suite.T(), 0, len(second))

	// insert element with a lower score
	third, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(30),
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id2"}, third)

	// Now the element id2 has the best score
	messages, err := suite.cache.PullMessages(ctx, "queue", 1, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 1)
	require.Equal(suite.T(), []string{"id2"}, messages)
}

func (suite *CacheIntegrationTestSuite) TestInsertSameObjectInSameRequestShouldPreserveLastScore() {
	first, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(50),
	}, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(20),
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1", "id1"}, first)

	// insert element with a higher score
	third, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(30),
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id2"}, third)

	// Now the element id2 has the best score
	messages, err := suite.cache.PullMessages(ctx, "queue", 1, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 1)
	require.Equal(suite.T(), []string{"id1"}, messages)
}

func (suite *CacheIntegrationTestSuite) TestCacheShouldSupportLowScoreDifferences() {
	now := time.Now()
	firstInsert, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(utils.TimeToMs(&now)),
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1"}, firstInsert)

	secondInsert, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "queue",
		Score:       float64(utils.TimeToMs(&now) - 1),
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id2"}, secondInsert)

	messages, err := suite.cache.PullMessages(ctx, "queue", 2, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 2)
	require.Equal(suite.T(), []string{"id2", "id1"}, messages)
}

func (suite *CacheIntegrationTestSuite) TestPullMoreThanAvailable() {
	inserts, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
	}, &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "queue",
	}, &entities.Message{
		ID:          "id3",
		Description: "desc",
		Queue:       "queue",
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1", "id2", "id3"}, inserts)

	messages, err := suite.cache.PullMessages(ctx, "queue", 100, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 3)

	sort.Strings(messages)
	expected := []string{"id1", "id2", "id3"}

	require.Equal(suite.T(), expected, messages)
}

func (suite *CacheIntegrationTestSuite) TestRemoveShouldDeleteFromProcessingQueue() {
	inserts, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1"}, inserts)

	_, pullError := suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), pullError)

	result, err := suite.cache.IsProcessing(ctx, "queue", "id1")
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	_, removeErr := suite.cache.Remove(ctx, "queue", "id1")
	require.NoError(suite.T(), removeErr)

	notProcessingResult, err := suite.cache.IsProcessing(ctx, "queue", "id1")
	require.NoError(suite.T(), err)
	require.False(suite.T(), notProcessingResult)
}

func (suite *CacheIntegrationTestSuite) TestRemoveShouldDeleteOnlyCorrectId() {
	inserts, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
	}, &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "queue",
	})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1", "id2"}, inserts)

	_, opErr := suite.cache.Remove(ctx, "queue", "id1")
	require.NoError(suite.T(), opErr)

	messages, pullError := suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), pullError)
	require.Len(suite.T(), messages, 1)
	require.Equal(suite.T(), "id2", messages[0])

	noResult, pullError := suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), pullError)
	require.Len(suite.T(), noResult, 0)
}

func (suite *CacheIntegrationTestSuite) TestRemoveShouldDeleteFromActiveQueue() {
	_, opErr := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
	})
	require.NoError(suite.T(), opErr)

	_, removeErr := suite.cache.Remove(ctx, "queue", "id1")
	require.NoError(suite.T(), removeErr)

	pullResult, err := suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), pullResult, 0)
}

func (suite *CacheIntegrationTestSuite) TestBulkElementsShouldNotError() {
	ids := make([]string, 15222)
	data := make([]*entities.Message, 15222)

	for i := 0; i < 15222; i++ {
		id := fmt.Sprintf("giganicstringtoconsumelotofmemoryofredisscript%d", i)

		ids[i] = id
		data[i] = &entities.Message{
			ID:          id,
			Description: "desc",
			Queue:       "queue",
			Score:       123456,
		}
	}

	insertedElements, opErr := suite.cache.Insert(ctx, "queue", data...)
	require.NoError(suite.T(), opErr)
	require.Equal(suite.T(), ids, insertedElements)

	pullResult, err := suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), pullResult, 1)

	count, removeErr := suite.cache.Remove(ctx, "queue", ids...)
	require.NoError(suite.T(), removeErr)
	require.Equal(suite.T(), int64(15222), count)

	pullResult, err = suite.cache.PullMessages(ctx, "queue", 1, 0)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), pullResult, 0)
}

func (suite *CacheIntegrationTestSuite) TestInsertOneWithInvalidQueue() {
	inserts, err := suite.cache.Insert(ctx, "queue", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "other_queue",
	})

	require.Error(suite.T(), err, "invalid message queue")
	require.Nil(suite.T(), inserts)
}

func (suite *CacheIntegrationTestSuite) TestMakeAvailableAfterPull() {
	message := &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "queue",
	}

	_, opErr := suite.cache.Insert(ctx, "queue", message)
	require.NoError(suite.T(), opErr)

	messages, err := suite.cache.PullMessages(ctx, "queue", 1, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 1)
	require.Equal(suite.T(), []string{"id1"}, messages)

	result, availableErr := suite.cache.MakeAvailable(ctx, message)
	require.NoError(suite.T(), availableErr)
	require.True(suite.T(), result)

	messagesAgain, err := suite.cache.PullMessages(ctx, "queue", 1, 0)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messagesAgain, 1)
	require.Equal(suite.T(), []string{"id1"}, messagesAgain)
}

func (suite *CacheIntegrationTestSuite) TestMakeAvailableWithoutQueue() {
	message := &entities.Message{
		ID:          "id1",
		Description: "desc",
	}

	result, makeErr := suite.cache.MakeAvailable(ctx, message)

	require.Error(suite.T(), makeErr)
	require.False(suite.T(), result)
}

func (suite *CacheIntegrationTestSuite) TestLockMessageAck() {
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})

	_, _ = suite.cache.PullMessages(ctx, "q1", 1, 0)

	result, makeErr := suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
		LockMs:      10,
	}, LOCK_ACK)

	require.NoError(suite.T(), makeErr)
	require.True(suite.T(), result)

	result, err := suite.cache.IsProcessing(ctx, "q1", "id1")
	require.NoError(suite.T(), err)
	require.False(suite.T(), result)

	queues, err := suite.cache.ListQueues(ctx, "*", entities.LOCK_ACK_POOL)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"q1"}, queues)

	// Check if all other pools are empty
	queues, err = suite.cache.ListQueues(ctx, "*", entities.PRIMARY_POOL)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), queues)

	queues, err = suite.cache.ListQueues(ctx, "*", entities.PROCESSING_POOL)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), queues)

	queues, err = suite.cache.ListQueues(ctx, "*", entities.LOCK_NACK_POOL)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), queues)
}

func (suite *CacheIntegrationTestSuite) TestLockWithInvalidLockMs() {
	result, makeErr := suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
		LockMs:      -1,
	}, LOCK_ACK)

	require.Error(suite.T(), makeErr)
	require.False(suite.T(), result)
}

func (suite *CacheIntegrationTestSuite) TestLockWithoutLockMsShouldError() {
	// Add one of them to lock ACK
	lockAckResult, err := suite.cache.LockMessage(ctx, &entities.Message{
		ID:    "1",
		Queue: "q1",
	}, LOCK_ACK)
	require.Error(suite.T(), err)
	require.False(suite.T(), lockAckResult)
}

func (suite *CacheIntegrationTestSuite) TestLockMessageNack() {
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})

	_, _ = suite.cache.PullMessages(ctx, "q1", 1, 0)

	result, makeErr := suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
		LockMs:      10,
	}, LOCK_NACK)

	require.NoError(suite.T(), makeErr)
	require.True(suite.T(), result)

	result, err := suite.cache.IsProcessing(ctx, "q1", "id1")
	require.NoError(suite.T(), err)
	require.False(suite.T(), result)

	queues, err := suite.cache.ListQueues(ctx, "*", entities.LOCK_NACK_POOL)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"q1"}, queues)

	// Check if all other pools are empty
	queues, err = suite.cache.ListQueues(ctx, "*", entities.PRIMARY_POOL)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), queues)

	queues, err = suite.cache.ListQueues(ctx, "*", entities.PROCESSING_POOL)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), queues)

	queues, err = suite.cache.ListQueues(ctx, "*", entities.LOCK_ACK_POOL)
	require.NoError(suite.T(), err)
	require.Empty(suite.T(), queues)
}

func (suite *CacheIntegrationTestSuite) TestLockMessageWithoutQueueShouldReturnError() {
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})

	result, makeErr := suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		LockMs:      10,
	}, LOCK_NACK)

	require.Error(suite.T(), makeErr)
	require.False(suite.T(), result)
}

func (suite *CacheIntegrationTestSuite) TestLockAckWithoutProcessingReturnFalse() {
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})

	result, makeErr := suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
		LockMs:      10,
	}, LOCK_ACK)

	require.NoError(suite.T(), makeErr)
	require.False(suite.T(), result)
}
func (suite *CacheIntegrationTestSuite) TestLockNackWithoutProcessingReturnFalse() {
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})

	result, makeErr := suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
		LockMs:      10,
	}, LOCK_NACK)

	require.NoError(suite.T(), makeErr)
	require.False(suite.T(), result)
}

func (suite *CacheIntegrationTestSuite) TestMakeAvailableMessageWithoutProcessingReturnFalse() {
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})

	result, makeErr := suite.cache.MakeAvailable(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})

	require.NoError(suite.T(), makeErr)
	require.False(suite.T(), result)
}

func (suite *CacheIntegrationTestSuite) TestListQueuesPrimaryPool() {
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})

	_, _ = suite.cache.Insert(ctx, "q2", &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "q2",
	})

	result, makeErr := suite.cache.ListQueues(ctx, "*", entities.PRIMARY_POOL)

	sort.Strings(result)

	require.NoError(suite.T(), makeErr)
	require.Equal(suite.T(), []string{"q1", "q2"}, result)
}

func (suite *CacheIntegrationTestSuite) TestRemoveShouldRemoveFromAllPools() {
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:    "id1",
		Queue: "q1",
	})
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:    "id2",
		Queue: "q1",
	})
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:    "id3",
		Queue: "q1",
	})
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:    "id4",
		Queue: "q1",
	})

	// Remove 3 elements from active
	ids, err := suite.cache.PullMessages(ctx, "q1", 3, 0)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), ids, 3)

	// Add one of them to lock ACK
	lockAckResult, err := suite.cache.LockMessage(ctx, &entities.Message{
		ID:     ids[0],
		Queue:  "q1",
		LockMs: 10000,
	}, LOCK_ACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), lockAckResult)

	// Add other to lock ACK
	lockNackResult, err := suite.cache.LockMessage(ctx, &entities.Message{
		ID:     ids[1],
		Queue:  "q1",
		LockMs: 10000,
	}, LOCK_ACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), lockNackResult)

	removed, err := suite.cache.Remove(ctx, "q1", "id1", "id2", "id3", "id4")
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(4), removed)

	// Asserts all pools are empty
	result, makeErr := suite.cache.ListQueues(ctx, "*", entities.PRIMARY_POOL)
	require.NoError(suite.T(), makeErr)
	require.Empty(suite.T(), result)

	result, makeErr = suite.cache.ListQueues(ctx, "*", entities.PROCESSING_POOL)
	require.NoError(suite.T(), makeErr)
	require.Empty(suite.T(), result)

	result, makeErr = suite.cache.ListQueues(ctx, "*", entities.LOCK_ACK_POOL)
	require.NoError(suite.T(), makeErr)
	require.Empty(suite.T(), result)

	result, makeErr = suite.cache.ListQueues(ctx, "*", entities.LOCK_NACK_POOL)
	require.NoError(suite.T(), makeErr)
	require.Empty(suite.T(), result)
}

func (suite *CacheIntegrationTestSuite) TestUnlockMessagesFromAckPool() {
	// Insert data
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "q1",
	})
	_, _ = suite.cache.Insert(ctx, "q2", &entities.Message{
		ID:          "id3",
		Description: "desc",
		Queue:       "q2",
	})

	// Pull data
	ids, _ := suite.cache.PullMessages(ctx, "q1", 2, 0)
	require.Len(suite.T(), ids, 2)

	ids, _ = suite.cache.PullMessages(ctx, "q2", 1, 0)
	require.Len(suite.T(), ids, 1)

	// Lock data
	result, err := suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
		LockMs:      100,
	}, LOCK_ACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	result, err = suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "q1",
		LockMs:      1,
	}, LOCK_ACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	result, err = suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id3",
		Description: "desc",
		Queue:       "q2",
		LockMs:      100,
	}, LOCK_ACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	<-time.After(1 * time.Millisecond)

	// Check for unlocked data
	messages, err := suite.cache.UnlockMessages(ctx, "q1", LOCK_ACK)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id2"}, messages)
}

func (suite *CacheIntegrationTestSuite) TestUnlockMessagesFromNackPool() {
	// Insert data
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "q1",
	})
	_, _ = suite.cache.Insert(ctx, "q2", &entities.Message{
		ID:          "id3",
		Description: "desc",
		Queue:       "q2",
	})

	// Pull data
	ids, _ := suite.cache.PullMessages(ctx, "q1", 2, 0)
	require.Len(suite.T(), ids, 2)

	ids, _ = suite.cache.PullMessages(ctx, "q2", 1, 0)
	require.Len(suite.T(), ids, 1)

	// Lock data
	result, err := suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
		LockMs:      100,
	}, LOCK_NACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	result, err = suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "q1",
		LockMs:      1,
	}, LOCK_NACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	result, err = suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id3",
		Description: "desc",
		Queue:       "q2",
		LockMs:      100,
	}, LOCK_NACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	<-time.After(1 * time.Millisecond)
	// Check for unlocked data
	messages, err := suite.cache.UnlockMessages(ctx, "q1", LOCK_NACK)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id2"}, messages)
}

func (suite *CacheIntegrationTestSuite) TestUnlockTiming() {
	// Insert data
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
	})
	_, _ = suite.cache.Insert(ctx, "q1", &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "q1",
	})
	_, _ = suite.cache.Insert(ctx, "q2", &entities.Message{
		ID:          "id3",
		Description: "desc",
		Queue:       "q2",
	})

	// Pull data
	ids, _ := suite.cache.PullMessages(ctx, "q1", 2, 0)
	require.Len(suite.T(), ids, 2)

	ids, _ = suite.cache.PullMessages(ctx, "q2", 1, 0)
	require.Len(suite.T(), ids, 1)

	// Lock data
	result, err := suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id1",
		Description: "desc",
		Queue:       "q1",
		LockMs:      50,
	}, LOCK_NACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	result, err = suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id2",
		Description: "desc",
		Queue:       "q1",
		LockMs:      100,
	}, LOCK_NACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	result, err = suite.cache.LockMessage(ctx, &entities.Message{
		ID:          "id3",
		Description: "desc",
		Queue:       "q2",
		LockMs:      150,
	}, LOCK_NACK)
	require.NoError(suite.T(), err)
	require.True(suite.T(), result)

	// Check for unlocked data

	<-time.After(time.Millisecond * 50)

	messages, err := suite.cache.UnlockMessages(ctx, "q1", LOCK_NACK)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id1"}, messages)

	<-time.After(time.Millisecond * 50)

	messages, err = suite.cache.UnlockMessages(ctx, "q1", LOCK_NACK)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id2"}, messages)

	<-time.After(time.Millisecond * 50)

	messages, err = suite.cache.UnlockMessages(ctx, "q2", LOCK_NACK)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), []string{"id3"}, messages)
}
