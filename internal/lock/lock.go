// Package lock provides a generic distributed mutual-exclusion primitive.
//
// It is intentionally decoupled from any specific backend: Locker only depends
// on the minimal Store interface below, which is satisfied structurally by
// github.com/takenet/deckard/internal/queue/cache.Cache (no import needed on
// either side). This keeps the package reusable for any future backend
// (e.g. a Postgres advisory lock or an etcd/k8s Lease based Store) by simply
// implementing Store, without any change to Locker or its callers.
package lock

//go:generate mockgen -destination=../mocks/mock_lock.go -package=mocks -source=lock.go

import (
	"context"
	"time"
)

// Store is the minimal key-value contract a Locker needs from a backend.
// github.com/takenet/deckard/internal/queue/cache.Cache already satisfies this
// interface structurally.
//
// A Store implementation is responsible for applying its own key namespacing
// (e.g. the deployment's configured cache prefix). In production, Locker is
// backed by queue/cache.Cache, which prefixes every key it receives with the
// deployment's configured cache prefix (config.CachePrefix, "deckard_v1" by
// default) - so lock names such as "housekeeper:lock:unlock" automatically
// share the same Redis key prefix as every other key Deckard stores, without
// Locker needing any prefix-related logic of its own.
type Store interface {
	// SetNX sets key to value with the given ttl only if key does not already exist.
	// Returns true if the key was set (i.e. the lock was acquired).
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)

	// CompareAndDelete atomically deletes key only if its current value equals value.
	// Returns true if the key was deleted.
	CompareAndDelete(ctx context.Context, key string, value string) (bool, error)

	// CompareAndExpire atomically sets a new ttl on key only if its current value
	// equals value. Returns true if the ttl was refreshed.
	CompareAndExpire(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
}

// Locker is a generic distributed mutual-exclusion primitive: at most one
// owner can hold a given named lock at a time, across any number of processes
// sharing the same backing Store.
type Locker interface {
	// TryAcquire attempts to acquire the named lock for ttl.
	// Returns true if the lock was acquired, false if already held by another owner.
	TryAcquire(ctx context.Context, name string, ttl time.Duration) (bool, error)

	// Release releases the named lock, but only if still held by this Locker's owner.
	Release(ctx context.Context, name string) error

	// Renew extends the ttl of the named lock, but only if still held by this
	// Locker's owner. Returns an error if the lock is not (or no longer) held by
	// this owner.
	Renew(ctx context.Context, name string, ttl time.Duration) error
}
