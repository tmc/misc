package testcontainers

import (
	"strings"
	"testing"
)

func TestSimpleTestcontainers(t *testing.T) {
	t.Parallel()
	// Test that we can create containers with testcontainers backend
	c := New(t, "redis:7-alpine")

	output, err := c.Exec("redis-cli", "PING")
	if err != nil {
		t.Fatalf("Failed to ping Redis: %v", err)
	}

	if !strings.Contains(output, "PONG") {
		t.Errorf("Expected PONG response, got: %s", output)
	}
}

func TestAlpineContainer(t *testing.T) {
	t.Parallel()
	c := New(t, "alpine:latest")

	output, err := c.Exec("echo", "hello from testcontainers")
	if err != nil {
		t.Fatalf("Failed to execute echo: %v", err)
	}

	if !strings.Contains(output, "hello from testcontainers") {
		t.Errorf("Unexpected output: %s", output)
	}
}
