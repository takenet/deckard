package config

import "github.com/takenet/deckard/internal/project"

var StorageType = Create(&ViperConfigKey{
	Key:     "storage.type",
	Default: "MEMORY",
})

var StorageUri = Create(&ViperConfigKey{
	Key:     "storage.uri",
	Aliases: []string{"mongo.uri", "mongodb.uri"},
	Default: "",
})

var StorageConnectionRetryEnabled = Create(&ViperConfigKey{
	Key:     "storage.connection.retry.enabled",
	Default: true,
})

var StorageConnectionRetryAttempts = Create(&ViperConfigKey{
	Key:     "storage.connection.retry.attempts",
	Default: 10,
})

var StorageConnectionRetryDelay = Create(&ViperConfigKey{
	Key:     "storage.connection.retry.delay",
	Default: "5s",
})

// MongoDB Configurations

var MongoAddresses = Create(&ViperConfigKey{
	Key:     "mongo.addresses",
	Aliases: []string{"mongodb.addresses"},
	Default: "localhost:27017",
})

var MongoDatabase = Create(&ViperConfigKey{
	Key:     "mongo.database",
	Aliases: []string{"mongodb.database"},
	Default: project.Name,
})

var MongoCollection = Create(&ViperConfigKey{
	Key:     "mongo.collection",
	Aliases: []string{"mongodb.collection"},
	Default: "queue",
})

var MongoUser = Create(&ViperConfigKey{
	Key:     "mongo.user",
	Aliases: []string{"mongodb.user"},
})

var MongoAuthDb = Create(&ViperConfigKey{
	Key:     "mongo.auth_db",
	Aliases: []string{"mongodb.auth_db"},
})

var MongoPassword = Create(&ViperConfigKey{
	Key:     "mongo.password",
	Aliases: []string{"mongodb.password"},
})

var MongoSsl = Create(&ViperConfigKey{
	Key:     "mongo.ssl",
	Aliases: []string{"mongodb.ssl"},
	Default: false,
})

var MongoMaxPoolSize = Create(&ViperConfigKey{
	Key:     "mongo.max_pool_size",
	Aliases: []string{"mongodb.max_pool_size"},
	Default: 100,
})

var MongoQueueConfigurationCollection = Create(&ViperConfigKey{
	Key:     "mongo.queue_configuration_collection",
	Aliases: []string{"mongodb.queue_configuration_collection"},
	Default: "queue_configuration",
})
