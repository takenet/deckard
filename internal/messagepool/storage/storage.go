package storage

//go:generate mockgen -destination=../../mocks/mock_storage.go -package=mocks -source=storage.go

import (
	"context"
	"errors"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/messagepool/entities"
)

type Type string

const (
	MONGODB Type = "MONGODB"
	MEMORY  Type = "MEMORY"
)

// Storage is an interface that stores the messages that have to be routed.
// It contains all Data of the message and is used as a storage only.
type Storage interface {
	Insert(ctx context.Context, messages ...*entities.Message) (inserted int64, updated int64, err error)

	Find(ctx context.Context, opt *FindOptions) ([]entities.Message, error)
	Remove(ctx context.Context, queue string, ids ...string) (deleted int64, err error)
	Ack(ctx context.Context, message *entities.Message) (modifiedCount int64, err error)

	ListQueueNames(ctx context.Context) (queues []string, err error)
	ListQueuePrefixes(ctx context.Context) (queues []string, err error)
	Count(ctx context.Context, opt *FindOptions) (int64, error)

	GetStringInternalId(ctx context.Context, message *entities.Message) string

	EditQueueConfiguration(ctx context.Context, configuration *entities.QueueConfiguration) error
	GetQueueConfiguration(ctx context.Context, queue string) (*entities.QueueConfiguration, error)
	ListQueueConfigurations(ctx context.Context) ([]*entities.QueueConfiguration, error)

	// Available to cleanup tests
	Flush(ctx context.Context) (deletedCount int64, err error)

	// Close connection to the storage
	Close(ctx context.Context) error
}

type InternalFilter struct {
	InternalIdBreakpointGt  string
	InternalIdBreakpointLte string
	Ids                     *[]string
	Queue                   string
	QueuePrefix             string

	// Used to manage TTL cleaner. It will always search for elements with expire date bigger than the value provided in this attribute.
	ExpiryDate *time.Time
}

type FindOptions struct {
	Projection *map[string]int
	Sort       *orderedmap.OrderedMap[string, int]
	Limit      int64
	*InternalFilter

	// Boolean to deal with storage retries when storage reports missing elements
	Retry bool
}

func CreateStorage(ctx context.Context, storageType Type) (Storage, error) {
	switch storageType {
	case MONGODB:
		return NewMongoStorage(ctx)

	case MEMORY:
		return NewMemoryStorage(ctx), nil
	}

	logger.S(ctx).Errorf("Invalid storage type: %s", storageType)

	return nil, errors.New("invalid storage type ")
}
