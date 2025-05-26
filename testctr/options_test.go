package testctr_test

import (
	"context"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

func TestWithOptions(t *testing.T) {
	t.Parallel()
	// Test basic options with a simple container
	c := testctr.New(t, "alpine:latest",
		testctr.WithEnv("FOO", "bar"),
		testctr.WithEnv("BAZ", "qux"),
		testctr.WithCommand("sh", "-c", "while true; do sleep 1; done"),
	)

	// Verify container is running
	ctx := context.Background()
	exitCode, output, err := c.Exec(ctx, []string{"sh", "-c", "echo $FOO"})
	if err != nil || exitCode != 0 {
		t.Fatalf("failed to exec: %v (exit: %d)", err, exitCode)
	}
	if output != "bar\n" {
		t.Errorf("expected FOO=bar, got %q", output)
	}
}

func TestCombinedOptions(t *testing.T) {
	t.Parallel()
	// Combine multiple option sources
	c := testctr.New(t, "redis:7",
		ctropts.WithEnvMap(map[string]string{
			"REDIS_ARGS": "--maxmemory 256mb",
			"DEBUG":      "true",
		}),
		testctr.WithPort("6380/tcp"),
	)

	t.Logf("Redis endpoint: %s", c.Endpoint("6379"))
}
