package testctr_test

import (
	"testing"

	"github.com/tmc/misc/testctr"
)

func TestLogging_AlwaysVisible(t *testing.T) {
	t.Parallel()

	// This test demonstrates that some logs are always visible
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sh", "-c", "echo 'hello world' && sleep 1"),
	)

	// The container creation logs should be visible even without verbose mode
	// Look for logs like:
	// testctr: [alpine:latest] applying default 'running' check
	// testctr: [alpine:latest] container ... is running after...

	if c.ID() == "" {
		t.Fatal("Expected container to have an ID")
	}
}

func TestLogging_VerboseOnly(t *testing.T) {
	t.Parallel()

	// To see verbose logs, run with: go test -v -run TestLogging_VerboseOnly -testctr.verbose
	// You'll see additional logs like:
	// testctr: [redis:7-alpine] container ... state: running=true, status=running, exitCode=0
	// testctr: [redis:7-alpine] container ... port mapping: 6379/tcp -> ...

	c := testctr.New(t, "redis:7-alpine",
		testctr.WithPort("6379"),
	)

	// Inspect will log verbose details only when -testctr.verbose is set
	_, err := c.Inspect()
	if err != nil {
		t.Fatalf("Failed to inspect: %v", err)
	}

	t.Logf("Container %s created successfully", c.ID()[:12])
}
