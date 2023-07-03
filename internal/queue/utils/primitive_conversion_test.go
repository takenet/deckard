package utils

import (
	"testing"

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

func TestInt64Ptr(t *testing.T) {
	t.Parallel()

	value := int64(123)

	ptr := Int64Ptr(value)

	require.Equal(t, &value, ptr)
}
