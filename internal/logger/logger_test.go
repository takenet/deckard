package logger

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/takenet/deckard/internal/config"
)

// Configure logger should not panic

func TestConfigureLoggerLocal(t *testing.T) {
	viper.Set(config.DEBUG, true)
	ConfigureLogger()
}

func TestConfigureLoggerNotLocal(t *testing.T) {
	viper.Set(config.DEBUG, false)
	ConfigureLogger()
}

func TestLoggerS(t *testing.T) {
	S(context.Background())
}

func TestLoggerL(t *testing.T) {
	L(context.Background())
}
