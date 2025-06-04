package dockerclient

import (
	"testing"

	testctrbackendtest "github.com/tmc/misc/testctr/backends/testing/testctrbackend"
)

func TestDockerClientBackend_FullSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full backend test suite in short mode")
	}

	backend := &DockerClientBackend{
		containers: make(map[string]containerInfo),
	}

	// Run the full test suite
	testctrbackendtest.RunBackendTests(t, backend)
}

func TestDockerClientBackend_Lifecycle(t *testing.T) {
	backend := &DockerClientBackend{
		containers: make(map[string]containerInfo),
	}

	// Run just the lifecycle tests
	testctrbackendtest.RunBackendLifecycleTests(t, backend)
}

func TestDockerClientBackend_WithConfigurations(t *testing.T) {
	backend := &DockerClientBackend{
		containers: make(map[string]containerInfo),
	}

	// Test with various configurations
	testctrbackendtest.RunBackendConfigTests(t, backend)
}

func TestDockerClientBackend_Databases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database tests in short mode")
	}

	backend := &DockerClientBackend{
		containers: make(map[string]containerInfo),
	}

	// Test database containers
	testctrbackendtest.RunBackendDatabaseTests(t, backend)
}

func BenchmarkDockerClientBackend(b *testing.B) {
	backend := &DockerClientBackend{
		containers: make(map[string]containerInfo),
	}

	testctrbackendtest.RunBackendBenchmarks(b, backend)
}
