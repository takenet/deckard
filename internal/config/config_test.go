package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/project"
)

func TestLoadConfigShouldResetBeforeConfiguring(t *testing.T) {
	LoadConfig()

	require.Equal(t, project.Name, viper.GetString(MONGO_DATABASE))

	viper.Set(MONGO_DATABASE, "test")

	require.Equal(t, "test", viper.GetString(MONGO_DATABASE))

	LoadConfig()

	require.Equal(t, project.Name, viper.GetString(MONGO_DATABASE))
}

func TestEnvReplacerShouldConsiderDotAsUnderline(t *testing.T) {
	LoadConfig()

	require.Equal(t, false, viper.GetBool(AUDIT_ENABLED))

	os.Setenv("DECKARD_AUDIT_ENABLED", "true")

	defer os.Unsetenv("DECKARD_AUDIT_ENABLED")

	require.Equal(t, true, viper.GetBool(AUDIT_ENABLED))
}

func TestEnvWithoutPrefixShouldReturnDefaultValue(t *testing.T) {
	LoadConfig()

	require.Equal(t, false, viper.GetBool(AUDIT_ENABLED))

	os.Setenv("AUDIT_ENABLED", "true")

	require.Equal(t, false, viper.GetBool(AUDIT_ENABLED))
}

func TestDefaultValues(t *testing.T) {
	LoadConfig()

	// Deckard configuration
	require.Equal(t, "MEMORY", viper.GetString(STORAGE_TYPE))
	require.Equal(t, "MEMORY", viper.GetString(CACHE_TYPE))

	// gRPC server
	require.Equal(t, true, viper.GetBool(GRPC_ENABLED))
	require.Equal(t, 8081, viper.GetInt(GRPC_PORT))

	// Redis configurations
	require.Equal(t, "localhost", viper.GetString(REDIS_ADDRESS))
	require.Equal(t, 6379, viper.GetInt(REDIS_PORT))
	require.Equal(t, 0, viper.GetInt(REDIS_DB))

	// Audit
	require.Equal(t, false, viper.GetBool(AUDIT_ENABLED))

	// ElasticSearch
	require.Equal(t, "http://localhost:9200/", viper.GetString(ELASTIC_ADDRESS))

	// MongoDB
	require.Equal(t, "localhost:27017", viper.GetString(MONGO_ADDRESSES))
	require.Equal(t, project.Name, viper.GetString(MONGO_DATABASE))
	require.Equal(t, "queue", viper.GetString(MONGO_COLLECTION))
	require.Equal(t, "queue_configuration", viper.GetString(MONGO_QUEUE_CONFIGURATION_COLLECTION))
	require.Equal(t, false, viper.GetBool(MONGO_SSL))

	// Environment
	require.Equal(t, false, viper.GetBool(DEBUG))

	// Housekeeper
	require.Equal(t, true, viper.GetBool(HOUSEKEEPER_ENABLED))
	require.Equal(t, 1*time.Second, viper.GetDuration(HOUSEKEEPER_TASK_TIMEOUT_DELAY))
	require.Equal(t, 1*time.Second, viper.GetDuration(HOUSEKEEPER_TASK_UNLOCK_DELAY))
	require.Equal(t, 1*time.Second, viper.GetDuration(HOUSEKEEPER_TASK_UPDATE_DELAY))
	require.Equal(t, 1*time.Second, viper.GetDuration(HOUSEKEEPER_TASK_TTL_DELAY))
	require.Equal(t, 1*time.Second, viper.GetDuration(HOUSEKEEPER_TASK_MAX_ELEMENTS_DELAY))
	require.Equal(t, 60*time.Second, viper.GetDuration(HOUSEKEEPER_TASK_METRICS_DELAY))
}
