package main

import (
	"testing"
)

func TestExtractVersionFromPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "environment variable",
			input:    "",
			expected: "go1.24.3", // default when GO_COV_VERSION not set
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionFromPath()
			if result != tt.expected {
				t.Errorf("extractVersionFromPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestVersionFormat(t *testing.T) {
	validVersions := []string{
		"go1.21.0",
		"go1.22.1", 
		"go1.23.2",
		"go1.24.3",
	}

	for _, version := range validVersions {
		t.Run(version, func(t *testing.T) {
			// Test that version follows expected format
			if len(version) < 6 {
				t.Errorf("Version %s too short", version)
			}
			if version[:2] != "go" {
				t.Errorf("Version %s should start with 'go'", version)
			}
		})
	}
}