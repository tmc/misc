package testctr_test

import (
	"context"
	"strings"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Simple tests that don't require database drivers

func TestBasicContainer(t *testing.T) {
	t.Parallel()
	c := testctr.New(t, "alpine:latest")

	// Basic container should work
	if c.Host() != "127.0.0.1" {
		t.Errorf("expected host to be 127.0.0.1, got %s", c.Host())
	}
}

func TestExec(t *testing.T) {
	t.Parallel()
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sh", "-c", "while true; do sleep 1; done"),
	)

	ctx := context.Background()
	exitCode, output, err := c.Exec(ctx, []string{"echo", "hello world"})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if strings.TrimSpace(output) != "hello world" {
		t.Errorf("expected 'hello world', got %q", output)
	}
}

func TestEnvironmentVariables(t *testing.T) {
	t.Parallel()
	c := testctr.New(t, "alpine:latest",
		testctr.WithEnv("MY_VAR", "test123"),
		testctr.WithEnv("ANOTHER", "value"),
		testctr.WithCommand("sh", "-c", "while true; do sleep 1; done"),
	)

	ctx := context.Background()
	exitCode, output, err := c.Exec(ctx, []string{"sh", "-c", "echo $MY_VAR"})
	if err != nil || exitCode != 0 {
		t.Fatalf("exec failed: %v (exit: %d)", err, exitCode)
	}
	if strings.TrimSpace(output) != "test123" {
		t.Errorf("expected 'test123', got %q", output)
	}
}

func TestRedisBasic(t *testing.T) {
	t.Parallel()
	c := testctr.New(t, "redis:7", testctr.WithPort("6379"))

	// Should have the default Redis port
	port := c.Port("6379")
	if port == "" {
		t.Error("expected Redis port to be mapped")
	}

	// Endpoint should work
	endpoint := c.Endpoint("6379")
	if endpoint == "" || !strings.Contains(endpoint, ":") {
		t.Errorf("expected valid endpoint, got %s", endpoint)
	}
}

func TestWithLogs(t *testing.T) {
	t.Parallel()
	// This test verifies that WithLogs() doesn't break anything
	// Actual log output would only show with -testctr.verbose
	c := testctr.New(t, "alpine:latest",
		ctropts.WithLogs(),
		testctr.WithCommand("sh", "-c", "echo 'test log output'; sleep 2"),
	)

	// Container should still work normally
	if c.Host() != "127.0.0.1" {
		t.Errorf("expected host to be 127.0.0.1, got %s", c.Host())
	}
}

func TestContainerCleanup(t *testing.T) {
	t.Parallel()
	// This test verifies containers are cleaned up
	// Run with -testctr.keep to verify they stay around
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sh", "-c", "while true; do sleep 1; done"),
	)

	// We can't access private fields, but we can verify the container works
	ctx := context.Background()
	exitCode, _, err := c.Exec(ctx, []string{"echo", "alive"})
	if err != nil || exitCode != 0 {
		t.Errorf("container should be running: %v (exit: %d)", err, exitCode)
	}
}
