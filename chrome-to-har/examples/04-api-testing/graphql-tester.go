// GraphQL API testing with browser context
// This example shows how to test GraphQL APIs that require browser authentication
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

type GraphQLTestResult struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	Response      map[string]interface{} `json:"response"`
	Success       bool                   `json:"success"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	Duration      float64                `json:"duration_ms"`
	NetworkStats  NetworkStats           `json:"network_stats"`
}

type NetworkStats struct {
	RequestSize    int64 `json:"request_size_bytes"`
	ResponseSize   int64 `json:"response_size_bytes"`
	StatusCode     int   `json:"status_code"`
	ResponseTime   int64 `json:"response_time_ms"`
}

type GraphQLQuery struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run graphql-tester.go <graphql-endpoint>")
		fmt.Println("Example: go run graphql-tester.go https://api.example.com/graphql")
		os.Exit(1)
	}

	endpoint := os.Args[1]
	
	// Define test queries
	testQueries := []GraphQLQuery{
		{
			Query: `query GetUsers {
				users {
					id
					name
					email
					createdAt
				}
			}`,
		},
		{
			Query: `query GetUser($id: ID!) {
				user(id: $id) {
					id
					name
					email
					profile {
						bio
						avatar
					}
				}
			}`,
			Variables: map[string]interface{}{
				"id": "1",
			},
		},
		{
			Query: `mutation CreateUser($input: CreateUserInput!) {
				createUser(input: $input) {
					id
					name
					email
					success
				}
			}`,
			Variables: map[string]interface{}{
				"input": map[string]interface{}{
					"name":  "Test User",
					"email": "test@example.com",
				},
			},
		},
		{
			Query: `subscription UserUpdated {
				userUpdated {
					id
					name
					email
					updatedAt
				}
			}`,
		},
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Create Chrome browser
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (compatible; GraphQLTester/1.0)"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var results []GraphQLTestResult

	// Test each query
	for _, query := range testQueries {
		result := testGraphQLQuery(chromeCtx, endpoint, query)
		results = append(results, result)
		
		fmt.Printf("Testing %s: %s\n", 
			getOperationType(query.Query),
			map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		
		if !result.Success {
			fmt.Printf("  Error: %s\n", result.ErrorMessage)
		}
		
		// Small delay between tests
		time.Sleep(1 * time.Second)
	}

	// Generate report
	generateGraphQLReport(results, endpoint)
}

func testGraphQLQuery(chromeCtx context.Context, endpoint string, query GraphQLQuery) GraphQLTestResult {
	startTime := time.Now()
	
	result := GraphQLTestResult{
		Query:     query.Query,
		Variables: query.Variables,
		Success:   false,
		Response:  make(map[string]interface{}),
	}

	// Create recorder for this test
	rec := recorder.New()

	// Test timeout context
	ctx, cancel := context.WithTimeout(chromeCtx, 30*time.Second)
	defer cancel()

	// Navigate to a page to establish browser context (for cookies, auth, etc.)
	err := chromedp.Run(ctx,
		rec.Start(),
		chromedp.Navigate("about:blank"),
		
		// Execute GraphQL query using fetch API
		chromedp.Evaluate(fmt.Sprintf(`
			(async function() {
				const query = %s;
				const variables = %s;
				
				const response = await fetch('%s', {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						'Accept': 'application/json',
					},
					body: JSON.stringify({
						query: query,
						variables: variables
					})
				});
				
				const result = await response.json();
				
				return {
					status: response.status,
					statusText: response.statusText,
					headers: Object.fromEntries(response.headers.entries()),
					data: result
				};
			})()
		`, jsonString(query.Query), jsonString(query.Variables), endpoint), &result.Response),
		
		rec.Stop(),
	)

	duration := time.Since(startTime)
	result.Duration = float64(duration.Nanoseconds()) / 1e6

	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	// Extract network statistics from HAR
	harData, err := rec.HAR()
	if err == nil {
		result.NetworkStats = extractGraphQLNetworkStats(harData, endpoint)
	}

	// Check for GraphQL errors
	if response, ok := result.Response["data"].(map[string]interface{}); ok {
		if errors, hasErrors := response["errors"]; hasErrors {
			result.ErrorMessage = fmt.Sprintf("GraphQL errors: %v", errors)
			return result
		}
	}

	// Check HTTP status
	if status, ok := result.Response["status"]; ok {
		if statusCode, ok := status.(float64); ok && statusCode >= 400 {
			result.ErrorMessage = fmt.Sprintf("HTTP error: %v", result.Response["statusText"])
			return result
		}
	}

	result.Success = true
	return result
}

func extractGraphQLNetworkStats(harData, endpoint string) NetworkStats {
	// Simple HAR parsing for GraphQL requests
	stats := NetworkStats{}
	
	// This is a simplified implementation
	// In practice, you'd parse the HAR JSON and find the GraphQL request
	if strings.Contains(harData, endpoint) {
		stats.StatusCode = 200  // Default assumption
		stats.ResponseTime = 100 // Default assumption
	}
	
	return stats
}

func getOperationType(query string) string {
	query = strings.TrimSpace(strings.ToLower(query))
	if strings.HasPrefix(query, "query") {
		return "Query"
	} else if strings.HasPrefix(query, "mutation") {
		return "Mutation"
	} else if strings.HasPrefix(query, "subscription") {
		return "Subscription"
	}
	return "Unknown"
}

func jsonString(v interface{}) string {
	if v == nil {
		return "null"
	}
	data, _ := json.Marshal(v)
	return string(data)
}

func generateGraphQLReport(results []GraphQLTestResult, endpoint string) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("GraphQL API Test Report for %s\n", endpoint)
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")

	passed := 0
	failed := 0
	totalDuration := 0.0
	
	queryCount := 0
	mutationCount := 0
	subscriptionCount := 0

	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
		totalDuration += result.Duration
		
		operationType := getOperationType(result.Query)
		switch operationType {
		case "Query":
			queryCount++
		case "Mutation":
			mutationCount++
		case "Subscription":
			subscriptionCount++
		}
	}

	fmt.Printf("Summary:\n")
	fmt.Printf("  Total Tests: %d\n", len(results))
	fmt.Printf("  Passed: %d\n", passed)
	fmt.Printf("  Failed: %d\n", failed)
	fmt.Printf("  Success Rate: %.1f%%\n", float64(passed)/float64(len(results))*100)
	fmt.Printf("  Average Duration: %.2f ms\n", totalDuration/float64(len(results)))

	fmt.Printf("\nOperation Types:\n")
	fmt.Printf("  Queries: %d\n", queryCount)
	fmt.Printf("  Mutations: %d\n", mutationCount)
	fmt.Printf("  Subscriptions: %d\n", subscriptionCount)

	fmt.Printf("\nDetailed Results:\n")
	for i, result := range results {
		fmt.Printf("\n%d. %s\n", i+1, getOperationType(result.Query))
		fmt.Printf("   Status: %s\n", map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		fmt.Printf("   Duration: %.2f ms\n", result.Duration)
		
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.ErrorMessage)
		}
		
		if result.NetworkStats.StatusCode > 0 {
			fmt.Printf("   HTTP Status: %d\n", result.NetworkStats.StatusCode)
		}
		
		// Show first few lines of query
		queryLines := strings.Split(strings.TrimSpace(result.Query), "\n")
		if len(queryLines) > 0 {
			fmt.Printf("   Query: %s\n", queryLines[0])
		}
		
		if result.Variables != nil && len(result.Variables) > 0 {
			fmt.Printf("   Variables: %v\n", result.Variables)
		}
	}

	// Save detailed report as JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}

	err = os.WriteFile("graphql-test-report.json", jsonData, 0644)
	if err != nil {
		log.Printf("Error writing report: %v", err)
		return
	}

	fmt.Printf("\nDetailed report saved to graphql-test-report.json\n")
}