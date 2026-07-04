package cache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/queue/message"
	"github.com/takenet/deckard/internal/queue/pool"
)

func TestRedisCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.RedisAddress.Set("localhost")

	cache, err := NewRedisCache(ctx)

	require.NoError(t, err)

	suite.Run(t, &CacheIntegrationTestSuite{
		cache: cache,
	})
}

func TestNewCacheWithoutServerShouldErrorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	defer viper.Reset()
	config.CacheConnectionRetryEnabled.Set(false)
	config.RedisPort.Set(12345)

	_, err := NewRedisCache(ctx)

	require.Error(t, err)
}

func TestInsertShouldInsertWithCorrectScoreIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	data := make([]*message.Message, 2)

	data[0] = &message.Message{
		ID:          "123",
		Description: "desc",
		Queue:       "queue",
		Score:       654231,
	}

	data[1] = &message.Message{
		ID:          "234",
		Description: "desc",
		Queue:       "queue",
		Score:       123456,
	}

	inserts, opErr := cache.Insert(ctx, "queue", data...)
	require.NoError(t, opErr)
	require.Equal(t, []string{"123", "234"}, inserts)

	cmd := cache.Client.ZScore(ctx, (&RedisCache{}).activePool("queue"), "123")
	require.Equal(t, float64(654231), cmd.Val())

	cmd = cache.Client.ZScore(ctx, (&RedisCache{}).activePool("queue"), "234")
	require.Equal(t, float64(123456), cmd.Val())

	cache.Flush(ctx)
}

func TestConnectWithRedisUsingConnectionURI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	os.Setenv("DECKARD_REDIS_URI", "redis://localhost:6379/0")
	os.Setenv("DECKARD_REDIS_ADDRESS", "none")
	os.Setenv("DECKARD_REDIS_PASSWORD", "none")
	os.Setenv("DECKARD_REDIS_PORT", "1234")
	os.Setenv("DECKARD_REDIS_DB", "5")

	defer os.Unsetenv("DECKARD_REDIS_URI")
	defer os.Unsetenv("DECKARD_REDIS_ADDRESS")
	defer os.Unsetenv("DECKARD_REDIS_PASSWORD")
	defer os.Unsetenv("DECKARD_REDIS_PORT")
	defer os.Unsetenv("DECKARD_REDIS_DB")

	config.Configure(true)

	require.Equal(t, "none", config.RedisAddress.Get())
	require.Equal(t, "none", config.RedisPassword.Get())
	require.Equal(t, 1234, config.RedisPort.GetInt())
	require.Equal(t, 5, config.RedisDB.GetInt())

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	data := make([]*message.Message, 1)
	data[0] = &message.Message{
		ID:          "234",
		Description: "desc",
		Queue:       "queue",
		Score:       123456,
	}

	inserts, opErr := cache.Insert(ctx, "queue", data...)
	require.NoError(t, opErr)
	require.Equal(t, []string{"234"}, inserts)

	cmd := cache.Client.ZScore(ctx, (&RedisCache{}).activePool("queue"), "234")
	require.Equal(t, float64(123456), cmd.Val())

	cache.Flush(ctx)
}

func TestGetActivePoolName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "deckard:queue:test", (&RedisCache{}).activePool("test"))
}

func TestGetProcessingPoolName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "deckard:queue:test:tmp", (&RedisCache{}).processingPool("test"))
}

// TestListQueuesWithManyCachedQueuesIntegration tests that ListQueues properly uses SCAN
// to iterate through all keys without blocking Redis, especially with many queues.
// Creates 1500 queues to ensure SCAN pagination is triggered (batch size is 1000).
func TestListQueuesWithManyCachedQueuesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.RedisAddress.Set("localhost")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)
	defer cache.Flush(ctx)

	// Insert messages into 1500 different queues to test SCAN pagination
	// This ensures multiple SCAN iterations (batch size is 1000)
	numQueues := 1500
	for i := 0; i < numQueues; i++ {
		queueName := fmt.Sprintf("test_queue_%d", i)
		_, opErr := cache.Insert(ctx, queueName, &message.Message{
			ID:          "msg1",
			Description: "desc",
			Queue:       queueName,
			Score:       123456,
		})
		require.NoError(t, opErr)
	}

	// Test listing queues with wildcard pattern
	result, listErr := cache.ListQueues(ctx, "*", pool.PRIMARY_POOL)
	require.NoError(t, listErr)
	require.Len(t, result, numQueues)

	// Verify all queue names are present
	queueMap := make(map[string]bool)
	for _, queue := range result {
		queueMap[queue] = true
	}

	for i := 0; i < numQueues; i++ {
		expectedQueue := fmt.Sprintf("test_queue_%d", i)
		require.True(t, queueMap[expectedQueue], "Queue %s should be in the result", expectedQueue)
	}
}

// TestListQueuesWithSpecificPatternIntegration tests pattern matching with SCAN
func TestListQueuesWithSpecificPatternIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.RedisAddress.Set("localhost")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)
	defer cache.Flush(ctx)

	// Insert messages into queues with different prefixes
	queues := []string{"prod_queue_1", "prod_queue_2", "dev_queue_1", "test_queue_1"}
	for _, queueName := range queues {
		_, opErr := cache.Insert(ctx, queueName, &message.Message{
			ID:          "msg1",
			Description: "desc",
			Queue:       queueName,
			Score:       123456,
		})
		require.NoError(t, opErr)
	}

	// Test listing queues with specific pattern
	result, listErr := cache.ListQueues(ctx, "prod*", pool.PRIMARY_POOL)
	require.NoError(t, listErr)
	require.Len(t, result, 2)

	// Verify only prod queues are present
	require.Contains(t, result, "prod_queue_1")
	require.Contains(t, result, "prod_queue_2")
	require.NotContains(t, result, "dev_queue_1")
	require.NotContains(t, result, "test_queue_1")
}

// TestCompareAndDeleteIntegration tests that CompareAndDelete only deletes a key
// when the current value matches the expected one, and is a no-op otherwise.
func TestCompareAndDeleteIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.RedisAddress.Set("localhost")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	key := "compare_and_delete_test_key"
	defer func() { _ = cache.Del(ctx, key) }()

	require.NoError(t, cache.Set(ctx, key, "owner-1"))

	// Should not delete when value does not match
	deleted, err := cache.CompareAndDelete(ctx, key, "owner-2")
	require.NoError(t, err)
	require.False(t, deleted)

	value, err := cache.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "owner-1", value)

	// Should delete when value matches
	deleted, err = cache.CompareAndDelete(ctx, key, "owner-1")
	require.NoError(t, err)
	require.True(t, deleted)

	value, err = cache.Get(ctx, key)
	require.NoError(t, err)
	require.Empty(t, value)

	// Should be a no-op when key no longer exists
	deleted, err = cache.CompareAndDelete(ctx, key, "owner-1")
	require.NoError(t, err)
	require.False(t, deleted)
}

// TestCompareAndExpireIntegration tests that CompareAndExpire only refreshes the TTL
// of a key when the current value matches the expected one.
func TestCompareAndExpireIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.RedisAddress.Set("localhost")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	key := "compare_and_expire_test_key"
	defer func() { _ = cache.Del(ctx, key) }()

	require.NoError(t, cache.Set(ctx, key, "owner-1"))

	// Should not refresh TTL when value does not match
	refreshed, err := cache.CompareAndExpire(ctx, key, "owner-2", time.Minute)
	require.NoError(t, err)
	require.False(t, refreshed)

	// Should refresh TTL when value matches
	refreshed, err = cache.CompareAndExpire(ctx, key, "owner-1", time.Minute)
	require.NoError(t, err)
	require.True(t, refreshed)

	ttl := cache.Client.TTL(ctx, fmt.Sprint("deckard:", key))
	require.NoError(t, ttl.Err())
	require.Greater(t, ttl.Val(), time.Duration(0))
}
