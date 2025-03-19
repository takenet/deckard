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

const DEL_MANY = "DeleteMany"
const FIND = "Find"

type MockDetails struct {
	args  []interface{}
	calls int
	err   error
}

type MockCollection struct {
	mockDetails map[string]*MockDetails
}

func newMockCollection() *MockCollection {
	return newMockCollectionErr(nil)
}
func newMockCollectionErr(err error) *MockCollection {
	return &MockCollection{
		mockDetails: map[string]*MockDetails{
			DEL_MANY: {args: []interface{}{},
				calls: 0,
				err:   err,
			},
			FIND: {args: []interface{}{},
				calls: 0,
				err:   err,
			},
		},
	}
}

func (col *MockCollection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	return nil, nil
}
func (col *MockCollection) BulkWrite(ctx context.Context, models []mongo.WriteModel,
	opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	return nil, nil
}
func (col *MockCollection) Distinct(ctx context.Context, fieldName string, filter interface{},
	opts ...*options.DistinctOptions) ([]interface{}, error) {
	return nil, nil
}
func (col *MockCollection) DeleteMany(ctx context.Context, filter interface{},
	opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	details := col.mockDetails[DEL_MANY]
	details.args = append(details.args, filter)
	details.calls++
	return &mongo.DeleteResult{
		DeletedCount: 1,
	}, details.err
}
func (col *MockCollection) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	return 0, nil
}
func (col *MockCollection) Find(ctx context.Context, filter interface{},
	opts ...*options.FindOptions) (cur *mongo.Cursor, err error) {
	details := col.mockDetails[FIND]
	details.args = append(details.args, filter, opts)
	details.calls++
	return mongo.NewCursorFromDocuments(make([]interface{}, 0), nil, nil)
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

	details := colMock.mockDetails[DEL_MANY]
	queue := "test_queue"
	count, err := storage.Remove(context.Background(), queue, "1", "2")
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	require.Equal(t, 2, details.calls, "should call delete many twice")
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
		}}, details.args)
}

func TestRemoveWithErrors(t *testing.T) {
	colMock := newMockCollectionErr(fmt.Errorf("Mocked error"))
	storage := &MongoStorage{
		messagesCollection: colMock,
	}

	details := colMock.mockDetails[DEL_MANY]
	queue := "test_queue"
	count, err := storage.Remove(context.Background(), queue, "1", "2")
	require.ErrorContains(t, err, "Mocked error")
	require.Equal(t, int64(0), count)
	require.Equal(t, 1, details.calls, "should call delete many once")
	require.Equal(t, []interface{}{bson.M{
		"queue": queue,
		"id": bson.M{
			"$in": []string{"1", "2"},
		},
	}}, details.args)
}

func TestFindWithLimitAndBatchVariations(t *testing.T) {
	lim := int64(1)
	batch := int32(1000)
	testFindBatch(t, &lim, &batch)
	lim = int64(2)
	batch = int32(2)
	testFindBatch(t, &lim, &batch)
	lim = int64(10_000)
	batch = int32(10_000)
	testFindBatch(t, &lim, &batch)
	lim = int64(10_001)
	batch = int32(10_000)
	testFindBatch(t, &lim, &batch)
}

func testFindBatch(t *testing.T, limit *int64, expectedBatch *int32) {
	colMock := newMockCollection()
	storage := &MongoStorage{
		messagesCollection: colMock,
	}
	messages, err := storage.Find(context.Background(), &FindOptions{
		Limit: *limit,
	})

	details := colMock.mockDetails[FIND]
	findOpt, ok := details.args[1].([]*options.FindOptions) // Type assertion
	if !ok {
		fmt.Println("Type assertion failed")
		return
	}

	require.NoError(t, err)
	require.Equal(t, 0, len(messages), "should return no messages")
	require.Equal(t, 1, details.calls, "should call find once")
	require.Equal(t, 1, len(findOpt), "only one find option was used")
	require.Equal(t, &options.FindOptions{
		Projection: &bson.M{},
		Sort:       &bson.D{},
		Limit:      limit,
		BatchSize:  expectedBatch,
	}, findOpt[0], fmt.Sprintf("should call with batch size = %d, lim = %d and empty projection and sort", *expectedBatch, *limit))
}
