package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMongoDatabaseEnvironmentVariable(t *testing.T) {
	defer func() {
		// Clean up
		os.Unsetenv("DECKARD_MONGODB_DATABASE")
		os.Unsetenv("DECKARD_MONGO_DATABASE")
	}()

	// Clean environment
	os.Unsetenv("DECKARD_MONGODB_DATABASE")
	os.Unsetenv("DECKARD_MONGO_DATABASE")

	// Test default value
	Configure(true)
	require.Equal(t, "deckard", MongoDatabase.Get())

	// Test with DECKARD_MONGODB_DATABASE environment variable
	os.Setenv("DECKARD_MONGODB_DATABASE", "custom_db_1")
	Configure(true)
	require.Equal(t, "custom_db_1", MongoDatabase.Get())

	// Test with DECKARD_MONGO_DATABASE environment variable
	os.Unsetenv("DECKARD_MONGODB_DATABASE")
	os.Setenv("DECKARD_MONGO_DATABASE", "custom_db_2")
	Configure(true)
	require.Equal(t, "custom_db_2", MongoDatabase.Get())
}

func TestMongoCollectionEnvironmentVariable(t *testing.T) {
	defer func() {
		// Clean up
		os.Unsetenv("DECKARD_MONGODB_COLLECTION")
		os.Unsetenv("DECKARD_MONGO_COLLECTION")
	}()

	// Clean environment
	os.Unsetenv("DECKARD_MONGODB_COLLECTION")
	os.Unsetenv("DECKARD_MONGO_COLLECTION")

	// Test default value
	Configure(true)
	require.Equal(t, "queue", MongoCollection.Get())

	// Test with DECKARD_MONGODB_COLLECTION environment variable
	os.Setenv("DECKARD_MONGODB_COLLECTION", "custom_collection")
	Configure(true)
	require.Equal(t, "custom_collection", MongoCollection.Get())

	// Test with DECKARD_MONGO_COLLECTION environment variable
	os.Unsetenv("DECKARD_MONGODB_COLLECTION")
	os.Setenv("DECKARD_MONGO_COLLECTION", "custom_collection_2")
	Configure(true)
	require.Equal(t, "custom_collection_2", MongoCollection.Get())
}

func TestMongoBooleanEnvironmentVariable(t *testing.T) {
	defer func() {
		// Clean up
		os.Unsetenv("DECKARD_MONGODB_SSL")
		os.Unsetenv("DECKARD_MONGO_SSL")
	}()

	// Clean environment
	os.Unsetenv("DECKARD_MONGODB_SSL")
	os.Unsetenv("DECKARD_MONGO_SSL")

	// Test default value
	Configure(true)
	require.Equal(t, false, MongoSsl.GetBool())

	// Test with DECKARD_MONGODB_SSL environment variable
	os.Setenv("DECKARD_MONGODB_SSL", "true")
	Configure(true)
	require.Equal(t, true, MongoSsl.GetBool())

	// Test with DECKARD_MONGO_SSL environment variable
	os.Unsetenv("DECKARD_MONGODB_SSL")
	os.Setenv("DECKARD_MONGO_SSL", "true")
	Configure(true)
	require.Equal(t, true, MongoSsl.GetBool())
}

func TestMongoIntegerEnvironmentVariable(t *testing.T) {
	defer func() {
		// Clean up
		os.Unsetenv("DECKARD_MONGODB_MAX_POOL_SIZE")
		os.Unsetenv("DECKARD_MONGO_MAX_POOL_SIZE")
	}()

	// Clean environment
	os.Unsetenv("DECKARD_MONGODB_MAX_POOL_SIZE")
	os.Unsetenv("DECKARD_MONGO_MAX_POOL_SIZE")

	// Test default value
	Configure(true)
	require.Equal(t, 100, MongoMaxPoolSize.GetInt())

	// Test with DECKARD_MONGODB_MAX_POOL_SIZE environment variable
	os.Setenv("DECKARD_MONGODB_MAX_POOL_SIZE", "250")
	Configure(true)
	require.Equal(t, 250, MongoMaxPoolSize.GetInt())

	// Test with DECKARD_MONGO_MAX_POOL_SIZE environment variable
	os.Unsetenv("DECKARD_MONGODB_MAX_POOL_SIZE")
	os.Setenv("DECKARD_MONGO_MAX_POOL_SIZE", "300")
	Configure(true)
	require.Equal(t, 300, MongoMaxPoolSize.GetInt())
}
