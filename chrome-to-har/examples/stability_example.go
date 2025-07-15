package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// StabilityExample demonstrates the enhanced stability detection features
func main() {
	// Create a new browser instance
	b, err := browser.New(
		browser.WithHeadless(false), // Show browser for demonstration
		browser.WithVerbose(true),   // Enable verbose logging
	)
	if err != nil {
		log.Fatalf("Failed to create browser: %v", err)
	}
	defer b.Close()

	// Launch the browser
	if err := b.Launch(); err != nil {
		log.Fatalf("Failed to launch browser: %v", err)
	}

	// Create a new page
	page, err := b.NewPage()
	if err != nil {
		log.Fatalf("Failed to create page: %v", err)
	}
	defer page.Close()

	// Example 1: Basic stability detection
	fmt.Println("=== Example 1: Basic Stability Detection ===")
	
	// Navigate to a dynamic page
	if err := page.Navigate("https://httpbin.org/delay/2"); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}

	// Wait for the page to be fully stable
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := page.WaitForStability(ctx, nil); err != nil {
		log.Printf("Stability detection failed: %v", err)
	} else {
		fmt.Println("Page is stable!")
	}

	// Example 2: Custom stability configuration
	fmt.Println("\n=== Example 2: Custom Stability Configuration ===")
	
	// Configure custom stability detection
	page.ConfigureStability(
		browser.WithNetworkIdleThreshold(0),                      // No network requests
		browser.WithNetworkIdleTimeout(500*time.Millisecond),     // Wait 500ms for network idle
		browser.WithDOMStableTimeout(1*time.Second),              // Wait 1s for DOM stability
		browser.WithResourceWaiting(true, true, true, true),      // Wait for all resources
		browser.WithMaxStabilityWait(30*time.Second),             // Max 30s total wait
		browser.WithCustomCheck("title", "document.title !== ''", 5*time.Second), // Custom check
		browser.WithVerboseLogging(true),                         // Enable verbose logging
	)

	// Navigate to a complex page
	if err := page.Navigate("https://example.com"); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}

	// Wait for stability with custom configuration
	ctx2, cancel2 := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel2()

	if err := page.WaitForStability(ctx2, nil); err != nil {
		log.Printf("Custom stability detection failed: %v", err)
	} else {
		fmt.Println("Page is stable with custom configuration!")
	}

	// Example 3: Using different load states
	fmt.Println("\n=== Example 3: Different Load States ===")
	
	// Test different load states
	testURL := "https://httpbin.org/html"
	
	// DOMContentLoaded
	fmt.Println("Testing DOMContentLoaded...")
	if err := page.Navigate(testURL); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}
	
	ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel3()
	
	if err := page.WaitForLoadState(ctx3, browser.LoadStateDOMContentLoaded); err != nil {
		log.Printf("DOMContentLoaded failed: %v", err)
	} else {
		fmt.Println("DOMContentLoaded completed!")
	}

	// Network Idle
	fmt.Println("Testing NetworkIdle...")
	if err := page.Navigate(testURL); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}
	
	ctx4, cancel4 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel4()
	
	if err := page.WaitForLoadState(ctx4, browser.LoadStateNetworkIdle); err != nil {
		log.Printf("NetworkIdle failed: %v", err)
	} else {
		fmt.Println("NetworkIdle completed!")
	}

	// Example 4: Monitoring stability metrics
	fmt.Println("\n=== Example 4: Stability Metrics ===")
	
	// Navigate to a page
	if err := page.Navigate("https://httpbin.org/json"); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}

	// Get stability metrics
	metrics := page.GetStabilityMetrics()
	if metrics != nil {
		fmt.Printf("Network requests: %d\n", metrics.NetworkRequests)
		fmt.Printf("DOM modifications: %d\n", metrics.DOMModifications)
		fmt.Printf("Pending requests: %d\n", len(metrics.PendingRequests))
		fmt.Printf("Loaded resources: %d\n", len(metrics.LoadedResources))
		fmt.Printf("Stability checks: %d\n", len(metrics.StabilityChecks))
	}

	// Example 5: SPA stability detection
	fmt.Println("\n=== Example 5: SPA Stability Detection ===")
	
	// Configure for SPA (single page application)
	page.ConfigureStability(
		browser.WithNetworkIdleThreshold(2),                  // Allow up to 2 concurrent requests
		browser.WithNetworkIdleTimeout(1*time.Second),       // Wait 1s for network idle
		browser.WithDOMStableTimeout(2*time.Second),         // Wait 2s for DOM stability
		browser.WithResourceWaiting(true, true, true, true), // Wait for all resources
		browser.WithMaxStabilityWait(60*time.Second),        // Allow more time for SPAs
		browser.WithCustomCheck("spa-ready", "window.appReady === true", 10*time.Second),
		browser.WithVerboseLogging(true),
	)

	// Navigate to a hypothetical SPA
	if err := page.Navigate("https://example.com"); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}

	// Inject a script to simulate SPA loading
	if err := page.Evaluate(`
		setTimeout(() => {
			// Simulate SPA initialization
			window.appReady = true;
			console.log('SPA is ready!');
		}, 1000);
	`, nil); err != nil {
		log.Printf("Failed to inject script: %v", err)
	}

	// Wait for SPA to be stable
	ctx5, cancel5 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel5()

	if err := page.WaitForStability(ctx5, nil); err != nil {
		log.Printf("SPA stability detection failed: %v", err)
	} else {
		fmt.Println("SPA is stable!")
	}

	fmt.Println("\n=== Stability Detection Examples Complete ===")
}