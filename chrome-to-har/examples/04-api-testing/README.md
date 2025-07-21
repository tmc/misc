# API Testing Examples

This directory contains examples for testing various types of APIs using chrome-to-har with browser context.

## Examples

### 1. GraphQL Tester (`graphql-tester.go`)
Comprehensive testing for GraphQL APIs with query, mutation, and subscription support.

**Usage:**
```bash
go run graphql-tester.go https://api.example.com/graphql
```

**Features:**
- Tests queries, mutations, and subscriptions
- Validates GraphQL responses and errors
- Measures query performance
- Supports variables and complex queries
- Generates detailed reports with operation analysis

**Output:**
- Console report with operation breakdown
- `graphql-test-report.json` with detailed analysis
- Network performance metrics

### 2. REST API Tester (`rest-api-tester.go`)
Full REST API testing suite with CRUD operations and validation.

**Usage:**
```bash
go run rest-api-tester.go https://api.example.com
```

**Features:**
- Tests all HTTP methods (GET, POST, PUT, DELETE)
- Validates status codes and response content
- Measures response times
- Checks headers and content types
- Supports request/response validation

**Output:**
- Console report with HTTP method breakdown
- `api-test-report.json` with detailed results
- Performance and error analysis

### 3. WebSocket Tester (`websocket-tester.go`)
Real-time WebSocket connection and messaging tests.

**Usage:**
```bash
go run websocket-tester.go wss://api.example.com/ws
```

**Features:**
- Tests WebSocket connection establishment
- Sends and receives messages
- Monitors connection stability
- Stress testing with multiple messages
- Subscription and event handling

**Output:**
- Console report with connection metrics
- `websocket-test-report.json` with message logs
- Connection performance analysis

## Shell Script Helpers

### API Test Suite Runner
```bash
#!/bin/bash
# run-api-tests.sh
BASE_URL="$1"

echo "Running comprehensive API tests for $BASE_URL"

echo "Testing REST API..."
go run rest-api-tester.go "$BASE_URL"

echo "Testing GraphQL API..."
go run graphql-tester.go "$BASE_URL/graphql"

echo "Testing WebSocket API..."
WS_URL=$(echo "$BASE_URL" | sed 's/http/ws/')
go run websocket-tester.go "$WS_URL/ws"

echo "All API tests completed!"
```

### API Health Monitor
```bash
#!/bin/bash
# api-health-monitor.sh
API_URL="$1"
INTERVAL=60  # 1 minute

while true; do
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    echo "Health check at $TIMESTAMP"
    
    # Quick health check
    if curl -s "$API_URL/health" > /dev/null; then
        echo "API is healthy"
        go run rest-api-tester.go "$API_URL" > "health_$TIMESTAMP.log"
    else
        echo "API is down!"
    fi
    
    sleep $INTERVAL
done
```

## Testing Patterns

### 1. Authentication Testing
```go
// Set authentication headers
headers := map[string]string{
    "Authorization": "Bearer " + token,
    "Content-Type": "application/json",
}

// Test with authentication
testCase := APITestCase{
    Method: "GET",
    URL: baseURL + "/protected",
    Headers: headers,
    Expected: APIExpectation{
        StatusCode: 200,
    },
}
```

### 2. Error Handling Testing
```go
// Test error responses
testCases := []APITestCase{
    {
        Name: "Invalid JSON",
        Method: "POST",
        URL: baseURL + "/users",
        Body: "invalid json",
        Expected: APIExpectation{
            StatusCode: 400,
        },
    },
    {
        Name: "Not Found",
        Method: "GET",
        URL: baseURL + "/users/999999",
        Expected: APIExpectation{
            StatusCode: 404,
        },
    },
}
```

### 3. Performance Testing
```go
// Measure response times
testCase := APITestCase{
    Name: "Performance Test",
    Method: "GET",
    URL: baseURL + "/heavy-endpoint",
    Expected: APIExpectation{
        StatusCode: 200,
        ResponseTime: 2000, // Max 2 seconds
    },
}
```

### 4. Data Validation
```go
// Validate response structure
expected := APIExpectation{
    StatusCode: 200,
    BodyContains: []string{"id", "name", "email"},
    HeaderExists: []string{"content-type"},
    JSONPath: map[string]interface{}{
        "id": "number",
        "name": "string",
        "email": "string",
    },
}
```

## Advanced Testing Techniques

### 1. Rate Limiting Tests
```go
// Test rate limiting
for i := 0; i < 100; i++ {
    result := testAPIEndpoint(ctx, testCase)
    if result.StatusCode == 429 {
        fmt.Printf("Rate limit hit after %d requests\n", i+1)
        break
    }
}
```

### 2. Concurrent Testing
```go
// Test concurrent requests
var wg sync.WaitGroup
results := make(chan APITestResult, 10)

for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        result := testAPIEndpoint(ctx, testCase)
        results <- result
    }()
}

wg.Wait()
close(results)
```

### 3. Environment-Specific Testing
```go
// Test different environments
environments := map[string]string{
    "dev": "https://dev-api.example.com",
    "staging": "https://staging-api.example.com",
    "prod": "https://api.example.com",
}

for env, url := range environments {
    fmt.Printf("Testing %s environment...\n", env)
    testAPIEndpoint(ctx, APITestCase{URL: url})
}
```

### 4. Schema Validation
```go
// Validate API schema
chromedp.Evaluate(`
    fetch('${apiUrl}/schema')
        .then(response => response.json())
        .then(schema => {
            // Validate schema structure
            return schema.data && schema.data.__schema;
        })
`, &schemaValid)
```

## Best Practices

### 1. Test Data Management
- Use test databases or isolated environments
- Clean up test data after each test
- Use predictable test data for consistency

### 2. Error Handling
- Test all error scenarios (4xx, 5xx)
- Validate error message formats
- Check error recovery mechanisms

### 3. Performance Considerations
- Set appropriate timeouts
- Monitor memory usage during tests
- Test with realistic data volumes

### 4. Security Testing
- Test authentication and authorization
- Validate input sanitization
- Check for common security vulnerabilities

### 5. Documentation
- Document API contracts and expectations
- Maintain test cases as living documentation
- Include examples for different use cases

## Troubleshooting

### Common Issues

1. **CORS Errors**: APIs may block browser requests
   - Solution: Use appropriate CORS headers or proxy

2. **Authentication Failures**: Token expiration or invalid credentials
   - Solution: Implement token refresh logic

3. **Rate Limiting**: Too many requests too quickly
   - Solution: Add delays between requests

4. **Network Timeouts**: Slow API responses
   - Solution: Increase timeout values

5. **WebSocket Connection Issues**: Firewall or proxy blocking
   - Solution: Test with different protocols (ws/wss)

### Debugging Tips

1. **Enable Verbose Logging**:
   ```go
   chromedp.Flag("enable-logging", true)
   chromedp.Flag("v", "1")
   ```

2. **Capture Network Traffic**:
   ```go
   rec := recorder.New()
   // Use recorder to capture all requests
   ```

3. **Browser DevTools**:
   ```go
   // Run with headless=false to see browser
   chromedp.Flag("headless", false)
   ```

4. **HAR Analysis**:
   - Use HAR files to analyze network requests
   - Check timing, headers, and response data

## Integration with CI/CD

### GitHub Actions Example
```yaml
name: API Tests
on: [push, pull_request]
jobs:
  api-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.21
      - name: Run API Tests
        run: |
          go run examples/04-api-testing/rest-api-tester.go ${{ secrets.API_URL }}
          go run examples/04-api-testing/graphql-tester.go ${{ secrets.GRAPHQL_URL }}
      - name: Upload Reports
        uses: actions/upload-artifact@v2
        with:
          name: api-test-reports
          path: "*-test-report.json"
```

### Docker Integration
```dockerfile
FROM golang:1.21-alpine
RUN apk add --no-cache chromium
COPY examples/ /app/examples/
WORKDIR /app
CMD ["go", "run", "examples/04-api-testing/rest-api-tester.go"]
```

## Metrics and Monitoring

### Key Metrics to Track
- Response times
- Success rates
- Error rates by type
- Throughput (requests per second)
- Resource usage

### Alerting
- Set up alerts for high error rates
- Monitor response time degradation
- Track API availability

### Reporting
- Generate daily/weekly reports
- Track trends over time
- Compare performance across environments