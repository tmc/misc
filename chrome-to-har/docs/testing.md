# Testing Guide for chrome-to-har

This document describes the testing infrastructure and how to run tests for the chrome-to-har project.

## Overview

The project has a comprehensive testing setup that includes:
- Unit tests for core functionality
- Integration tests with real Chrome browser
- CI/CD pipeline with GitHub Actions
- Docker-based testing environment
- Test utilities and helpers

## Test Categories

### Unit Tests
Fast tests that don't require Chrome:
- Mock-based testing for Chrome interactions
- Core logic testing (filtering, HAR generation, etc.)
- CLI argument parsing

### Integration Tests
Tests that require a real Chrome browser:
- End-to-end HAR capture testing
- Network request/response verification
- Dynamic content handling
- Remote Chrome connectivity

## Running Tests

### Quick Start

```bash
# Run unit tests only (no Chrome required)
make test

# Run all tests including integration (requires Chrome)
make test-all

# Run with the test script (auto-detects Chrome)
./test-chrome.sh --all
```

### Detailed Test Commands

#### Using Make

```bash
# Unit tests only
make test-unit

# Integration tests only (requires Chrome)
make test-integration

# All tests with coverage
make test-coverage

# Run tests in Docker (no Chrome needed on host)
make test-docker

# CI mode (for GitHub Actions)
make test-ci
```

#### Using Go Test Directly

```bash
# Unit tests
go test -v -short ./...

# Integration tests
go test -v -tags=integration -timeout 10m ./...

# Specific test
go test -v -run TestBasicRun ./...

# With coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

#### Using Test Script

The `test-chrome.sh` script provides the most flexible testing:

```bash
# Show help
./test-chrome.sh --help

# Run all tests
./test-chrome.sh --all

# Integration tests only
./test-chrome.sh --integration

# Run specific test
./test-chrome.sh --test TestIntegration_BasicHARCapture

# Run with visible Chrome (not headless)
./test-chrome.sh --all --no-headless

# Verbose output
./test-chrome.sh --all --verbose
```

## Docker Testing

For consistent testing across environments:

```bash
# Build and run all tests
docker-compose -f docker-compose.test.yml run test

# Run specific test suite
docker-compose -f docker-compose.test.yml run test-unit
docker-compose -f docker-compose.test.yml run test-integration

# Generate coverage report
docker-compose -f docker-compose.test.yml run test-coverage

# Interactive shell for debugging
docker-compose -f docker-compose.test.yml run test-dev
```

## CI/CD Setup

### GitHub Actions

The project uses GitHub Actions for automated testing on:
- Ubuntu (latest)
- macOS (latest)
- Windows (latest)

With Go versions:
- 1.19
- 1.20
- 1.21

The CI pipeline:
1. Installs Chrome on each platform
2. Runs unit tests
3. Runs integration tests in headless mode
4. Uploads coverage reports to Codecov

### Running CI Tests Locally

To simulate CI environment:

```bash
# Linux/macOS
export CI=true
export HEADLESS=true
make test-ci

# Or use Docker
docker-compose -f docker-compose.test.yml run test
```

## Test Utilities

### ChromeTestHelper

Located in `internal/testutil/chrome.go`, provides:
- Chrome detection and setup
- Headless/visible mode support
- Test server creation
- Chrome lifecycle management

Example usage:

```go
func TestWithChrome(t *testing.T) {
    testutil.SkipIfNoChrome(t)
    
    ctx := context.Background()
    chromeCtx, cancel := testutil.MustStartChrome(t, ctx, true)
    defer cancel()
    
    // Use chromeCtx for Chrome operations
}
```

### Test Server

Create HTTP test servers easily:

```go
mux := http.NewServeMux()
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "<html><body>Test</body></html>")
})

server := testutil.TestServer(t, mux)
defer server.Close()

// Use server.URL() for the test URL
```

## Environment Variables

### Chrome Configuration
- `CHROME_PATH`: Path to Chrome executable (auto-detected if not set)
- `HEADLESS`: Set to "true" for headless mode (default in CI)

### Test Control
- `CI`: Set to "true" in CI environments
- `SKIP_BROWSER_TESTS`: Skip all browser-dependent tests
- `TIMEOUT`: Override default test timeout

## Troubleshooting

### Chrome Not Found

If tests can't find Chrome:

1. Install Chrome:
   ```bash
   # Ubuntu/Debian
   sudo apt-get install google-chrome-stable
   
   # macOS
   brew install --cask google-chrome
   
   # Windows
   # Download from https://www.google.com/chrome/
   ```

2. Or set Chrome path:
   ```bash
   export CHROME_PATH="/path/to/chrome"
   ```

### Headless Mode Issues

On Linux without a display:

```bash
# Install Xvfb
sudo apt-get install xvfb

# Run tests with Xvfb
xvfb-run -a go test ./...
```

### Timeout Issues

For slow systems or complex tests:

```bash
# Increase timeout
go test -timeout 30m ./...

# Or with make
make test-all TIMEOUT=30m
```

### Permission Issues

On Linux, Chrome might need additional permissions:

```bash
# Add Chrome sandbox permissions
sudo sysctl -w kernel.unprivileged_userns_clone=1

# Or run Chrome with --no-sandbox (not recommended for production)
```

## Writing Tests

### Unit Test Example

```go
func TestFilterParsing(t *testing.T) {
    tests := []struct {
        name    string
        filter  string
        wantErr bool
    }{
        {
            name:    "valid_filter",
            filter:  "select(.response.status >= 400)",
            wantErr: false,
        },
        {
            name:    "invalid_filter",
            filter:  "invalid jq syntax",
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := ParseFilter(tt.filter)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseFilter() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Test Example

```go
// +build integration

func TestIntegration_CaptureWithChrome(t *testing.T) {
    testutil.SkipIfNoChrome(t)
    
    // Create test server
    server := testutil.TestServer(t, testHandler)
    defer server.Close()
    
    // Start Chrome
    ctx := context.Background()
    chromeCtx, cancel := testutil.MustStartChrome(t, ctx, true)
    defer cancel()
    
    // Run test with Chrome
    err := CaptureHAR(chromeCtx, server.URL(), "output.har")
    if err != nil {
        t.Fatalf("Failed to capture HAR: %v", err)
    }
    
    // Verify output
    // ...
}
```

## Best Practices

1. **Skip Tests Appropriately**: Use `testutil.SkipIfNoChrome(t)` for Chrome-dependent tests
2. **Clean Up Resources**: Always defer cleanup functions
3. **Use Test Helpers**: Leverage the testutil package for common operations
4. **Parallel Testing**: Mark independent tests with `t.Parallel()`
5. **Descriptive Names**: Use clear test names that describe what's being tested
6. **Table-Driven Tests**: Use subtests for similar test cases
7. **Timeout Management**: Set appropriate timeouts for integration tests

## Coverage Goals

- Unit test coverage: > 80%
- Integration test coverage: Key user workflows
- CI/CD: All platforms and Go versions

## Future Improvements

1. Performance benchmarks
2. Fuzz testing for parsers
3. Load testing for concurrent captures
4. Visual regression testing
5. Cross-browser testing (Firefox, Safari)