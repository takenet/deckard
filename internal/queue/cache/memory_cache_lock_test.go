package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// These tests exercise MemoryCache's Store-facing methods (Get/Set/Del/Expire/
// SetNX/CompareAndDelete/CompareAndExpire) directly against a real MemoryCache
// instance - no mocks - since MemoryCache is used as the backing lock.Store in
// integration tests elsewhere (see internal/lock/redis_lock_test.go).

func TestMemoryCacheSetNXShouldOnlySucceedWhenKeyAbsent(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	acquired, err := cache.SetNX(ctx, "key", "value-1", time.Second)
	require.NoError(t, err)
	require.True(t, acquired)

	acquired, err = cache.SetNX(ctx, "key", "value-2", time.Second)
	require.NoError(t, err)
	require.False(t, acquired, "SetNX must fail when the key already exists")

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, "value-1", value, "the original value must be preserved")
}

func TestMemoryCacheSetNXWithoutTTLShouldNotExpire(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	acquired, err := cache.SetNX(ctx, "key", "value", 0)
	require.NoError(t, err)
	require.True(t, acquired)

	time.Sleep(20 * time.Millisecond)

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, "value", value)
}

func TestMemoryCacheSetNXShouldLazilyExpireAfterTTL(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	acquired, err := cache.SetNX(ctx, "key", "value-1", 20*time.Millisecond)
	require.NoError(t, err)
	require.True(t, acquired)

	time.Sleep(40 * time.Millisecond)

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Empty(t, value, "expired key must not be returned by Get")

	acquired, err = cache.SetNX(ctx, "key", "value-2", time.Second)
	require.NoError(t, err)
	require.True(t, acquired, "SetNX must succeed again once the previous key has expired")
}

func TestMemoryCacheSetShouldClearExistingExpiration(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	_, err := cache.SetNX(ctx, "key", "value-1", 20*time.Millisecond)
	require.NoError(t, err)

	err = cache.Set(ctx, "key", "value-2")
	require.NoError(t, err)

	time.Sleep(40 * time.Millisecond)

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, "value-2", value, "Set must clear any previous TTL so the key does not expire")
}

func TestMemoryCacheDelShouldRemoveKeyAndExpiration(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	_, err := cache.SetNX(ctx, "key", "value", time.Second)
	require.NoError(t, err)

	err = cache.Del(ctx, "key")
	require.NoError(t, err)

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Empty(t, value)

	acquired, err := cache.SetNX(ctx, "key", "value-2", time.Second)
	require.NoError(t, err)
	require.True(t, acquired)
}

func TestMemoryCacheExpireOnMissingKeyShouldBeNoop(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	err := cache.Expire(ctx, "missing-key", time.Second)
	require.NoError(t, err)
}

func TestMemoryCacheExpireShouldExtendTTL(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	_, err := cache.SetNX(ctx, "key", "value", 20*time.Millisecond)
	require.NoError(t, err)

	err = cache.Expire(ctx, "key", time.Second)
	require.NoError(t, err)

	time.Sleep(40 * time.Millisecond)

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, "value", value, "Expire must extend the TTL so the key survives past the original expiration")
}

func TestMemoryCacheExpireWithNonPositiveTTLShouldDeleteKey(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	_, err := cache.SetNX(ctx, "key", "value", time.Second)
	require.NoError(t, err)

	err = cache.Expire(ctx, "key", 0)
	require.NoError(t, err)

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Empty(t, value)
}

func TestMemoryCacheCompareAndDeleteShouldRequireMatchingValue(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	_, err := cache.SetNX(ctx, "key", "value", time.Second)
	require.NoError(t, err)

	deleted, err := cache.CompareAndDelete(ctx, "key", "wrong-value")
	require.NoError(t, err)
	require.False(t, deleted)

	deleted, err = cache.CompareAndDelete(ctx, "key", "value")
	require.NoError(t, err)
	require.True(t, deleted)

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Empty(t, value)
}

func TestMemoryCacheCompareAndDeleteOnMissingKeyShouldReturnFalse(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	deleted, err := cache.CompareAndDelete(ctx, "missing-key", "value")
	require.NoError(t, err)
	require.False(t, deleted)
}

func TestMemoryCacheCompareAndExpireShouldRequireMatchingValue(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	_, err := cache.SetNX(ctx, "key", "value", 20*time.Millisecond)
	require.NoError(t, err)

	renewed, err := cache.CompareAndExpire(ctx, "key", "wrong-value", time.Second)
	require.NoError(t, err)
	require.False(t, renewed)

	renewed, err = cache.CompareAndExpire(ctx, "key", "value", time.Second)
	require.NoError(t, err)
	require.True(t, renewed)

	time.Sleep(40 * time.Millisecond)

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, "value", value, "CompareAndExpire must extend the TTL on match")
}

func TestMemoryCacheCompareAndExpireOnMissingKeyShouldReturnFalse(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	renewed, err := cache.CompareAndExpire(ctx, "missing-key", "value", time.Second)
	require.NoError(t, err)
	require.False(t, renewed)
}

func TestMemoryCacheCompareAndExpireWithNonPositiveTTLShouldDeleteKey(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()

	_, err := cache.SetNX(ctx, "key", "value", time.Second)
	require.NoError(t, err)

	renewed, err := cache.CompareAndExpire(ctx, "key", "value", 0)
	require.NoError(t, err)
	require.True(t, renewed)

	value, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Empty(t, value)
}
