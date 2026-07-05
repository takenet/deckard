package config

var CacheType = Create(&ViperConfigKey{
	Key:     "cache.type",
	Default: "MEMORY",
})

var CacheUri = Create(&ViperConfigKey{
	Key:     "cache.uri",
	Aliases: []string{"redis.uri"},
	Default: nil,
})

var CacheConnectionRetryEnabled = Create(&ViperConfigKey{
	Key:     "cache.connection.retry.enabled",
	Default: true,
})

var CacheConnectionRetryAttempts = Create(&ViperConfigKey{
	Key:     "cache.connection.retry.attempts",
	Default: 10,
})

var CacheConnectionRetryDelay = Create(&ViperConfigKey{
	Key:     "cache.connection.retry.delay",
	Default: "5s",
})

// Redis Configurations
//
// Connection details (address, credentials, database, TLS, timeouts, pool size, etc.) are
// configured through CacheUri (DECKARD_CACHE_URI): a standalone redis:// or rediss:// URI for
// single-node mode, or go-redis's cluster URL format for cluster mode - see
// redis.ParseClusterURL and clusterOptionsFromConfig.

// Redis Cluster Configurations

var RedisClusterMode = Create(&ViperConfigKey{
	Key:     "redis.cluster.mode",
	Default: false,
})
