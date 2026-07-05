package cache

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/meirf/gopart"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/dtime"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/metrics"
	"github.com/takenet/deckard/internal/queue/message"
	"github.com/takenet/deckard/internal/queue/pool"
	"github.com/takenet/deckard/internal/queue/score"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var errCacheUriRequired = errors.New("cache.uri (DECKARD_CACHE_URI) is required when cache.type is REDIS")
var errQueueNameContainsClusterHashTag = errors.New("queue names cannot contain '{' or '}'")

// RedisClient interface abstracts the differences between single-node and cluster Redis clients
type RedisClient interface {
	redis.Cmdable
	Close() error
	Ping(ctx context.Context) *redis.StatusCmd
	FlushDB(ctx context.Context) *redis.StatusCmd
}

type RedisCache struct {
	Client      RedisClient
	scripts     map[string]*redis.Script
	clusterMode bool
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
	SCORE_SUFFIX           = ":score"
	LOCK_ACK_SCORE_SUFFIX  = ":" + string(LOCK_ACK) + SCORE_SUFFIX
	LOCK_NACK_SCORE_SUFFIX = ":" + string(LOCK_NACK) + SCORE_SUFFIX
)

var PROCESSING_POOL_REGEX = regexp.MustCompile("(.+)" + PROCESSING_POOL_SUFFIX + "$")
var LOCK_ACK_POOL_REGEX = regexp.MustCompile("(.+)" + LOCK_ACK_SUFFIX + "$")
var LOCK_NACK_POOL_REGEX = regexp.MustCompile("(.+)" + LOCK_NACK_SUFFIX + "$")

func NewRedisCache(ctx context.Context) (*RedisCache, error) {
	clusterMode := config.RedisClusterMode.GetBool()

	var client RedisClient
	var err error

	if clusterMode {
		client, err = createClusterClient(ctx)
	} else {
		client, err = createSingleNodeClient(ctx)
	}

	if err != nil {
		return nil, err
	}

	return &RedisCache{
		Client:      client,
		clusterMode: clusterMode,
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

func createSingleNodeClient(ctx context.Context) (RedisClient, error) {
	options, err := singleNodeOptionsFromConfig()
	if err != nil {
		return nil, err
	}

	logger.S(ctx).Info("Connecting to ", options.Addr, " Redis instance")

	start := dtime.Now()

	redisClient, err := waitForSingleNodeClient(ctx, options)
	if err != nil {
		return nil, err
	}

	logger.S(ctx).Debug("Connected to Redis cache in ", time.Since(start))

	return redisClient, nil
}

func createClusterClient(ctx context.Context) (RedisClient, error) {
	options, err := clusterOptionsFromConfig()
	if err != nil {
		return nil, err
	}

	logger.S(ctx).Info("Connecting to Redis cluster with addresses: ", options.Addrs)

	start := dtime.Now()

	clusterClient, err := waitForClusterClient(ctx, options)
	if err != nil {
		return nil, err
	}

	logger.S(ctx).Debug("Connected to Redis cluster in ", time.Since(start))

	return clusterClient, nil
}

func singleNodeOptionsFromConfig() (*redis.Options, error) {
	uri := config.CacheUri.Get()
	if uri == "" {
		return nil, errCacheUriRequired
	}

	options, err := redis.ParseURL(uri)
	if err != nil {
		return nil, fmt.Errorf("error parsing redis uri: %w", err)
	}

	return options, nil
}

// clusterOptionsFromConfig delegates to redis.ParseClusterURL, go-redis's own URL format for
// Redis Cluster: a base redis://|rediss:// URI for the first seed node, with additional seed
// nodes appended as repeated addr= query parameters (e.g.
// "redis://node-1:6379?addr=node-2:6379&addr=node-3:6379"). Credentials and TLS are part of the
// base URI and apply to every node.
func clusterOptionsFromConfig() (*redis.ClusterOptions, error) {
	uri := config.CacheUri.Get()
	if uri == "" {
		return nil, errCacheUriRequired
	}

	options, err := redis.ParseClusterURL(uri)
	if err != nil {
		return nil, fmt.Errorf("error parsing redis cluster uri: %w", err)
	}

	return options, nil
}

func waitForSingleNodeClient(ctx context.Context, options *redis.Options) (*redis.Client, error) {
	var err error

	redisClient := redis.NewClient(options)

	// OpenTelemetry APM
	if err := redisotel.InstrumentTracing(redisClient); err != nil {
		if closeErr := redisClient.Close(); closeErr != nil {
			return nil, fmt.Errorf("error setting up redis tracing and closing redis client: %w", errors.Join(err, closeErr))
		}

		return nil, fmt.Errorf("error setting up redis tracing: %w", err)
	}

	for i := 1; i <= config.CacheConnectionRetryAttempts.GetInt(); i++ {
		var pingResult string
		pingResult, err = redisClient.Ping(ctx).Result()

		if (err == nil && pingResult == "PONG") || !config.CacheConnectionRetryEnabled.GetBool() {
			break
		}

		logger.S(ctx).Warnf("Failed to connect to Redis (%d times). Trying again in %s.", i, config.CacheConnectionRetryDelay.GetDuration())

		if waitErr := waitForRetry(ctx, config.CacheConnectionRetryDelay.GetDuration()); waitErr != nil {
			err = waitErr
			break
		}
	}

	if err != nil {
		if closeErr := redisClient.Close(); closeErr != nil {
			return nil, fmt.Errorf("error connecting to redis and closing redis client: %w", errors.Join(err, closeErr))
		}

		return nil, err
	}

	return redisClient, err
}

func waitForClusterClient(ctx context.Context, options *redis.ClusterOptions) (*redis.ClusterClient, error) {
	var err error

	clusterClient := redis.NewClusterClient(options)

	// OpenTelemetry APM
	if err := redisotel.InstrumentTracing(clusterClient); err != nil {
		if closeErr := clusterClient.Close(); closeErr != nil {
			return nil, fmt.Errorf("error setting up redis cluster tracing and closing redis cluster client: %w", errors.Join(err, closeErr))
		}

		return nil, fmt.Errorf("error setting up redis cluster tracing: %w", err)
	}

	for i := 1; i <= config.CacheConnectionRetryAttempts.GetInt(); i++ {
		var pingResult string
		pingResult, err = clusterClient.Ping(ctx).Result()

		if (err == nil && pingResult == "PONG") || !config.CacheConnectionRetryEnabled.GetBool() {
			break
		}

		logger.S(ctx).Warnf("Failed to connect to Redis cluster (%d times). Trying again in %s.", i, config.CacheConnectionRetryDelay.GetDuration())

		if waitErr := waitForRetry(ctx, config.CacheConnectionRetryDelay.GetDuration()); waitErr != nil {
			err = waitErr
			break
		}
	}

	if err != nil {
		if closeErr := clusterClient.Close(); closeErr != nil {
			return nil, fmt.Errorf("error connecting to redis cluster and closing redis cluster client: %w", errors.Join(err, closeErr))
		}

		return nil, err
	}

	return clusterClient, err
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (cache *RedisCache) Flush(ctx context.Context) {
	now := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "flush")))
	}()

	// FLUSHDB is a keyless command: just like SCAN, a single ClusterClient.FlushDB call only
	// reaches one (round-robin-picked) master shard, silently leaving every other shard's data
	// in place. Fan out via ForEachMaster so Flush actually clears the whole cluster.
	if cache.clusterMode {
		clusterClient, ok := cache.Client.(*redis.ClusterClient)
		if !ok {
			logger.S(ctx).Error("cluster mode enabled but redis client is not a *redis.ClusterClient, skipping flush")
			return
		}

		err := clusterClient.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
			return master.FlushDB(ctx).Err()
		})

		if err != nil {
			logger.S(ctx).Errorf("error flushing redis cluster: %v", err)
		}

		return
	}

	if err := cache.Client.FlushDB(ctx).Err(); err != nil {
		logger.S(ctx).Errorf("error flushing redis: %v", err)
	}
}

func (cache *RedisCache) Remove(ctx context.Context, queue string, ids ...string) (removed int64, err error) {
	if err := cache.validateQueueName(queue); err != nil {
		return 0, err
	}

	total := int64(0)

	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), metric.WithAttributes(attribute.String("op", "remove")))
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

// Queue listing currently scans Redis keys directly.
func (cache *RedisCache) ListQueues(ctx context.Context, pattern string, poolType pool.PoolType) (queues []string, err error) {
	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), metric.WithAttributes(attribute.String("op", "list_queue")))
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

	// Use SCAN instead of KEYS to avoid blocking Redis.
	//
	// SCAN is a "keyless" command, so in cluster mode go-redis cannot route it by hash slot: it
	// picks one shard per call (round-robin by default) and each shard only knows about its own
	// keyspace/cursor. Scanning through a single ClusterClient.Scan call would therefore silently
	// miss queues living on other shards (and could even hand a shard's cursor to a different
	// shard). We fan out with ForEachMaster and scan every master node's keyspace independently.
	var data []string
	if cache.clusterMode {
		clusterClient, ok := cache.Client.(*redis.ClusterClient)
		if !ok {
			return nil, errors.New("cluster mode enabled but redis client is not a *redis.ClusterClient")
		}

		var mu sync.Mutex
		data = make([]string, 0)

		err = clusterClient.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
			keys, scanErr := scanKeys(ctx, master, searchPattern)
			if scanErr != nil {
				return scanErr
			}

			mu.Lock()
			data = append(data, keys...)
			mu.Unlock()

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error listing cache queues across cluster shards: %w", err)
		}
	} else {
		data, err = scanKeys(ctx, cache.Client, searchPattern)

		if err != nil {
			return nil, fmt.Errorf("error listing cache queues: %w", err)
		}
	}

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
		data[i] = cache.parseQueueKey(queue, regex)
	}

	if pool.PRIMARY_POOL == poolType {
		return filterQueueSuffix(data), nil
	}

	return data, nil
}

// scanKeys iterates the full SCAN cursor sequence against a single node/client and returns all
// matching keys. Must be called once per shard in cluster mode (see ListQueues) since SCAN cursors
// are only valid against the node that issued them.
func scanKeys(ctx context.Context, client redis.Cmdable, pattern string) ([]string, error) {
	keys := make([]string, 0)
	cursor := uint64(0)

	for {
		var batch []string
		var err error

		batch, cursor, err = client.Scan(ctx, cursor, pattern, 1000).Result()

		if err != nil {
			return nil, err
		}

		keys = append(keys, batch...)

		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// parseQueueKey converts a raw Redis key (as returned by SCAN) back into a plain
// queue name by removing the pool prefix, any pool-type suffix matched by regex,
// and, in cluster mode, the hash tag braces ("{" and "}") added by
// activePool/processingPool/lockPool/lockPoolScore to keep a queue's keys in the
// same hash slot. Those braces are an internal key-naming detail and must not
// leak to callers.
func (cache *RedisCache) parseQueueKey(key string, suffixRegex *regexp.Regexp) string {
	queue := strings.Replace(key, POOL_PREFIX, "", 1)

	if suffixRegex != nil {
		queue = suffixRegex.ReplaceAllString(queue, "$1")
	}

	if cache.clusterMode {
		queue = unwrapClusterHashTag(queue)
	}

	return queue
}

func unwrapClusterHashTag(queue string) string {
	if len(queue) < 2 || !strings.HasPrefix(queue, "{") || !strings.HasSuffix(queue, "}") {
		return queue
	}

	return queue[1 : len(queue)-1]
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

	if err := cache.validateQueueName(message.Queue); err != nil {
		return false, err
	}

	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), metric.WithAttributes(attribute.String("op", "make_available")))
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

	if err := cache.validateQueueName(message.Queue); err != nil {
		return false, err
	}

	if message.LockMs <= 0 {
		return false, errors.New("invalid lock time")
	}

	lockScore := dtime.NowMs() + message.LockMs

	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), metric.WithAttributes(attribute.String("op", "lock")))
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
	if err := cache.validateQueueName(queue); err != nil {
		return nil, err
	}

	defaultScore := score.Min

	if lockType == LOCK_ACK {
		defaultScore = score.GetScoreByDefaultAlgorithm()
	}

	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), metric.WithAttributes(attribute.String("op", "unlock_messages")))
	}()

	cmd := cache.scripts[unlockElements].Run(
		context.Background(),
		cache.Client,
		[]string{cache.lockPool(queue, lockType), cache.activePool(queue), cache.lockPoolScore(queue, lockType)},
		1000, dtime.NowMs(), defaultScore,
	)

	return parseResult(cmd)
}

func (cache *RedisCache) PullMessages(ctx context.Context, queue string, n int64, minScore *float64, maxScore *float64, ackDeadlineMs int64) (ids []string, err error) {
	if err := cache.validateQueueName(queue); err != nil {
		return nil, err
	}

	var cmd *redis.Cmd

	now := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "pull")))
	}()

	if ackDeadlineMs == 0 {
		ackDeadlineMs = DefaultTimeoutMs
	}

	newScore := dtime.TimeToMs(&now) + ackDeadlineMs

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
	if err := cache.validateQueueName(queue); err != nil {
		return nil, err
	}

	now := dtime.Now()
	timeoutTime := dtime.TimeToMs(&now)

	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("op", "timeout")))
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
	if err := cache.validateQueueName(queue); err != nil {
		return nil, err
	}

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
				metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), metric.WithAttributes(attribute.String("op", "insert")))
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
	if err := cache.validateQueueName(queue); err != nil {
		return false, err
	}

	execStart := dtime.Now()
	defer func() {
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), metric.WithAttributes(attribute.String("op", "is_processing")))
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
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), metric.WithAttributes(attribute.String("op", "get")))
	}()

	cmd := cache.Client.Get(ctx, cache.generalCacheKey(key))
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
		metrics.CacheLatency.Record(ctx, dtime.ElapsedTime(execStart), metric.WithAttributes(attribute.String("op", "set")))
	}()

	cmd := cache.Client.Set(ctx, cache.generalCacheKey(key), value, 0)

	if cmd.Err() != nil {
		return fmt.Errorf("error setting cache element: %w", cmd.Err())
	}

	return nil
}

func (cache *RedisCache) Close(ctx context.Context) error {
	return cache.Client.Close()
}

func (cache *RedisCache) generalCacheKey(key string) string {
	if cache.clusterMode {
		return fmt.Sprint("deckard:{", key, "}")
	}

	return fmt.Sprint("deckard:", key)
}

func (cache *RedisCache) validateQueueName(queue string) error {
	// Braces are reserved even in standalone mode so existing queues remain
	// migratable to Redis Cluster without ambiguous hash-tag parsing.
	if strings.ContainsAny(queue, "{}") {
		return errQueueNameContainsClusterHashTag
	}

	return nil
}

// activePool returns the name of the active pool of messages.
func (cache *RedisCache) activePool(queue string) string {
	if cache.clusterMode {
		return POOL_PREFIX + "{" + queue + "}"
	}
	return POOL_PREFIX + queue
}

// processingPool returns the name of the processing pool of messages.
func (cache *RedisCache) processingPool(queue string) string {
	if cache.clusterMode {
		return POOL_PREFIX + "{" + queue + "}" + PROCESSING_POOL_SUFFIX
	}
	return POOL_PREFIX + queue + PROCESSING_POOL_SUFFIX
}

// lockPool returns the name of the lock pool of messages.
func (cache *RedisCache) lockPool(queue string, lockType LockType) string {
	if cache.clusterMode {
		return POOL_PREFIX + "{" + queue + "}" + ":" + string(lockType)
	}
	return POOL_PREFIX + queue + ":" + string(lockType)
}

// lockPoolScore returns the name of the lock pool scores of messages.
//
// used to unlock messages with a predefined score.
func (cache *RedisCache) lockPoolScore(queue string, lockType LockType) string {
	if cache.clusterMode {
		return POOL_PREFIX + "{" + queue + "}" + ":" + string(lockType) + SCORE_SUFFIX
	}
	return POOL_PREFIX + queue + ":" + string(lockType) + SCORE_SUFFIX
}
