package testcontainers_test

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts/testcontainerbridgeopts"
)

func TestTestcontainersCustomizer(t *testing.T) {
	t.Parallel()

	// Skip if testcontainers backend is not available
	if testing.Short() {
		t.Skip("Skipping testcontainers test in short mode")
	}

	// Create a container with testcontainers-specific customizations
	c := testctr.New(t, "alpine:latest",
		// Use testcontainers backend
		testctr.WithBackend("testcontainers"),

		// Basic testctr options still work
		testctr.WithCommand("sleep", "30"),
		testctr.WithEnv("TEST_ENV", "value"),

		// Testcontainers-specific options
		testcontainerbridgeopts.WithTestcontainersPrivileged(),
		testcontainerbridgeopts.WithTestcontainersAutoRemove(true),

		// Custom wait strategy
		testcontainerbridgeopts.WithTestcontainersWaitStrategy(
			wait.ForLog("test").WithStartupTimeout(10),
		),

		// Host config modifier
		testcontainerbridgeopts.WithTestcontainersHostConfigModifier(func(hc interface{}) {
			if hostConfig, ok := hc.(*container.HostConfig); ok {
				hostConfig.Memory = 512 * 1024 * 1024 // 512MB
				hostConfig.CPUShares = 512
			}
		}),

		// Generic customizer for full control
		testcontainerbridgeopts.WithTestcontainersCustomizer(func(cfg interface{}) {
			if gcr, ok := cfg.(*testcontainers.GenericContainerRequest); ok {
				// Access the full GenericContainerRequest
				gcr.Name = "test-custom-container"
				gcr.ContainerRequest.Labels["custom"] = "label"
			}
		}),
	)

	// Verify container is running by trying to exec a command
	// (InspectContainer is not part of the public testctr API)

	// Test that basic testctr functionality still works
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

func TestTestcontainersSkipReaper(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping testcontainers test in short mode")
	}

	// Create a container that skips the reaper
	c := testctr.New(t, "alpine:latest",
		testctr.WithBackend("testcontainers"),
		testcontainerbridgeopts.WithTestcontainersReaper(true), // Skip reaper
		testctr.WithCommand("sleep", "10"),
	)

	// Container should be created successfully
	if c == nil {
		t.Fatal("Failed to create container")
	}
}
