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

var RedisPassword = Create(&ViperConfigKey{
	Key: "redis.password",
})

var RedisAddress = Create(&ViperConfigKey{
	Key:     "redis.address",
	Default: "localhost",
})

var RedisPort = Create(&ViperConfigKey{
	Key:     "redis.port",
	Default: 6379,
})

var RedisDB = Create(&ViperConfigKey{
	Key:     "redis.db",
	Default: 0,
})
