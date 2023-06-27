package utils

import "testing"

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		pattern  string
		expected bool
	}{
		{
			name:     "match",
			value:    "hello",
			pattern:  "h*o",
			expected: true,
		},
		{
			name:     "no match",
			value:    "world",
			pattern:  "h*o",
			expected: false,
		},
		{
			name:     "empty pattern",
			value:    "hello",
			pattern:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchGlob(tt.value, tt.pattern)

			if got != tt.expected {
				t.Errorf("MatchGlob() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
