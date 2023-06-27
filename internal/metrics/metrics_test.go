package metrics

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/config"
)

func TestParseHistogramBuckets(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected []float64
		wantErr  bool
	}{
		{
			name:     "valid buckets",
			data:     "1, 2, 3, 4, 5",
			expected: []float64{1, 2, 3, 4, 5},
			wantErr:  false,
		},
		{
			name:     "invalid bucket value",
			data:     "1, 2, 3, 4, 5, invalid",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "empty buckets",
			data:     "",
			expected: []float64{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHistogramBuckets(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseHistogramBuckets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseHistogramBuckets() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestGetHistogramBuckets(t *testing.T) {
	defaultBuckets, err := parseHistogramBuckets(config.MetricsHistogramBuckets.GetDefault().(string))
	require.NoError(t, err)

	tests := []struct {
		name     string
		buckets  string
		expected []float64
	}{
		{
			name:     "valid buckets",
			buckets:  "1, 2, 3, 4, 5",
			expected: []float64{1, 2, 3, 4, 5},
		},
		{
			name:     "invalid bucket value",
			buckets:  "1, 2, 3, 4, 5, invalid",
			expected: defaultBuckets,
		},
		{
			name:     "empty buckets",
			buckets:  "",
			expected: defaultBuckets,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.MetricsHistogramBuckets.Set(tt.buckets)

			got := getHistogramBuckets()

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("%s getHistogramBuckets() = %v, expected %v", tt.name, got, tt.expected)
			}
		})
	}
}
