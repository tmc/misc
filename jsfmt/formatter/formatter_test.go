package formatter

import (
	"bytes"
	"testing"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Simple function",
			input: `function test(){return 1;}`,
			expected: "function test() {\n  return 1;\n}\n",
		},
		{
			name: "If-else statement",
			input: `if(x){y();}else{z();}`,
			expected: "if (x) {\n  y();\n} else {\n  z();\n}\n",
		},
		{
			name: "Operators",
			input: `const x=1+2*3;`,
			expected: "const x = 1 + 2 * 3;\n",
		},
		{
			name: "Object literal",
			input: `const obj={a:1,b:2};`,
			expected: "const obj = { a: 1, b: 2 };\n",
		},
		{
			name: "Array literal",
			input: `const arr=[1,2,3];`,
			expected: "const arr = [1, 2, 3];\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the native formatter for testing to avoid Node.js dependency
			result, err := FormatNative([]byte(tt.input), 2)
			if err != nil {
				t.Fatalf("FormatNative() error = %v", err)
			}

			// Normalize line endings for consistent testing
			expected := bytes.ReplaceAll([]byte(tt.expected), []byte("\r\n"), []byte("\n"))
			result = bytes.ReplaceAll(result, []byte("\r\n"), []byte("\n"))

			if !bytes.Equal(result, expected) {
				t.Errorf("FormatNative() = %q, want %q", result, expected)
			}
		})
	}
}

func TestIsFormatted(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectFormatted bool
	}{
		{
			name:           "Already formatted",
			input:          "function test() {\n  return 1;\n}\n",
			expectFormatted: true,
		},
		{
			name:           "Not formatted",
			input:          "function test(){return 1;}",
			expectFormatted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip this test if it relies on external formatter
			t.Skip("Requires Node.js and prettier")
			
			formatted, err := IsFormatted(bytes.NewReader([]byte(tt.input)), 2)
			if err != nil {
				t.Fatalf("IsFormatted() error = %v", err)
			}
			if formatted != tt.expectFormatted {
				t.Errorf("IsFormatted() = %v, want %v", formatted, tt.expectFormatted)
			}
		})
	}
}