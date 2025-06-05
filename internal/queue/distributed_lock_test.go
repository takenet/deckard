package queue

import (
	"context"
	"testing"
	"time"

	"github.com/takenet/deckard/internal/queue/cache"
)

func TestRedisDistributedLock(t *testing.T) {
	// Create a memory cache for testing
	memCache := cache.NewMemoryCache()
	
	instanceID1 := "instance-1"
	instanceID2 := "instance-2"
	
	lock1 := NewRedisDistributedLock(memCache, instanceID1)
	lock2 := NewRedisDistributedLock(memCache, instanceID2)
	
	ctx := context.Background()
	lockName := "test-lock"
	ttl := time.Second * 10
	
	// Test 1: First instance should acquire lock
	acquired, err := lock1.TryLock(ctx, lockName, ttl)
	if err != nil {
		t.Fatalf("Error acquiring lock: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire lock but failed")
	}
	
	// Test 2: Second instance should fail to acquire same lock
	acquired, err = lock2.TryLock(ctx, lockName, ttl)
	if err != nil {
		t.Fatalf("Error trying to acquire lock: %v", err)
	}
	if acquired {
		t.Fatal("Expected to fail acquiring lock but succeeded")
	}
	
	// Test 3: First instance should be able to release lock
	err = lock1.ReleaseLock(ctx, lockName)
	if err != nil {
		t.Fatalf("Error releasing lock: %v", err)
	}
	
	// Test 4: Second instance should now be able to acquire lock
	acquired, err = lock2.TryLock(ctx, lockName, ttl)
	if err != nil {
		t.Fatalf("Error acquiring lock after release: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire lock after release but failed")
	}
	
	// Test 5: Second instance should be able to refresh the lock
	err = lock2.RefreshLock(ctx, lockName, ttl)
	if err != nil {
		t.Fatalf("Error refreshing lock: %v", err)
	}
	
	// Cleanup
	err = lock2.ReleaseLock(ctx, lockName)
	if err != nil {
		t.Fatalf("Error releasing lock in cleanup: %v", err)
	}
}

func TestNoOpDistributedLock(t *testing.T) {
	lock := NewNoOpDistributedLock()
	ctx := context.Background()
	lockName := "test-lock"
	ttl := time.Second * 10
	
	// NoOp lock should always succeed
	acquired, err := lock.TryLock(ctx, lockName, ttl)
	if err != nil {
		t.Fatalf("Error with NoOp lock: %v", err)
	}
	if !acquired {
		t.Fatal("NoOp lock should always succeed")
	}
	
	// Release should not error
	err = lock.ReleaseLock(ctx, lockName)
	if err != nil {
		t.Fatalf("Error releasing NoOp lock: %v", err)
	}
	
	// Refresh should not error
	err = lock.RefreshLock(ctx, lockName, ttl)
	if err != nil {
		t.Fatalf("Error refreshing NoOp lock: %v", err)
	}
}