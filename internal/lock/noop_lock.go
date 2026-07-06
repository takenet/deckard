package lock

import (
	"context"
	"time"
)

// noopLocker is a Locker implementation that always succeeds. It is used for
// single-instance deployments (distributed execution disabled), preserving the
// original non-distributed behavior with zero backend calls.
type noopLocker struct{}

// NewNoopLocker creates a Locker that always succeeds, for backward
// compatibility with single-instance deployments.
func NewNoopLocker() Locker {
	return &noopLocker{}
}

func (n *noopLocker) TryAcquire(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (n *noopLocker) Release(_ context.Context, _ string) error {
	return nil
}

func (n *noopLocker) Renew(_ context.Context, _ string, _ time.Duration) error {
	return nil
}
