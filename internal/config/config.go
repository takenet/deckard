package config

import (
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/takenet/deckard/internal/project"
)

type ConfigKey interface {
	GetKey() string
	GetDefault() any
	Set(any)
	GetAliases() []string

	Get() string
	GetBool() bool
	GetInt() int
	GetDuration() time.Duration
}

func Create(key ConfigKey) ConfigKey {
	defaultConfigs = append(defaultConfigs, key)

	return key
}

var (
	defaultConfigs = []ConfigKey{}

	configured = false

	configMutex = &sync.Mutex{}
)

func Configure(reset ...bool) {
	configMutex.Lock()
	defer configMutex.Unlock()

	if configured && len(reset) == 0 {
		return
	}

	viper.Reset()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	viper.SetEnvPrefix(project.Name)
	viper.AutomaticEnv()

	// Default configuration values
	for _, config := range defaultConfigs {
		viper.SetDefault(config.GetKey(), config.GetDefault())

		for _, alias := range config.GetAliases() {
			viper.SetDefault(alias, config.GetDefault())
		}
	}

	configured = true
}
