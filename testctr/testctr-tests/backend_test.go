package testctr_tests

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/tmc/misc/testctr"
)

var useTestcontainers = flag.Bool("testcontainers", false, "Use testcontainers backend for tests")

// TestWithTestcontainersBackend tests using the testcontainers backend if available
func TestWithTestcontainersBackend(t *testing.T) {
	t.Parallel()

	// Check if we should run testcontainers tests
	if !*useTestcontainers && os.Getenv("TESTCTR_USE_TESTCONTAINERS") != "1" {
		t.Skip("Skipping testcontainers backend test (use -testcontainers flag or set TESTCTR_USE_TESTCONTAINERS=1)")
	}

	// Import testcontainers backend if needed
	// This would normally be done with a build tag or dynamic import

	// Test with Redis using testcontainers backend
	t.Run("Redis", func(t *testing.T) {
		t.Parallel()

		c := testctr.New(t, "redis:7-alpine",
			testctr.WithBackend("testcontainers"),
			testctr.WithPort("6379"),
		)

		// Test basic connectivity
		output := c.ExecSimple("redis-cli", "PING")
		if output != "PONG" {
			t.Errorf("Expected PONG, got %s", output)
		}
	})

	// Test with Alpine using testcontainers backend
	t.Run("Alpine", func(t *testing.T) {
		t.Parallel()

		c := testctr.New(t, "alpine:latest",
			testctr.WithBackend("testcontainers"),
			testctr.WithCommand("sleep", "30"),
		)

		// Test exec
		output := c.ExecSimple("echo", "hello world")
		if output != "hello world" {
			t.Errorf("Expected 'hello world', got %s", output)
		}
	})
}

// TestBackendSelection tests that backend selection works correctly
func TestBackendSelection(t *testing.T) {
	t.Parallel()

	// Test default backend (Docker)
	t.Run("DefaultBackend", func(t *testing.T) {
		t.Parallel()

		c := testctr.New(t, "alpine:latest",
			testctr.WithCommand("sleep", "10"),
		)

		// Should use docker/podman
		runtime := c.Runtime()
		if runtime != "docker" && runtime != "podman" {
			t.Errorf("Expected docker or podman runtime, got %s", runtime)
		}
	})

	// Test explicit testcontainers backend if available
	t.Run("TestcontainersBackend", func(t *testing.T) {
		t.Parallel()

		// Skip if not enabled
		if !*useTestcontainers && os.Getenv("TESTCTR_USE_TESTCONTAINERS") != "1" {
			t.Skip("Testcontainers backend not enabled")
		}

		c := testctr.New(t, "alpine:latest",
			testctr.WithBackend("testcontainers"),
			testctr.WithCommand("sleep", "10"),
		)

		// Should use testcontainers backend
		runtime := c.Runtime()
		if runtime != "testcontainers" {
			t.Errorf("Expected testcontainers runtime, got %s", runtime)
		}
	})
}

// TestBackendCompatibility verifies that both backends work with the same API
func TestBackendCompatibility(t *testing.T) {
	t.Parallel()

	backends := []struct {
		name    string
		enabled bool
		option  testctr.Option
	}{
		{
			name:    "default",
			enabled: true,
			option:  nil, // No backend option means use default
		},
		{
			name:    "testcontainers",
			enabled: os.Getenv("TESTCTR_USE_TESTCONTAINERS") == "1",
			option:  testctr.WithBackend("testcontainers"),
		},
	}

	for _, backend := range backends {
		backend := backend // capture range variable
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			if !backend.enabled {
				t.Skipf("Backend %s not enabled", backend.name)
			}

			// Create options slice
			opts := []testctr.Option{
				testctr.WithCommand("sleep", "30"),
			}
			if backend.option != nil {
				opts = append(opts, backend.option)
			}

			// Test container creation and basic operations
			c := testctr.New(t, "alpine:latest", opts...)

			// Test exec
			output := c.ExecSimple("echo", "test")
			if output != "test" {
				t.Errorf("Expected 'test', got %s", output)
			}

			// Test multiple exec calls
			for i := 0; i < 3; i++ {
				output := c.ExecSimple("echo", fmt.Sprintf("test%d", i))
				expected := fmt.Sprintf("test%d", i)
				if output != expected {
					t.Errorf("Expected %s, got %s", expected, output)
				}
			}
		})
	}
}
