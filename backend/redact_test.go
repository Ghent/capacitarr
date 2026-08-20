package main

import "testing"

func TestRedactRequestURI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no query",
			input:    "/api/v1/health",
			expected: "/api/v1/health",
		},
		{
			name:     "unrelated query kept",
			input:    "/api/v1/integrations?page=1",
			expected: "/api/v1/integrations?page=1",
		},
		{
			name:     "apikey redacted",
			input:    "/api/v1/arr?apikey=supersecret",
			expected: "/api/v1/arr?apikey=%5Bredacted%5D",
		},
		{
			name:     "apikey redacted among other params",
			input:    "/api/v1/arr?apikey=supersecret&id=12",
			expected: "/api/v1/arr?apikey=%5Bredacted%5D&id=12",
		},
		{
			name:     "invalid url returned unchanged",
			input:    "://bad",
			expected: "://bad",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := redactRequestURI(tc.input)
			if got != tc.expected {
				t.Errorf("redactRequestURI(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}
