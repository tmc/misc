package tests

import (
	"testing"

	"github.com/tmc/misc/testctr/backend"
	_ "github.com/tmc/misc/testctr/backends/cli" // register CLI backend
	testctrbackendtest "github.com/tmc/misc/testctr/backends/testing/testctrbackend"
)

// TestCLIBackend runs the backend test suite against the CLI implementation.
func TestCLIBackend(t *testing.T) {
	t.Parallel()

	// Get the registered CLI backend
	cliBackend, err := backend.Get("cli")
	if err != nil {
		t.Fatalf("Failed to get CLI backend: %v", err)
	}

	testctrbackendtest.RunBackendTests(t, cliBackend)
}