package metrics

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
	"time"

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

func TestListenAndServe(t *testing.T) {
	// Start the HTTP server in a separate goroutine
	go ListenAndServe()

	var err error
	var resp *http.Response
	var body []byte

	// Retry 10 times to wait for the HTTP server to start
	for i := 0; i < 10; i++ {
		<-time.After(5 * time.Millisecond)

		// Send a GET request to the metrics endpoint
		resp, err = http.Get(fmt.Sprintf("http://localhost:%d%s", config.MetricsPort.GetInt(), config.MetricsPath.Get()))
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		// Check if the response status code is 200 OK
		if resp.StatusCode != http.StatusOK {
			continue
		}

		body, _ = ioutil.ReadAll(resp.Body)

		break
	}

	if err != nil {
		t.Fatalf("Error sending request to metrics endpoint: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Unexpected status code: got %v, want %v", resp.StatusCode, http.StatusOK)
	}

	require.Contains(t, string(body), "target_info")

}
