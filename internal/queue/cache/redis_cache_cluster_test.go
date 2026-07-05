package cache

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/queue/message"
	"github.com/takenet/deckard/internal/queue/pool"
)

const testRedisClusterURI = "redis://localhost:7000?addr=localhost:7001&addr=localhost:7002"

func configureClusterIntegrationTest() {
	config.Configure(true)
	config.RedisClusterMode.Set(true)
	config.CacheUri.Set(testRedisClusterURI)
}

func TestRedisCacheClusterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	configureClusterIntegrationTest()

	cache, err := NewRedisCache(ctx)

	require.NoError(t, err)
	require.True(t, cache.clusterMode, "Cache should be in cluster mode")

	suite.Run(t, &CacheIntegrationTestSuite{
		cache: cache,
	})
}

// TestNewClusterCacheWithoutServerShouldErrorIntegration exercises the cluster-mode connection
// path (createClusterClient/waitForClusterClient), which has its own retry/error handling separate
// from the single-node client and would otherwise have no failure-path coverage at all.
func TestNewClusterCacheWithoutServerShouldErrorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheConnectionRetryEnabled.Set(false)
	config.RedisClusterMode.Set(true)
	config.CacheUri.Set("redis://localhost:19999")

	_, err := NewRedisCache(ctx)

	require.Error(t, err)
}

// TestRedisCacheClusterListQueuesIntegration verifies that ListQueues strips the
// Redis Cluster hash tag braces ("{" and "}") from the queue names it returns,
// since those braces are an internal key-naming detail used to keep all keys for
// the same queue in the same hash slot and must not leak to callers (e.g. the
// housekeeper, which re-derives pool keys from the returned queue names).
func TestRedisCacheClusterListQueuesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	configureClusterIntegrationTest()

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	queueName := "list-queues-test-queue"

	_, err = cache.Insert(ctx, queueName, &message.Message{
		ID:          "msg1",
		Description: "test message",
		Queue:       queueName,
		Score:       1,
	})
	require.NoError(t, err)

	activeQueues, err := cache.ListQueues(ctx, "*", pool.PRIMARY_POOL)
	require.NoError(t, err)
	require.Contains(t, activeQueues, queueName)

	for _, q := range activeQueues {
		require.NotContains(t, q, "{")
		require.NotContains(t, q, "}")
	}

	cache.Flush(ctx)
}

// TestRedisCacheClusterListQueuesFansOutAcrossShards verifies that ListQueues finds queues
// regardless of which cluster shard they hash to. A naive single ClusterClient.Scan call is a
// "keyless" command that go-redis routes to only one (round-robin-picked) master node per call,
// so it would silently miss queues living on other shards. Inserting enough distinctly-named
// queues all but guarantees they spread across all 3 masters of the local test cluster, so this
// test fails if ListQueues stops fanning out via ForEachMaster.
func TestRedisCacheClusterListQueuesFansOutAcrossShards(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	configureClusterIntegrationTest()

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	const queueCount = 30
	expectedQueues := make([]string, 0, queueCount)

	for i := 0; i < queueCount; i++ {
		queueName := fmt.Sprintf("fanout-test-queue-%d", i)
		expectedQueues = append(expectedQueues, queueName)

		_, err = cache.Insert(ctx, queueName, &message.Message{
			ID:          fmt.Sprintf("msg-%d", i),
			Description: "test message",
			Queue:       queueName,
			Score:       1,
		})
		require.NoError(t, err)
	}

	activeQueues, err := cache.ListQueues(ctx, "*", pool.PRIMARY_POOL)
	require.NoError(t, err)
	require.ElementsMatch(t, expectedQueues, activeQueues, "ListQueues must find every queue regardless of which shard it hashes to")

	cache.Flush(ctx)
}

func TestRedisCacheClusterLuaScriptsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	configureClusterIntegrationTest()

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

	msg1Processing, err := cache.IsProcessing(ctx, queueName, "msg1")
	require.NoError(t, err)
	require.True(t, msg1Processing)
	msg2Processing, err := cache.IsProcessing(ctx, queueName, "msg2")
	require.NoError(t, err)
	require.True(t, msg2Processing)

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

	msg1Processing, err = cache.IsProcessing(ctx, queueName, "msg1")
	require.NoError(t, err)
	require.False(t, msg1Processing)
	msg2Processing, err = cache.IsProcessing(ctx, queueName, "msg2")
	require.NoError(t, err)
	require.True(t, msg2Processing)

	// Test Remove operation (uses removeElementScript)
	//
	// removed counts ZREM hits across every pool (active, processing, lock_ack, lock_nack,
	// lock_ack:score, lock_nack:score), not distinct message IDs: msg2 is only ever in the
	// processing pool (1 hit), but msg1 was locked above, so lockElementScript put it in both the
	// lock_ack pool and its companion lock_ack:score pool (2 hits) - hence 3, not len(ids)=2.
	removed, err := cache.Remove(ctx, queueName, "msg1", "msg2")
	require.NoError(t, err)
	require.Equal(t, int64(3), removed)

	msg1Processing, err = cache.IsProcessing(ctx, queueName, "msg1")
	require.NoError(t, err)
	require.False(t, msg1Processing)
	msg2Processing, err = cache.IsProcessing(ctx, queueName, "msg2")
	require.NoError(t, err)
	require.False(t, msg2Processing)

	pulledMessages, err = cache.PullMessages(ctx, queueName, 1, nil, nil, 5000)
	require.NoError(t, err)
	require.Nil(t, pulledMessages)

	cache.Flush(ctx)
}

// TestInsertShouldInsertWithCorrectScoreClusterIntegration is the cluster-mode counterpart of
// TestInsertShouldInsertWithCorrectScoreIntegration (redis_cache_test.go), reusing the same
// assertQueueScoreIntegration helper so the raw-score assertion logic isn't duplicated per mode.
func TestInsertShouldInsertWithCorrectScoreClusterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	configureClusterIntegrationTest()

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	queueName := "insert-score-test-queue"

	inserts, opErr := cache.Insert(ctx, queueName, &message.Message{
		ID:          "123",
		Description: "desc",
		Queue:       queueName,
		Score:       654231,
	})
	require.NoError(t, opErr)
	require.Equal(t, []string{"123"}, inserts)

	assertQueueScoreIntegration(t, cache, queueName, "123", 654231)

	cache.Flush(ctx)
}

func TestRedisClusterConfigurationValidation(t *testing.T) {
	config.Configure(true)
	config.RedisClusterMode.Set(true)
	config.CacheUri.Set("") // Empty URI should cause error

	_, err := NewRedisCache(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, errCacheUriRequired)
}

// TestRedisClusterKeyGeneration is a pure unit test (no live Redis needed) covering the same
// hash-tag key naming that TestRedisCacheClusterKeyNamingIntegration used to check against a live
// cluster - key generation is plain string logic, so it doesn't need a real connection to verify.
func TestRedisClusterKeyGeneration(t *testing.T) {
	t.Parallel()

	cache := &RedisCache{clusterMode: true}

	// Test cluster mode key generation with hash tags
	queueName := "test-queue-with-special-chars_123"

	require.Equal(t, "deckard:queue:{test-queue-with-special-chars_123}", cache.activePool(queueName))
	require.Equal(t, "deckard:queue:{test-queue-with-special-chars_123}:tmp", cache.processingPool(queueName))
	require.Equal(t, "deckard:queue:{test-queue-with-special-chars_123}:lock_ack", cache.lockPool(queueName, LOCK_ACK))
	require.Equal(t, "deckard:queue:{test-queue-with-special-chars_123}:lock_nack", cache.lockPool(queueName, LOCK_NACK))
	require.Equal(t, "deckard:queue:{test-queue-with-special-chars_123}:lock_ack:score", cache.lockPoolScore(queueName, LOCK_ACK))
	require.Equal(t, "deckard:queue:{test-queue-with-special-chars_123}:lock_nack:score", cache.lockPoolScore(queueName, LOCK_NACK))
}

func TestSingleNodeKeyGeneration(t *testing.T) {
	t.Parallel()

	cache := &RedisCache{clusterMode: false}

	// Test single node mode key generation without hash tags
	queueName := "test-queue"

	require.Equal(t, "deckard:queue:test-queue", cache.activePool(queueName))
	require.Equal(t, "deckard:queue:test-queue:tmp", cache.processingPool(queueName))
	require.Equal(t, "deckard:queue:test-queue:lock_ack", cache.lockPool(queueName, LOCK_ACK))
	require.Equal(t, "deckard:queue:test-queue:lock_nack", cache.lockPool(queueName, LOCK_NACK))
	require.Equal(t, "deckard:queue:test-queue:lock_ack:score", cache.lockPoolScore(queueName, LOCK_ACK))
	require.Equal(t, "deckard:queue:test-queue:lock_nack:score", cache.lockPoolScore(queueName, LOCK_NACK))
}

func TestParseQueueKeyStripsClusterHashTag(t *testing.T) {
	t.Parallel()

	cache := &RedisCache{clusterMode: true}

	require.Equal(t, "test-queue", cache.parseQueueKey("deckard:queue:{test-queue}", nil))
	require.Equal(t, "test-queue", cache.parseQueueKey("deckard:queue:{test-queue}:tmp", PROCESSING_POOL_REGEX))
	require.Equal(t, "test-queue", cache.parseQueueKey("deckard:queue:{test-queue}:lock_ack", LOCK_ACK_POOL_REGEX))
	require.Equal(t, "test-queue", cache.parseQueueKey("deckard:queue:{test-queue}:lock_nack", LOCK_NACK_POOL_REGEX))
	require.Equal(t, "queue}name", cache.parseQueueKey("deckard:queue:{queue}name}", nil))
	require.Equal(t, "queue{name}", cache.parseQueueKey("deckard:queue:{queue{name}}", nil))
}

func TestParseQueueKeyWithoutClusterMode(t *testing.T) {
	t.Parallel()

	cache := &RedisCache{clusterMode: false}

	require.Equal(t, "test-queue", cache.parseQueueKey("deckard:queue:test-queue", nil))
	require.Equal(t, "test-queue", cache.parseQueueKey("deckard:queue:test-queue:tmp", PROCESSING_POOL_REGEX))
}

func TestValidateQueueNameRejectsClusterHashTagCharsInClusterMode(t *testing.T) {
	t.Parallel()

	cache := &RedisCache{clusterMode: true}

	require.ErrorIs(t, cache.validateQueueName("bad{queue"), errQueueNameContainsClusterHashTag)
	require.ErrorIs(t, cache.validateQueueName("bad}queue"), errQueueNameContainsClusterHashTag)
	require.NoError(t, cache.validateQueueName("valid-queue"))
}

func TestValidateQueueNameRejectsClusterHashTagCharsOutsideClusterMode(t *testing.T) {
	t.Parallel()

	cache := &RedisCache{clusterMode: false}

	require.ErrorIs(t, cache.validateQueueName("bad{queue"), errQueueNameContainsClusterHashTag)
	require.ErrorIs(t, cache.validateQueueName("bad}queue"), errQueueNameContainsClusterHashTag)
	require.NoError(t, cache.validateQueueName("valid-queue"))
}

func TestInsertRejectsQueueNamesContainingClusterHashTagCharsInClusterMode(t *testing.T) {
	t.Parallel()

	cache := &RedisCache{clusterMode: true}

	_, err := cache.Insert(ctx, "bad{queue}")
	require.ErrorIs(t, err, errQueueNameContainsClusterHashTag)
}

// Not run with t.Parallel(): config.Configure/Set mutate process-global viper state, which would
// race with other tests that also mutate it in parallel.
func TestClusterOptionsFromConfigWithoutURIShouldError(t *testing.T) {
	config.Configure(true)
	config.CacheUri.Set("")

	_, err := clusterOptionsFromConfig()
	require.ErrorIs(t, err, errCacheUriRequired)
}

// Not run with t.Parallel(): see TestClusterOptionsFromConfigWithoutURIShouldError.
func TestClusterOptionsFromConfigWithAddrParams(t *testing.T) {
	config.Configure(true)
	config.CacheUri.Set("redis://redis-user:redis-pass@redis-a:6379?addr=redis-b:6379")

	options, err := clusterOptionsFromConfig()
	require.NoError(t, err)
	require.Equal(t, []string{"redis-a:6379", "redis-b:6379"}, options.Addrs)
	require.Equal(t, "redis-user", options.Username)
	require.Equal(t, "redis-pass", options.Password)
	require.Nil(t, options.TLSConfig)
}

// Not run with t.Parallel(): see TestClusterOptionsFromConfigWithoutURIShouldError.
func TestClusterOptionsFromConfigWithRedissURI(t *testing.T) {
	config.Configure(true)
	config.CacheUri.Set("rediss://uri-user:uri-pass@redis-1:6379?addr=redis-2:6379")

	options, err := clusterOptionsFromConfig()
	require.NoError(t, err)
	require.Equal(t, []string{"redis-1:6379", "redis-2:6379"}, options.Addrs)
	require.Equal(t, "uri-user", options.Username)
	require.Equal(t, "uri-pass", options.Password)
	require.NotNil(t, options.TLSConfig)
}

// go-redis's cluster URL parser (unlike its standalone one) does not support skip_verify: this
// documents that asymmetry so it isn't mistaken for a bug if someone tries it against a cluster.
//
// Not run with t.Parallel(): see TestClusterOptionsFromConfigWithoutURIShouldError.
func TestClusterOptionsFromConfigWithSkipVerifyShouldError(t *testing.T) {
	config.Configure(true)
	config.CacheUri.Set("rediss://redis-1:6379?skip_verify=true")

	_, err := clusterOptionsFromConfig()
	require.Error(t, err)
}
