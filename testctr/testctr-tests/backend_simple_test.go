package testctr_tests

import (
	"testing"

	"github.com/tmc/misc/testctr/backend"
	_ "github.com/tmc/misc/testctr/testctr-testcontainers" // Register testcontainers backend
)

func TestBackendRegistration(t *testing.T) {
	t.Parallel()

	// Test that the default backend exists
	be, err := backend.Get("")
	if err == nil && be != nil {
		t.Log("Default backend is available")
	} else {
		t.Log("Default backend not available (this is normal)")
	}

	// Test that testcontainers backend is registered
	be, err = backend.Get("testcontainers")
	if err != nil {
		t.Fatalf("Expected testcontainers backend to be registered, but got error: %v", err)
	}
	if be == nil {
		t.Fatal("Expected testcontainers backend to be non-nil")
	}
	t.Log("Testcontainers backend successfully registered")
}
