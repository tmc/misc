// Core Web Vitals monitoring example
// This example shows how to measure Core Web Vitals using chrome-to-har
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

type WebVitalsResult struct {
	URL            string                 `json:"url"`
	Timestamp      time.Time              `json:"timestamp"`
	LCP            float64                `json:"lcp_ms"`           // Largest Contentful Paint
	FID            float64                `json:"fid_ms"`           // First Input Delay
	CLS            float64                `json:"cls"`              // Cumulative Layout Shift
	FCP            float64                `json:"fcp_ms"`           // First Contentful Paint
	TTFB           float64                `json:"ttfb_ms"`          // Time to First Byte
	TotalTime      float64                `json:"total_time_ms"`    // Total page load time
	Success        bool                   `json:"success"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	PerformanceAPI map[string]interface{} `json:"performance_api"`
	ResourceStats  ResourceStats          `json:"resource_stats"`
}

type ResourceStats struct {
	TotalResources int     `json:"total_resources"`
	ImageCount     int     `json:"image_count"`
	ScriptCount    int     `json:"script_count"`
	StylesheetCount int    `json:"stylesheet_count"`
	TotalSize      int64   `json:"total_size_bytes"`
	LoadTime       float64 `json:"load_time_ms"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run core-web-vitals.go <url>")
		fmt.Println("Example: go run core-web-vitals.go https://example.com")
		os.Exit(1)
	}

	url := os.Args[1]
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create Chrome browser optimized for performance testing
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		// Performance-specific flags
		chromedp.Flag("enable-precise-memory-info", true),
		chromedp.Flag("enable-heap-profiling", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Measure Core Web Vitals
	result := measureWebVitals(chromeCtx, url)
	
	fmt.Printf("Core Web Vitals Report for %s\n", url)
	fmt.Printf("Status: %s\n", map[bool]string{true: "✓ SUCCESS", false: "✗ FAILED"}[result.Success])
	
	if !result.Success {
		fmt.Printf("Error: %s\n", result.ErrorMessage)
		os.Exit(1)
	}

	// Display results
	displayResults(result)
	
	// Save detailed report
	saveReport(result)
}

func measureWebVitals(chromeCtx context.Context, url string) WebVitalsResult {
	startTime := time.Now()
	
	result := WebVitalsResult{
		URL:           url,
		Timestamp:     startTime,
		Success:       false,
		PerformanceAPI: make(map[string]interface{}),
	}

	// Create recorder for network analysis
	rec := recorder.New()

	// Test timeout context
	ctx, cancel := context.WithTimeout(chromeCtx, 60*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		rec.Start(),
		chromedp.Navigate(url),
		
		// Wait for page load
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for dynamic content
		
		// Inject Web Vitals measurement script
		chromedp.Evaluate(`
			(function() {
				return new Promise((resolve) => {
					const vitals = {};
					
					// Measure Core Web Vitals
					if ('web-vitals' in window) {
						// If web-vitals library is available
						import('https://unpkg.com/web-vitals@2').then(({ getCLS, getFID, getFCP, getLCP, getTTFB }) => {
							getCLS(vitals => vitals.cls = vitals.value);
							getFID(vitals => vitals.fid = vitals.value);
							getFCP(vitals => vitals.fcp = vitals.value);
							getLCP(vitals => vitals.lcp = vitals.value);
							getTTFB(vitals => vitals.ttfb = vitals.value);
							
							setTimeout(() => resolve(vitals), 2000);
						});
					} else {
						// Fallback: Use Performance API
						const perfEntries = performance.getEntriesByType('navigation')[0];
						const paintEntries = performance.getEntriesByType('paint');
						
						vitals.ttfb = perfEntries.responseStart - perfEntries.requestStart;
						vitals.fcp = paintEntries.find(entry => entry.name === 'first-contentful-paint')?.startTime || 0;
						vitals.lcp = 0; // LCP requires observer
						vitals.fid = 0; // FID requires user interaction
						vitals.cls = 0; // CLS requires observer
						
						// Set up LCP observer
						if ('PerformanceObserver' in window) {
							const lcpObserver = new PerformanceObserver((list) => {
								const entries = list.getEntries();
								const lastEntry = entries[entries.length - 1];
								vitals.lcp = lastEntry.startTime;
							});
							lcpObserver.observe({ entryTypes: ['largest-contentful-paint'] });
							
							// Set up CLS observer
							let cumulativeLayoutShift = 0;
							const clsObserver = new PerformanceObserver((list) => {
								for (const entry of list.getEntries()) {
									if (!entry.hadRecentInput) {
										cumulativeLayoutShift += entry.value;
									}
								}
								vitals.cls = cumulativeLayoutShift;
							});
							clsObserver.observe({ entryTypes: ['layout-shift'] });
						}
						
						setTimeout(() => resolve(vitals), 2000);
					}
				});
			})()
		`, &result.PerformanceAPI),
		
		// Get additional performance metrics
		chromedp.Evaluate(`
			(function() {
				const perfEntries = performance.getEntriesByType('navigation')[0];
				const resourceEntries = performance.getEntriesByType('resource');
				
				return {
					navigation: {
						domContentLoaded: perfEntries.domContentLoadedEventEnd - perfEntries.domContentLoadedEventStart,
						loadComplete: perfEntries.loadEventEnd - perfEntries.loadEventStart,
						totalTime: perfEntries.loadEventEnd - perfEntries.navigationStart,
						dnsLookup: perfEntries.domainLookupEnd - perfEntries.domainLookupStart,
						tcpConnect: perfEntries.connectEnd - perfEntries.connectStart,
						serverResponse: perfEntries.responseEnd - perfEntries.requestStart,
						domParsing: perfEntries.domInteractive - perfEntries.domLoading,
						resourceLoad: perfEntries.loadEventStart - perfEntries.domContentLoadedEventEnd
					},
					resources: {
						total: resourceEntries.length,
						images: resourceEntries.filter(r => r.initiatorType === 'img').length,
						scripts: resourceEntries.filter(r => r.initiatorType === 'script').length,
						stylesheets: resourceEntries.filter(r => r.initiatorType === 'link' && r.name.includes('.css')).length,
						totalSize: resourceEntries.reduce((sum, r) => sum + (r.transferSize || 0), 0)
					},
					memory: performance.memory ? {
						usedJSHeapSize: performance.memory.usedJSHeapSize,
						totalJSHeapSize: performance.memory.totalJSHeapSize,
						jsHeapSizeLimit: performance.memory.jsHeapSizeLimit
					} : null
				};
			})()
		`, &result.PerformanceAPI),
		
		rec.Stop(),
	)

	totalTime := time.Since(startTime)
	result.TotalTime = float64(totalTime.Nanoseconds()) / 1e6

	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	// Extract Core Web Vitals from performance API
	if perfData, ok := result.PerformanceAPI.(map[string]interface{}); ok {
		if vitals, ok := perfData["vitals"].(map[string]interface{}); ok {
			if lcp, ok := vitals["lcp"].(float64); ok {
				result.LCP = lcp
			}
			if fid, ok := vitals["fid"].(float64); ok {
				result.FID = fid
			}
			if cls, ok := vitals["cls"].(float64); ok {
				result.CLS = cls
			}
			if fcp, ok := vitals["fcp"].(float64); ok {
				result.FCP = fcp
			}
			if ttfb, ok := vitals["ttfb"].(float64); ok {
				result.TTFB = ttfb
			}
		}
	}

	// Extract resource statistics from HAR
	harData, err := rec.HAR()
	if err == nil {
		result.ResourceStats = extractResourceStats(harData)
	}

	result.Success = true
	return result
}

func extractResourceStats(harData string) ResourceStats {
	// Simple HAR parsing for resource statistics
	stats := ResourceStats{}
	
	// This is a simplified implementation
	// In practice, you'd parse the HAR JSON properly
	stats.TotalResources = countOccurrences(harData, `"request":`)
	stats.ImageCount = countOccurrences(harData, `"image/`)
	stats.ScriptCount = countOccurrences(harData, `"javascript"`)
	stats.StylesheetCount = countOccurrences(harData, `"text/css"`)
	
	return stats
}

func countOccurrences(text, pattern string) int {
	count := 0
	start := 0
	for {
		pos := strings.Index(text[start:], pattern)
		if pos == -1 {
			break
		}
		count++
		start += pos + len(pattern)
	}
	return count
}

func displayResults(result WebVitalsResult) {
	fmt.Printf("\n=== Core Web Vitals Results ===\n")
	
	// Core Web Vitals
	fmt.Printf("LCP (Largest Contentful Paint): %.2f ms %s\n", 
		result.LCP, getVitalStatus(result.LCP, 2500, 4000))
	fmt.Printf("FID (First Input Delay): %.2f ms %s\n", 
		result.FID, getVitalStatus(result.FID, 100, 300))
	fmt.Printf("CLS (Cumulative Layout Shift): %.3f %s\n", 
		result.CLS, getVitalStatus(result.CLS, 0.1, 0.25))
	
	// Additional metrics
	fmt.Printf("\n=== Additional Metrics ===\n")
	fmt.Printf("FCP (First Contentful Paint): %.2f ms %s\n", 
		result.FCP, getVitalStatus(result.FCP, 1800, 3000))
	fmt.Printf("TTFB (Time to First Byte): %.2f ms %s\n", 
		result.TTFB, getVitalStatus(result.TTFB, 800, 1800))
	fmt.Printf("Total Load Time: %.2f ms\n", result.TotalTime)
	
	// Resource statistics
	fmt.Printf("\n=== Resource Statistics ===\n")
	fmt.Printf("Total Resources: %d\n", result.ResourceStats.TotalResources)
	fmt.Printf("Images: %d\n", result.ResourceStats.ImageCount)
	fmt.Printf("Scripts: %d\n", result.ResourceStats.ScriptCount)
	fmt.Printf("Stylesheets: %d\n", result.ResourceStats.StylesheetCount)
	fmt.Printf("Total Size: %d bytes\n", result.ResourceStats.TotalSize)
	
	// Performance grades
	fmt.Printf("\n=== Performance Grade ===\n")
	grade := calculatePerformanceGrade(result)
	fmt.Printf("Overall Grade: %s\n", grade)
}

func getVitalStatus(value, good, needsImprovement float64) string {
	if value <= good {
		return "✓ Good"
	} else if value <= needsImprovement {
		return "⚠ Needs Improvement"
	} else {
		return "✗ Poor"
	}
}

func calculatePerformanceGrade(result WebVitalsResult) string {
	score := 0
	
	// LCP scoring
	if result.LCP <= 2500 {
		score += 25
	} else if result.LCP <= 4000 {
		score += 15
	}
	
	// FID scoring
	if result.FID <= 100 {
		score += 25
	} else if result.FID <= 300 {
		score += 15
	}
	
	// CLS scoring
	if result.CLS <= 0.1 {
		score += 25
	} else if result.CLS <= 0.25 {
		score += 15
	}
	
	// FCP scoring
	if result.FCP <= 1800 {
		score += 25
	} else if result.FCP <= 3000 {
		score += 15
	}
	
	// Grade calculation
	if score >= 90 {
		return "A (Excellent)"
	} else if score >= 80 {
		return "B (Good)"
	} else if score >= 70 {
		return "C (Fair)"
	} else if score >= 60 {
		return "D (Poor)"
	} else {
		return "F (Very Poor)"
	}
}

func saveReport(result WebVitalsResult) {
	// Save as JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}

	filename := fmt.Sprintf("web-vitals-report-%d.json", time.Now().Unix())
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		log.Printf("Error writing report: %v", err)
		return
	}

	fmt.Printf("\nDetailed report saved to %s\n", filename)
}