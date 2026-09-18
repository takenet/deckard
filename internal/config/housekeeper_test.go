package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetHousekeeperInstanceIDShouldReturnConfiguredValueWhenSet(t *testing.T) {
	Configure(true)

	HousekeeperDistributedExecutionInstanceID.Set("my-fixed-instance-id")
	defer HousekeeperDistributedExecutionInstanceID.Set("")

	require.Equal(t, "my-fixed-instance-id", GetHousekeeperInstanceID())
}

func TestGetHousekeeperInstanceIDShouldGenerateHostnameBasedFallbackWhenUnset(t *testing.T) {
	Configure(true)

	HousekeeperDistributedExecutionInstanceID.Set("")

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	id := GetHousekeeperInstanceID()

	require.Contains(t, id, hostname)
}

func TestGetHousekeeperInstanceIDShouldGenerateUniqueValuesAcrossCalls(t *testing.T) {
	Configure(true)

	HousekeeperDistributedExecutionInstanceID.Set("")

	first := GetHousekeeperInstanceID()
	second := GetHousekeeperInstanceID()

	require.NotEqual(t, first, second, "fallback instance ID should be unique per call (UnixNano-based)")
}
