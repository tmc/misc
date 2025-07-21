// React SPA testing example
// This example shows how to test React applications with dynamic routing
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

type TestResult struct {
	URL           string                 `json:"url"`
	Title         string                 `json:"title"`
	LoadTime      float64                `json:"load_time_ms"`
	Success       bool                   `json:"success"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	ComponentInfo map[string]interface{} `json:"component_info,omitempty"`
	NetworkStats  NetworkStats           `json:"network_stats"`
}

type NetworkStats struct {
	TotalRequests int     `json:"total_requests"`
	FailedRequests int    `json:"failed_requests"`
	TotalSize     int64   `json:"total_size_bytes"`
	LoadTime      float64 `json:"load_time_ms"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run react-app-tester.go <spa-base-url>")
		fmt.Println("Example: go run react-app-tester.go https://my-react-app.com")
		os.Exit(1)
	}

	baseURL := os.Args[1]
	
	// Define test routes for React SPA
	testRoutes := []string{
		"/",
		"/about",
		"/contact",
		"/dashboard",
		"/profile",
		"/products",
		"/login",
		"/settings",
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create Chrome browser optimized for SPA testing
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (compatible; SPATest/1.0)"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var results []TestResult

	// Test each route
	for _, route := range testRoutes {
		result := testSPARoute(chromeCtx, baseURL, route)
		results = append(results, result)
		
		fmt.Printf("Testing %s%s: %s\n", baseURL, route, 
			map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		
		if !result.Success {
			fmt.Printf("  Error: %s\n", result.ErrorMessage)
		}
		
		// Small delay between tests
		time.Sleep(1 * time.Second)
	}

	// Generate report
	generateReport(results, baseURL)
}

func testSPARoute(chromeCtx context.Context, baseURL, route string) TestResult {
	fullURL := strings.TrimSuffix(baseURL, "/") + route
	startTime := time.Now()
	
	result := TestResult{
		URL:          fullURL,
		Success:      false,
		ComponentInfo: make(map[string]interface{}),
	}

	// Create recorder for this test
	rec := recorder.New()

	// Test timeout context
	ctx, cancel := context.WithTimeout(chromeCtx, 30*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		rec.Start(),
		chromedp.Navigate(fullURL),
		
		// Wait for React app to load
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		
		// Wait for React root element
		chromedp.WaitVisible("#root, #app, [data-react-root]", chromedp.ByQuery),
		
		// Extract page title
		chromedp.Title(&result.Title),
		
		// Extract React component information
		chromedp.Evaluate(`
			(function() {
				const info = {};
				
				// Check for React DevTools
				if (window.React) {
					info.react_version = window.React.version;
				}
				
				// Check for React Router
				if (window.location.pathname !== '/') {
					info.current_route = window.location.pathname;
				}
				
				// Check for common React patterns
				const reactRoot = document.querySelector('#root, #app, [data-react-root]');
				if (reactRoot) {
					info.has_react_root = true;
					info.react_root_children = reactRoot.children.length;
				}
				
				// Check for loading states
				const loadingElements = document.querySelectorAll('[class*="loading"], [class*="spinner"], [aria-label*="loading"]');
				info.loading_elements = loadingElements.length;
				
				// Check for error boundaries
				const errorElements = document.querySelectorAll('[class*="error"], [role="alert"]');
				info.error_elements = errorElements.length;
				
				// Check for common UI framework classes
				const frameworks = ['ant-', 'mui-', 'chakra-', 'mantine-', 'bp3-'];
				frameworks.forEach(framework => {
					const elements = document.querySelectorAll('[class*="' + framework + '"]');
					if (elements.length > 0) {
						info['ui_framework_' + framework.replace('-', '')] = elements.length;
					}
				});
				
				// Check for hydration
				const hydrationMarkers = document.querySelectorAll('[data-reactroot], [data-react-checksum]');
				info.hydration_markers = hydrationMarkers.length;
				
				// Performance metrics
				if (window.performance && window.performance.timing) {
					const timing = window.performance.timing;
					info.dom_content_loaded = timing.domContentLoadedEventEnd - timing.navigationStart;
					info.load_complete = timing.loadEventEnd - timing.navigationStart;
				}
				
				return info;
			})()
		`, &result.ComponentInfo),
		
		// Check for console errors
		chromedp.Evaluate(`
			(function() {
				const errors = [];
				const originalError = console.error;
				console.error = function(...args) {
					errors.push(args.join(' '));
					originalError.apply(console, args);
				};
				
				// Wait a bit for any async errors
				setTimeout(() => {
					window.reactTestErrors = errors;
				}, 1000);
				
				return errors;
			})()
		`, nil),
		
		rec.Stop(),
	)

	loadTime := time.Since(startTime)
	result.LoadTime = float64(loadTime.Nanoseconds()) / 1e6

	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	// Extract network statistics from HAR
	harData, err := rec.HAR()
	if err == nil {
		result.NetworkStats = extractNetworkStats(harData)
	}

	// Validate SPA loading
	if result.Title == "" {
		result.ErrorMessage = "Page title is empty"
		return result
	}

	if val, ok := result.ComponentInfo["react_root_children"]; ok {
		if children, ok := val.(float64); ok && children == 0 {
			result.ErrorMessage = "React root has no children"
			return result
		}
	}

	result.Success = true
	return result
}

func extractNetworkStats(harData string) NetworkStats {
	// Simple HAR parsing for basic stats
	// In a real implementation, you'd use a proper HAR parser
	stats := NetworkStats{}
	
	// Count requests (simplified)
	stats.TotalRequests = strings.Count(harData, `"request":`)
	
	// Count failures (simplified)
	stats.FailedRequests = strings.Count(harData, `"status":4`) + strings.Count(harData, `"status":5`)
	
	return stats
}

func generateReport(results []TestResult, baseURL string) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("SPA Test Report for %s\n", baseURL)
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")

	passed := 0
	failed := 0
	totalLoadTime := 0.0

	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
		totalLoadTime += result.LoadTime
	}

	fmt.Printf("Summary:\n")
	fmt.Printf("  Total Tests: %d\n", len(results))
	fmt.Printf("  Passed: %d\n", passed)
	fmt.Printf("  Failed: %d\n", failed)
	fmt.Printf("  Success Rate: %.1f%%\n", float64(passed)/float64(len(results))*100)
	fmt.Printf("  Average Load Time: %.2f ms\n", totalLoadTime/float64(len(results)))

	fmt.Printf("\nDetailed Results:\n")
	for i, result := range results {
		fmt.Printf("\n%d. %s\n", i+1, result.URL)
		fmt.Printf("   Status: %s\n", map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		fmt.Printf("   Load Time: %.2f ms\n", result.LoadTime)
		fmt.Printf("   Title: %s\n", result.Title)
		
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.ErrorMessage)
		}
		
		if reactVersion, ok := result.ComponentInfo["react_version"]; ok {
			fmt.Printf("   React Version: %v\n", reactVersion)
		}
		
		if children, ok := result.ComponentInfo["react_root_children"]; ok {
			fmt.Printf("   React Components: %v\n", children)
		}
		
		fmt.Printf("   Network: %d requests, %d failed\n", 
			result.NetworkStats.TotalRequests, result.NetworkStats.FailedRequests)
	}

	// Save detailed report as JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}

	err = os.WriteFile("spa-test-report.json", jsonData, 0644)
	if err != nil {
		log.Printf("Error writing report: %v", err)
		return
	}

	fmt.Printf("\nDetailed report saved to spa-test-report.json\n")
}