package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeMapShouldChangeSuccessfully(t *testing.T) {
	data := NewQueueMetricsMap()

	require.Empty(t, data.OldestElement)

	newMap := make(map[string]int64, 0)
	newMap["a"] = int64(123)

	data.UpdateOldestElementMap(newMap)

	require.Equal(t, int64(123), data.OldestElement["a"])
}

func TestChangeMapWithNilMapShouldEmptyMap(t *testing.T) {
	data := NewQueueMetricsMap()

	data.OldestElement["a"] = int64(123)

	require.NotEmpty(t, data.OldestElement)

	data.UpdateOldestElementMap(nil)

	require.Empty(t, data.OldestElement)
}

func TestChangeMapWithMissingQueueShouldKeepElementAsZero(t *testing.T) {
	data := NewQueueMetricsMap()

	data.OldestElement["a"] = int64(123)
	data.OldestElement["b"] = int64(432)

	newMap := make(map[string]int64, 0)
	newMap["b"] = 123

	data.UpdateOldestElementMap(newMap)

	require.Contains(t, data.OldestElement, "a")
	require.Equal(t, int64(0), data.OldestElement["a"])
	require.Equal(t, int64(123), data.OldestElement["b"])
}
