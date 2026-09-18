package lock

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/queue/cache"
)

// TestRedisLockUsesConfiguredCachePrefixIntegration verifies that lock keys
// automatically share the same Redis key prefix as every other key Deckard
// stores, since Locker relies entirely on the Store (queue/cache.Cache in
// production) to apply the deployment's configured cache prefix - Locker
// itself never manipulates key names beyond the raw lock name.
//
// Not run with t.Parallel(): config.Configure/.Set mutate process-global
// viper state, which would race with other tests that also mutate it.
func TestRedisLockUsesConfiguredCachePrefixIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	defer viper.Reset()

	config.Configure(true)
	config.CacheUri.Set("redis://localhost:6379/0")
	config.CachePrefix.Set("lock_prefix_test")

	redisCache, err := cache.NewRedisCache(context.Background())
	require.NoError(t, err)
	defer func() { _ = redisCache.Close(context.Background()) }()

	locker := NewLocker(redisCache, "owner-1")

	ctx := context.Background()
	name := "prefix-check-lock"
	ttl := 10 * time.Second

	acquired, err := locker.TryAcquire(ctx, name, ttl)
	require.NoError(t, err)
	require.True(t, acquired)
	defer func() { _ = locker.Release(ctx, name) }()

	keys, err := redisCache.Client.Keys(ctx, "*"+name+"*").Result()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Contains(t, keys[0], "lock_prefix_test",
		"lock key must use the same configured cache prefix as every other key Deckard stores in Redis")
}
