package backend_test

import (
	"testing"
	"time"

	"github.com/tmc/misc/testctr/backend"
)

// mockBackend is a test implementation of Backend
type mockBackend struct {
	name string
}

func (m *mockBackend) CreateContainer(t testing.TB, image string, config interface{}) (string, error) {
	return "mock-container-" + m.name, nil
}

func (m *mockBackend) StartContainer(containerID string) error  { return nil }
func (m *mockBackend) StopContainer(containerID string) error   { return nil }
func (m *mockBackend) RemoveContainer(containerID string) error { return nil }
func (m *mockBackend) InspectContainer(containerID string) (*backend.ContainerInfo, error) {
	return nil, nil
}
func (m *mockBackend) ExecInContainer(containerID string, cmd []string) (int, string, error) {
	return 0, "", nil
}
func (m *mockBackend) GetContainerLogs(containerID string) (string, error) { return "", nil }
func (m *mockBackend) WaitForLog(containerID string, logLine string, timeout time.Duration) error {
	return nil
}
func (m *mockBackend) InternalIP(containerID string) (string, error)     { return "", nil }
func (m *mockBackend) Commit(containerID string, imageName string) error { return nil }

func TestCustomRegistry(t *testing.T) {
	// Create a custom registry
	customRegistry := backend.NewRegistry()

	// Register backends
	backend1 := &mockBackend{name: "backend1"}
	backend2 := &mockBackend{name: "backend2"}

	customRegistry.Register("backend1", backend1)
	customRegistry.Register("backend2", backend2)

	// Test Get
	got1, err := customRegistry.Get("backend1")
	if err != nil {
		t.Fatalf("Failed to get backend1: %v", err)
	}
	if got1 != backend1 {
		t.Error("Got wrong backend1")
	}

	got2, err := customRegistry.Get("backend2")
	if err != nil {
		t.Fatalf("Failed to get backend2: %v", err)
	}
	if got2 != backend2 {
		t.Error("Got wrong backend2")
	}

	// Test Get non-existent
	_, err = customRegistry.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent backend")
	}

	// Test List
	names := customRegistry.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(names))
	}

	// Verify names are in the list
	foundBackend1, foundBackend2 := false, false
	for _, name := range names {
		if name == "backend1" {
			foundBackend1 = true
		}
		if name == "backend2" {
			foundBackend2 = true
		}
	}
	if !foundBackend1 || !foundBackend2 {
		t.Errorf("List did not contain expected backends: %v", names)
	}
}

func TestRegistryPanics(t *testing.T) {
	registry := backend.NewRegistry()

	// Test panic on empty name
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for empty name")
			}
		}()
		registry.Register("", &mockBackend{})
	}()

	// Test panic on nil backend
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for nil backend")
			}
		}()
		registry.Register("test", nil)
	}()

	// Test panic on duplicate registration
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for duplicate registration")
			}
		}()
		registry.Register("dup", &mockBackend{})
		registry.Register("dup", &mockBackend{})
	}()
}

func TestDefaultRegistry(t *testing.T) {
	// The default registry should be accessible
	if backend.DefaultRegistry == nil {
		t.Fatal("DefaultRegistry should not be nil")
	}

	// Test that global functions use DefaultRegistry
	// We can't easily test this without side effects, but we can verify
	// the functions exist and have the expected signatures
	_ = backend.Register
	_ = backend.Get
}
