package testctr_tests

import (
	"testing"

	"github.com/tmc/misc/testctr"
	_ "github.com/tmc/misc/testctr/testctr-testcontainers" // Register testcontainers backend
)

func TestBackendRegistration(t *testing.T) {
	t.Parallel()

	// Test that the default backend exists
	backend, err := testctr.GetBackend("")
	if err == nil && backend != nil {
		t.Log("Default backend is available")
	} else {
		t.Log("Default backend not available (this is normal)")
	}

	// Test that testcontainers backend is registered
	backend, err = testctr.GetBackend("testcontainers")
	if err != nil {
		t.Fatalf("Expected testcontainers backend to be registered, but got error: %v", err)
	}
	if backend == nil {
		t.Fatal("Expected testcontainers backend to be non-nil")
	}
	t.Log("Testcontainers backend successfully registered")
}