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

// getWithFallback is a helper that implements the common logic for all getter methods.
// It checks the main key and aliases for values different from the default, returning
// the first override found, or the default if no overrides exist.
func getWithFallback[T comparable](config *ViperConfigKey, defaultVal T, keyGetter func(string) T) T {
	// Check main key - if it differs from default, use it (environment variable takes precedence)
	if viper.IsSet(config.Key) {
		keyVal := keyGetter(config.Key)
		if keyVal != defaultVal {
			return keyVal
		}
	}

	// Check aliases - if any differs from default, use it (environment variable takes precedence)
	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			aliasVal := keyGetter(alias)
			if aliasVal != defaultVal {
				return aliasVal
			}
		}
	}

	// Return default value
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

	// Use a custom getter that ensures consistent parsing behavior
	getDuration := func(key string) time.Duration {
		if viper.IsSet(key) {
			// Try viper's built-in parsing first
			if duration := viper.GetDuration(key); duration != 0 {
				return duration
			}
			// Fall back to manual parsing if viper returns 0 but key is set
			if str := viper.GetString(key); str != "" {
				if parsed, err := time.ParseDuration(str); err == nil {
					return parsed
				}
			}
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
