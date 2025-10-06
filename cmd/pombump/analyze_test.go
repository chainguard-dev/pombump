package pombump

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		expectError bool
	}{
		{
			name:        "valid text format",
			format:      "text",
			expectError: false,
		},
		{
			name:        "valid json format",
			format:      "json",
			expectError: false,
		},
		{
			name:        "valid yaml format",
			format:      "yaml",
			expectError: false,
		},
		{
			name:        "invalid format",
			format:      "invalid",
			expectError: true,
		},
		{
			name:        "empty format",
			format:      "",
			expectError: true,
		},
		{
			name:        "mixed case format",
			format:      "JSON",
			expectError: true,
		},
		{
			name:        "old human format",
			format:      "human",
			expectError: true,
		},
		{
			name:        "xml format",
			format:      "xml",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOutputFormat(tt.format)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported output format")
				assert.Contains(t, err.Error(), "Supported formats: text, json, yaml")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOutputResults_ErrorHandling(t *testing.T) {
	// Test that outputResults gracefully handles nil analysis - should not error
	err := outputResults(nil, nil, nil, "json")
	assert.NoError(t, err)

	// Test with invalid format (should not error due to default case)
	err = outputResults(nil, nil, nil, "invalid")
	// Should not error due to default case falling back to text output
	// This tests the safety fallback behavior
	assert.NoError(t, err)

	// Test all valid formats handle nil gracefully
	formats := []string{"text", "json", "yaml"}
	for _, format := range formats {
		err = outputResults(nil, nil, nil, format)
		assert.NoError(t, err, "format %s should handle nil analysis gracefully", format)
	}
}