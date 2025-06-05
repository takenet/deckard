package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestMongoDatabaseEnvironmentVariable(t *testing.T) {
	// Import viper for debugging
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
	
	// Debug: let's see what viper sees
	t.Logf("MongoDatabase key: %s", MongoDatabase.GetKey())
	t.Logf("MongoDatabase aliases: %v", MongoDatabase.GetAliases())
	
	// Let's check what viper sees for each key
	t.Logf("viper.IsSet(mongo.database): %v", viper.IsSet("mongo.database"))
	t.Logf("viper.IsSet(mongodb.database): %v", viper.IsSet("mongodb.database"))
	t.Logf("viper.GetString(mongo.database): %s", viper.GetString("mongo.database"))
	t.Logf("viper.GetString(mongodb.database): %s", viper.GetString("mongodb.database"))
	
	require.Equal(t, "custom_db_1", MongoDatabase.Get())

	// Test with DECKARD_MONGO_DATABASE environment variable
	os.Unsetenv("DECKARD_MONGODB_DATABASE")
	os.Setenv("DECKARD_MONGO_DATABASE", "custom_db_2")
	Configure(true)
	require.Equal(t, "custom_db_2", MongoDatabase.Get())
}