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

// The tests below reproduce the exact regression from
// https://github.com/takenet/deckard/issues/51: when a ViperConfigKey has a
// Default set, viper.SetDefault() is called for both the main key AND every
// alias (see Configure()), which makes viper.IsSet() return true for all of
// them even if no real value was ever provided. The old getters checked
// viper.IsSet(config.Key) first and returned immediately, so an override
// applied only to an alias (e.g. DECKARD_MONGODB_DATABASE, bound to the
// "mongodb.database" alias of the "mongo.database" key) was never seen.
//
// These tests set defaults on both the key and its aliases (mirroring
// Configure()) and confirm that overriding only an alias is correctly
// reflected by Get()/GetBool()/GetInt()/GetDuration(), instead of silently
// falling back to the default.

func TestViperConfigKey_Get_AliasOverridesDefault(t *testing.T) {
	defer viper.Reset()

	config := &ViperConfigKey{
		Key:     "test_key_with_default",
		Aliases: []string{"alias1", "alias2"},
		Default: "default_value",
	}

	// Simulate Configure(): defaults are set for the main key and all aliases.
	viper.SetDefault(config.GetKey(), config.GetDefault())
	for _, alias := range config.GetAliases() {
		viper.SetDefault(alias, config.GetDefault())
	}

	// No explicit value anywhere: Get() must return the default.
	require.Equal(t, "default_value", config.Get())

	// Overriding only an alias (as an env var mapped to an alias would) must
	// be honored, even though viper.IsSet(config.Key) is true due to the
	// default set above.
	viper.Set("alias1", "overridden_value")
	require.Equal(t, "overridden_value", config.Get())

	// Once cleared, it must fall back to the default again.
	viper.Set("alias1", nil)
	require.Equal(t, "default_value", config.Get())

	// Overriding the main key directly must still take precedence.
	viper.Set(config.GetKey(), "main_key_value")
	require.Equal(t, "main_key_value", config.Get())
}

func TestViperConfigKey_GetBool_AliasOverridesDefault(t *testing.T) {
	defer viper.Reset()

	config := &ViperConfigKey{
		Key:     "test_bool_key_with_default",
		Aliases: []string{"alias1", "alias2"},
		Default: false,
	}

	viper.SetDefault(config.GetKey(), config.GetDefault())
	for _, alias := range config.GetAliases() {
		viper.SetDefault(alias, config.GetDefault())
	}

	require.Equal(t, false, config.GetBool())

	viper.Set("alias1", true)
	require.Equal(t, true, config.GetBool())
}

func TestViperConfigKey_GetInt_AliasOverridesDefault(t *testing.T) {
	defer viper.Reset()

	config := &ViperConfigKey{
		Key:     "test_int_key_with_default",
		Aliases: []string{"alias1", "alias2"},
		Default: 100,
	}

	viper.SetDefault(config.GetKey(), config.GetDefault())
	for _, alias := range config.GetAliases() {
		viper.SetDefault(alias, config.GetDefault())
	}

	require.Equal(t, 100, config.GetInt())

	viper.Set("alias1", 250)
	require.Equal(t, 250, config.GetInt())
}

func TestViperConfigKey_GetDuration_AliasOverridesDefault(t *testing.T) {
	defer viper.Reset()

	config := &ViperConfigKey{
		Key:     "test_duration_key_with_default",
		Aliases: []string{"alias1", "alias2"},
		Default: "5s",
	}

	viper.SetDefault(config.GetKey(), config.GetDefault())
	for _, alias := range config.GetAliases() {
		viper.SetDefault(alias, config.GetDefault())
	}

	require.Equal(t, 5*time.Second, config.GetDuration())

	viper.Set("alias1", "10s")
	require.Equal(t, 10*time.Second, config.GetDuration())
}

func TestViperConfigKey_GetDuration_InvalidStringFallsBackToDefault(t *testing.T) {
	defer viper.Reset()

	config := &ViperConfigKey{
		Key:     "test_duration_invalid",
		Aliases: []string{"alias1"},
		Default: "5s",
	}

	viper.SetDefault(config.GetKey(), config.GetDefault())
	for _, alias := range config.GetAliases() {
		viper.SetDefault(alias, config.GetDefault())
	}

	// Sanity: unset returns default.
	require.Equal(t, 5*time.Second, config.GetDuration())

	// Setting the key to an unparseable string must NOT override the default.
	viper.Set(config.GetKey(), "not-a-duration")
	require.Equal(t, 5*time.Second, config.GetDuration(),
		"invalid duration string must not override the configured default")
}
