package storage

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/queue/message"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MockCollection struct {
	deleteManyArgs  []interface{}
	deleteManyCalls int
	errorDeleteMany error
}

func newMockCollection() *MockCollection {
	return newMockCollectionErr(nil)
}
func newMockCollectionErr(err error) *MockCollection {
	return &MockCollection{
		deleteManyArgs:  []interface{}{},
		deleteManyCalls: 0,
		errorDeleteMany: err,
	}
}

func (this *MockCollection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	return nil, nil
}
func (this *MockCollection) BulkWrite(ctx context.Context, models []mongo.WriteModel,
	opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	return nil, nil
}
func (this *MockCollection) Distinct(ctx context.Context, fieldName string, filter interface{},
	opts ...*options.DistinctOptions) ([]interface{}, error) {
	return nil, nil
}
func (this *MockCollection) DeleteMany(ctx context.Context, filter interface{},
	opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	this.deleteManyArgs = append(this.deleteManyArgs, filter)
	this.deleteManyCalls++
	return &mongo.DeleteResult{
		DeletedCount: 1,
	}, this.errorDeleteMany
}
func (this *MockCollection) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	return 0, nil
}
func (this *MockCollection) Find(ctx context.Context, filter interface{},
	opts ...*options.FindOptions) (cur *mongo.Cursor, err error) {
	return nil, nil
}

func TestMongoStorageIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.MongoDatabase.Set("unit_test")

	storage, err := NewMongoStorage(context.Background())

	require.NoError(t, err)

	suite.Run(t, &StorageTestSuite{
		storage: storage,
	})
}

func TestMongoConnectionWithURIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	os.Setenv("DECKARD_MONGO_URI", "mongodb://localhost:27017")
	os.Setenv("DECKARD_MONGO_ADDRESSES", "none")
	os.Setenv("DECKARD_MONGO_PASSWORD", "none")

	defer os.Unsetenv("DECKARD_MONGO_URI")
	defer os.Unsetenv("DECKARD_MONGO_ADDRESSES")
	defer os.Unsetenv("DECKARD_MONGO_PASSWORD")

	config.Configure(true)

	storage, err := NewMongoStorage(context.Background())
	require.NoError(t, err)

	defer storage.Flush(context.Background())

	insert, updated, err := storage.Insert(context.Background(), &message.Message{
		ID:    "123",
		Queue: "queue",
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), insert)
	require.Equal(t, int64(0), updated)
}

func TestNewStorageWithoutServerShouldErrorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	defer viper.Reset()
	config.StorageConnectionRetryEnabled.Set(false)
	config.StorageUri.Set("mongodb://localhost:41343/unit_test?connectTimeoutMS=200&socketTimeoutMS=200")

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

func TestRemove(t *testing.T) {
	deleteChunkSize = 1
	defer func() {
		deleteChunkSize = 100
	}()

	colMock := newMockCollection()
	storage := &MongoStorage{
		messagesCollection: colMock,
	}

	queue := "test_queue"
	count, err := storage.Remove(context.Background(), queue, "1", "2")
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	require.Equal(t, 2, colMock.deleteManyCalls)
	require.Equal(t, []interface{}{bson.M{
		"queue": queue,
		"id": bson.M{
			"$in": []string{"1"},
		},
	},
		bson.M{
			"queue": queue,
			"id": bson.M{
				"$in": []string{"2"},
			},
		}}, colMock.deleteManyArgs)
}

func TestRemoveErrors(t *testing.T) {
	colMock := newMockCollectionErr(fmt.Errorf("Mocked error"))
	storage := &MongoStorage{
		messagesCollection: colMock,
	}

	queue := "test_queue"
	count, err := storage.Remove(context.Background(), queue, "1", "2")
	require.ErrorContains(t, err, "Mocked error")
	require.Equal(t, int64(0), count)
	require.Equal(t, 1, colMock.deleteManyCalls)
	require.Equal(t, []interface{}{bson.M{
		"queue": queue,
		"id": bson.M{
			"$in": []string{"1", "2"},
		},
	}}, colMock.deleteManyArgs)
}
