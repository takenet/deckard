package audit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignals(t *testing.T) {
	require.Equal(t, Signal("ACK"), ACK)
	require.Equal(t, Signal("NACK"), NACK)
	require.Equal(t, Signal("TIMEOUT"), TIMEOUT)
	require.Equal(t, Signal("REMOVE"), REMOVE)
	require.Equal(t, Signal("UNLOCK"), UNLOCK)
	require.Equal(t, Signal("INSERT_CACHE"), INSERT_CACHE)
	require.Equal(t, Signal("MISSING_STORAGE"), MISSING_STORAGE)
}
