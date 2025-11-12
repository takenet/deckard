package cache

import (
	"fmt"
	"os"
	"testing"

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
// to iterate through all keys without blocking Redis, especially with many queues
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

	// Insert messages into 50 different queues to test SCAN pagination
	numQueues := 50
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
