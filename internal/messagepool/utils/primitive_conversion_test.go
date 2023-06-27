package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStrToBool(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		wantErr  bool
	}{
		{
			name:     "valid true input",
			input:    "true",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "valid false input",
			input:    "false",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "invalid input",
			input:    "invalid",
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StrToBool(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("StrToBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.expected {
				t.Errorf("StrToBool() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestElapsedTime(t *testing.T) {
	// Create a time object that is 1 second in the past
	since := time.Now().Add(-time.Second)

	// Call the ElapsedTime function
	got := ElapsedTime(since)

	// Check if the result is within 1 millisecond of 1000
	if got < 999 || got > 1001 {
		t.Errorf("ElapsedTime() = %d, expected 1000 +/- 1", got)
	}
}

func TestNowMs(t *testing.T) {
	// Get the current time in milliseconds
	now := time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))

	<-time.After(1 * time.Millisecond)

	// Call the NowMs function
	got := NowMs()

	// Check if the result is within 1 millisecond of the current time
	if got < now-1 || got > now+1 {
		t.Errorf("NowMs() = %d, expected %d +/- 1", got, now)
	}
}

func TestStrToFloat64(t *testing.T) {
	t.Parallel()

	value, err := StrToFloat64("123.32")

	require.NoError(t, err)

	require.Equal(t, float64(123.32), value)
}

// TODO use property based testing using one of these:
// https://github.com/flyingmutant/rapid - newer and with better API but in alpha development
// https://github.com/leanovate/gopter - didn't like the API and last commit long time ago

func TestStrToInt64(t *testing.T) {
	t.Parallel()

	value, err := StrToInt64("1651984934")

	require.NoError(t, err)

	require.Equal(t, int64(1651984934), value)
}

func TestStrToInt64Error(t *testing.T) {
	t.Parallel()

	_, err := StrToInt64("3f23r")

	require.Error(t, err)
}

func TestStrToInt64RangeError(t *testing.T) {
	t.Parallel()

	// Max int64 + 1
	_, err := StrToInt64("9223372036854775808")

	require.Error(t, err)
}

func TestStrToInt64TypeError(t *testing.T) {
	t.Parallel()

	_, err := StrToInt64("123.32")

	require.Error(t, err)
}

func TestStrToInt32TypeError(t *testing.T) {
	t.Parallel()

	_, err := StrToInt64("123.32")

	require.Error(t, err)
}

func TestMsPrecision(t *testing.T) {
	t.Parallel()

	fixedTime := time.Unix(1610578652, 894654759)

	require.Equal(t, int64(1610578652894654759), fixedTime.UnixNano())

	msPrecision := MsPrecision(&fixedTime)

	require.Equal(t, int64(1610578652894000000), msPrecision.UnixNano())
}

func TestTimeToMs(t *testing.T) {
	t.Parallel()

	fixedTime := time.Unix(1610578652, 894654759)

	require.Equal(t, int64(1610578652894), TimeToMs(&fixedTime))
}

func TestMsToTime(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(1610578652894000000), MsToTime(int64(1610578652894)).UnixNano())
}
