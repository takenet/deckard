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
	if viper.IsSet(config.Key) {
		return viper.GetString(config.Key)
	}

	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			return viper.GetString(alias)
		}
	}

	return ""
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetDuration() time.Duration {
	if viper.IsSet(config.Key) {
		return viper.GetDuration(config.Key)
	}

	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			return viper.GetDuration(alias)
		}
	}

	return 0
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetBool() bool {
	if viper.IsSet(config.Key) {
		return viper.GetBool(config.Key)
	}

	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			return viper.GetBool(alias)
		}
	}

	return false
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetInt() int {
	if viper.IsSet(config.Key) {
		return viper.GetInt(config.Key)
	}

	for _, alias := range config.GetAliases() {
		if viper.IsSet(alias) {
			return viper.GetInt(alias)
		}
	}

	return 0
}
