package utils

import "testing"

func TestMinInt64(t *testing.T) {
	tests := []struct {
		name     string
		x        int64
		y        int64
		expected int64
	}{
		{
			name:     "x is smaller",
			x:        1,
			y:        2,
			expected: 1,
		},
		{
			name:     "y is smaller",
			x:        2,
			y:        1,
			expected: 1,
		},
		{
			name:     "x and y are equal",
			x:        1,
			y:        1,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MinInt64(tt.x, tt.y)

			if got != tt.expected {
				t.Errorf("MinInt64() = %d, expected %d", got, tt.expected)
			}
		})
	}
}
