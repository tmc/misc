// Vue.js SPA testing example
// This example shows how to test Vue applications with Vuex state management
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

type VueTestResult struct {
	URL           string                 `json:"url"`
	Title         string                 `json:"title"`
	LoadTime      float64                `json:"load_time_ms"`
	Success       bool                   `json:"success"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	VueInfo       map[string]interface{} `json:"vue_info,omitempty"`
	VuexState     map[string]interface{} `json:"vuex_state,omitempty"`
	ComponentTree []Component            `json:"component_tree,omitempty"`
	NetworkStats  NetworkStats           `json:"network_stats"`
}

type Component struct {
	Name     string      `json:"name"`
	Props    interface{} `json:"props,omitempty"`
	Children []Component `json:"children,omitempty"`
}

type NetworkStats struct {
	TotalRequests  int     `json:"total_requests"`
	FailedRequests int     `json:"failed_requests"`
	TotalSize      int64   `json:"total_size_bytes"`
	LoadTime       float64 `json:"load_time_ms"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run vue-app-tester.go <vue-app-url>")
		fmt.Println("Example: go run vue-app-tester.go https://my-vue-app.com")
		os.Exit(1)
	}

	baseURL := os.Args[1]
	
	// Define test routes for Vue SPA
	testRoutes := []string{
		"/",
		"/about",
		"/contact",
		"/dashboard",
		"/profile",
		"/products",
		"/cart",
		"/checkout",
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create Chrome browser optimized for Vue testing
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (compatible; VueTest/1.0)"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var results []VueTestResult

	// Test each route
	for _, route := range testRoutes {
		result := testVueRoute(chromeCtx, baseURL, route)
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
	generateVueReport(results, baseURL)
}

func testVueRoute(chromeCtx context.Context, baseURL, route string) VueTestResult {
	fullURL := strings.TrimSuffix(baseURL, "/") + route
	startTime := time.Now()
	
	result := VueTestResult{
		URL:     fullURL,
		Success: false,
		VueInfo: make(map[string]interface{}),
	}

	// Create recorder for this test
	rec := recorder.New()

	// Test timeout context
	ctx, cancel := context.WithTimeout(chromeCtx, 30*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		rec.Start(),
		chromedp.Navigate(fullURL),
		
		// Wait for Vue app to load
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		
		// Wait for Vue root element
		chromedp.WaitVisible("#app, [data-vue-root]", chromedp.ByQuery),
		
		// Extract page title
		chromedp.Title(&result.Title),
		
		// Extract Vue application information
		chromedp.Evaluate(`
			(function() {
				const info = {};
				
				// Check for Vue instance
				if (window.Vue) {
					info.vue_version = window.Vue.version;
				}
				
				// Check for Vue DevTools
				if (window.__VUE_DEVTOOLS_GLOBAL_HOOK__) {
					info.has_vue_devtools = true;
				}
				
				// Check for Vue app instance
				const app = document.querySelector('#app').__vue__;
				if (app) {
					info.has_vue_app = true;
					info.vue_app_name = app.$options.name || 'Unknown';
				}
				
				// Check for Vue Router
				if (window.Vue && window.Vue.router) {
					info.has_vue_router = true;
					info.current_route = window.location.pathname;
				}
				
				// Check for Vuex store
				if (app && app.$store) {
					info.has_vuex_store = true;
					info.vuex_modules = Object.keys(app.$store._modules.root._children || {});
				}
				
				// Check for common Vue UI frameworks
				const frameworks = ['v-', 'el-', 'ivu-', 'ant-', 'q-'];
				frameworks.forEach(framework => {
					const elements = document.querySelectorAll('[class*="' + framework + '"]');
					if (elements.length > 0) {
						info['ui_framework_' + framework.replace('-', '')] = elements.length;
					}
				});
				
				// Check for Vue components
				const vueComponents = document.querySelectorAll('[data-vue-component]');
				info.vue_components_count = vueComponents.length;
				
				// Performance metrics
				if (window.performance && window.performance.timing) {
					const timing = window.performance.timing;
					info.dom_content_loaded = timing.domContentLoadedEventEnd - timing.navigationStart;
					info.load_complete = timing.loadEventEnd - timing.navigationStart;
				}
				
				return info;
			})()
		`, &result.VueInfo),
		
		// Extract Vuex state if available
		chromedp.Evaluate(`
			(function() {
				const app = document.querySelector('#app').__vue__;
				if (app && app.$store) {
					return app.$store.state;
				}
				return {};
			})()
		`, &result.VuexState),
		
		// Extract component tree
		chromedp.Evaluate(`
			(function() {
				function extractComponent(vm) {
					if (!vm) return null;
					
					const component = {
						name: vm.$options.name || vm.$options._componentTag || 'Unknown',
						props: vm.$props || {},
						children: []
					};
					
					if (vm.$children) {
						vm.$children.forEach(child => {
							const childComponent = extractComponent(child);
							if (childComponent) {
								component.children.push(childComponent);
							}
						});
					}
					
					return component;
				}
				
				const app = document.querySelector('#app').__vue__;
				if (app) {
					return [extractComponent(app)];
				}
				return [];
			})()
		`, &result.ComponentTree),
		
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

	// Validate Vue app loading
	if result.Title == "" {
		result.ErrorMessage = "Page title is empty"
		return result
	}

	if val, ok := result.VueInfo["has_vue_app"]; !ok || !val.(bool) {
		result.ErrorMessage = "Vue app instance not found"
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

func generateVueReport(results []VueTestResult, baseURL string) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("Vue.js SPA Test Report for %s\n", baseURL)
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

	fmt.Printf("\nVue.js Analysis:\n")
	for _, result := range results {
		if result.Success {
			if vueVersion, ok := result.VueInfo["vue_version"]; ok {
				fmt.Printf("  Vue Version: %v\n", vueVersion)
				break
			}
		}
	}

	// Check for common patterns across all results
	hasVueRouter := false
	hasVuexStore := false
	for _, result := range results {
		if val, ok := result.VueInfo["has_vue_router"]; ok && val.(bool) {
			hasVueRouter = true
		}
		if val, ok := result.VueInfo["has_vuex_store"]; ok && val.(bool) {
			hasVuexStore = true
		}
	}

	fmt.Printf("  Vue Router: %s\n", map[bool]string{true: "✓ Present", false: "✗ Not detected"}[hasVueRouter])
	fmt.Printf("  Vuex Store: %s\n", map[bool]string{true: "✓ Present", false: "✗ Not detected"}[hasVuexStore])

	fmt.Printf("\nDetailed Results:\n")
	for i, result := range results {
		fmt.Printf("\n%d. %s\n", i+1, result.URL)
		fmt.Printf("   Status: %s\n", map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		fmt.Printf("   Load Time: %.2f ms\n", result.LoadTime)
		fmt.Printf("   Title: %s\n", result.Title)
		
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.ErrorMessage)
		}
		
		if appName, ok := result.VueInfo["vue_app_name"]; ok {
			fmt.Printf("   Vue App: %v\n", appName)
		}
		
		if components, ok := result.VueInfo["vue_components_count"]; ok {
			fmt.Printf("   Vue Components: %v\n", components)
		}
		
		if len(result.VuexState) > 0 {
			fmt.Printf("   Vuex State Keys: %v\n", getKeys(result.VuexState))
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

	err = os.WriteFile("vue-test-report.json", jsonData, 0644)
	if err != nil {
		log.Printf("Error writing report: %v", err)
		return
	}

	fmt.Printf("\nDetailed report saved to vue-test-report.json\n")
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}