package auth

import (
	"testing"
)

func TestCommaSeparatedStrings_UnmarshalParam(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single value",
			input:    "abc123",
			expected: []string{"abc123"},
		},
		{
			name:     "multiple values",
			input:    "695c2971a9eeb67a47ce5fa0,695c2971a9eeb67a47ce5f55",
			expected: []string{"695c2971a9eeb67a47ce5fa0", "695c2971a9eeb67a47ce5f55"},
		},
		{
			name:     "values with spaces",
			input:    "id1, id2 , id3",
			expected: []string{"id1", "id2", "id3"},
		},
		{
			name:     "trailing comma",
			input:    "id1,id2,",
			expected: []string{"id1", "id2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var css CommaSeparatedStrings
			err := css.UnmarshalParam(tt.input)
			if err != nil {
				t.Errorf("UnmarshalParam() error = %v", err)
				return
			}

			if len(css) != len(tt.expected) {
				t.Errorf("UnmarshalParam() got %d elements, want %d", len(css), len(tt.expected))
				return
			}

			for i, v := range css {
				if v != tt.expected[i] {
					t.Errorf("UnmarshalParam() got[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}
