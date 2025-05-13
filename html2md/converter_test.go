package main

import (
	"strings"
	"testing"
)

func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple paragraph",
			input:    "<p>Hello world</p>",
			expected: "Hello world",
		},
		{
			name:     "heading",
			input:    "<h1>Title</h1>",
			expected: "# Title",
		},
		{
			name:     "bold text",
			input:    "<p>This is <strong>bold</strong> text</p>",
			expected: "This is **bold** text",
		},
		{
			name:     "link",
			input:    "<a href=\"https://example.com\">Example</a>",
			expected: "[Example](https://example.com)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.input)
			result, err := convert(reader, false)
			if err != nil {
				t.Fatalf("convert returned error: %v", err)
			}

			// Trim whitespace for comparison
			result = strings.TrimSpace(result)
			expected := strings.TrimSpace(tc.expected)

			if result != expected {
				t.Errorf("expected: %q, got: %q", expected, result)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sanitize bool
		expected string
	}{
		{
			name:     "with script tags no sanitize",
			input:    "<p>Text with <script>alert('evil');</script> script</p>",
			sanitize: false,
			expected: "Text with  script",
		},
		{
			name:     "with script tags with sanitize",
			input:    "<p>Text with <script>alert('evil');</script> script</p>",
			sanitize: true,
			expected: "Text with script",
		},
		{
			name:     "with onclick attribute no sanitize",
			input:    "<a href=\"#\" onclick=\"evil()\">Click me</a>",
			sanitize: false,
			expected: "Click me",
		},
		{
			name:     "with onclick attribute with sanitize",
			input:    "<a href=\"#\" onclick=\"evil()\">Click me</a>",
			sanitize: true,
			expected: "Click me",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.input)
			result, err := convert(reader, tc.sanitize)
			if err != nil {
				t.Fatalf("convert returned error: %v", err)
			}

			// Trim whitespace for comparison
			result = strings.TrimSpace(result)
			expected := strings.TrimSpace(tc.expected)

			if result != expected {
				t.Errorf("expected: %q, got: %q", expected, result)
			}
		})
	}
}
