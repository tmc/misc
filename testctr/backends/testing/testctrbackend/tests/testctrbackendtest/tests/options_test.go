package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

func TestWithOptions(t *testing.T) {
	t.Parallel()
	// Test basic options with a simple container
	c := testctr.New(t, "alpine:latest",
		testctr.WithEnv("FOO", "bar"),
		testctr.WithEnv("BAZ", "qux"),
		testctr.WithCommand("sh", "-c", "echo Container started && sleep infinity"),
	)

	// Verify container is running and environment variable is set
	ctx := context.Background()
	exitCode, output, err := c.Exec(ctx, []string{"sh", "-c", "echo $FOO"})
	if err != nil {
		t.Fatalf("failed to exec: %v, output: %s", err, output)
	}
	if exitCode != 0 {
		t.Fatalf("exec exited with code %d, output: %s", exitCode, output)
	}
	// Expecting "bar\n" because echo appends a newline
	if output != "bar\n" {
		t.Errorf("expected FOO=bar (with newline), got %q", output)
	}
}

func TestCombinedOptions(t *testing.T) {
	t.Parallel()
	// Combine multiple option sources
	c := testctr.New(t, "redis:7-alpine", // Use a more specific alpine tag for redis
		ctropts.WithEnvMap(map[string]string{
			"REDIS_ARGS": "--maxmemory 256mb",
			"DEBUG_MODE": "true", // Changed to DEBUG_MODE to avoid conflict with actual Redis debug env vars
		}),
		testctr.WithPort("6379"), // Expose standard Redis port
		// Add a wait strategy for Redis, common practice
		ctropts.WithWaitForLog("Ready to accept connections", 10*time.Second),
	)

	// Verify endpoint can be retrieved
	endpoint := c.Endpoint("6379")
	if endpoint == "" {
		t.Fatal("Failed to get Redis endpoint")
	}
	t.Logf("Redis endpoint: %s", endpoint)

	// Verify environment variable (optional, depends on image if it prints env vars)
	ctx := context.Background()
	_, output, _ := c.Exec(ctx, []string{"env"})
	if !strings.Contains(output, "DEBUG_MODE=true") {
		t.Errorf("Expected DEBUG_MODE=true in env output, got:\n%s", output)
	}
}
