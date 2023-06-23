package storage

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/messagepool/entities"
)

func TestMemoryStorage(t *testing.T) {
	config.Configure(true)

	storage := NewMemoryStorage(context.Background())

	suite.Run(t, &StorageTestSuite{
		storage: storage,
	})
}

func TestInternalIdIncrement(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage(context.Background())

	for i := 1; i < 10; i++ {
		message := &entities.Message{
			ID:         strconv.Itoa(i),
			Queue:      "q",
			ExpiryDate: time.Now().Add(10 * time.Hour),
		}

		_, _, _ = storage.Insert(context.Background(), message)

		require.Equal(t, int64(i), storage.docs[getKey(message)].InternalId)
		require.Equal(t, int64(i), storage.internalCounter)
	}
}
