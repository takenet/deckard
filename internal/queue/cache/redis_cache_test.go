package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/lock"
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

// TestCompareAndDeleteIntegration tests that CompareAndDelete only deletes a key
// when the current value matches the expected one, and is a no-op otherwise.
func TestCompareAndDeleteIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")

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
	config.CacheUri.Set("redis://localhost:6379/0")

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

	ttl := cache.Client.TTL(ctx, cache.generalCacheKey(key))
	require.NoError(t, ttl.Err())
	require.Greater(t, ttl.Val(), time.Duration(0))
}

// TestSetNXIntegration tests that SetNX only succeeds when the key does not
// already exist, mirroring the lock.Store contract used by internal/lock.
func TestSetNXIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	key := "setnx_test_key"
	defer func() { _ = cache.Del(ctx, key) }()

	acquired, err := cache.SetNX(ctx, key, "owner-1", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)

	acquired, err = cache.SetNX(ctx, key, "owner-2", time.Minute)
	require.NoError(t, err)
	require.False(t, acquired, "SetNX must fail when the key already exists")

	value, err := cache.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "owner-1", value)
}

// TestExpireIntegration tests that Expire refreshes the TTL of an existing key.
func TestExpireIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	key := "expire_test_key"
	defer func() { _ = cache.Del(ctx, key) }()

	require.NoError(t, cache.Set(ctx, key, "value"))

	err = cache.Expire(ctx, key, time.Minute)
	require.NoError(t, err)

	ttl := cache.Client.TTL(ctx, cache.generalCacheKey(key))
	require.NoError(t, ttl.Err())
	require.Greater(t, ttl.Val(), time.Duration(0))
}

// TestCloseIntegration tests that Close releases the underlying Redis client
// connection without error.
func TestCloseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	require.NoError(t, cache.Close(ctx))
}

// TestLockStoreMethodsWithCanceledContextShouldReturnErrorIntegration exercises
// the error branches of SetNX/Del/Expire/CompareAndDelete/CompareAndExpire by
// passing an already-canceled context, which go-redis genuinely rejects - a
// real error path, not a mocked one.
func TestLockStoreMethodsWithCanceledContextShouldReturnErrorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	key := "canceled_ctx_test_key"

	_, err = cache.SetNX(canceledCtx, key, "value", time.Minute)
	require.Error(t, err)

	err = cache.Del(canceledCtx, key)
	require.Error(t, err)

	err = cache.Expire(canceledCtx, key, time.Minute)
	require.Error(t, err)

	_, err = cache.CompareAndDelete(canceledCtx, key, "value")
	require.Error(t, err)

	_, err = cache.CompareAndExpire(canceledCtx, key, "value", time.Minute)
	require.Error(t, err)
}

// TestRedisLockThroughRealRedisCacheIntegration exercises internal/lock.Locker's
// TryAcquire/Release/Renew against a real RedisCache-backed Store (rather than
// the MemoryCache used elsewhere), covering the actual production Store
// implementation end-to-end.
func TestRedisLockThroughRealRedisCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")

	redisCache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	name := "lock_through_real_redis_cache_test"
	defer func() { _ = redisCache.Del(ctx, name) }()

	locker := lock.NewLocker(redisCache, "owner-1")

	acquired, err := locker.TryAcquire(ctx, name, time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)

	err = locker.Renew(ctx, name, time.Minute)
	require.NoError(t, err)

	err = locker.Release(ctx, name)
	require.NoError(t, err)

	value, err := redisCache.Get(ctx, name)
	require.NoError(t, err)
	require.Empty(t, value)
}

// TestRedisLockErrorPropagationWithCanceledContextIntegration exercises the
// err != nil branches of storeLocker's TryAcquire/Release/Renew (internal/lock)
// by routing a genuinely-canceled context through a real RedisCache-backed
// Store - a real error path, not a mocked one.
func TestRedisLockErrorPropagationWithCanceledContextIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")

	redisCache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	locker := lock.NewLocker(redisCache, "owner-1")
	name := "lock_canceled_ctx_test"

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = locker.TryAcquire(canceledCtx, name, time.Minute)
	require.Error(t, err)

	err = locker.Release(canceledCtx, name)
	require.Error(t, err)

	err = locker.Renew(canceledCtx, name, time.Minute)
	require.Error(t, err)
}
