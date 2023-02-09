package storage

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMongoStorageIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.LoadConfig()

	viper.Set(config.MONGO_ADDRESSES, "localhost:27017")
	viper.Set(config.MONGO_DATABASE, "unit_test")

	storage, err := NewMongoStorage(context.Background())

	require.NoError(t, err)

	suite.Run(t, &StorageTestSuite{
		storage: storage,
	})
}

func TestNewStorageWithoutServerShouldErrorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	viper.Set(config.MONGO_ADDRESSES, "localhost:41343")
	viper.Set(config.MONGO_DATABASE, "unit_test")

	_, err := NewMongoStorage(context.Background())

	require.Error(t, err)
}

func TestGetNilProjectionShouldReturnEmptyBson(t *testing.T) {
	t.Parallel()

	require.Equal(t, bson.M{}, *getMongoProjection(nil))
}

func TestGetEmptyProjectionShouldReturnEmptyBson(t *testing.T) {
	t.Parallel()

	require.Equal(t, bson.M{}, *getMongoProjection(&map[string]int{}))
}

func TestGetProjection(t *testing.T) {
	t.Parallel()

	require.Equal(t, bson.M{
		"a":   1,
		"b":   2,
		"c":   0,
		"etc": 1234,
	}, *getMongoProjection(&map[string]int{
		"a":   1,
		"b":   2,
		"c":   0,
		"etc": 1234,
	}))
}

func TestGetMongoMessageWithQueue(t *testing.T) {
	t.Parallel()

	message, err := getMongoMessage(&FindOptions{
		InternalFilter: &InternalFilter{
			Queue: "queue_test",
		},
	})

	require.NoError(t, err)
	require.Equal(
		t,
		bson.M{
			"queue": "queue_test",
		},
		message,
	)
}

func TestGetMongoMessageWithBreakpointGt(t *testing.T) {
	t.Parallel()

	objectId := primitive.NewObjectID()

	message, err := getMongoMessage(&FindOptions{
		InternalFilter: &InternalFilter{
			InternalIdBreakpointGt: objectId.Hex(),
		},
	})

	require.NoError(t, err)
	require.Equal(
		t,
		bson.M{
			"_id": bson.M{
				"$gt": objectId,
			},
		},
		message,
	)
}

func TestGetMongoMessageWithBreakpointLte(t *testing.T) {
	t.Parallel()

	objectId := primitive.NewObjectID()

	message, err := getMongoMessage(&FindOptions{
		InternalFilter: &InternalFilter{
			InternalIdBreakpointLte: objectId.Hex(),
		},
	})

	require.NoError(t, err)
	require.Equal(
		t,
		bson.M{
			"_id": bson.M{
				"$lte": objectId,
			},
		},
		message,
	)
}

func TestGetMongoMessageWithBreakpointGtAndLte(t *testing.T) {
	t.Parallel()

	objectId := primitive.NewObjectID()
	objectId2 := primitive.NewObjectID()

	message, err := getMongoMessage(&FindOptions{
		InternalFilter: &InternalFilter{
			InternalIdBreakpointLte: objectId.Hex(),
			InternalIdBreakpointGt:  objectId2.Hex(),
		},
	})

	require.NoError(t, err)
	require.Equal(
		t,
		bson.M{
			"_id": bson.M{
				"$lte": objectId,
				"$gt":  objectId2,
			},
		},
		message,
	)
}

func TestGetMongoMessageWithOneId(t *testing.T) {
	t.Parallel()

	message, err := getMongoMessage(&FindOptions{
		InternalFilter: &InternalFilter{
			Ids: &[]string{"oneId"},
		},
	})

	require.NoError(t, err)
	require.Equal(
		t,
		bson.M{
			"id": "oneId",
		},
		message,
	)
}

func TestGetMongoMessageWithManyIds(t *testing.T) {
	t.Parallel()

	message, err := getMongoMessage(&FindOptions{
		InternalFilter: &InternalFilter{
			Ids: &[]string{"oneId", "twoId"},
		},
	})

	require.NoError(t, err)
	require.Equal(
		t,
		bson.M{
			"id": bson.M{
				"$in": []string{"oneId", "twoId"},
			},
		},
		message,
	)
}
