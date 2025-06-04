# testctrtest

Package testctrtest provides a comprehensive test suite for testing testctr backend implementations.

## Overview

Similar to Go's `testing/fstest` package, testctrtest provides a standardized way to verify that a backend correctly implements the `backend.Backend` interface. This ensures consistent behavior across different backend implementations.

## Usage

### Testing a Complete Backend

```go
package mybackend_test

import (
    "testing"
    "github.com/tmc/misc/testctr/testctrtest"
    "github.com/mycompany/mybackend"
)

func TestMyBackend(t *testing.T) {
    backend := mybackend.New()
    testctrtest.TestBackend(t, backend)
}
```

### Running Specific Test Suites

You can run individual test suites for more granular testing:

```go
func TestMyBackendLifecycle(t *testing.T) {
    backend := mybackend.New()
    testctrtest.TestBackendLifecycle(t, backend)
}

func TestMyBackendNetworking(t *testing.T) {
    backend := mybackend.New()
    testctrtest.TestBackendNetworking(t, backend)
}
```

### Available Test Suites

- **TestBackendLifecycle**: Tests container creation, starting, stopping, and removal
- **TestBackendNetworking**: Tests port mapping and network configuration
- **TestBackendExecution**: Tests command execution in containers
- **TestBackendLogs**: Tests log retrieval and waiting for log patterns
- **TestBackendInspection**: Tests container inspection and metadata
- **TestBackendErrorHandling**: Tests error conditions and edge cases
- **TestBackendConcurrent**: Tests concurrent operations
- **TestBackendWithConfig**: Tests various container configurations
- **TestBackendDatabaseContainers**: Tests common database containers (Redis, PostgreSQL)
- **TestBackendStressTest**: Performs stress testing with multiple containers

### Benchmarking

Benchmark your backend implementation:

```go
func BenchmarkMyBackend(b *testing.B) {
    backend := mybackend.New()
    testctrtest.BenchmarkBackend(b, backend)
}
```

## Configuration Options

The package provides configuration helpers for testing:

```go
// Create a test configuration
cfg := testctrtest.NewTestConfig(
    testctrtest.WithEnv("KEY", "value"),
    testctrtest.WithPort("8080"),
    testctrtest.WithCommand("echo", "hello"),
    testctrtest.WithFile("/path/to/file", "/container/path"),
    testctrtest.WithLabel("test", "true"),
)
```

## Writing Backend-Specific Tests

You can extend the test suite with backend-specific tests:

```go
func TestMyBackendSpecificFeature(t *testing.T) {
    backend := mybackend.New()
    
    // Use testctrtest helpers
    cfg := testctrtest.NewTestConfig(
        testctrtest.WithEnv("SPECIAL", "true"),
    )
    
    id, err := backend.CreateContainer(t, "alpine:latest", cfg)
    if err != nil {
        t.Fatal(err)
    }
    defer backend.RemoveContainer(id)
    
    // Test your specific feature...
}
```

## Requirements

- Go 1.21 or later
- Docker daemon accessible (for most backends)
- Sufficient permissions to create/manage containers

## Test Behavior

The test suite expects backends to:

1. Return non-empty container IDs from CreateContainer
2. Support basic lifecycle operations (create, start, stop, remove)
3. Handle errors gracefully and return appropriate error messages
4. Support concurrent operations safely
5. Implement idempotent operations where applicable (e.g., double-start)

Some tests may be skipped if a backend doesn't support certain features (e.g., Commit operation).

## Contributing

When adding new test cases, ensure they:
- Test documented behavior from the Backend interface
- Are backend-agnostic (work with any correct implementation)
- Include appropriate error checking
- Clean up resources properly