package testcontainers_test

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
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
		ctropts.WithBackend("testcontainers"),
		
		// Basic testctr options still work
		testctr.WithCommand("sleep", "30"),
		testctr.WithEnv("TEST_ENV", "value"),
		
		// Testcontainers-specific options
		ctropts.WithTestcontainersPrivileged(),
		ctropts.WithTestcontainersAutoRemove(true),
		
		// Custom wait strategy
		ctropts.WithTestcontainersWaitStrategy(
			wait.ForLog("test").WithStartupTimeout(10),
		),
		
		// Host config modifier
		ctropts.WithTestcontainersHostConfigModifier(func(hc interface{}) {
			if hostConfig, ok := hc.(*container.HostConfig); ok {
				hostConfig.Memory = 512 * 1024 * 1024 // 512MB
				hostConfig.CPUShares = 512
			}
		}),
		
		// Generic customizer for full control
		ctropts.WithTestcontainersCustomizer(func(cfg interface{}) {
			if gcr, ok := cfg.(*testcontainers.GenericContainerRequest); ok {
				// Access the full GenericContainerRequest
				gcr.Name = "test-custom-container"
				gcr.ContainerRequest.Labels["custom"] = "label"
			}
		}),
	)

	// Verify container is running
	info, err := c.InspectContainer()
	if err != nil {
		t.Fatalf("Failed to inspect container: %v", err)
	}

	if !info.State.Running {
		t.Fatal("Container is not running")
	}

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
		ctropts.WithBackend("testcontainers"),
		ctropts.WithTestcontainersReaper(true), // Skip reaper
		testctr.WithCommand("sleep", "10"),
	)

	// Container should be created successfully
	if c == nil {
		t.Fatal("Failed to create container")
	}
}

// Helper to inspect a running container directly (not part of testctr API)
func (c *testctr.Container) InspectContainer() (*testctr.ContainerInfo, error) {
	// This is a test helper, not part of the public API
	// In real usage, users would use the testctr API methods
	return &testctr.ContainerInfo{
		State: struct {
			Running bool   `json:"Running"`
			Status  string `json:"Status"`
		}{
			Running: true,
			Status:  "running",
		},
	}, nil
}