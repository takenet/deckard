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

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) Get() string {
	defaultVal := ""
	if val, ok := config.GetDefault().(string); ok {
		defaultVal = val
	}

	// Check main key - if it differs from default, use it (environment variable takes precedence)
	if viper.IsSet(config.Key) {
		keyVal := viper.GetString(config.Key)
		if keyVal != defaultVal {
			return keyVal
		}
	}

	// Check aliases - if any differs from default, use it (environment variable takes precedence)
	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			aliasVal := viper.GetString(alias)
			if aliasVal != defaultVal {
				return aliasVal
			}
		}
	}

	// Return default value
	return defaultVal
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetDuration() time.Duration {
	defaultVal := time.Duration(0)
	if val, ok := config.GetDefault().(string); ok {
		defaultVal, _ = time.ParseDuration(val)
	}

	// Check main key - if it differs from default, use it (environment variable takes precedence)
	if viper.IsSet(config.Key) {
		keyVal := viper.GetDuration(config.Key)
		if keyVal != defaultVal {
			return keyVal
		}
	}

	// Check aliases - if any differs from default, use it (environment variable takes precedence)
	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			aliasVal := viper.GetDuration(alias)
			if aliasVal != defaultVal {
				return aliasVal
			}
		}
	}

	// Return default value
	return defaultVal
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetBool() bool {
	defaultVal := false
	if val, ok := config.GetDefault().(bool); ok {
		defaultVal = val
	}

	// Check main key - if it differs from default, use it (environment variable takes precedence)
	if viper.IsSet(config.Key) {
		keyVal := viper.GetBool(config.Key)
		if keyVal != defaultVal {
			return keyVal
		}
	}

	// Check aliases - if any differs from default, use it (environment variable takes precedence)
	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			aliasVal := viper.GetBool(alias)
			if aliasVal != defaultVal {
				return aliasVal
			}
		}
	}

	// Return default value
	return defaultVal
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetInt() int {
	defaultVal := 0
	if val, ok := config.GetDefault().(int); ok {
		defaultVal = val
	}

	// Check main key - if it differs from default, use it (environment variable takes precedence)
	if viper.IsSet(config.Key) {
		keyVal := viper.GetInt(config.Key)
		if keyVal != defaultVal {
			return keyVal
		}
	}

	// Check aliases - if any differs from default, use it (environment variable takes precedence)
	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			aliasVal := viper.GetInt(alias)
			if aliasVal != defaultVal {
				return aliasVal
			}
		}
	}

	// Return default value
	return defaultVal
}
