package lock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/queue/cache"
)

func TestRedisLockTryAcquireReleaseRenew(t *testing.T) {
	t.Parallel()

	// cache.NewMemoryCache() satisfies lock.Store structurally.
	store := cache.NewMemoryCache()

	owner1 := NewLocker(store, "owner-1")
	owner2 := NewLocker(store, "owner-2")

	ctx := context.Background()
	name := "test-lock"
	ttl := time.Second * 10

	acquired, err := owner1.TryAcquire(ctx, name, ttl)
	require.NoError(t, err)
	require.True(t, acquired, "expected owner-1 to acquire the lock")

	acquired, err = owner2.TryAcquire(ctx, name, ttl)
	require.NoError(t, err)
	require.False(t, acquired, "expected owner-2 to fail acquiring an already held lock")

	err = owner1.Release(ctx, name)
	require.NoError(t, err)

	acquired, err = owner2.TryAcquire(ctx, name, ttl)
	require.NoError(t, err)
	require.True(t, acquired, "expected owner-2 to acquire the lock after release")

	err = owner2.Renew(ctx, name, ttl)
	require.NoError(t, err)

	err = owner2.Release(ctx, name)
	require.NoError(t, err)
}

func TestRedisLockReleaseNotHeldIsNoop(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryCache()
	owner1 := NewLocker(store, "owner-1")
	owner2 := NewLocker(store, "owner-2")

	ctx := context.Background()
	name := "test-lock-release"
	ttl := time.Second * 10

	acquired, err := owner1.TryAcquire(ctx, name, ttl)
	require.NoError(t, err)
	require.True(t, acquired)

	// owner2 does not hold the lock, so releasing it must be a no-op (no error,
	// and owner1's lock must remain held).
	err = owner2.Release(ctx, name)
	require.NoError(t, err)

	acquired, err = owner2.TryAcquire(ctx, name, ttl)
	require.NoError(t, err)
	require.False(t, acquired, "owner1's lock should still be held")
}

func TestRedisLockRenewNotHeldReturnsError(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryCache()
	owner1 := NewLocker(store, "owner-1")
	owner2 := NewLocker(store, "owner-2")

	ctx := context.Background()
	name := "test-lock-renew"
	ttl := time.Second * 10

	acquired, err := owner1.TryAcquire(ctx, name, ttl)
	require.NoError(t, err)
	require.True(t, acquired)

	err = owner2.Renew(ctx, name, ttl)
	require.Error(t, err, "owner2 should not be able to renew a lock it does not hold")
}
