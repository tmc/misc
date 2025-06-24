# testctr Testing Guide

This document describes the test architecture and testing practices for the testctr project.

## Test Coverage Overview

Current test coverage as of latest improvements:

| Component | Coverage | Description |
|-----------|----------|-------------|
| **Main Package** | 59.9% | Core functionality including container lifecycle |
| **Backend Package** | 91.7% | Backend registry and interface |
| **File Operations** | 100% | WithFile, WithFileMode, WithFileReader, WithFiles |
| **DSN Functionality** | 100% | Database connection string generation |
| **Service Options** | 100% | MySQL, PostgreSQL configuration options |
| **Backend Selection** | 100% | WithBackend option testing |

## Test Architecture

### 1. Unit Tests

**Backend Registry Tests** (`backend/registry_test.go`)
- Tests the pluggable backend system
- Validates registration and retrieval of backends
- 91.7% coverage

### 2. Integration Tests

**Core Container Tests** (`simple_test.go`, `parallel_test.go`)
- Basic container lifecycle
- Port mapping and networking
- Command execution
- Environment variables

**File Operation Tests** (`file_test.go`)
- Tests all file copying mechanisms
- Validates permissions and content
- Tests with files, io.Reader, and []byte sources

**DSN Tests** (`dsn_test.go`)
- Database connection string generation
- Per-test database isolation
- Cleanup verification

**Service Option Tests** (`service_options_test.go`)
- Service-specific configurations
- Wait strategies
- Environment variable validation

**Backend Option Tests** (`backend_options_test.go`)
- Backend selection and switching
- Cross-backend compatibility
- Port mapping consistency

### 3. Stress Tests

**Concurrent Container Tests** (`concurrent_stress_test.go`)
- Tests with build tag `stress`
- Validates behavior under high concurrency
- Resource cleanup verification

### 4. End-to-End Tests

**Example Tests** (`example_test.go`)
- Documentation examples that also serve as tests
- Real-world usage patterns

## Running Tests

### Basic Test Run
```bash
go test -v ./...
```

### With Coverage
```bash
go test -v -cover ./...
```

### Run Specific Test Suites
```bash
# File operations
go test -v -run "TestWithFile"

# DSN functionality
go test -v -run "TestDSN"

# Service options
go test -v -run "TestService"

# Backend options
go test -v -run "TestBackend"
```

### Run Stress Tests
```bash
go test -v -tags=stress -run "TestConcurrent"
```

### Run with Verbose Logging
```bash
go test -v -testctr.verbose
```

### Keep Failed Containers for Debugging
```bash
go test -v -testctr.keep-failed
```

## Test Patterns

### 1. Parallel Testing

All tests use `t.Parallel()` for better performance:

```go
func TestExample(t *testing.T) {
    t.Parallel()
    // test code
}
```

Current parallel test adoption: **118%** (includes subtests)

### 2. Error Handling

Consistent error handling pattern:

```go
exitCode, output, err := c.Exec(ctx, []string{"echo", "test"})
if err != nil {
    t.Fatalf("Exec failed: %v, output: %s", err, output)
}
if exitCode != 0 {
    t.Fatalf("Expected exit code 0, got %d", exitCode)
}
```

### 3. Container Cleanup

Automatic cleanup via `t.Cleanup()`:

```go
func TestExample(t *testing.T) {
    c := testctr.New(t, "alpine:latest") // Cleanup registered automatically
    // test code
}
```

### 4. Database Isolation

Each test gets its own database:

```go
func TestDatabase(t *testing.T) {
    c := testctr.New(t, "postgres:15", postgres.Default())
    dsn := c.DSN(t) // Unique database for this test
}
```

## Known Issues

### 1. Port Binding Conflicts
- **Issue**: Tests may fail with "port already allocated" errors
- **Solution**: Tests now use dynamic port allocation (no explicit host port binding)

### 2. MySQL Startup Time
- **Issue**: MySQL containers take longer to initialize
- **Solution**: mysql2 package includes appropriate wait strategies

### 3. Docker Daemon Dependencies
- **Issue**: Some backend tests require Docker daemon
- **Solution**: Tests skip appropriately when Docker is not available

### 4. Generated Module Compatibility
- **Issue**: Generated modules in exp/gen may have API mismatches
- **Solution**: Use manual modules in ctropts/ for production use

## Adding New Tests

When adding new functionality:

1. **Write tests first** - TDD approach
2. **Use t.Parallel()** - Enable parallel execution
3. **Follow existing patterns** - Consistent error handling
4. **Add to appropriate test file** - Group related tests
5. **Update coverage expectations** - Maintain or improve coverage

## Continuous Integration

The project uses GitHub Actions for CI:

1. **Test Matrix**: Tests across Go versions and platforms
2. **Coverage Reporting**: Track coverage trends
3. **Linting**: Ensure code quality
4. **Build Verification**: Ensure all modules build

## Performance Considerations

1. **Container Reuse**: Not implemented (each test gets fresh container)
2. **Parallel Execution**: Maximized via t.Parallel()
3. **Resource Cleanup**: Automatic and reliable
4. **Wait Strategies**: Optimized per service type

## Future Improvements

1. **Error Path Coverage**: Expand negative test cases
2. **Cross-Backend Matrix**: Systematic testing across all backends
3. **Performance Benchmarks**: Add benchmark tests
4. **Resource Usage Monitoring**: Track memory/CPU during tests