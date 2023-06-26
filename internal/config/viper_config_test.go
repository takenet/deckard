package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestViperConfigKey_Set(t *testing.T) {
	defer viper.Reset()

	// Create a new instance of ViperConfigKey for testing
	config := &ViperConfigKey{
		Key:     "test_key",
		Aliases: []string{"alias1", "alias2"},
	}

	// Set the value using the Set() method
	expectedValue := "test_value"
	config.Set(expectedValue)

	// Check if the value was set correctly
	actualValue := viper.GetString(config.GetKey())
	require.Equal(t, expectedValue, actualValue, "Set() failed: expected value %s, got %s", expectedValue, actualValue)

	// Check if the value was set correctly for the aliases
	viper.Set(config.GetKey(), nil)
	for _, alias := range config.GetAliases() {
		viper.Set(alias, expectedValue)
		aliasValue := config.Get()
		require.Equal(t, expectedValue, aliasValue, "Set() failed for alias %s: expected value %s, got %s", alias, expectedValue, aliasValue)
	}
}

func TestViperConfigKey_Get(t *testing.T) {
	defer viper.Reset()

	// Create a new instance of ViperConfigKey for testing
	config := &ViperConfigKey{
		Key:     "test_key",
		Aliases: []string{"alias1", "alias2"},
	}

	require.Equal(t, "", config.Get())

	// Set the value using the Set() method
	expectedValue := "test_value"
	viper.Set(config.GetKey(), expectedValue)

	// Check if the Get() method returns the correct value
	actualValue := config.Get()
	require.Equal(t, expectedValue, actualValue, "Get() failed: expected value %s, got %s", expectedValue, actualValue)

	// Check if the Get() method returns the correct value for the aliases
	viper.Set(config.GetKey(), nil)
	for _, alias := range config.GetAliases() {
		viper.Set(alias, expectedValue)
		aliasValue := config.Get()
		require.Equal(t, expectedValue, aliasValue, "Get() failed for alias %s: expected value %s, got %s", alias, expectedValue, aliasValue)
	}
}

func TestViperConfigKey_GetDuration(t *testing.T) {
	defer viper.Reset()

	// Create a new instance of ViperConfigKey for testing
	config := &ViperConfigKey{
		Key:     "test_key",
		Aliases: []string{"alias1", "alias2"},
	}

	require.Equal(t, time.Duration(0), config.GetDuration())

	// Set the duration using the Set() method
	expectedDuration := time.Second * 10
	viper.Set(config.GetKey(), expectedDuration)

	// Check if the GetDuration() method returns the correct value
	actualDuration := config.GetDuration()
	require.Equal(t, expectedDuration, actualDuration, "GetDuration() failed: expected duration %s, got %s", expectedDuration, actualDuration)

	// Check if the GetDuration() method returns the correct value for the aliases
	viper.Set(config.GetKey(), nil)
	for _, alias := range config.GetAliases() {
		viper.Set(alias, expectedDuration)
		aliasDuration := config.GetDuration()
		require.Equal(t, expectedDuration, aliasDuration, "GetDuration() failed for alias %s: expected duration %s, got %s", alias, expectedDuration, aliasDuration)
	}
}

func TestViperConfigKey_GetBool(t *testing.T) {
	defer viper.Reset()

	// Create a new instance of ViperConfigKey for testing
	config := &ViperConfigKey{
		Key:     "test_key",
		Aliases: []string{"alias1", "alias2"},
	}

	require.Equal(t, false, config.GetBool())

	// Set the boolean value using the Set() method
	expectedBool := true
	viper.Set(config.GetKey(), expectedBool)

	// Check if the GetBool() method returns the correct value
	actualBool := config.GetBool()
	require.Equal(t, expectedBool, actualBool, "GetBool() failed: expected bool %t, got %t", expectedBool, actualBool)

	// Check if the GetBool() method returns the correct value for the aliases
	viper.Set(config.GetKey(), nil)
	for _, alias := range config.GetAliases() {
		viper.Set(alias, expectedBool)
		aliasBool := config.GetBool()
		require.Equal(t, expectedBool, aliasBool, "GetBool() failed for alias %s: expected bool %t, got %t", alias, expectedBool, aliasBool)
	}
}

func TestViperConfigKey_GetInt(t *testing.T) {
	defer viper.Reset()

	// Create a new instance of ViperConfigKey for testing
	config := &ViperConfigKey{
		Key:     "test_key",
		Aliases: []string{"alias1", "alias2"},
	}

	require.Equal(t, 0, config.GetInt())

	// Set the integer value using the Set() method
	expectedInt := 42
	viper.Set(config.GetKey(), expectedInt)

	// Check if the GetInt() method returns the correct value
	actualInt := config.GetInt()
	require.Equal(t, expectedInt, actualInt, "GetInt() failed: expected int %d, got %d", expectedInt, actualInt)

	// Check if the GetInt() method returns the correct value for the aliases
	viper.Set(config.GetKey(), nil)
	for _, alias := range config.GetAliases() {
		viper.Set(alias, expectedInt)
		aliasInt := config.GetInt()
		require.Equal(t, expectedInt, aliasInt, "GetInt() failed for alias %s: expected int %d, got %d", alias, expectedInt, aliasInt)
	}
}
