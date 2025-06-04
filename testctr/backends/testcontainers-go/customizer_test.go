package testcontainers_test

import (
	"context"
	"testing"

	"github.com/tmc/misc/testctr"
)

func TestTestcontainersBasic(t *testing.T) {
	t.Parallel()

	// Skip if Docker is not available
	if testing.Short() {
		t.Skip("Skipping testcontainers test in short mode")
	}

	// Create a basic container using testcontainers backend
	c := testctr.New(t, "alpine:latest",
		// Basic testctr options
		testctr.WithCommand("echo", "hello world"),
		testctr.WithEnv("TEST_ENV", "value"),
	)

	// Test that container was created
	if c == nil {
		t.Fatal("Failed to create container")
	}

	// Test that basic testctr functionality works
	exitCode, output, err := c.Exec(context.Background(), []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("Failed to exec: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Unexpected exit code: %d", exitCode)
	}
	if output != "hello\n" {
		t.Fatalf("Unexpected output: %q", output)
	}
}

func TestTestcontainersLongRunning(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping testcontainers test in short mode")
	}

	// Create a container that runs longer
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sleep", "10"),
	)

	// Container should be created successfully
	if c == nil {
		t.Fatal("Failed to create container")
	}

	// Test exec functionality
	exitCode, output, err := c.Exec(context.Background(), []string{"echo", "test"})
	if err != nil {
		t.Fatalf("Failed to exec: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Unexpected exit code: %d", exitCode)
	}
	if output != "test\n" {
		t.Fatalf("Unexpected output: %q", output)
	}
}