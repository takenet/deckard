package config

var CacheType = Create(&ViperConfigKey{
	Key:     "cache.type",
	Default: "MEMORY",
})

var RedisUri = Create(&ViperConfigKey{
	Key: "redis.uri",
})

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
