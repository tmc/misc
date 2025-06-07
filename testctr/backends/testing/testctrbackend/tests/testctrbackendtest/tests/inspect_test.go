package tests

import (
	"testing"

	"github.com/tmc/misc/testctr"
)

func TestContainerInspect(t *testing.T) {
	t.Parallel()

	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sleep", "infinity"),
	)

	// Use the new Inspect method
	info, err := c.Inspect()
	if err != nil {
		t.Fatalf("Failed to inspect container: %v", err)
	}

	// Verify we got useful information
	if info.ID == "" {
		t.Error("Expected container ID in inspect info")
	}

	if !info.State.Running {
		t.Error("Expected container to be running")
	}

	if info.State.Status != "running" {
		t.Errorf("Expected status 'running', got %q", info.State.Status)
	}

	t.Logf("Container %s is %s", info.ID[:12], info.State.Status)
}

func TestContainerInspect_Redis(t *testing.T) {
	t.Parallel()

	c := testctr.New(t, "redis:7-alpine",
		testctr.WithPort("6379"),
	)

	// Inspect should return container details
	info, err := c.Inspect()
	if err != nil {
		t.Fatalf("Failed to inspect container: %v", err)
	}

	// Check that we have port mappings for Redis
	if len(info.NetworkSettings.Ports) == 0 {
		t.Error("Expected Redis container to have port mappings")
	}

	// Redis should expose port 6379
	redisPort := "6379/tcp"
	if bindings, ok := info.NetworkSettings.Ports[redisPort]; !ok || len(bindings) == 0 {
		t.Errorf("Expected Redis to expose port %s", redisPort)
	} else {
		t.Logf("Redis port %s is mapped to host port %s", redisPort, bindings[0].HostPort)
	}

	// Verify container name and labels
	if info.Name == "" {
		t.Error("Expected container to have a name")
	}

	if info.Config.Labels == nil {
		t.Error("Expected container to have labels")
	} else {
		// Check for testctr labels
		if _, ok := info.Config.Labels["testctr"]; !ok {
			t.Error("Expected container to have 'testctr' label")
		}
		t.Logf("Container has %d labels", len(info.Config.Labels))
	}
}
