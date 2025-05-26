package testctr_test

import (
	"os/exec"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

func TestRuntimeDetection(t *testing.T) {
	t.Parallel()
	// Just test that we can create a container with the detected runtime
	_ = testctr.New(t, "alpine:latest",
		testctr.WithCommand("echo", "hello from", "testctr"),
	)

	t.Logf("Using container runtime: %s", detectRuntime())
}

func TestWithPodman(t *testing.T) {
	t.Parallel()
	// Check if podman is available and working
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("Podman not available")
	}

	// Try to verify podman is actually working
	if err := exec.Command("podman", "version").Run(); err != nil {
		t.Skip("Podman not properly configured: " + err.Error())
	}

	// WithPodman is now in ctropts package
	// Import "github.com/tmc/misc/testctr/ctropts" to use ctropts.WithPodman()
	c := testctr.New(t, "alpine:latest",
		ctropts.WithRuntime("podman"),
		testctr.WithCommand("echo", "hello from podman"),
	)

	_ = c // Container will be cleaned up automatically
}

func TestWithRuntime(t *testing.T) {
	t.Parallel()
	// Test generic runtime option
	c := testctr.New(t, "alpine:latest",
		ctropts.WithRuntime("docker"), // Explicitly use docker
		testctr.WithCommand("echo", "hello"),
	)

	_ = c
}

// Helper to detect which runtime is being used
func detectRuntime() string {
	runtimes := []string{"docker", "podman", "nerdctl", "finch", "colima"}
	for _, rt := range runtimes {
		if _, err := exec.LookPath(rt); err == nil {
			if err := exec.Command(rt, "version").Run(); err == nil {
				return rt
			}
		}
	}
	return "unknown"
}
