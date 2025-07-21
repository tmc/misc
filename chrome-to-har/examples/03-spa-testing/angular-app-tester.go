// Angular SPA testing example
// This example shows how to test Angular applications with services and routing
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

type AngularTestResult struct {
	URL           string                 `json:"url"`
	Title         string                 `json:"title"`
	LoadTime      float64                `json:"load_time_ms"`
	Success       bool                   `json:"success"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	AngularInfo   map[string]interface{} `json:"angular_info,omitempty"`
	Components    []string               `json:"components,omitempty"`
	Services      []string               `json:"services,omitempty"`
	NetworkStats  NetworkStats           `json:"network_stats"`
}

type NetworkStats struct {
	TotalRequests  int     `json:"total_requests"`
	FailedRequests int     `json:"failed_requests"`
	TotalSize      int64   `json:"total_size_bytes"`
	LoadTime       float64 `json:"load_time_ms"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run angular-app-tester.go <angular-app-url>")
		fmt.Println("Example: go run angular-app-tester.go https://my-angular-app.com")
		os.Exit(1)
	}

	baseURL := os.Args[1]
	
	// Define test routes for Angular SPA
	testRoutes := []string{
		"/",
		"/about",
		"/contact",
		"/dashboard",
		"/profile",
		"/products",
		"/orders",
		"/settings",
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create Chrome browser optimized for Angular testing
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (compatible; AngularTest/1.0)"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var results []AngularTestResult

	// Test each route
	for _, route := range testRoutes {
		result := testAngularRoute(chromeCtx, baseURL, route)
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
	generateAngularReport(results, baseURL)
}

func testAngularRoute(chromeCtx context.Context, baseURL, route string) AngularTestResult {
	fullURL := strings.TrimSuffix(baseURL, "/") + route
	startTime := time.Now()
	
	result := AngularTestResult{
		URL:         fullURL,
		Success:     false,
		AngularInfo: make(map[string]interface{}),
	}

	// Create recorder for this test
	rec := recorder.New()

	// Test timeout context
	ctx, cancel := context.WithTimeout(chromeCtx, 30*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		rec.Start(),
		chromedp.Navigate(fullURL),
		
		// Wait for Angular app to load
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		
		// Wait for Angular root element
		chromedp.WaitVisible("app-root, [ng-app]", chromedp.ByQuery),
		
		// Extract page title
		chromedp.Title(&result.Title),
		
		// Extract Angular application information
		chromedp.Evaluate(`
			(function() {
				const info = {};
				
				// Check for Angular
				if (window.ng) {
					info.has_angular = true;
					
					// Get Angular version
					if (window.ng.version) {
						info.angular_version = window.ng.version.full;
					}
					
					// Check for Angular core
					if (window.ng.core) {
						info.has_angular_core = true;
					}
				}
				
				// Check for Angular DevTools
				if (window.ng && window.ng.probe) {
					info.has_angular_devtools = true;
				}
				
				// Check for Angular Material
				const materialElements = document.querySelectorAll('[class*="mat-"]');
				info.angular_material_elements = materialElements.length;
				
				// Check for Angular Bootstrap
				const bootstrapElements = document.querySelectorAll('[class*="ngb-"]');
				info.angular_bootstrap_elements = bootstrapElements.length;
				
				// Check for Angular Router
				const routerOutlet = document.querySelector('router-outlet');
				if (routerOutlet) {
					info.has_angular_router = true;
					info.current_route = window.location.pathname;
				}
				
				// Check for Angular Forms
				const formElements = document.querySelectorAll('form[formGroup], [formControlName]');
				info.angular_forms_count = formElements.length;
				
				// Check for Angular animations
				const animatedElements = document.querySelectorAll('[class*="ng-"]');
				info.angular_animation_elements = animatedElements.length;
				
				// Performance metrics
				if (window.performance && window.performance.timing) {
					const timing = window.performance.timing;
					info.dom_content_loaded = timing.domContentLoadedEventEnd - timing.navigationStart;
					info.load_complete = timing.loadEventEnd - timing.navigationStart;
				}
				
				return info;
			})()
		`, &result.AngularInfo),
		
		// Extract Angular components
		chromedp.Evaluate(`
			(function() {
				const components = [];
				
				// Get all Angular components
				const componentElements = document.querySelectorAll('[ng-reflect-ng-for-of], [ng-reflect-ng-if], app-*, [class*="ng-"]');
				
				componentElements.forEach(el => {
					const tagName = el.tagName.toLowerCase();
					if (tagName.startsWith('app-') || tagName.includes('ng-')) {
						if (components.indexOf(tagName) === -1) {
							components.push(tagName);
						}
					}
				});
				
				return components;
			})()
		`, &result.Components),
		
		// Extract Angular services (this is more complex and might not work in all cases)
		chromedp.Evaluate(`
			(function() {
				const services = [];
				
				// This is a simplified approach - in real Angular apps, services are not easily accessible
				// from the DOM. You'd need Angular DevTools or specific debugging setup.
				
				// Check for common service patterns in window object
				if (window.ng && window.ng.probe) {
					try {
						const rootElement = document.querySelector('app-root');
						if (rootElement) {
							const componentRef = window.ng.probe(rootElement);
							if (componentRef && componentRef.injector) {
								// This is a simplified example - actual service detection would be more complex
								services.push('HttpClient', 'Router', 'ActivatedRoute');
							}
						}
					} catch (e) {
						// Ignore errors in service detection
					}
				}
				
				return services;
			})()
		`, &result.Services),
		
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

	// Validate Angular app loading
	if result.Title == "" {
		result.ErrorMessage = "Page title is empty"
		return result
	}

	if val, ok := result.AngularInfo["has_angular"]; !ok || !val.(bool) {
		result.ErrorMessage = "Angular framework not detected"
		return result
	}

	result.Success = true
	return result
}

func extractNetworkStats(harData string) NetworkStats {
	// Simple HAR parsing for basic stats
	stats := NetworkStats{}
	
	// Count requests (simplified)
	stats.TotalRequests = strings.Count(harData, `"request":`)
	
	// Count failures (simplified)
	stats.FailedRequests = strings.Count(harData, `"status":4`) + strings.Count(harData, `"status":5`)
	
	return stats
}

func generateAngularReport(results []AngularTestResult, baseURL string) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("Angular SPA Test Report for %s\n", baseURL)
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

	fmt.Printf("\nAngular Analysis:\n")
	for _, result := range results {
		if result.Success {
			if angularVersion, ok := result.AngularInfo["angular_version"]; ok {
				fmt.Printf("  Angular Version: %v\n", angularVersion)
				break
			}
		}
	}

	// Check for common patterns across all results
	hasAngularRouter := false
	hasAngularMaterial := false
	totalComponents := 0
	for _, result := range results {
		if val, ok := result.AngularInfo["has_angular_router"]; ok && val.(bool) {
			hasAngularRouter = true
		}
		if val, ok := result.AngularInfo["angular_material_elements"]; ok {
			if count, ok := val.(float64); ok && count > 0 {
				hasAngularMaterial = true
			}
		}
		totalComponents += len(result.Components)
	}

	fmt.Printf("  Angular Router: %s\n", map[bool]string{true: "✓ Present", false: "✗ Not detected"}[hasAngularRouter])
	fmt.Printf("  Angular Material: %s\n", map[bool]string{true: "✓ Present", false: "✗ Not detected"}[hasAngularMaterial])
	fmt.Printf("  Total Components Found: %d\n", totalComponents)

	fmt.Printf("\nDetailed Results:\n")
	for i, result := range results {
		fmt.Printf("\n%d. %s\n", i+1, result.URL)
		fmt.Printf("   Status: %s\n", map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		fmt.Printf("   Load Time: %.2f ms\n", result.LoadTime)
		fmt.Printf("   Title: %s\n", result.Title)
		
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.ErrorMessage)
		}
		
		if len(result.Components) > 0 {
			fmt.Printf("   Components: %v\n", result.Components)
		}
		
		if len(result.Services) > 0 {
			fmt.Printf("   Services: %v\n", result.Services)
		}
		
		if materialCount, ok := result.AngularInfo["angular_material_elements"]; ok {
			if count, ok := materialCount.(float64); ok && count > 0 {
				fmt.Printf("   Angular Material Elements: %.0f\n", count)
			}
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

	err = os.WriteFile("angular-test-report.json", jsonData, 0644)
	if err != nil {
		log.Printf("Error writing report: %v", err)
		return
	}

	fmt.Printf("\nDetailed report saved to angular-test-report.json\n")
}