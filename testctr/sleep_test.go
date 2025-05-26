package testctr_test

import (
	"context"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

func TestContainerWithLongInternalStartup(t *testing.T) {
	t.Parallel()

	// This test creates a container that has a long internal startup process
	// The container itself starts quickly (becomes "running"), but the application
	// inside takes 15 seconds to be ready

	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	start := time.Now()

	// Create container - this should succeed because the container becomes "running" quickly
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sh", "-c", "sleep 5 && echo 'finally ready' && sleep infinity"),
	)

	elapsed := time.Since(start)

	// Container should start quickly (default wait only checks "running" status)
	if elapsed > 2*time.Second {
		t.Errorf("Expected container to start quickly, but took %v", elapsed)
	}

	t.Logf("Container started in %v", elapsed)

	// Now let's verify that the internal process takes 5 seconds
	// We'll wait for the sleep process to complete
	start2 := time.Now()

	// Use a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Wait for the internal process to complete its startup
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var ready bool
	for !ready {
		select {
		case <-ctx.Done():
			t.Fatalf("got context done: %v", context.Cause(ctx))
		case <-ticker.C:
			output := c.ExecSimple("sh", "-c", "ps aux | grep -v grep | grep 'sleep 5' || echo 'ready'")
			if output == "ready" {
				t.Logf("Internal process is ready after %v", time.Since(start2))
				ready = true
			}
		}
	}

	elapsed2 := time.Since(start2)

	// The internal process should take about 5 seconds
	if elapsed2 < 4*time.Second || elapsed2 > 6*time.Second {
		t.Errorf("Expected internal process to take ~5s, but took %v", elapsed2)
	}

	// Now verify the container is fully functional
	output := c.ExecSimple("echo", "test after startup")
	if output != "test after startup" {
		t.Errorf("Expected 'test after startup', got %q", output)
	}

	t.Logf("Internal process completed startup after %v", elapsed2)
}

func TestContainerWithLongStartupAndWaitCondition(t *testing.T) {
	t.Parallel()

	// This test shows how to use a custom wait condition for containers
	// with long startup processes

	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	start := time.Now()

	// Create container with a custom wait condition that waits for the "finally ready" log
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sh", "-c", "sleep 10 && echo 'finally ready' && sleep infinity"),
		// This wait condition will wait up to 20 seconds for the log message
		ctropts.WithWaitForLog("finally ready", 20*time.Second),
	)

	elapsed := time.Since(start)

	// With the custom wait condition, this should take about 15 seconds
	if elapsed < 9*time.Second || elapsed > 12*time.Second {
		t.Errorf("Expected container to be ready after ~10s, but took %v", elapsed)
	}

	// Verify the container is fully functional
	output := c.ExecSimple("echo", "test after full startup")
	if output != "test after full startup" {
		t.Errorf("Expected 'test after full startup', got %q", output)
	}

	t.Logf("Container fully ready after %v", elapsed)
}

func TestContainerWithQuickStartup(t *testing.T) {
	t.Parallel()

	// This test creates a container that starts quickly
	// to contrast with the long startup test

	start := time.Now()

	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sh", "-c", "echo 'quick start' && sleep infinity"),
	)

	elapsed := time.Since(start)

	// This should succeed quickly
	if elapsed > 2*time.Second {
		t.Errorf("Expected quick startup, but took %v", elapsed)
	}

	// Verify the container is working
	output := c.ExecSimple("echo", "test")
	if output != "test" {
		t.Errorf("Expected 'test', got %q", output)
	}

	t.Logf("Container started successfully in %v", elapsed)
}

func TestContainerWithLongInternalStartup_UsingWaitExec(t *testing.T) {
	t.Parallel()

	// This test demonstrates using WithWaitForExec to wait for internal processes
	// Much cleaner than manual polling!

	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	start := time.Now()

	// Create container with a wait condition that checks if the sleep process has completed
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sh", "-c", "sleep 5 && touch /tmp/ready && echo 'finally ready' && sleep infinity"),
		// Wait for the marker file to exist
		ctropts.WithWaitForExec([]string{"test", "-f", "/tmp/ready"}, 10*time.Second),
	)

	elapsed := time.Since(start)

	// With the wait exec condition, the container should be ready after ~5 seconds
	if elapsed < 5*time.Second || elapsed > 7*time.Second {
		t.Errorf("Expected container to be ready after ~5s, but took %v", elapsed)
	}

	t.Logf("Container ready after %v (using WithWaitForExec)", elapsed)

	// Verify the container is fully functional
	output := c.ExecSimple("echo", "test after startup")
	if output != "test after startup" {
		t.Errorf("Expected 'test after startup', got %q", output)
	}
}
