// Package scripttest provides utilities for testing MCP tools using script files.
package scripttest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ScriptMessage represents a message in a test script.
type ScriptMessage struct {
	Direction string // "mcp-in" or "mcp-out"
	Content   string // The message content
}

// TestScript runs a test script against a handler function.
func TestScript(t *testing.T, handler func([]byte) ([]byte, error), scriptPath string) {
	t.Helper()

	// Read script file
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}

	// Process script lines
	scanner := bufio.NewScanner(bytes.NewReader(script))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse direction and message
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			t.Errorf("line %d: invalid format", lineNum)
			continue
		}

		dir, msg := parts[0], parts[1]
		switch dir {
		case "mcp-in":
			// Send message to handler
			resp, err := handler([]byte(msg))
			if err != nil {
				t.Errorf("line %d: handler error: %v", lineNum, err)
				continue
			}

			// Read next line for expected output
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					t.Fatal(err)
				}
				t.Fatalf("line %d: unexpected end of script", lineNum)
			}
			lineNum++

			// Parse expected output
			line := strings.TrimSpace(scanner.Text())
			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 || parts[0] != "mcp-out" {
				t.Errorf("line %d: expected mcp-out, got %q", lineNum, line)
				continue
			}

			// Compare output
			got := string(resp)
			want := parts[1]
			if !compareJSON(t, got, want) {
				t.Errorf("line %d:\ngot:  %s\nwant: %s", lineNum, got, want)
			}

		case "mcp-out":
			t.Errorf("line %d: unexpected mcp-out", lineNum)
		default:
			t.Errorf("line %d: unknown direction %q", lineNum, dir)
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

// compareJSON compares two JSON strings for equality, ignoring formatting.
func compareJSON(t *testing.T, got, want string) bool {
	t.Helper()

	var gotJSON, wantJSON interface{}
	if err := json.Unmarshal([]byte(got), &gotJSON); err != nil {
		t.Errorf("invalid JSON response: %v", err)
		return false
	}
	if err := json.Unmarshal([]byte(want), &wantJSON); err != nil {
		t.Errorf("invalid JSON in script: %v", err)
		return false
	}

	return jsonEqual(gotJSON, wantJSON)
}

// jsonEqual recursively compares two JSON values for equality.
func jsonEqual(a, b interface{}) bool {
	switch va := a.(type) {
	case map[string]interface{}:
		vb, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		if len(va) != len(vb) {
			return false
		}
		for k, v := range va {
			if !jsonEqual(v, vb[k]) {
				return false
			}
		}
		return true

	case []interface{}:
		vb, ok := b.([]interface{})
		if !ok {
			return false
		}
		if len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !jsonEqual(va[i], vb[i]) {
				return false
			}
		}
		return true

	default:
		return a == b
	}
}

// FindTestScripts returns all test script files in a directory.
func FindTestScripts(dir string) ([]string, error) {
	pattern := filepath.Join(dir, "*.txt")
	return filepath.Glob(pattern)
}
