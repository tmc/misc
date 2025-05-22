package main

import (
	"os"
	"testing"

	"github.com/tmc/misc/ts2go/internal/cdn"
)

func TestCacheHandling(t *testing.T) {
	// Just test the cache handling
	_, err := cdn.FetchTypeScript()
	if err != nil {
		t.Fatalf("Failed to fetch TypeScript: %v", err)
	}

	// Call it again to test the cached version
	cachedPath, err := cdn.FetchTypeScript()
	if err != nil {
		t.Fatalf("Failed to fetch cached TypeScript: %v", err)
	}

	// Verify the file exists
	if _, err := os.Stat(cachedPath); os.IsNotExist(err) {
		t.Fatalf("Cache file does not exist: %v", err)
	}

	// The test passes if we get here without segfaults
}