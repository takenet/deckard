package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/config"
)

// Configure logger should not panic

func TestConfigureLoggerWithDebug(t *testing.T) {
	config.DebugEnabled.Set(true)
	ConfigureLogger()

	require.Equal(t, true, config.DebugEnabled.GetBool())

}

func TestConfigureLoggerWithoutDebug(t *testing.T) {
	config.DebugEnabled.Set(false)
	ConfigureLogger()

	require.Equal(t, false, config.DebugEnabled.GetBool())
}

func TestLoggerS(t *testing.T) {
	S(context.Background())
}

func TestLoggerL(t *testing.T) {
	L(context.Background())
}
