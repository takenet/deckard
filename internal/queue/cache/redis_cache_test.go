package cache

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/queue/message"
)

func TestRedisCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")

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
	config.CacheUri.Set("redis://localhost:12345/0")

	_, err := NewRedisCache(ctx)

	require.Error(t, err)
}

func TestNewCacheWithoutCacheUriShouldErrorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("")

	_, err := NewRedisCache(ctx)

	require.ErrorIs(t, err, errCacheUriRequired)
}

func TestInsertShouldInsertWithCorrectScoreIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")

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

	assertQueueScoreIntegration(t, cache, "queue", "123", 654231)
	assertQueueScoreIntegration(t, cache, "queue", "234", 123456)

	cache.Flush(ctx)
}

// assertQueueScoreIntegration inspects the raw active pool sorted set to confirm an element was
// stored with the expected score. Shared between single-node and cluster tests so the key-naming
// difference (hash tags in cluster mode) is resolved once, via the live cache's own activePool,
// instead of being duplicated or hardcoded per mode.
func assertQueueScoreIntegration(t *testing.T, cache *RedisCache, queue string, id string, score float64) {
	t.Helper()

	cmd := cache.Client.ZScore(ctx, cache.activePool(queue), id)
	require.NoError(t, cmd.Err())
	require.Equal(t, score, cmd.Val())
}

func TestConnectWithRedisUsingConnectionURI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// DECKARD_REDIS_URI resolves to the same config key as DECKARD_CACHE_URI (CacheUri's alias).
	_ = os.Setenv("DECKARD_REDIS_URI", "redis://localhost:6379/0")

	defer func() { _ = os.Unsetenv("DECKARD_REDIS_URI") }()

	config.Configure(true)

	require.Equal(t, "redis://localhost:6379/0", config.CacheUri.Get())

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

	assertQueueScoreIntegration(t, cache, "queue", "234", 123456)

	cache.Flush(ctx)
}

// Not run with t.Parallel(): config.Configure/Set mutate process-global viper state, which would
// race with other tests that also mutate it in parallel.
func TestSingleNodeOptionsFromConfigWithoutURIShouldError(t *testing.T) {
	config.Configure(true)
	config.CacheUri.Set("")

	_, err := singleNodeOptionsFromConfig()
	require.ErrorIs(t, err, errCacheUriRequired)
}

// Not run with t.Parallel(): see TestSingleNodeOptionsFromConfigWithoutURIShouldError.
func TestSingleNodeOptionsFromConfigWithRedissURI(t *testing.T) {
	config.Configure(true)
	config.CacheUri.Set("rediss://uri-user:uri-pass@redis-uri:6380/2?skip_verify=true")

	options, err := singleNodeOptionsFromConfig()
	require.NoError(t, err)
	require.Equal(t, "redis-uri:6380", options.Addr)
	require.Equal(t, "uri-user", options.Username)
	require.Equal(t, "uri-pass", options.Password)
	require.Equal(t, 2, options.DB)
	require.NotNil(t, options.TLSConfig)
	require.True(t, options.TLSConfig.InsecureSkipVerify)
}
