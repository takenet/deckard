package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMongoDatabaseEnvironmentVariable(t *testing.T) {
	defer func() {
		// Clean up
		_ = os.Unsetenv("DECKARD_MONGODB_DATABASE")
		_ = os.Unsetenv("DECKARD_MONGO_DATABASE")
	}()

	// Clean environment
	_ = os.Unsetenv("DECKARD_MONGODB_DATABASE")
	_ = os.Unsetenv("DECKARD_MONGO_DATABASE")

	// Test default value
	Configure(true)
	require.Equal(t, "deckard", MongoDatabase.Get())

	// Test with DECKARD_MONGODB_DATABASE environment variable (alias)
	_ = os.Setenv("DECKARD_MONGODB_DATABASE", "custom_db_1")
	Configure(true)
	require.Equal(t, "custom_db_1", MongoDatabase.Get())

	// Test with DECKARD_MONGO_DATABASE environment variable (main key)
	_ = os.Unsetenv("DECKARD_MONGODB_DATABASE")
	_ = os.Setenv("DECKARD_MONGO_DATABASE", "custom_db_2")
	Configure(true)
	require.Equal(t, "custom_db_2", MongoDatabase.Get())
}

func TestMongoCollectionEnvironmentVariable(t *testing.T) {
	defer func() {
		// Clean up
		_ = os.Unsetenv("DECKARD_MONGODB_COLLECTION")
		_ = os.Unsetenv("DECKARD_MONGO_COLLECTION")
	}()

	// Clean environment
	_ = os.Unsetenv("DECKARD_MONGODB_COLLECTION")
	_ = os.Unsetenv("DECKARD_MONGO_COLLECTION")

	// Test default value
	Configure(true)
	require.Equal(t, "queue", MongoCollection.Get())

	// Test with DECKARD_MONGODB_COLLECTION environment variable (alias)
	_ = os.Setenv("DECKARD_MONGODB_COLLECTION", "custom_collection")
	Configure(true)
	require.Equal(t, "custom_collection", MongoCollection.Get())

	// Test with DECKARD_MONGO_COLLECTION environment variable (main key)
	_ = os.Unsetenv("DECKARD_MONGODB_COLLECTION")
	_ = os.Setenv("DECKARD_MONGO_COLLECTION", "custom_collection_2")
	Configure(true)
	require.Equal(t, "custom_collection_2", MongoCollection.Get())
}
