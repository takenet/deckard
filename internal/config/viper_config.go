package config

import (
	"time"

	"github.com/spf13/viper"
)

type ViperConfigKey struct {
	Key     string
	Default any
}

func (config *ViperConfigKey) GetKey() string {
	return config.Key
}

func (config *ViperConfigKey) GetDefault() any {
	return config.Default
}

func (config *ViperConfigKey) Set(value any) {
	viper.Set(config.GetKey(), value)
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) Get() string {
	return viper.GetString(config.Key)
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetDuration() time.Duration {
	return viper.GetDuration(config.Key)
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetBool() bool {
	return viper.GetBool(config.Key)
}

// Should never be called before config is initialized using config.Configure()
func (config *ViperConfigKey) GetInt() int {
	return viper.GetInt(config.Key)
}
