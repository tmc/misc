// Package testctrbackendtest provides a test suite for testing testctr backend implementations.
//
// This package is similar to the standard library's fstest package, providing
// a comprehensive set of tests that backend authors can use to verify their
// implementation conforms to the Backend interface contract.
//
// # Usage
//
// Backend authors should use TestBackend to run the full test suite:
//
//	func TestMyBackend(t *testing.T) {
//	    backend := &MyBackend{}
//	    testctrbackendtest.TestBackend(t, backend)
//	}
//
// For more granular testing, individual test functions are also available:
//
//	func TestMyBackendLifecycle(t *testing.T) {
//	    backend := &MyBackend{}
//	    testctrbackendtest.TestBackendLifecycle(t, backend)
//	}
//
// # Test Coverage
//
// The test suite covers:
//   - Container lifecycle (create, start, stop, remove)
//   - Port mapping and networking
//   - Environment variables
//   - Command execution
//   - Log retrieval and waiting
//   - Container inspection
//   - File operations
//   - Error handling
//   - Concurrent operations
package testctrbackendtest
