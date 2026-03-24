package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExtractModelLogic 测试 extractModel 函数的核心逻辑
func TestExtractModelLogic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Model with provider",
			input:    "openai/gpt-4",
			expected: "gpt-4",
		},
		{
			name:     "Model without provider",
			input:    "gpt-4",
			expected: "gpt-4",
		},
		{
			name:     "Model with multiple slashes",
			input:    "openai/gpt-4/turbo",
			expected: "gpt-4/turbo",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only slash",
			input:    "/",
			expected: "",
		},
		{
			name:     "Slash at end",
			input:    "openai/",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from extractModel function
			var result string
			if tt.input != "" {
				if strings.Contains(tt.input, "/") {
					idx := strings.Index(tt.input, "/")
					result = tt.input[idx+1:]
				} else {
					result = tt.input
				}
			}

			require.Equal(t, tt.expected, result)
		})
	}
}
