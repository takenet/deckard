package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/queue/cache"
)

// DistributedLock represents a distributed locking mechanism for coordinating
// tasks across multiple housekeeper instances
type DistributedLock interface {
	// TryLock attempts to acquire a distributed lock with the given name
	// Returns true if lock was acquired, false if already held by another instance
	TryLock(ctx context.Context, lockName string, ttl time.Duration) (bool, error)

	// ReleaseLock releases a distributed lock with the given name
	ReleaseLock(ctx context.Context, lockName string) error

	// RefreshLock extends the TTL of an existing lock
	RefreshLock(ctx context.Context, lockName string, ttl time.Duration) error
}

// RedisDistributedLock implements DistributedLock using Redis
type RedisDistributedLock struct {
	cache      cache.Cache
	instanceID string
}

// NewRedisDistributedLock creates a new Redis-based distributed lock
func NewRedisDistributedLock(cache cache.Cache, instanceID string) DistributedLock {
	return &RedisDistributedLock{
		cache:      cache,
		instanceID: instanceID,
	}
}

// TryLock attempts to acquire a distributed lock
func (r *RedisDistributedLock) TryLock(ctx context.Context, lockName string, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("housekeeper:lock:%s", lockName)

	// Try to set the lock key with our instance ID and TTL
	// Use SET with NX (only if not exists) and EX (expiration)
	acquired, err := r.cache.SetNX(ctx, lockKey, r.instanceID, ttl)
	if err != nil {
		logger.S(ctx).Errorf("Failed to acquire distributed lock %s: %v", lockName, err)
		return false, err
	}

	if acquired {
		logger.S(ctx).Debugf("Acquired distributed lock %s by instance %s", lockName, r.instanceID)
	} else {
		logger.S(ctx).Debugf("Failed to acquire distributed lock %s (already held)", lockName)
	}

	return acquired, nil
}

// ReleaseLock releases a distributed lock
func (r *RedisDistributedLock) ReleaseLock(ctx context.Context, lockName string) error {
	lockKey := fmt.Sprintf("housekeeper:lock:%s", lockName)

	// Atomically delete the lock only if it's still held by our instance. This avoids a race
	// where the lock expired and was re-acquired by another instance between a read and a delete,
	// which would otherwise release a lock we no longer own.
	released, err := r.cache.CompareAndDelete(ctx, lockKey, r.instanceID)
	if err != nil {
		logger.S(ctx).Errorf("Failed to release distributed lock %s: %v", lockName, err)
		return err
	}

	if released {
		logger.S(ctx).Debugf("Released distributed lock %s by instance %s", lockName, r.instanceID)
	} else {
		logger.S(ctx).Debugf("Cannot release lock %s - not held by this instance (may have expired or been re-acquired)", lockName)
	}

	return nil
}

// RefreshLock extends the TTL of an existing lock
func (r *RedisDistributedLock) RefreshLock(ctx context.Context, lockName string, ttl time.Duration) error {
	lockKey := fmt.Sprintf("housekeeper:lock:%s", lockName)

	// Atomically refresh the TTL only if the lock is still held by our instance, to avoid
	// extending a lock that expired and was re-acquired by another instance in the meantime.
	refreshed, err := r.cache.CompareAndExpire(ctx, lockKey, r.instanceID, ttl)
	if err != nil {
		logger.S(ctx).Errorf("Failed to refresh distributed lock %s: %v", lockName, err)
		return err
	}

	if !refreshed {
		return fmt.Errorf("cannot refresh lock %s - not held by this instance", lockName)
	}

	logger.S(ctx).Debugf("Refreshed distributed lock %s by instance %s", lockName, r.instanceID)
	return nil
}

// NoOpDistributedLock is a no-op implementation for single-instance deployments
type NoOpDistributedLock struct{}

// NewNoOpDistributedLock creates a no-op distributed lock for backward compatibility
func NewNoOpDistributedLock() DistributedLock {
	return &NoOpDistributedLock{}
}

func (n *NoOpDistributedLock) TryLock(ctx context.Context, lockName string, ttl time.Duration) (bool, error) {
	// Always succeed in single-instance mode
	return true, nil
}

func (n *NoOpDistributedLock) ReleaseLock(ctx context.Context, lockName string) error {
	// No-op in single-instance mode
	return nil
}

func (n *NoOpDistributedLock) RefreshLock(ctx context.Context, lockName string, ttl time.Duration) error {
	// No-op in single-instance mode
	return nil
}
