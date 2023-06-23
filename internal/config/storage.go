package config

import "github.com/takenet/deckard/internal/project"

var StorageType = Create(&ViperConfigKey{
	Key:     "storage.type",
	Default: "MEMORY",
})

// MongoDB Configurations

var MongoUri = Create(&ViperConfigKey{
	Key:     "mongo.uri",
	Default: "",
})

var MongoAddresses = Create(&ViperConfigKey{
	Key:     "mongo.addresses",
	Default: "localhost:27017",
})

var MongoDatabase = Create(&ViperConfigKey{
	Key:     "mongo.database",
	Default: project.Name,
})

var MongoCollection = Create(&ViperConfigKey{
	Key:     "mongo.collection",
	Default: "queue",
})

var MongoUser = Create(&ViperConfigKey{
	Key: "mongo.user",
})

var MongoAuthDb = Create(&ViperConfigKey{
	Key: "mongo.auth_db",
})

var MongoPassword = Create(&ViperConfigKey{
	Key: "mongo.password",
})

var MongoSsl = Create(&ViperConfigKey{
	Key:     "mongo.ssl",
	Default: false,
})

var MongoMaxPoolSize = Create(&ViperConfigKey{
	Key:     "mongo.max_pool_size",
	Default: 100,
})

var MongoQueueConfigurationCollection = Create(&ViperConfigKey{
	Key:     "mongo.queue_configuration_collection",
	Default: "queue_configuration",
})
