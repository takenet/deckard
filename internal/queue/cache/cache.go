package cache

//go:generate mockgen -destination=../../mocks/mock_cache.go -package=mocks -source=cache.go

import (
	"context"
	"errors"
	"time"

	"github.com/takenet/deckard/internal/queue/message"
	"github.com/takenet/deckard/internal/queue/pool"
)

type Type string
type LockType string

const (
	REDIS  Type = "REDIS"
	MEMORY Type = "MEMORY"

	LOCK_ACK  LockType = "lock_ack"
	LOCK_NACK LockType = "lock_nack"

	RECOVERY_RUNNING                = "recovery"
	RECOVERY_FINISHED               = "recovery_finished"
	RECOVERY_STORAGE_BREAKPOINT_KEY = "recovery_storage_breakpoint"
	RECOVERY_BREAKPOINT_KEY         = "recovery_breakpoint"
)

var (
	DefaultTimeoutMs = time.Duration(5 * time.Minute).Milliseconds()
)

type Cache interface {
	MakeAvailable(ctx context.Context, message *message.Message) (bool, error)
	IsProcessing(ctx context.Context, queue string, id string) (bool, error)
	PullMessages(ctx context.Context, queue string, n int64, minScore *float64, maxScore *float64, ackDeadlineMs int64) (ids []string, err error)
	TimeoutMessages(ctx context.Context, queue string) (ids []string, err error)

	// Locks a message for message.LockMs milliseconds.
	LockMessage(ctx context.Context, message *message.Message, lockType LockType) (bool, error)

	// Unlocks all messages from a queue
	UnlockMessages(ctx context.Context, queue string, lockType LockType) (messages []string, err error)

	// Lists all queues from a pool using a pattern search. Only glob-style pattern is supported.
	ListQueues(ctx context.Context, pattern string, poolType pool.PoolType) (queues []string, err error)

	// Inserts 1..n elements to cache and return the number of new elements.
	// Elements already in cache should have its score updated.
	Insert(ctx context.Context, queue string, messages ...*message.Message) (insertions []string, err error)
	Remove(ctx context.Context, queue string, ids ...string) (removed int64, err error)
	Flush(ctx context.Context)

	// Returns the values of a specified key. Returns an empty string if the key is not set.
	Get(ctx context.Context, key string) (value string, err error)
	// Sets the given key to its respective value. Use empty string to unset cache element.
	Set(ctx context.Context, key string, value string) error

	// SetNX sets a key only if it doesn't exist, with a TTL
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	// Del deletes a key
	Del(ctx context.Context, key string) error
	// Expire sets a TTL on an existing key
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// Close connection to the cache
	Close(ctx context.Context) error
}

func CreateCache(ctx context.Context, cacheType Type) (Cache, error) {
	switch cacheType {
	case REDIS:
		return NewRedisCache(ctx)

	case MEMORY:
		return NewMemoryCache(), nil
	}

	return nil, errors.New("invalid cache type")
}
