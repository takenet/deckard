package cache

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/queue/message"
)

func TestRedisCacheClusterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Check if cluster test environment is available
	if os.Getenv("REDIS_CLUSTER_TEST") == "" {
		t.Skip("REDIS_CLUSTER_TEST environment variable not set, skipping cluster tests")
	}

	config.Configure(true)
	config.RedisClusterMode.Set(true)
	config.RedisClusterAddresses.Set("localhost:7000,localhost:7001,localhost:7002")

	cache, err := NewRedisCache(ctx)

	require.NoError(t, err)
	require.True(t, cache.clusterMode, "Cache should be in cluster mode")

	suite.Run(t, &CacheIntegrationTestSuite{
		cache: cache,
	})
}

func TestRedisCacheClusterKeyNamingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Check if cluster test environment is available
	if os.Getenv("REDIS_CLUSTER_TEST") == "" {
		t.Skip("REDIS_CLUSTER_TEST environment variable not set, skipping cluster tests")
	}

	config.Configure(true)
	config.RedisClusterMode.Set(true)
	config.RedisClusterAddresses.Set("localhost:7000,localhost:7001,localhost:7002")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	// Test that hash tags are used for key naming in cluster mode
	queueName := "test-queue"
	
	expectedActivePool := "deckard:queue:{test-queue}"
	expectedProcessingPool := "deckard:queue:{test-queue}:tmp"
	expectedLockAckPool := "deckard:queue:{test-queue}:lock_ack"
	expectedLockNackPool := "deckard:queue:{test-queue}:lock_nack"
	expectedLockAckScorePool := "deckard:queue:{test-queue}:lock_ack:score"
	expectedLockNackScorePool := "deckard:queue:{test-queue}:lock_nack:score"

	require.Equal(t, expectedActivePool, cache.activePool(queueName))
	require.Equal(t, expectedProcessingPool, cache.processingPool(queueName))
	require.Equal(t, expectedLockAckPool, cache.lockPool(queueName, LOCK_ACK))
	require.Equal(t, expectedLockNackPool, cache.lockPool(queueName, LOCK_NACK))
	require.Equal(t, expectedLockAckScorePool, cache.lockPoolScore(queueName, LOCK_ACK))
	require.Equal(t, expectedLockNackScorePool, cache.lockPoolScore(queueName, LOCK_NACK))

	cache.Flush(ctx)
}

func TestRedisCacheClusterLuaScriptsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Check if cluster test environment is available
	if os.Getenv("REDIS_CLUSTER_TEST") == "" {
		t.Skip("REDIS_CLUSTER_TEST environment variable not set, skipping cluster tests")
	}

	config.Configure(true)
	config.RedisClusterMode.Set(true)
	config.RedisClusterAddresses.Set("localhost:7000,localhost:7001,localhost:7002")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	// Test that Lua scripts work correctly with clustered Redis
	queueName := "cluster-test-queue"
	
	messages := []*message.Message{
		{
			ID:          "msg1",
			Description: "test message 1",
			Queue:       queueName,
			Score:       100,
		},
		{
			ID:          "msg2",
			Description: "test message 2",
			Queue:       queueName,
			Score:       200,
		},
	}

	// Test Insert operation (uses addElementsScript)
	inserts, err := cache.Insert(ctx, queueName, messages...)
	require.NoError(t, err)
	require.Equal(t, []string{"msg1", "msg2"}, inserts)

	// Test PullMessages operation (uses pullElementsScript)
	pulledMessages, err := cache.PullMessages(ctx, queueName, 2, nil, nil, 5000)
	require.NoError(t, err)
	require.Len(t, pulledMessages, 2)
	require.Contains(t, pulledMessages, "msg1")
	require.Contains(t, pulledMessages, "msg2")

	// Test LockMessage operation (uses lockElementScript)
	message1 := &message.Message{
		ID:     "msg1",
		Queue:  queueName,
		LockMs: 10000,
		Score:  100,
	}
	locked, err := cache.LockMessage(ctx, message1, LOCK_ACK)
	require.NoError(t, err)
	require.True(t, locked)

	// Test Remove operation (uses removeElementScript)
	removed, err := cache.Remove(ctx, queueName, "msg1", "msg2")
	require.NoError(t, err)
	require.Equal(t, int64(2), removed)

	cache.Flush(ctx)
}

func TestRedisClusterConfigurationValidation(t *testing.T) {
	t.Parallel()

	config.Configure(true)
	config.RedisClusterMode.Set(true)
	config.RedisClusterAddresses.Set("") // Empty addresses should cause error

	_, err := NewRedisCache(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "redis.cluster.addresses must be specified")
}

func TestRedisClusterKeyGeneration(t *testing.T) {
	t.Parallel()

	cache := &RedisCache{clusterMode: true}

	// Test cluster mode key generation with hash tags
	queueName := "test-queue-with-special-chars_123"
	
	activePool := cache.activePool(queueName)
	require.Equal(t, "deckard:queue:{test-queue-with-special-chars_123}", activePool)
	
	processingPool := cache.processingPool(queueName)
	require.Equal(t, "deckard:queue:{test-queue-with-special-chars_123}:tmp", processingPool)
	
	lockPool := cache.lockPool(queueName, LOCK_ACK)
	require.Equal(t, "deckard:queue:{test-queue-with-special-chars_123}:lock_ack", lockPool)
}

func TestSingleNodeKeyGeneration(t *testing.T) {
	t.Parallel()

	cache := &RedisCache{clusterMode: false}

	// Test single node mode key generation without hash tags
	queueName := "test-queue"
	
	activePool := cache.activePool(queueName)
	require.Equal(t, "deckard:queue:test-queue", activePool)
	
	processingPool := cache.processingPool(queueName)
	require.Equal(t, "deckard:queue:test-queue:tmp", processingPool)
	
	lockPool := cache.lockPool(queueName, LOCK_ACK)
	require.Equal(t, "deckard:queue:test-queue:lock_ack", lockPool)
}