package lock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNoopLockerAlwaysSucceeds(t *testing.T) {
	t.Parallel()

	l := NewNoopLocker()
	ctx := context.Background()
	name := "test-lock"
	ttl := time.Second * 10

	acquired, err := l.TryAcquire(ctx, name, ttl)
	require.NoError(t, err)
	require.True(t, acquired)

	err = l.Renew(ctx, name, ttl)
	require.NoError(t, err)

	err = l.Release(ctx, name)
	require.NoError(t, err)
}
