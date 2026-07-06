package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/takenet/deckard/internal/logger"
)

// storeLocker implements Locker on top of a Store, identifying lock ownership
// by ownerID. It is the first (and currently only) Locker implementation,
// backed in practice by Redis via queue/cache.Cache, but it only depends on
// the generic Store interface.
type storeLocker struct {
	store   Store
	ownerID string
}

// NewLocker creates a Locker backed by the given Store, identifying this
// process/instance as ownerID.
func NewLocker(store Store, ownerID string) Locker {
	return &storeLocker{
		store:   store,
		ownerID: ownerID,
	}
}

func (l *storeLocker) TryAcquire(ctx context.Context, name string, ttl time.Duration) (bool, error) {
	acquired, err := l.store.SetNX(ctx, name, l.ownerID, ttl)
	if err != nil {
		logger.S(ctx).Errorf("Failed to acquire lock %s: %v", name, err)
		return false, err
	}

	if acquired {
		logger.S(ctx).Debugf("Acquired lock %s by owner %s", name, l.ownerID)
	} else {
		logger.S(ctx).Debugf("Failed to acquire lock %s (already held)", name)
	}

	return acquired, nil
}

func (l *storeLocker) Release(ctx context.Context, name string) error {
	// Atomically delete the lock only if it's still held by this owner. This avoids
	// a race where the lock expired and was re-acquired by another owner between a
	// read and a delete, which would otherwise release a lock we no longer own.
	released, err := l.store.CompareAndDelete(ctx, name, l.ownerID)
	if err != nil {
		logger.S(ctx).Errorf("Failed to release lock %s: %v", name, err)
		return err
	}

	if released {
		logger.S(ctx).Debugf("Released lock %s by owner %s", name, l.ownerID)
	} else {
		logger.S(ctx).Debugf("Cannot release lock %s - not held by this owner (may have expired or been re-acquired)", name)
	}

	return nil
}

func (l *storeLocker) Renew(ctx context.Context, name string, ttl time.Duration) error {
	// Atomically refresh the ttl only if the lock is still held by this owner, to
	// avoid extending a lock that expired and was re-acquired by another owner in
	// the meantime.
	renewed, err := l.store.CompareAndExpire(ctx, name, l.ownerID, ttl)
	if err != nil {
		logger.S(ctx).Errorf("Failed to renew lock %s: %v", name, err)
		return err
	}

	if !renewed {
		return fmt.Errorf("cannot renew lock %s - not held by this owner", name)
	}

	logger.S(ctx).Debugf("Renewed lock %s by owner %s", name, l.ownerID)
	return nil
}
