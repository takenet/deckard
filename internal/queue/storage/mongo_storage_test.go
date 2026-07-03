package storage

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/queue/message"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type failingFindOptionsLister struct{}

func (failingFindOptionsLister) List() []func(*options.FindOptions) error {
	return []func(*options.FindOptions) error{
		func(*options.FindOptions) error {
			return fmt.Errorf("forced options application error")
		},
	}
}

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

func (col *MockCollection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	return nil, nil
}
func (col *MockCollection) BulkWrite(ctx context.Context, models []mongo.WriteModel,
	opts ...options.Lister[options.BulkWriteOptions]) (*mongo.BulkWriteResult, error) {
	return nil, nil
}
func (col *MockCollection) Distinct(ctx context.Context, fieldName string, filter interface{},
	opts ...options.Lister[options.DistinctOptions]) *mongo.DistinctResult {
	return nil
}
func (col *MockCollection) DeleteMany(ctx context.Context, filter interface{},
	opts ...options.Lister[options.DeleteManyOptions]) (*mongo.DeleteResult, error) {
	details := col.mockDetails[DEL_MANY]
	details.args = append(details.args, filter)
	details.calls++
	return &mongo.DeleteResult{
		DeletedCount: 1,
	}, details.err
}
func (col *MockCollection) CountDocuments(ctx context.Context, filter interface{}, opts ...options.Lister[options.CountOptions]) (int64, error) {
	return 0, nil
}
func (col *MockCollection) Find(ctx context.Context, filter interface{},
	opts ...options.Lister[options.FindOptions]) (cur *mongo.Cursor, err error) {
	details := col.mockDetails[FIND]
	details.args = append(details.args, filter, opts)
	details.calls++
	if details.err != nil {
		return nil, details.err
	}
	return mongo.NewCursorFromDocuments(make([]interface{}, 0), nil, nil)
}

func resolveFindOptions(lister options.Lister[options.FindOptions]) (*options.FindOptions, error) {
	fo := &options.FindOptions{}
	for _, fn := range lister.List() {
		if err := fn(fo); err != nil {
			return nil, err
		}
	}

	return fo, nil
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

	objectId := bson.NewObjectID()

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

	objectId := bson.NewObjectID()

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

	objectId := bson.NewObjectID()
	objectId2 := bson.NewObjectID()

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

func TestResolveFindOptionsShouldReturnErrorWhenApplyFails(t *testing.T) {
	t.Parallel()

	_, err := resolveFindOptions(failingFindOptionsLister{})

	require.ErrorContains(t, err, "forced options application error")
}

func testFindBatch(t *testing.T, limit *int64, expectedBatch *int32) {
	colMock := newMockCollection()
	storage := &MongoStorage{
		messagesCollection: colMock,
	}
	expectedQueue := "queue-test"
	expectedSort := orderedmap.NewOrderedMap[string, int]()
	expectedSort.Set("created_at", 1)
	expectedSort.Set("priority", -1)
	expectedProjection := map[string]int{"id": 1, "queue": 1, "_id": 0}
	expectedComment := "testFindBatch"
	messages, err := storage.Find(context.Background(), &FindOptions{
		Limit:        *limit,
		Comment:      expectedComment,
		Sort:         expectedSort,
		Projection:   &expectedProjection,
		InternalFilter: &InternalFilter{Queue: expectedQueue},
	})

	details := colMock.mockDetails[FIND]
	listerOpts, ok := details.args[1].([]options.Lister[options.FindOptions])
	if !ok {
		fmt.Println("Type assertion failed")
		return
	}

	require.NoError(t, err)
	require.Equal(t, 0, len(messages), "should return no messages")
	require.Equal(t, 1, details.calls, "should call find once")
	require.Equal(t, 1, len(listerOpts), "only one find option was used")

	// Resolve the builder to get concrete FindOptions for assertion
	fo, optionsErr := resolveFindOptions(listerOpts[0])
	require.NoError(t, optionsErr)
	require.Equal(t, bson.M{"queue": expectedQueue}, details.args[0], "find filter should match internal filter")
	require.Equal(t, limit, fo.Limit, fmt.Sprintf("expected limit = %d", *limit))
	require.Equal(t, expectedBatch, fo.BatchSize, fmt.Sprintf("expected batch size = %d", *expectedBatch))
	require.NotNil(t, fo.Projection, "projection should be set")
	switch projection := fo.Projection.(type) {
	case bson.M:
		require.Equal(t, bson.M{"id": 1, "queue": 1, "_id": 0}, projection, "projection passthrough should be preserved")
	case *bson.M:
		require.Equal(t, bson.M{"id": 1, "queue": 1, "_id": 0}, *projection, "projection passthrough should be preserved")
	default:
		require.Failf(t, "unexpected projection type", "got %T", fo.Projection)
	}
	require.NotNil(t, fo.Sort, "sort should be set")
	switch sort := fo.Sort.(type) {
	case bson.D:
		require.Equal(t, bson.D{{Key: "created_at", Value: 1}, {Key: "priority", Value: -1}}, sort, "sort passthrough should be preserved")
	case *bson.D:
		require.Equal(t, bson.D{{Key: "created_at", Value: 1}, {Key: "priority", Value: -1}}, *sort, "sort passthrough should be preserved")
	default:
		require.Failf(t, "unexpected sort type", "got %T", fo.Sort)
	}
	require.NotNil(t, fo.Comment, "comment should be set")
	switch comment := fo.Comment.(type) {
	case string:
		require.Equal(t, expectedComment, comment, "comment passthrough should be preserved")
	case *interface{}:
		require.NotNil(t, comment, "comment interface pointer should not be nil")
		value, ok := (*comment).(string)
		require.True(t, ok, "comment should contain string value")
		require.Equal(t, expectedComment, value, "comment passthrough should be preserved")
	default:
		require.Failf(t, "unexpected comment type", "got %T", fo.Comment)
	}
}
