package browser

import (
	"testing"
)

func TestDetectContentType(t *testing.T) {
	testCases := []struct {
		name     string
		data     string
		headers  map[string]string
		expected string
	}{
		{
			name:     "JSON object",
			data:     `{"key": "value"}`,
			headers:  map[string]string{},
			expected: "application/json",
		},
		{
			name:     "JSON array",
			data:     `[1, 2, 3]`,
			headers:  map[string]string{},
			expected: "application/json",
		},
		{
			name:     "Form data",
			data:     "key=value&another=test",
			headers:  map[string]string{},
			expected: "application/x-www-form-urlencoded",
		},
		{
			name:     "Plain text",
			data:     "simple text data",
			headers:  map[string]string{},
			expected: "text/plain",
		},
		{
			name:     "Empty data",
			data:     "",
			headers:  map[string]string{},
			expected: "text/plain",
		},
		{
			name:     "Custom content type in headers",
			data:     "anything",
			headers:  map[string]string{"Content-Type": "custom/type"},
			expected: "custom/type",
		},
		{
			name:     "Case insensitive header",
			data:     "anything",
			headers:  map[string]string{"content-type": "another/type"},
			expected: "another/type",
		},
		{
			name:     "Single form field",
			data:     "field=value",
			headers:  map[string]string{},
			expected: "application/x-www-form-urlencoded",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detectContentType(tc.data, tc.headers)
			if result != tc.expected {
				t.Errorf("Expected content type '%s', got '%s' for data: %s", tc.expected, result, tc.data)
			}
		})
	}
}
