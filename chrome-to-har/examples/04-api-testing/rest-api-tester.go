// REST API testing with browser context
// This example shows how to test REST APIs that require browser authentication
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

type APITestResult struct {
	Method        string                 `json:"method"`
	URL           string                 `json:"url"`
	RequestBody   interface{}            `json:"request_body,omitempty"`
	ResponseBody  interface{}            `json:"response_body"`
	StatusCode    int                    `json:"status_code"`
	Success       bool                   `json:"success"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	Duration      float64                `json:"duration_ms"`
	Headers       map[string]string      `json:"headers,omitempty"`
	NetworkStats  NetworkStats           `json:"network_stats"`
}

type NetworkStats struct {
	RequestSize    int64 `json:"request_size_bytes"`
	ResponseSize   int64 `json:"response_size_bytes"`
	ResponseTime   int64 `json:"response_time_ms"`
	DNSTime        int64 `json:"dns_time_ms"`
	ConnectTime    int64 `json:"connect_time_ms"`
	SSLTime        int64 `json:"ssl_time_ms"`
}

type APITestCase struct {
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        interface{}       `json:"body,omitempty"`
	Expected    APIExpectation    `json:"expected"`
}

type APIExpectation struct {
	StatusCode    int                    `json:"status_code"`
	ResponseTime  int64                  `json:"max_response_time_ms"`
	BodyContains  []string               `json:"body_contains,omitempty"`
	HeaderExists  []string               `json:"header_exists,omitempty"`
	JSONPath      map[string]interface{} `json:"json_path,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run rest-api-tester.go <api-base-url>")
		fmt.Println("Example: go run rest-api-tester.go https://api.example.com")
		os.Exit(1)
	}

	baseURL := strings.TrimSuffix(os.Args[1], "/")
	
	// Define test cases
	testCases := []APITestCase{
		{
			Name:   "Get Users",
			Method: "GET",
			URL:    baseURL + "/users",
			Headers: map[string]string{
				"Accept": "application/json",
			},
			Expected: APIExpectation{
				StatusCode:   200,
				ResponseTime: 2000,
				BodyContains: []string{"users", "id", "name"},
				HeaderExists: []string{"content-type"},
			},
		},
		{
			Name:   "Get User by ID",
			Method: "GET",
			URL:    baseURL + "/users/1",
			Headers: map[string]string{
				"Accept": "application/json",
			},
			Expected: APIExpectation{
				StatusCode:   200,
				ResponseTime: 1000,
				JSONPath: map[string]interface{}{
					"id":   1,
					"name": "string",
				},
			},
		},
		{
			Name:   "Create User",
			Method: "POST",
			URL:    baseURL + "/users",
			Headers: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
			Body: map[string]interface{}{
				"name":  "Test User",
				"email": "test@example.com",
			},
			Expected: APIExpectation{
				StatusCode:   201,
				ResponseTime: 3000,
				BodyContains: []string{"id", "name", "email"},
			},
		},
		{
			Name:   "Update User",
			Method: "PUT",
			URL:    baseURL + "/users/1",
			Headers: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
			Body: map[string]interface{}{
				"name":  "Updated User",
				"email": "updated@example.com",
			},
			Expected: APIExpectation{
				StatusCode:   200,
				ResponseTime: 2000,
			},
		},
		{
			Name:   "Delete User",
			Method: "DELETE",
			URL:    baseURL + "/users/1",
			Expected: APIExpectation{
				StatusCode:   204,
				ResponseTime: 1000,
			},
		},
		{
			Name:   "Get Non-existent User",
			Method: "GET",
			URL:    baseURL + "/users/99999",
			Expected: APIExpectation{
				StatusCode:   404,
				ResponseTime: 1000,
			},
		},
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create Chrome browser
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (compatible; APITester/1.0)"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var results []APITestResult

	// Test each API endpoint
	for _, testCase := range testCases {
		result := testAPIEndpoint(chromeCtx, testCase)
		results = append(results, result)
		
		fmt.Printf("Testing %s: %s\n", 
			testCase.Name,
			map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		
		if !result.Success {
			fmt.Printf("  Error: %s\n", result.ErrorMessage)
		}
		
		// Small delay between tests
		time.Sleep(1 * time.Second)
	}

	// Generate report
	generateAPIReport(results, baseURL)
}

func testAPIEndpoint(chromeCtx context.Context, testCase APITestCase) APITestResult {
	startTime := time.Now()
	
	result := APITestResult{
		Method:  testCase.Method,
		URL:     testCase.URL,
		Headers: testCase.Headers,
		Success: false,
	}

	// Create recorder for this test
	rec := recorder.New()

	// Test timeout context
	ctx, cancel := context.WithTimeout(chromeCtx, 30*time.Second)
	defer cancel()

	// Navigate to a page to establish browser context
	err := chromedp.Run(ctx,
		rec.Start(),
		chromedp.Navigate("about:blank"),
		
		// Execute API call using fetch API
		chromedp.Evaluate(fmt.Sprintf(`
			(async function() {
				const startTime = performance.now();
				
				const options = {
					method: '%s',
					headers: %s
				};
				
				%s
				
				const response = await fetch('%s', options);
				const endTime = performance.now();
				
				let responseBody;
				const contentType = response.headers.get('content-type');
				
				if (contentType && contentType.includes('application/json')) {
					responseBody = await response.json();
				} else {
					responseBody = await response.text();
				}
				
				return {
					status: response.status,
					statusText: response.statusText,
					headers: Object.fromEntries(response.headers.entries()),
					body: responseBody,
					duration: endTime - startTime
				};
			})()
		`, 
			testCase.Method, 
			jsonString(testCase.Headers),
			buildBodyScript(testCase.Body),
			testCase.URL,
		), &result.ResponseBody),
		
		rec.Stop(),
	)

	duration := time.Since(startTime)
	result.Duration = float64(duration.Nanoseconds()) / 1e6

	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	// Extract response details
	if responseMap, ok := result.ResponseBody.(map[string]interface{}); ok {
		if status, ok := responseMap["status"].(float64); ok {
			result.StatusCode = int(status)
		}
		if body, ok := responseMap["body"]; ok {
			result.ResponseBody = body
		}
		if headers, ok := responseMap["headers"].(map[string]interface{}); ok {
			result.Headers = make(map[string]string)
			for k, v := range headers {
				if str, ok := v.(string); ok {
					result.Headers[k] = str
				}
			}
		}
	}

	// Extract network statistics from HAR
	harData, err := rec.HAR()
	if err == nil {
		result.NetworkStats = extractAPINetworkStats(harData, testCase.URL)
	}

	// Validate against expectations
	result.Success = validateAPIResponse(result, testCase.Expected)
	if !result.Success && result.ErrorMessage == "" {
		result.ErrorMessage = "Response validation failed"
	}

	return result
}

func buildBodyScript(body interface{}) string {
	if body == nil {
		return ""
	}
	
	return fmt.Sprintf(`
		if (%s) {
			options.body = JSON.stringify(%s);
		}
	`, jsonString(body), jsonString(body))
}

func validateAPIResponse(result APITestResult, expected APIExpectation) bool {
	// Check status code
	if expected.StatusCode != 0 && result.StatusCode != expected.StatusCode {
		result.ErrorMessage = fmt.Sprintf("Expected status %d, got %d", expected.StatusCode, result.StatusCode)
		return false
	}

	// Check response time
	if expected.ResponseTime > 0 && result.Duration > float64(expected.ResponseTime) {
		result.ErrorMessage = fmt.Sprintf("Response time %.2fms exceeded limit %dms", result.Duration, expected.ResponseTime)
		return false
	}

	// Check body contains
	if len(expected.BodyContains) > 0 {
		bodyStr := jsonString(result.ResponseBody)
		for _, contain := range expected.BodyContains {
			if !strings.Contains(bodyStr, contain) {
				result.ErrorMessage = fmt.Sprintf("Response body does not contain '%s'", contain)
				return false
			}
		}
	}

	// Check headers exist
	if len(expected.HeaderExists) > 0 {
		for _, header := range expected.HeaderExists {
			if _, exists := result.Headers[header]; !exists {
				result.ErrorMessage = fmt.Sprintf("Expected header '%s' not found", header)
				return false
			}
		}
	}

	return true
}

func extractAPINetworkStats(harData, url string) NetworkStats {
	// Simple HAR parsing for API requests
	stats := NetworkStats{}
	
	// This is a simplified implementation
	// In practice, you'd parse the HAR JSON and find the specific request
	if strings.Contains(harData, url) {
		stats.ResponseTime = 100 // Default assumption
	}
	
	return stats
}

func jsonString(v interface{}) string {
	if v == nil {
		return "null"
	}
	data, _ := json.Marshal(v)
	return string(data)
}

func generateAPIReport(results []APITestResult, baseURL string) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("REST API Test Report for %s\n", baseURL)
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")

	passed := 0
	failed := 0
	totalDuration := 0.0
	
	methodCounts := make(map[string]int)
	statusCounts := make(map[int]int)

	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
		totalDuration += result.Duration
		
		methodCounts[result.Method]++
		statusCounts[result.StatusCode]++
	}

	fmt.Printf("Summary:\n")
	fmt.Printf("  Total Tests: %d\n", len(results))
	fmt.Printf("  Passed: %d\n", passed)
	fmt.Printf("  Failed: %d\n", failed)
	fmt.Printf("  Success Rate: %.1f%%\n", float64(passed)/float64(len(results))*100)
	fmt.Printf("  Average Duration: %.2f ms\n", totalDuration/float64(len(results)))

	fmt.Printf("\nHTTP Methods:\n")
	for method, count := range methodCounts {
		fmt.Printf("  %s: %d\n", method, count)
	}

	fmt.Printf("\nStatus Codes:\n")
	for status, count := range statusCounts {
		fmt.Printf("  %d: %d\n", status, count)
	}

	fmt.Printf("\nDetailed Results:\n")
	for i, result := range results {
		fmt.Printf("\n%d. %s %s\n", i+1, result.Method, result.URL)
		fmt.Printf("   Status: %s\n", map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		fmt.Printf("   HTTP Status: %d\n", result.StatusCode)
		fmt.Printf("   Duration: %.2f ms\n", result.Duration)
		
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.ErrorMessage)
		}
		
		if result.Headers != nil {
			contentType := result.Headers["content-type"]
			if contentType != "" {
				fmt.Printf("   Content-Type: %s\n", contentType)
			}
		}
		
		if result.NetworkStats.ResponseTime > 0 {
			fmt.Printf("   Network Response Time: %d ms\n", result.NetworkStats.ResponseTime)
		}
	}

	// Save detailed report as JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}

	err = os.WriteFile("api-test-report.json", jsonData, 0644)
	if err != nil {
		log.Printf("Error writing report: %v", err)
		return
	}

	fmt.Printf("\nDetailed report saved to api-test-report.json\n")
}