package config

import "github.com/takenet/deckard/internal/project"

var StorageType = Create(&ViperConfigKey{
	Key:     "storage.type",
	Default: "MEMORY",
})

var StorageUri = Create(&ViperConfigKey{
	Key:     "storage.uri",
	Aliases: []string{"mongo.uri", "mongodb.uri"},
	Default: nil,
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
//
// Connection details (hosts, credentials, auth source, TLS, pool size, timeouts, etc.) are
// configured through StorageUri (DECKARD_STORAGE_URI), applied via the driver's own ApplyURI
// connection-string parsing. MongoDatabase/MongoCollection/MongoQueueConfigurationCollection below
// are schema configuration (which database/collections Deckard uses), not connection parameters,
// so they remain independent of the URI.

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

var MongoQueueConfigurationCollection = Create(&ViperConfigKey{
	Key:     "mongo.queue_configuration_collection",
	Aliases: []string{"mongodb.queue_configuration_collection"},
	Default: "queue_configuration",
})
