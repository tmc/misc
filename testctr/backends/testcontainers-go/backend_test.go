package testcontainers

import (
	"testing"

	testctrbackendtest "github.com/tmc/misc/testctr/backends/testing/testctrbackend"
)

func TestTestcontainersBackend_FullSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full backend test suite in short mode")
	}

	backend := &TestcontainersBackend{}

	// Run the full test suite
	testctrbackendtest.RunBackendTests(t, backend)
}

func TestTestcontainersBackend_Lifecycle(t *testing.T) {
	backend := &TestcontainersBackend{}

	// Run just the lifecycle tests
	testctrbackendtest.RunBackendLifecycleTests(t, backend)
}

func TestTestcontainersBackend_Execution(t *testing.T) {
	backend := &TestcontainersBackend{}

	// Test command execution
	testctrbackendtest.RunBackendExecutionTests(t, backend)
}

func TestTestcontainersBackend_Logs(t *testing.T) {
	backend := &TestcontainersBackend{}

	// Test log operations
	testctrbackendtest.RunBackendLogsTests(t, backend)
}

func TestTestcontainersBackend_Databases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database tests in short mode")
	}

	backend := &TestcontainersBackend{}

	// Test database containers
	testctrbackendtest.RunBackendDatabaseTests(t, backend)
}

func BenchmarkTestcontainersBackend(b *testing.B) {
	backend := &TestcontainersBackend{}

	testctrbackendtest.RunBackendBenchmarks(b, backend)
}
