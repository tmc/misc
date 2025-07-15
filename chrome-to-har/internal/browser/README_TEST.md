# Browser Package Integration Tests

This directory contains comprehensive integration tests for the `internal/browser` package, which provides Chrome browser automation capabilities.

## Test Files Overview

### Core Tests
- **browser_test.go**: Tests basic browser operations including launch, navigation, HTML retrieval, script execution, and authentication
- **page_test.go**: Tests page-level operations like element interaction, form handling, screenshots, and multiple tab management
- **element_test.go**: Tests element querying, manipulation, attributes, and interactions
- **network_test.go**: Tests network interception, request routing, response modification, and request/response waiting

### Advanced Tests
- **remote_test.go**: Tests remote Chrome connectivity, debugging protocol, and multi-tab remote operations
- **performance_test.go**: Performance benchmarks and stress tests including memory leak detection, concurrent operations, and large DOM handling
- **testutil_test.go**: Test utilities and helper functions for browser testing

## Running Tests

### Basic Test Run
```bash
# Run all browser tests
go test ./internal/browser/

# Run with verbose output
go test -v ./internal/browser/

# Run specific test
go test -v -run TestBrowserLaunch ./internal/browser/
```

### Performance Tests
```bash
# Run including performance tests (excluded by default with -short)
go test -v ./internal/browser/

# Run only benchmarks
go test -bench=. ./internal/browser/

# Run specific benchmark
go test -bench=BenchmarkPageNavigation ./internal/browser/
```

### Remote Chrome Tests
```bash
# Remote tests require Chrome to be installed
go test -v -run TestRemote ./internal/browser/
```

## Test Environment Requirements

### Chrome Installation
Tests require Google Chrome or Chromium to be installed. The test suite looks for Chrome in common locations:
- macOS: `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`
- Linux: `/usr/bin/google-chrome`, `/usr/bin/chromium`
- Windows: `C:\Program Files\Google\Chrome\Application\chrome.exe`

### CI Environment
Tests detect CI environments via the `CI` environment variable and may skip certain tests on non-Linux platforms in CI.

## Test Coverage Areas

### 1. Browser Management
- Browser launching with various options
- Headless and headed modes
- Profile management
- Timeout handling
- Context management

### 2. Page Operations
- Navigation with different wait strategies
- JavaScript execution
- Element querying and interaction
- Form handling
- Screenshot capture (full page and elements)
- PDF generation
- Multiple tab/page management

### 3. Element Handling
- Single and multiple element queries
- Click, type, and focus operations
- Attribute getting/setting
- Visibility checks
- Hover and scroll operations
- Element screenshots

### 4. Network Interception
- Request routing and modification
- Response fulfillment
- Request/response abortion
- Header manipulation
- POST data capture
- Concurrent request handling

### 5. Remote Chrome
- Connecting to running Chrome instances
- Tab enumeration and connection
- Remote debugging protocol
- Multi-tab remote operations
- Error handling for connection failures

### 6. Performance
- Navigation speed benchmarks
- Concurrent page operations
- Large DOM handling
- Memory leak detection
- Script execution performance
- Screenshot performance
- Network load testing

## Test Utilities

The test suite includes several utility functions:

- `createTestBrowser()`: Creates a test browser with cleanup
- `newTestServer()`: Creates an HTTP test server with various endpoints
- `AssertElement*()`: Element assertion helpers
- `WaitForCondition()`: Waits for conditions with timeout
- `TakeDebugScreenshot()`: Captures debug screenshots (verbose mode)

## Known Issues and Limitations

1. **Platform Differences**: Some tests may behave differently on different operating systems
2. **Chrome Versions**: Tests are designed to work with recent Chrome versions (90+)
3. **Timing Sensitivity**: Some tests involve timing and may be flaky under heavy system load
4. **Resource Usage**: Performance tests can be resource-intensive

## Debugging Failed Tests

### Enable Verbose Output
```bash
go test -v -run TestName ./internal/browser/
```

### Debug Screenshots
Run tests with `-v` flag to enable debug screenshot capture:
```bash
go test -v ./internal/browser/
# Screenshots saved to /tmp/browser-test-*.png
```

### Disable Headless Mode
Modify test to use headed mode for visual debugging:
```go
b, cleanup := createTestBrowser(t, browser.WithHeadless(false))
```

### Check Chrome Logs
Enable Chrome logging in tests:
```go
b, cleanup := createTestBrowser(t, browser.WithVerbose(true))
```

## Contributing

When adding new tests:
1. Follow existing test patterns and naming conventions
2. Use test utilities for common operations
3. Include both positive and negative test cases
4. Add performance benchmarks for new features
5. Ensure tests are reliable and not flaky
6. Document any special requirements or setup