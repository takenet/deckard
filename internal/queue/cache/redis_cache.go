package cache

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
	"github.com/meirf/gopart"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/dtime"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/metrics"
	"github.com/takenet/deckard/internal/queue/message"
	"github.com/takenet/deckard/internal/queue/pool"
	"github.com/takenet/deckard/internal/queue/score"
	"go.opentelemetry.io/otel/attribute"
)

type RedisCache struct {
	Client  *redis.Client
	scripts map[string]*redis.Script
}

var _ Cache = &RedisCache{}

const (
	pullElement          = "pull"
	removeElement        = "remove"
	moveElement          = "move"
	lockElement          = "lock"
	addElements          = "add"
	containsElement      = "contains"
	moveFilteredElements = "move_primary_pool"
	unlockElements       = "unlock_elements"

	POOL_PREFIX            = "deckard:queue:"
	PROCESSING_POOL_SUFFIX = ":tmp"
	LOCK_ACK_SUFFIX        = ":" + string(LOCK_ACK)
	LOCK_NACK_SUFFIX       = ":" + string(LOCK_NACK)
	LOCK_ACK_SCORE_SUFFIX  = ":" + string(LOCK_ACK) + ":score"
	LOCK_NACK_SCORE_SUFFIX = ":" + string(LOCK_NACK) + ":score"
)

var PROCESSING_POOL_REGEX = regexp.MustCompile("(.+)" + PROCESSING_POOL_SUFFIX + "$")
var LOCK_ACK_POOL_REGEX = regexp.MustCompile("(.+)" + LOCK_ACK_SUFFIX + "$")
var LOCK_NACK_POOL_REGEX = regexp.MustCompile("(.+)" + LOCK_NACK_SUFFIX + "$")

func NewRedisCache(ctx context.Context) (*RedisCache, error) {
	uri := config.CacheUri.Get()

	var options *redis.Options
	if uri == "" {
		address := fmt.Sprint(config.RedisAddress.Get(), ":", config.RedisPort.GetInt())

		options = &redis.Options{
			Addr:     address,
			Password: config.RedisPassword.Get(),
			DB:       config.RedisDB.GetInt(),
		}
	} else {
		var err error
		options, err = redis.ParseURL(uri)

		if err != nil {
			return nil, fmt.Errorf("error parsing redis uri: %w", err)
		}
	}

	logger.S(ctx).Info("Connecting to ", options.Addr, " Redis instance(s)")

	start := dtime.Now()

	redisClient, err := waitForClient(options)
	if err != nil {
		return nil, err
	}

	logger.S(ctx).Debug("Connected to Redis cache in ", time.Since(start))

	return &RedisCache{
		Client: redisClient,
		scripts: map[string]*redis.Script{
			removeElement:        redis.NewScript(removeElementScript),
			pullElement:          redis.NewScript(pullElementsScript),
			moveElement:          redis.NewScript(moveElementScript),
			lockElement:          redis.NewScript(lockElementScript),
			moveFilteredElements: redis.NewScript(moveFilteredElementsScript),
			unlockElements:       redis.NewScript(unlockElementsScript),
			addElements:          redis.NewScript(addElementsScript),
			containsElement:      redis.NewScript(containsElementScript),
		},
	}, nil
}

func waitForClient(options *redis.Options) (*redis.Client, error) {
	var err error

	redisClient := redis.NewClient(options)

	// OpenTelemetry APM
	redisClient.AddHook(redisotel.NewTracingHook())

	for i := 1; i <= config.CacheConnectionRetryAttempts.GetInt(); i++ {
		var pingResult string
		pingResult, err = redisClient.Ping(context.Background()).Result()

		if (err == nil && pingResult == "PONG") || !config.CacheConnectionRetryEnabled.GetBool() {
			break
		}

		logger.S(context.Background()).Warnf("Failed to connect to Redis (%d times). Trying again in %s.", i, config.CacheConnectionRetryDelay.GetDuration())

		<-time.After(config.CacheConnectionRetryDelay.GetDuration())
	}

	return redisClient, err
}

func (cache *RedisCache) Flush(ctx context.Context) {
	now := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(now), attribute.String("op", "flush"))
	}()

	cache.Client.FlushDB(ctx)
}

func (cache *RedisCache) Remove(ctx context.Context, queue string, ids ...string) (removed int64, err error) {
	total := int64(0)

	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), attribute.String("op", "remove"))
	}()

	idList := make([]interface{}, len(ids))
	for i := range ids {
		idList[i] = ids[i]
	}

	for index := range gopart.Partition(len(idList), 4000) {
		cmd := cache.scripts[removeElement].Run(
			context.Background(),
			cache.Client,
			[]string{
				cache.activePool(queue),
				cache.processingPool(queue),
				cache.lockPool(queue, LOCK_ACK),
				cache.lockPool(queue, LOCK_NACK),
				cache.lockPoolScore(queue, LOCK_ACK),
				cache.lockPoolScore(queue, LOCK_NACK),
			},
			idList[index.Low:index.High]...,
		)

		result, err := cmd.Int64()

		if err != nil {
			logger.S(ctx).Errorf("Error removing %d elements from cache. %v", len(idList), err)

			return total + result, fmt.Errorf("error removing cache elements: %w", err)
		}

		total += result
	}

	return total, nil
}

// TODO: This should be optimized.
// TODO: We should list queues using storage with iterator, and not redis. Rethink this usage
func (cache *RedisCache) ListQueues(ctx context.Context, pattern string, poolType pool.PoolType) (queues []string, err error) {
	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), attribute.String("op", "list_queue"))
	}()

	var searchPattern string

	switch poolType {
	case pool.PRIMARY_POOL:
		searchPattern = cache.activePool(pattern)

	case pool.PROCESSING_POOL:
		searchPattern = cache.processingPool(pattern)

	case pool.LOCK_ACK_POOL:
		searchPattern = cache.lockPool(pattern, LOCK_ACK)

	case pool.LOCK_NACK_POOL:
		searchPattern = cache.lockPool(pattern, LOCK_NACK)
	}

	result := cache.Client.Keys(context.Background(), searchPattern)

	if result.Err() != nil {
		return nil, fmt.Errorf("error listing cache queues: %w", result.Err())
	}

	data := result.Val()

	var regex *regexp.Regexp
	switch poolType {
	case pool.PROCESSING_POOL:
		regex = PROCESSING_POOL_REGEX

	case pool.LOCK_ACK_POOL:
		regex = LOCK_ACK_POOL_REGEX

	case pool.LOCK_NACK_POOL:
		regex = LOCK_NACK_POOL_REGEX
	}

	for i, queue := range data {
		queue = strings.Replace(queue, POOL_PREFIX, "", 1)

		if regex != nil {
			queue = regex.ReplaceAllString(queue, "$1")
		}

		data[i] = queue
	}

	if pool.PRIMARY_POOL == poolType {
		return filterQueueSuffix(data), nil
	}

	return data, nil
}

func filterQueueSuffix(data []string) []string {
	result := make([]string, 0, len(data))

	for i := range data {
		if strings.HasSuffix(data[i], PROCESSING_POOL_SUFFIX) {
			continue
		}

		if strings.HasSuffix(data[i], LOCK_ACK_SUFFIX) {
			continue
		}

		if strings.HasSuffix(data[i], LOCK_NACK_SUFFIX) {
			continue
		}

		if strings.HasSuffix(data[i], LOCK_NACK_SCORE_SUFFIX) {
			continue
		}

		if strings.HasSuffix(data[i], LOCK_ACK_SCORE_SUFFIX) {
			continue
		}

		result = append(result, data[i])
	}

	return result
}

func (cache *RedisCache) MakeAvailable(ctx context.Context, message *message.Message) (bool, error) {
	if message.Queue == "" {
		return false, errors.New("invalid message queue")
	}

	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), attribute.String("op", "make_available"))
	}()

	cmd := cache.scripts[moveElement].Run(
		context.Background(),
		cache.Client,
		[]string{cache.activePool(message.Queue), cache.processingPool(message.Queue)},
		message.Score,
		message.ID,
	)

	if cmd.Err() != nil {
		return false, fmt.Errorf("error making element available on cache: %w", cmd.Err())
	}

	return cmd.Val().(int64) == 1, nil
}

func (cache *RedisCache) LockMessage(ctx context.Context, message *message.Message, lockType LockType) (bool, error) {
	if message.Queue == "" {
		return false, errors.New("invalid queue to lock")
	}

	if message.LockMs <= 0 {
		return false, errors.New("invalid lock time")
	}

	lockScore := dtime.NowMs() + message.LockMs

	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), attribute.String("op", "lock"))
	}()

	cmd := cache.scripts[lockElement].Run(
		context.Background(),
		cache.Client,
		[]string{cache.processingPool(message.Queue), cache.lockPool(message.Queue, lockType), cache.lockPoolScore(message.Queue, lockType)},
		lockScore,
		message.ID,
		message.Score,
	)

	if cmd.Err() != nil {
		return false, fmt.Errorf("error locking cache element: %w", cmd.Err())
	}

	return cmd.Val().(int64) == 1, nil
}

func (cache *RedisCache) UnlockMessages(ctx context.Context, queue string, lockType LockType) ([]string, error) {
	defaultScore := score.Min

	if lockType == LOCK_ACK {
		defaultScore = score.GetScoreByDefaultAlgorithm()
	}

	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), attribute.String("op", "unlock_messages"))
	}()

	cmd := cache.scripts[unlockElements].Run(
		context.Background(),
		cache.Client,
		[]string{cache.lockPool(queue, lockType), cache.activePool(queue), cache.lockPoolScore(queue, lockType)},
		1000, dtime.NowMs(), defaultScore,
	)

	return parseResult(cmd)
}

func (cache *RedisCache) PullMessages(ctx context.Context, queue string, n int64, minScore *float64, maxScore *float64, ackDeadlineSeconds int64) (ids []string, err error) {
	var cmd *redis.Cmd

	now := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(now), attribute.String("op", "pull"))
	}()

	if ackDeadlineSeconds == 0 {
		ackDeadlineSeconds = 300
	}

	newScore := dtime.TimeToMs(&now) + ackDeadlineSeconds*1000

	args := []any{
		n, newScore,
	}

	if minScore != nil {
		args = append(args, *minScore)
	} else {
		args = append(args, "-inf")
	}

	if maxScore != nil {
		args = append(args, *maxScore)
	} else {
		args = append(args, "+inf")
	}

	cmd = cache.scripts[pullElement].Run(
		context.Background(),
		cache.Client,
		[]string{cache.activePool(queue), cache.processingPool(queue)},
		args...,
	)

	return parseResult(cmd)
}

func parseResult(cmd *redis.Cmd) ([]string, error) {
	result, err := cmd.Result()

	if err != nil {
		return nil, fmt.Errorf("error parsing result: %w", err)
	}

	if result == "" {
		return nil, nil
	}

	resultIds := resultToIds(result)

	if resultIds != nil {
		return resultIds, nil
	}

	return nil, fmt.Errorf("invalid redis response: %v", result)
}

func resultToIds(result interface{}) []string {
	if keys, ok := result.([]interface{}); ok {
		ids := make([]string, len(keys))

		for i := range keys {
			ids[i] = fmt.Sprint(keys[i])
		}

		return ids
	}

	return nil
}

func (cache *RedisCache) TimeoutMessages(ctx context.Context, queue string) ([]string, error) {
	now := dtime.Now()
	timeoutTime := dtime.TimeToMs(&now)

	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(now), attribute.String("op", "timeout"))
	}()

	cmd := cache.scripts[moveFilteredElements].Run(
		context.Background(),
		cache.Client,
		[]string{cache.activePool(queue), cache.processingPool(queue)},
		timeoutTime, score.Min, 1000,
	)

	return parseResult(cmd)
}

func (cache *RedisCache) Insert(ctx context.Context, queue string, messages ...*message.Message) ([]string, error) {
	for i := range messages {
		if messages[i].Queue != queue {
			return nil, errors.New("invalid queue to insert data")
		}
	}

	insertions := make([]string, 0)
	for index := range gopart.Partition(len(messages), 2000) {
		partition := messages[index.Low:index.High]

		args := make([]interface{}, 2*len(partition))

		index := 0
		for _, message := range partition {
			args[index] = message.Score
			args[index+1] = message.ID

			index += 2
		}

		var cmd *redis.Cmd
		func() {
			execStart := dtime.Now()
			defer func() {
				metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), attribute.String("op", "insert"))
			}()

			cmd = cache.scripts[addElements].Run(
				context.Background(),
				cache.Client,
				[]string{cache.activePool(queue), cache.lockPool(queue, LOCK_NACK), cache.lockPool(queue, LOCK_ACK), cache.processingPool(queue)},
				args...,
			)
		}()

		inserts, err := parseResult(cmd)

		if err != nil {
			return nil, fmt.Errorf("error inserting cache: %w", cmd.Err())
		}

		insertions = append(insertions, inserts...)
	}

	return insertions, nil
}

func (cache *RedisCache) IsProcessing(ctx context.Context, queue string, id string) (bool, error) {
	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), attribute.String("op", "is_processing"))
	}()

	return cache.containsElement(ctx, cache.processingPool(queue), id)
}

func (cache *RedisCache) containsElement(ctx context.Context, queuePool string, id string) (bool, error) {
	result := cache.Client.ZScore(context.Background(), queuePool, id)

	if result.Err() == redis.Nil {
		return false, nil
	}

	if result.Err() != nil {
		return false, fmt.Errorf("error checking processing: %w", result.Err())
	}

	return true, nil
}

func (cache *RedisCache) Get(ctx context.Context, key string) (string, error) {
	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), attribute.String("op", "get"))
	}()

	cmd := cache.Client.Get(context.Background(), fmt.Sprint("deckard:", key))
	value, err := cmd.Result()

	if err == redis.Nil {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("error getting cache element: %w", err)
	}

	return value, nil
}

func (cache *RedisCache) Set(ctx context.Context, key string, value string) error {
	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), attribute.String("op", "set"))
	}()

	cmd := cache.Client.Set(context.Background(), fmt.Sprint("deckard:", key), value, 0)

	if cmd.Err() != nil {
		return fmt.Errorf("error setting cache element: %w", cmd.Err())
	}

	return nil
}

func (cache *RedisCache) Close(ctx context.Context) error {
	return cache.Client.Close()
}

// activePool returns the name of the active pool of messages.
func (cache *RedisCache) activePool(queue string) string {
	return POOL_PREFIX + queue
}

// processingPool returns the name of the processing pool of messages.
func (cache *RedisCache) processingPool(queue string) string {
	return POOL_PREFIX + queue + PROCESSING_POOL_SUFFIX
}

// lockPool returns the name of the lock pool of messages.
func (cache *RedisCache) lockPool(queue string, lockType LockType) string {
	return POOL_PREFIX + queue + ":" + string(lockType)
}

// lockPool returns the name of the lock pool scores of messages.
//
// used to unlock messages with a predefined score.
func (cache *RedisCache) lockPoolScore(queue string, lockType LockType) string {
	return POOL_PREFIX + queue + ":" + string(lockType) + ":score"
}
