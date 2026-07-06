package storage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/dtime"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/metrics"
	"github.com/takenet/deckard/internal/project"
	"github.com/takenet/deckard/internal/queue/configuration"
	"github.com/takenet/deckard/internal/queue/message"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var errStorageUriRequired = errors.New("storage.uri (DECKARD_STORAGE_URI) is required when storage.type is MONGODB")

type MongoCollectionInterface interface {
	UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error)
	BulkWrite(ctx context.Context, models []mongo.WriteModel,
		opts ...options.Lister[options.BulkWriteOptions]) (*mongo.BulkWriteResult, error)
	Distinct(ctx context.Context, fieldName string, filter interface{},
		opts ...options.Lister[options.DistinctOptions]) *mongo.DistinctResult
	DeleteMany(ctx context.Context, filter interface{},
		opts ...options.Lister[options.DeleteManyOptions]) (*mongo.DeleteResult, error)
	CountDocuments(ctx context.Context, filter interface{}, opts ...options.Lister[options.CountOptions]) (int64, error)
	Find(ctx context.Context, filter interface{},
		opts ...options.Lister[options.FindOptions]) (cur *mongo.Cursor, err error)
}

// MongoStorage is an implementation of the Storage Interface using MongoDB.
type MongoStorage struct {
	client                        *mongo.Client
	clientPrimaryPreference       *mongo.Client
	messagesCollection            MongoCollectionInterface
	messagesCollectionPrimaryRead *mongo.Collection
	queueConfigurationCollection  *mongo.Collection
}

var _ Storage = &MongoStorage{}
var deleteChunkSize = 100

func NewMongoStorage(ctx context.Context) (*MongoStorage, error) {
	mongoSecondaryOpts, err := createOptions()
	if err != nil {
		return nil, err
	}

	logger.S(ctx).Info("Connecting to ", mongoSecondaryOpts.Hosts, " MongoDB instance(s).")

	start := dtime.Now()

	mongoSecondaryOpts.SetReadPreference(readpref.SecondaryPreferred())
	clientSecondaryPreference, err := waitForClient(ctx, mongoSecondaryOpts)
	if err != nil {
		return nil, err
	}

	mongoPrimaryOptions, err := createOptions()
	if err != nil {
		return nil, err
	}

	mongoPrimaryOptions.SetReadPreference(readpref.PrimaryPreferred())
	clientPrimaryPreference, err := waitForClient(ctx, mongoPrimaryOptions)
	if err != nil {
		return nil, err
	}

	logger.S(ctx).Debug("Connected to MongoDB storage in ", time.Since(start))

	database := config.MongoDatabase.Get()
	queueCollection := config.MongoCollection.Get()
	queueConfigurationCollection := config.MongoQueueConfigurationCollection.Get()

	return &MongoStorage{
		client:                        clientSecondaryPreference,
		clientPrimaryPreference:       clientPrimaryPreference,
		messagesCollection:            clientSecondaryPreference.Database(database).Collection(queueCollection),
		messagesCollectionPrimaryRead: clientPrimaryPreference.Database(database).Collection(queueCollection),
		queueConfigurationCollection:  clientSecondaryPreference.Database(database).Collection(queueConfigurationCollection),
	}, nil
}

func createOptions() (*options.ClientOptions, error) {
	uri := config.StorageUri.Get()
	if uri == "" {
		return nil, errStorageUriRequired
	}

	mongoOpts := options.Client()
	mongoOpts.SetAppName(project.Name)

	// OpenTelemetry APM
	mongoOpts.SetMonitor(otelmongo.NewMonitor())

	mongoOpts.ApplyURI(uri)

	// Local/CI MongoDB instances typically fail fast; a short server selection timeout avoids
	// waiting out the full default (30s) per retry attempt against a host that's simply not up yet.
	if strings.Contains(uri, "localhost") {
		duration := time.Second
		mongoOpts.ServerSelectionTimeout = &duration
	}

	return mongoOpts, nil
}

func waitForClient(ctx context.Context, opts *options.ClientOptions) (*mongo.Client, error) {
	attempts := config.StorageConnectionRetryAttempts.GetInt()
	retryEnabled := config.StorageConnectionRetryEnabled.GetBool()
	retryDelay := config.StorageConnectionRetryDelay.GetDuration()

	if attempts < 1 {
		attempts = 1
	}

	var err error
	var client *mongo.Client

	for i := 1; i <= attempts; i++ {
		attemptCtx := ctx
		var cancel context.CancelFunc
		if opts.ConnectTimeout != nil {
			attemptCtx, cancel = context.WithTimeout(ctx, *opts.ConnectTimeout)
		}

		client, err = createClient(attemptCtx, opts)
		if cancel != nil {
			cancel()
		}

		if err == nil {
			return client, nil
		}

		if !retryEnabled || i == attempts {
			break
		}

		logger.S(ctx).Warnf("Failed to connect to MongoDB (%d/%d attempts). Trying again in %s.", i, attempts, retryDelay)

		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, fmt.Errorf("mongo connection retries canceled: %w", ctx.Err())
		case <-timer.C:
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB after %d attempt(s): %w", attempts, err)
	}

	return client, nil
}

func createClient(ctx context.Context, opts *options.ClientOptions) (*mongo.Client, error) {
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("error connecting to client: %w", err)
	}

	err = client.Ping(ctx, readpref.SecondaryPreferred())
	if err != nil {
		return nil, fmt.Errorf("error pinging client: %w", err)
	}

	return client, nil
}

func (storage *MongoStorage) EditQueueConfiguration(ctx context.Context, configuration *configuration.QueueConfiguration) error {
	set := bson.M{}

	maxElements := configuration.MaxElements
	if maxElements != 0 {
		if maxElements < 0 {
			maxElements = 0
		}

		set["max_elements"] = maxElements
	}

	if len(set) == 0 {
		return nil
	}

	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "edit_configuration")))
	}()

	_, updateErr := storage.queueConfigurationCollection.UpdateOne(
		ctx,
		bson.M{
			"_id": configuration.Queue,
		},
		bson.M{
			"$set": set,
		},
		options.UpdateOne().SetUpsert(true),
	)

	return updateErr
}

func (storage *MongoStorage) ListQueueConfigurations(ctx context.Context) ([]*configuration.QueueConfiguration, error) {
	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "list_configuration")))
	}()

	cursor, err := storage.queueConfigurationCollection.Find(ctx, bson.M{})

	if err != nil {
		return nil, fmt.Errorf("error finding queue configurations: %w", err)
	}

	configurations := make([]*configuration.QueueConfiguration, 0)

	cursorErr := cursor.All(ctx, &configurations)

	if cursorErr != nil {
		return nil, fmt.Errorf("error to fetch cursor: %w", cursorErr)
	}

	return configurations, nil
}

func (storage *MongoStorage) GetQueueConfiguration(ctx context.Context, queue string) (*configuration.QueueConfiguration, error) {
	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "find_configuration")))
	}()

	var configuration configuration.QueueConfiguration

	err := storage.queueConfigurationCollection.FindOne(
		ctx,
		bson.M{
			"_id": queue,
		},
	).Decode(&configuration)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}

		return nil, fmt.Errorf("error getting queue configuration: %w", err)
	}

	return &configuration, nil
}

func (storage *MongoStorage) Flush(ctx context.Context) (int64, error) {
	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "flush")))
	}()

	result, err := storage.messagesCollection.DeleteMany(ctx, bson.M{})

	if err != nil || result == nil {
		return 0, fmt.Errorf("error deleting storage elements: %w", err)
	}

	deletedMessages := result.DeletedCount

	result, err = storage.queueConfigurationCollection.DeleteMany(ctx, bson.M{})

	if err != nil || result == nil {
		return 0, fmt.Errorf("error deleting queue configurations on storage: %w", err)
	}

	return result.DeletedCount + deletedMessages, nil
}

func (storage *MongoStorage) Count(ctx context.Context, findOpt *FindOptions, countOpt *CountOptions) (int64, error) {
	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "count")))
	}()

	mongoFilter, err := getMongoMessage(findOpt)

	if err != nil {
		return 0, err
	}

	logger.S(ctx).Debugw("Storage operation: count operation.", "filter", mongoFilter)

	result, err := storage.messagesCollection.CountDocuments(ctx, mongoFilter, options.Count().SetComment(countOpt.Comment))

	if err != nil {
		return 0, fmt.Errorf("error counting elements in storage: %w", err)
	}

	return result, nil
}

func (storage *MongoStorage) ListQueueNames(ctx context.Context) (queues []string, err error) {
	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "list_queue")))
	}()

	return storage.distinct(ctx, "queue")
}

func (storage *MongoStorage) ListQueuePrefixes(ctx context.Context) (queues []string, err error) {
	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "list_queue_prefix")))
	}()

	return storage.distinct(ctx, "queue_prefix")
}

func (storage *MongoStorage) distinct(ctx context.Context, field string) (data []string, err error) {
	filter := bson.M{}

	logger.S(ctx).Debugw(fmt.Sprintf("Storage operation: distinct for field '%s'.", field), "filter", filter)

	distinctResult := storage.messagesCollection.Distinct(ctx, field, filter)

	if err := distinctResult.Decode(&data); err != nil {
		return nil, fmt.Errorf("error to fetch distinct elements from storage: %w", err)
	}

	return data, nil
}

// Find returns a cursor with the specified projection for fetching
// all valid messages sorted by its ascending insertion date.
func (storage *MongoStorage) Find(ctx context.Context, opt *FindOptions) ([]message.Message, error) {
	if opt == nil {
		opt = &FindOptions{}
	}

	mongoFilter, err := getMongoMessage(opt)

	if err != nil {
		return nil, err
	}

	mongoSort := getMongoSort(opt.Sort)
	mongoProjection := getMongoProjection(opt.Projection)

	batchSize := int32(opt.Limit)
	if batchSize <= 1 {
		batchSize = 1_000
	} else if batchSize > 10_000 {
		batchSize = 10_000
	}

	findOptions := options.Find().
		SetProjection(mongoProjection).
		SetSort(mongoSort).
		SetLimit(opt.Limit).
		SetBatchSize(batchSize).
		SetComment(opt.Comment)

	logger.S(ctx).Debugw("Storage operation: find operation.",
		"filter", mongoFilter,
		"sort", mongoSort,
		"projection", mongoProjection)

	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "find"), attribute.String("retry", strconv.FormatBool(opt.Retry))))
	}()

	collection := storage.messagesCollection
	if opt.Retry {
		collection = storage.messagesCollectionPrimaryRead
	}

	cursor, err := collection.Find(ctx, mongoFilter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("error finding storage elements: %w", err)
	}

	messages := make([]message.Message, 0, opt.Limit)

	cursorErr := cursor.All(ctx, &messages)

	if cursorErr != nil {
		return nil, fmt.Errorf("error to fetch cursor: %w", cursorErr)
	}

	return messages, nil
}

func (storage *MongoStorage) Remove(ctx context.Context, queue string, ids ...string) (deleted int64, err error) {
	if len(ids) == 0 {
		return 0, nil
	}

	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "remove")))
	}()

	for i := 0; i < len(ids); i += deleteChunkSize {
		res, err := attemptChunkDeletion(ctx, i, ids, queue, storage)
		if err != nil {
			return deleted, fmt.Errorf("error deleting storage elements: %w, ids = %v", err, ids)
		}

		deleted += res.DeletedCount
	}

	return deleted, nil
}

func attemptChunkDeletion(ctx context.Context, i int, ids []string, queue string, storage *MongoStorage) (*mongo.DeleteResult, error) {
	end := i + deleteChunkSize
	if end > len(ids) {
		end = len(ids)
	}
	chunk := ids[i:end]

	filter := bson.M{
		"queue": queue,
		"id": bson.M{
			"$in": chunk,
		},
	}

	logger.S(ctx).Debugw("Storage operation: delete many operation.", "filter", filter)

	res, err := storage.messagesCollection.DeleteMany(ctx, filter)
	return res, err
}

func (storage *MongoStorage) Insert(ctx context.Context, messages ...*message.Message) (insertedCount int64, modifiedCount int64, err error) {
	updates := make([]mongo.WriteModel, 0, len(messages))

	now := dtime.Now()

	upsert := true
	for _, q := range messages {
		if q.Queue == "" {
			return 0, 0, errors.New("message has a invalid queue")
		}

		if q.ID == "" {
			return 0, 0, errors.New("message has a invalid ID")
		}

		setOnInsert := bson.M{}
		setOnInsert["last_usage"] = now
		setOnInsert["score"] = q.Score

		setFields := bson.M{}

		setFields["expiry_date"] = q.ExpiryDate

		if q.Description != "" {
			setFields["description"] = q.Description
		}

		if q.Metadata != nil {
			setFields["metadata"] = q.Metadata
		}

		if q.Payload != nil {
			setFields["payload"] = q.Payload
		}

		if q.StringPayload != "" {
			setFields["string_payload"] = q.StringPayload
		}

		setFields["queue_prefix"] = q.QueuePrefix

		if q.QueueSuffix != "" {
			setFields["queue_suffix"] = q.QueueSuffix
		}

		updates = append(updates, &mongo.UpdateOneModel{
			Upsert: &upsert,
			Filter: bson.M{
				"id":    q.ID,
				"queue": q.Queue,
			},
			Update: bson.M{
				"$set":         setFields,
				"$setOnInsert": setOnInsert,
			},
		})
	}

	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "insert")))
	}()

	res, err := storage.messagesCollection.BulkWrite(ctx, updates, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, 0, fmt.Errorf("error writing to mongodb storage: %w", err)
	}

	return res.InsertedCount + res.UpsertedCount, res.ModifiedCount, nil
}

// Ack updates the messages on mongostorage with updated status, score and diagnostic information.
func (storage *MongoStorage) Ack(ctx context.Context, message *message.Message) (modifiedCount int64, err error) {
	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "ack")))
	}()

	filter := bson.M{
		"id":    message.ID,
		"queue": message.Queue,
	}

	update := bson.M{
		"$set": bson.M{
			"last_usage":                    message.LastUsage,
			"last_score_subtract":           message.LastScoreSubtract,
			"breakpoint":                    message.Breakpoint,
			"score":                         message.Score,
			"lock_ms":                       message.LockMs,
			"locked_until":                  message.LockedUntil,
			"diagnostics.consecutive_nacks": 0,
		},
		"$inc": bson.M{
			"usage_count":                  1,
			"total_score_subtract":         message.LastScoreSubtract,
			"diagnostics.acks":             1,
			"diagnostics.consecutive_acks": 1,
		},
	}

	logger.S(ctx).Debugw("Storage operation: update one operation.", "filter", filter, "update", update)

	res, err := storage.messagesCollection.UpdateOne(ctx, filter, update)

	if err != nil {
		return 0, fmt.Errorf("error updating storage element: %w", err)
	}

	return res.ModifiedCount, nil
}

// Nack updates the messages on mongostorage with updated status, score and diagnostic information.
func (storage *MongoStorage) Nack(ctx context.Context, message *message.Message) (modifiedCount int64, err error) {
	now := dtime.Now()
	defer func() {
		metrics.StorageLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "nack")))
	}()

	filter := bson.M{
		"id":    message.ID,
		"queue": message.Queue,
	}

	update := bson.M{
		"$set": bson.M{
			"score":                        message.Score,
			"lock_ms":                      message.LockMs,
			"locked_until":                 message.LockedUntil,
			"diagnostics.consecutive_acks": 0,
		},
		"$inc": bson.M{
			"diagnostics.nacks":             1,
			"diagnostics.consecutive_nacks": 1,
		},
	}

	logger.S(ctx).Debugw("Storage operation: update one operation.", "filter", filter, "update", update)

	res, err := storage.messagesCollection.UpdateOne(ctx, filter, update)

	if err != nil {
		return 0, fmt.Errorf("error updating storage element: %w", err)
	}

	return res.ModifiedCount, nil
}

func getMongoProjection(projection *map[string]int) *bson.M {
	mongoProjection := bson.M{}

	if projection != nil {
		for key, value := range *projection {
			mongoProjection[key] = value
		}
	}

	return &mongoProjection
}

func getMongoSort(sort *orderedmap.OrderedMap[string, int]) *bson.D {
	mongoSort := bson.D{}

	if sort != nil {
		for _, key := range sort.Keys() {
			value, _ := sort.Get(key)

			mongoSort = append(mongoSort, bson.E{Key: key, Value: value})
		}
	}

	return &mongoSort
}

func getMongoMessage(opt *FindOptions) (bson.M, error) {
	mongoFilter := bson.M{}

	if opt.InternalFilter == nil {
		return mongoFilter, nil
	}

	if opt.InternalFilter.Ids != nil {
		idsLen := len(*opt.InternalFilter.Ids)

		if idsLen != 0 {
			if len(*opt.InternalFilter.Ids) == 1 {
				mongoFilter["id"] = (*opt.InternalFilter.Ids)[0]
			} else {
				mongoFilter["id"] = bson.M{
					"$in": *opt.InternalFilter.Ids,
				}
			}
		}
	}

	if opt.InternalFilter.Queue != "" {
		mongoFilter["queue"] = opt.InternalFilter.Queue
	}

	if opt.InternalFilter.QueuePrefix != "" {
		mongoFilter["queue_prefix"] = opt.InternalFilter.QueuePrefix
	}

	idFilter := bson.M{}

	if opt.InternalFilter.InternalIdBreakpointGt != "" {
		internalIdGt, err := bson.ObjectIDFromHex(opt.InternalFilter.InternalIdBreakpointGt)
		if err != nil {
			return nil, fmt.Errorf("invalid breakpoint to filter: %w", err)
		}
		idFilter["$gt"] = internalIdGt
	}

	if opt.InternalFilter.InternalIdBreakpointLte != "" {
		internalIdLte, err := bson.ObjectIDFromHex(opt.InternalFilter.InternalIdBreakpointLte)
		if err != nil {
			return nil, fmt.Errorf("invalid breakpoint to filter: %w", err)
		}
		idFilter["$lte"] = internalIdLte
	}

	if len(idFilter) > 0 {
		mongoFilter["_id"] = idFilter
	}

	if opt.InternalFilter.ExpiryDate != nil {
		mongoFilter["expiry_date"] = bson.M{
			"$lte": *opt.InternalFilter.ExpiryDate,
		}
	}

	return mongoFilter, nil
}

func (storage *MongoStorage) GetStringInternalId(_ context.Context, message *message.Message) string {
	if message.InternalId == nil {
		return ""
	}

	return message.InternalId.(bson.ObjectID).Hex()
}

func (storage *MongoStorage) Close(ctx context.Context) error {
	err := storage.client.Disconnect(ctx)

	secondErr := storage.clientPrimaryPreference.Disconnect(ctx)

	if err != nil {
		return err
	}

	return secondErr
}
