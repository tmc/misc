package main

import (
	"bytes"
	"testing"
)

func TestEscapeTxtarContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal content",
			input:    "Hello world\nThis is normal text",
			expected: "Hello world\nThis is normal text",
		},
		{
			name:     "file marker line",
			input:    "-- fake-file.txt --\nSome content",
			expected: "\\-- fake-file.txt --\nSome content",
		},
		{
			name:     "leading backslash",
			input:    "\\escaped text\nNormal text",
			expected: "\\\\escaped text\nNormal text",
		},
		{
			name:     "multiple escapes",
			input:    "-- file1.txt --\n\\test\n-- file2.txt --",
			expected: "\\-- file1.txt --\n\\\\test\n\\-- file2.txt --",
		},
		{
			name:     "no trailing newline",
			input:    "Hello world",
			expected: "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeTxtarContent([]byte(tt.input))
			if !bytes.Equal(result, []byte(tt.expected)) {
				t.Errorf("escapeTxtarContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestUnescapeTxtarContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal content",
			input:    "Hello world\nThis is normal text",
			expected: "Hello world\nThis is normal text",
		},
		{
			name:     "escaped file marker",
			input:    "\\-- fake-file.txt --\nSome content",
			expected: "-- fake-file.txt --\nSome content",
		},
		{
			name:     "escaped backslash",
			input:    "\\\\escaped text\nNormal text",
			expected: "\\escaped text\nNormal text",
		},
		{
			name:     "multiple unescapes",
			input:    "\\-- file1.txt --\n\\\\test\n\\-- file2.txt --",
			expected: "-- file1.txt --\n\\test\n-- file2.txt --",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unescapeTxtarContent([]byte(tt.input))
			if !bytes.Equal(result, []byte(tt.expected)) {
				t.Errorf("unescapeTxtarContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that escape -> unescape returns original content
	original := "-- fake-file.txt --\n\\escaped content\nNormal line"
	
	escaped := escapeTxtarContent([]byte(original))
	unescaped := unescapeTxtarContent(escaped)
	
	if !bytes.Equal(unescaped, []byte(original)) {
		t.Errorf("Round trip failed: got %q, want %q", unescaped, original)
	}
}