package rrweb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsingTestFiles(t *testing.T) {
	// ensure all testdata/*.json files are parsed without error (unless they are expected to fail given their name).
	files, err := filepath.Glob("testdata/*.json")
	if err != nil {
		t.Fatalf("failed to glob testdata files: %v", err)
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("failed to read file %s: %v", file, err)
			}
			var events []EventWithTime
			err = json.Unmarshal(data, &events)
			if err != nil {
				if strings.Contains(file, "fail") {
					t.Logf("expected failure for file %s: %v", file, err)
				} else {
					t.Fatalf("failed to unmarshal file %s: %v", file, err)
				}
			} else if strings.Contains(file, "fail") {
				t.Fatalf("expected failure but succeeded for file %s", file)
			}
		})
	}
}
