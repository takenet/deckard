package election

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/lock"
	"github.com/takenet/deckard/internal/queue/cache"
)

func TestLeaseElectorSingleInstanceBecomesLeader(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryCache()
	locker := lock.NewLocker(store, "instance-1")

	ctx := context.Background()
	elector := NewLeaseElector(locker, 300*time.Millisecond, "instance-1")

	elector.Start(ctx)
	defer elector.Stop(ctx)

	require.Eventually(t, elector.IsLeader, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, "instance-1", elector.ID())
}

func TestLeaseElectorOnlyOneLeaderAtATime(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryCache()
	locker1 := lock.NewLocker(store, "instance-1")
	locker2 := lock.NewLocker(store, "instance-2")

	ctx := context.Background()
	ttl := 300 * time.Millisecond

	elector1 := NewLeaseElector(locker1, ttl, "instance-1")
	elector2 := NewLeaseElector(locker2, ttl, "instance-2")

	elector1.Start(ctx)
	defer elector1.Stop(ctx)

	require.Eventually(t, elector1.IsLeader, 2*time.Second, 10*time.Millisecond)

	elector2.Start(ctx)
	defer elector2.Stop(ctx)

	// Give elector2 several campaign cycles to attempt and fail to acquire
	// leadership while elector1 keeps renewing.
	time.Sleep(ttl)
	require.True(t, elector1.IsLeader())
	require.False(t, elector2.IsLeader())
}

func TestLeaseElectorFailoverAfterLeaderStops(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryCache()
	locker1 := lock.NewLocker(store, "instance-1")
	locker2 := lock.NewLocker(store, "instance-2")

	ctx := context.Background()
	ttl := 300 * time.Millisecond

	elector1 := NewLeaseElector(locker1, ttl, "instance-1")
	elector2 := NewLeaseElector(locker2, ttl, "instance-2")

	elector1.Start(ctx)
	require.Eventually(t, elector1.IsLeader, 2*time.Second, 10*time.Millisecond)

	elector2.Start(ctx)
	defer elector2.Stop(ctx)

	// Stop should release leadership immediately, allowing elector2 to take over
	// well before the lease ttl would naturally expire.
	elector1.Stop(ctx)
	require.False(t, elector1.IsLeader())

	require.Eventually(t, elector2.IsLeader, 2*time.Second, 10*time.Millisecond)
}

func TestLeaseElectorStopWithoutStartIsNoop(t *testing.T) {
	t.Parallel()

	store := cache.NewMemoryCache()
	locker := lock.NewLocker(store, "instance-1")

	elector := NewLeaseElector(locker, 300*time.Millisecond, "instance-1")

	// Stop before Start must not block or panic.
	elector.Stop(context.Background())
	require.False(t, elector.IsLeader())
}
