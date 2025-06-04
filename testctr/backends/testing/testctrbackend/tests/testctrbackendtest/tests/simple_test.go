package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Simple tests that don't require database drivers or complex setups.

func TestBasicContainerCreationAndHost(t *testing.T) {
	t.Parallel()
	c := testctr.New(t, "alpine:latest",
		// Add a command to keep it running for a bit, or it might exit too quickly.
		testctr.WithCommand("sleep", "1"),
	)

	// Basic container should work and have a host.
	// For CLI-based backends, host is typically 127.0.0.1 for port forwards.
	if host := c.Host(); host != "127.0.0.1" {
		// This assumption might change if different backends or network modes are used.
		t.Errorf("expected host to be 127.0.0.1, got %s", host)
	}
}

func TestExecCommandInRunningContainer(t *testing.T) {
	t.Parallel()
	c := testctr.New(t, "alpine:latest",
		// Keep the container running so we can exec into it.
		testctr.WithCommand("sleep", "infinity"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Timeout for the exec
	defer cancel()

	exitCode, output, err := c.Exec(ctx, []string{"echo", "hello world from exec"})
	if err != nil {
		// For "exec failed: ...", it means the exec.Command itself failed (e.g., context deadline)
		// For non-zero exit codes, err will be *exec.ExitError.
		t.Fatalf("Exec command failed: %v, output: %s", err, output)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d. Output: %s", exitCode, output)
	}
	// echo typically appends a newline.
	if strings.TrimSpace(output) != "hello world from exec" {
		t.Errorf("expected 'hello world from exec', got %q", output)
	}
}

func TestEnvironmentVariablesAreSet(t *testing.T) {
	t.Parallel()
	c := testctr.New(t, "alpine:latest",
		testctr.WithEnv("MY_VAR", "test123value"),
		testctr.WithEnv("ANOTHER_VAR", "another_value"),
		testctr.WithCommand("sleep", "infinity"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test MY_VAR
	exitCode, output, err := c.Exec(ctx, []string{"sh", "-c", "echo $MY_VAR"})
	if err != nil || exitCode != 0 { // Also check exitCode here
		t.Fatalf("exec for MY_VAR failed: %v (exit: %d), output: %s", err, exitCode, output)
	}
	if strings.TrimSpace(output) != "test123value" {
		t.Errorf("expected MY_VAR to be 'test123value', got %q", output)
	}

	// Test ANOTHER_VAR
	exitCode, output, err = c.Exec(ctx, []string{"sh", "-c", "echo $ANOTHER_VAR"})
	if err != nil || exitCode != 0 {
		t.Fatalf("exec for ANOTHER_VAR failed: %v (exit: %d), output: %s", err, exitCode, output)
	}
	if strings.TrimSpace(output) != "another_value" {
		t.Errorf("expected ANOTHER_VAR to be 'another_value', got %q", output)
	}
}

func TestRedisContainerPortsAndEndpoint(t *testing.T) {
	t.Parallel()
	// Ensure Redis default wait strategy is applied if not using full redis.Default()
	c := testctr.New(t, "redis:7-alpine", // Using a more specific alpine tag
		testctr.WithPort("6379"), // Expose the standard Redis port
		ctropts.WithWaitForLog("Ready to accept connections", 10*time.Second),
	)

	// Check Port mapping
	mappedPort := c.Port("6379")
	if mappedPort == "" {
		t.Error("expected Redis port 6379 to be mapped to a host port, but got empty string")
	} else {
		t.Logf("Redis container port 6379 mapped to host port %s", mappedPort)
	}

	// Check Endpoint construction
	endpoint := c.Endpoint("6379")
	if endpoint == "" {
		t.Error("expected valid endpoint for Redis port 6379, got empty string")
	} else if !strings.Contains(endpoint, ":") || !strings.HasPrefix(endpoint, "127.0.0.1:") {
		t.Errorf("expected endpoint like '127.0.0.1:xxxxx', got %s", endpoint)
	} else {
		t.Logf("Redis endpoint: %s", endpoint)
	}
}

func TestWithLogsOption(t *testing.T) {
	t.Parallel()
	// This test verifies that WithLogs() doesn't break anything and allows normal operation.
	// Actual log output streaming would typically be verified by observing test output
	// when running with `go test -v -testctr.verbose`.
	c := testctr.New(t, "alpine:latest",
		ctropts.WithLogs(), // Enable log streaming for this container
		testctr.WithCommand("sh", "-c", "echo 'test log output for WithLogsOption'; sleep 1"),
	)

	// Container should still work normally
	if host := c.Host(); host != "127.0.0.1" {
		t.Errorf("expected host to be 127.0.0.1, got %s", host)
	}
	// Perform a simple exec to ensure it's operational
	output := c.ExecSimple("echo", "ping from WithLogs test")
	if output != "ping from WithLogs test" {
		t.Errorf("Container with WithLogs enabled failed basic exec. Got: %s", output)
	}
	t.Log("Container with WithLogs option created and executed successfully.")
}

func TestContainerCleanupIsAutomatic(t *testing.T) {
	t.Parallel()
	// This test verifies containers are cleaned up by t.Cleanup by default.
	// Actual verification would involve checking `docker ps -a` after the test suite,
	// or using the -testctr.keep-failed flag and intentionally failing a test.
	// For this automated test, we just ensure creation and basic operation work.
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sleep", "infinity"), // Keep it running
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exitCode, output, err := c.Exec(ctx, []string{"echo", "cleanup test"})
	if err != nil {
		t.Fatalf("Failed to exec in cleanup test container: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Expected exit code 0, got %d", exitCode)
	}
	if strings.TrimSpace(output) != "cleanup test" {
		t.Errorf("Expected 'cleanup test', got %q", output)
	}
}
