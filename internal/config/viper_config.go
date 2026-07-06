package config

import (
	"time"

	"github.com/spf13/viper"
)

type ViperConfigKey struct {
	Key     string
	Default any
	Aliases []string
}

func (config *ViperConfigKey) GetKey() string {
	return config.Key
}

func (config *ViperConfigKey) GetAliases() []string {
	return config.Aliases
}

func (config *ViperConfigKey) GetDefault() any {
	return config.Default
}

func (config *ViperConfigKey) Set(value any) {
	viper.Set(config.GetKey(), value)

	for _, alias := range config.GetAliases() {
		viper.Set(alias, value)
	}
}

// getWithFallback implements the common logic for all getter methods below.
//
// viper.IsSet(key) always returns true for any key that has had viper.SetDefault()
// called on it (see Configure()), regardless of whether an actual environment
// variable/config value was provided. Because every ViperConfigKey's main key AND
// all of its aliases get a default registered, the old implementation's
// `if viper.IsSet(config.Key) { return ... }` always short-circuited on the main
// key and never fell through to check the aliases - so environment variables bound
// only to an alias (e.g. DECKARD_MONGODB_DATABASE for the mongo.database key, whose
// alias is mongodb.database) were silently ignored.
//
// To fix this, we resolve the value for the main key and each alias and only treat
// it as an override if it differs from the default - the first non-default value
// found (main key first, then aliases in order) wins. If everything resolves to the
// default, the default is returned.
func getWithFallback[T comparable](config *ViperConfigKey, defaultVal T, keyGetter func(string) T) T {
	if viper.IsSet(config.Key) {
		keyVal := keyGetter(config.Key)
		if keyVal != defaultVal {
			return keyVal
		}
	}

	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			aliasVal := keyGetter(alias)
			if aliasVal != defaultVal {
				return aliasVal
			}
		}
	}

	return defaultVal
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) Get() string {
	defaultVal := ""
	if val, ok := config.GetDefault().(string); ok {
		defaultVal = val
	}

	return getWithFallback(config, defaultVal, viper.GetString)
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetDuration() time.Duration {
	defaultVal := time.Duration(0)
	if val, ok := config.GetDefault().(string); ok {
		parsed, err := time.ParseDuration(val)
		if err == nil {
			defaultVal = parsed
		}
	}

	// Custom getter so both the default and the resolved values go through the same
	// parsing path (time.ParseDuration), keeping comparisons in getWithFallback consistent.
	//
	// The closure captures defaultVal so it can return defaultVal (rather than 0) when
	// the env var is set to an invalid/unparseable string. Without this, getDuration
	// would return 0 for a typo like DECKARD_HOUSEKEEPER_TASK_TIMEOUT_DELAY=bad, and
	// getWithFallback would compare 0 != defaultVal (e.g. 1s) and return 0 - silently
	// discarding the configured default. Returning defaultVal signals "no valid override"
	// so getWithFallback falls through to check aliases and ultimately returns defaultVal.
	getDuration := func(key string) time.Duration {
		if duration := viper.GetDuration(key); duration != 0 {
			return duration
		}

		if str := viper.GetString(key); str != "" {
			if parsed, err := time.ParseDuration(str); err == nil {
				return parsed
			}
			// Non-empty but unparseable: treat as "no valid override".
			return defaultVal
		}

		return 0
	}

	return getWithFallback(config, defaultVal, getDuration)
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetBool() bool {
	defaultVal := false
	if val, ok := config.GetDefault().(bool); ok {
		defaultVal = val
	}

	return getWithFallback(config, defaultVal, viper.GetBool)
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetInt() int {
	defaultVal := 0
	if val, ok := config.GetDefault().(int); ok {
		defaultVal = val
	}

	return getWithFallback(config, defaultVal, viper.GetInt)
}
