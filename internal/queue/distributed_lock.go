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
	
	// Only release the lock if it's held by our instance
	currentHolder, err := r.cache.Get(ctx, lockKey)
	if err != nil {
		// Lock might have already expired, which is fine
		logger.S(ctx).Debugf("Lock %s not found when releasing (may have expired): %v", lockName, err)
		return nil
	}
	
	if currentHolder == r.instanceID {
		err = r.cache.Del(ctx, lockKey)
		if err != nil {
			logger.S(ctx).Errorf("Failed to release distributed lock %s: %v", lockName, err)
			return err
		}
		logger.S(ctx).Debugf("Released distributed lock %s by instance %s", lockName, r.instanceID)
	} else {
		logger.S(ctx).Debugf("Cannot release lock %s - held by different instance: %s", lockName, currentHolder)
	}
	
	return nil
}

// RefreshLock extends the TTL of an existing lock
func (r *RedisDistributedLock) RefreshLock(ctx context.Context, lockName string, ttl time.Duration) error {
	lockKey := fmt.Sprintf("housekeeper:lock:%s", lockName)
	
	// Only refresh if the lock is held by our instance
	currentHolder, err := r.cache.Get(ctx, lockKey)
	if err != nil {
		return fmt.Errorf("lock %s not found: %w", lockName, err)
	}
	
	if currentHolder != r.instanceID {
		return fmt.Errorf("cannot refresh lock %s - held by different instance: %s", lockName, currentHolder)
	}
	
	// Refresh the TTL
	err = r.cache.Expire(ctx, lockKey, ttl)
	if err != nil {
		logger.S(ctx).Errorf("Failed to refresh distributed lock %s: %v", lockName, err)
		return err
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