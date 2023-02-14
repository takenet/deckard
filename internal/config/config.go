package config

import (
	"strings"

	"github.com/spf13/viper"
	"github.com/takenet/deckard/internal/project"
)

type ConfigKey string

const (
	STORAGE_TYPE string = "storage.type"
	CACHE_TYPE   string = "cache.type"

	GRPC_ENABLED string = "grpc.enabled"
	GRPC_PORT    string = "grpc.port"

	REDIS_URI      string = "redis.uri"
	REDIS_PASSWORD string = "redis.password"
	REDIS_ADDRESS  string = "redis.address"
	REDIS_PORT     string = "redis.port"
	REDIS_DB       string = "redis.db"

	AUDIT_ENABLED string = "audit.enabled"

	ELASTIC_ADDRESS  string = "elastic.address"
	ELASTIC_PASSWORD string = "elastic.password"
	ELASTIC_USER     string = "elastic.user"

	MONGO_URI                            string = "mongo.uri"
	MONGO_ADDRESSES                      string = "mongo.addresses"
	MONGO_AUTH_DB                        string = "mongo.auth_db"
	MONGO_PASSWORD                       string = "mongo.password"
	MONGO_DATABASE                       string = "mongo.database"
	MONGO_COLLECTION                     string = "mongo.collection"
	MONGO_USER                           string = "mongo.user"
	MONGO_SSL                            string = "mongo.ssl"
	MONGO_MAX_POOL_SIZE                  string = "mongo.max_pool_size"
	MONGO_QUEUE_CONFIGURATION_COLLECTION string = "mongo.queue_configuration_collection"

	HOUSEKEEPER_ENABLED                 string = "housekeeper.enabled"
	HOUSEKEEPER_TASK_TIMEOUT_DELAY      string = "housekeeper.task.timeout.delay"
	HOUSEKEEPER_TASK_UNLOCK_DELAY       string = "housekeeper.task.unlock.delay"
	HOUSEKEEPER_TASK_UPDATE_DELAY       string = "housekeeper.task.update.delay"
	HOUSEKEEPER_TASK_TTL_DELAY          string = "housekeeper.task.ttl.delay"
	HOUSEKEEPER_TASK_MAX_ELEMENTS_DELAY string = "housekeeper.task.max_elements.delay"
	HOUSEKEEPER_TASK_METRICS_DELAY      string = "housekeeper.task.metrics.delay"

	TLS_SERVER_CERT_FILE_PATHS string = "tls.server.cert_file_paths"
	TLS_SERVER_KEY_FILE_PATHS  string = "tls.server.key_file_paths"
	TLS_CLIENT_CERT_FILE_PATHS string = "tls.client.cert_file_paths"
	TLS_CLIENT_AUTH_TYPE       string = "tls.client.auth_type"

	DEBUG    string = "debug"
	LOG_TYPE string = "log.type"
)

func init() {
	LoadConfig()
}

func LoadConfig() {
	viper.Reset()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	viper.SetEnvPrefix(project.Name)
	viper.AutomaticEnv()

	// Deckard configuration
	viper.SetDefault(STORAGE_TYPE, "MEMORY")
	viper.SetDefault(CACHE_TYPE, "MEMORY")

	// gRPC server
	viper.SetDefault(GRPC_ENABLED, true)
	viper.SetDefault(GRPC_PORT, 8081)

	// Redis configurations
	viper.SetDefault(REDIS_ADDRESS, "localhost")
	viper.SetDefault(REDIS_PORT, 6379)
	viper.SetDefault(REDIS_DB, 0)

	// Audit
	viper.SetDefault(AUDIT_ENABLED, false)

	// ElasticSearch
	viper.SetDefault(ELASTIC_ADDRESS, "http://localhost:9200/")

	// MongoDB
	viper.SetDefault(MONGO_ADDRESSES, "localhost:27017")
	viper.SetDefault(MONGO_DATABASE, project.Name)
	viper.SetDefault(MONGO_COLLECTION, "queue")
	viper.SetDefault(MONGO_QUEUE_CONFIGURATION_COLLECTION, "queue_configuration")
	viper.SetDefault(MONGO_MAX_POOL_SIZE, 100)
	viper.SetDefault(MONGO_SSL, false)

	// Environment
	viper.SetDefault(DEBUG, false)
	err := viper.BindEnv(DEBUG, "DEBUG", "debug")
	if err != nil {
		panic(err)
	}

	viper.SetDefault(LOG_TYPE, "json")
	err = viper.BindEnv(LOG_TYPE, "LOG_TYPE", "log_type")
	if err != nil {
		panic(err)
	}

	// Housekeeper
	viper.SetDefault(HOUSEKEEPER_ENABLED, true)
	viper.SetDefault(HOUSEKEEPER_TASK_TIMEOUT_DELAY, "1s")
	viper.SetDefault(HOUSEKEEPER_TASK_UNLOCK_DELAY, "1s")
	viper.SetDefault(HOUSEKEEPER_TASK_UPDATE_DELAY, "1s")
	viper.SetDefault(HOUSEKEEPER_TASK_TTL_DELAY, "1s")
	viper.SetDefault(HOUSEKEEPER_TASK_MAX_ELEMENTS_DELAY, "1s")
	viper.SetDefault(HOUSEKEEPER_TASK_METRICS_DELAY, "60s")

	// TLS
	viper.SetDefault(TLS_CLIENT_AUTH_TYPE, "NoClientCert")
}
