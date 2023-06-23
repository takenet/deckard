package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/project"
)

func TestLoadConfigShouldResetBeforeConfiguring(t *testing.T) {
	Configure(true)

	require.Equal(t, project.Name, MongoDatabase.Get())

	MongoDatabase.Set("test")

	require.Equal(t, "test", MongoDatabase.Get())

	Configure(true)

	require.Equal(t, project.Name, MongoDatabase.Get())
}

func TestEnvReplacerShouldConsiderDotAsUnderline(t *testing.T) {
	Configure(true)

	require.Equal(t, false, AuditEnabled.GetBool())

	os.Setenv("DECKARD_AUDIT_ENABLED", "true")

	defer os.Unsetenv("DECKARD_AUDIT_ENABLED")

	require.Equal(t, true, AuditEnabled.GetBool())
}

func TestEnvWithoutPrefixShouldReturnDefaultValue(t *testing.T) {
	Configure(true)

	require.Equal(t, false, AuditEnabled.GetBool())

	os.Setenv("AUDIT_ENABLED", "true")

	require.Equal(t, false, AuditEnabled.GetBool())
}

func TestDefaultValues(t *testing.T) {
	Configure(true)

	// Deckard configuration
	require.Equal(t, "MEMORY", StorageType.Get())
	require.Equal(t, "MEMORY", CacheType.Get())

	// gRPC server
	require.Equal(t, true, GrpcEnabled.GetBool())
	require.Equal(t, 8081, GrpcPort.GetInt())

	// Redis configurations
	require.Equal(t, "localhost", RedisAddress.Get())
	require.Equal(t, 6379, RedisPort.GetInt())
	require.Equal(t, 0, RedisDB.GetInt())

	// Audit
	require.Equal(t, false, AuditEnabled.GetBool())

	// ElasticSearch
	require.Equal(t, "http://localhost:9200/", ElasticAddress.Get())

	// MongoDB
	require.Equal(t, "localhost:27017", MongoAddresses.Get())
	require.Equal(t, project.Name, MongoDatabase.Get())
	require.Equal(t, "queue", MongoCollection.Get())
	require.Equal(t, "queue_configuration", MongoQueueConfigurationCollection.Get())
	require.Equal(t, false, MongoSsl.GetBool())

	// Environment
	require.Equal(t, false, DebugEnabled.GetBool())

	// Housekeeper
	require.Equal(t, true, HousekeeperEnabled.GetBool())
	require.Equal(t, 1*time.Second, HousekeeperTaskTimeoutDelay.GetDuration(), HousekeeperTaskTimeoutDelay.GetDuration())
	require.Equal(t, 1*time.Second, HousekeeperTaskUnlockDelay.GetDuration())
	require.Equal(t, 1*time.Second, HousekeeperTaskUpdateDelay.GetDuration())
	require.Equal(t, 1*time.Second, HousekeeperTaskTTLDelay.GetDuration())
	require.Equal(t, 1*time.Second, HousekeeperTaskMaxElementsDelay.GetDuration())
	require.Equal(t, 60*time.Second, HousekeeperTaskMetricsDelay.GetDuration())
}
