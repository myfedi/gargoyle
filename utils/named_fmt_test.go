package utils

import (
	"testing"
)

func TestNamedFormat(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		params   FormatParams
		expected string
		wantErr  bool
	}{
		{
			name:     "basic substitution",
			format:   "Hello, {{.name}}!",
			params:   FormatParams{"name": "Alice"},
			expected: "Hello, Alice!",
			wantErr:  false,
		},
		{
			name:     "missing param",
			format:   "Hello, {{.name}}!",
			params:   FormatParams{},
			expected: "Hello, <no value>!",
			wantErr:  false,
		},
		{
			name:     "invalid template syntax",
			format:   "Hello, {{.name",
			params:   FormatParams{"name": "Alice"},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NamedFormat(tt.format, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("NamedFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("NamedFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}
