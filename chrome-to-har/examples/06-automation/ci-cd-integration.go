// CI/CD Integration example
// This example shows how to integrate chrome-to-har into CI/CD pipelines
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

type CITestResult struct {
	URL          string    `json:"url"`
	Timestamp    time.Time `json:"timestamp"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	LoadTime     float64   `json:"load_time_ms"`
	StatusCode   int       `json:"status_code"`
	PageTitle    string    `json:"page_title"`
	TestsPassed  int       `json:"tests_passed"`
	TestsFailed  int       `json:"tests_failed"`
	Assertions   []Assertion `json:"assertions"`
}

type Assertion struct {
	Name     string `json:"name"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Passed   bool   `json:"passed"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ci-cd-integration.go <url>")
		os.Exit(1)
	}

	url := os.Args[1]
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create Chrome browser for CI environment
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.UserAgent("Mozilla/5.0 (compatible; CI-Bot/1.0)"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Run CI tests
	result := runCITests(chromeCtx, url)
	
	// Output results
	fmt.Printf("CI Test Results for %s\n", url)
	fmt.Printf("Success: %t\n", result.Success)
	fmt.Printf("Tests Passed: %d\n", result.TestsPassed)
	fmt.Printf("Tests Failed: %d\n", result.TestsFailed)
	fmt.Printf("Load Time: %.2f ms\n", result.LoadTime)
	
	// Save results
	saveResults(result)
	
	// Exit with appropriate code
	if !result.Success {
		os.Exit(1)
	}
}

func runCITests(chromeCtx context.Context, url string) CITestResult {
	startTime := time.Now()
	
	result := CITestResult{
		URL:       url,
		Timestamp: startTime,
		Success:   false,
	}

	rec := recorder.New()
	
	err := chromedp.Run(chromeCtx,
		rec.Start(),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Title(&result.PageTitle),
		rec.Stop(),
	)
	
	result.LoadTime = float64(time.Since(startTime).Nanoseconds()) / 1e6
	
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	
	// Run assertions
	assertions := []func() Assertion{
		func() Assertion {
			return Assertion{
				Name: "Page loads successfully",
				Expected: "non-empty title",
				Actual: result.PageTitle,
				Passed: result.PageTitle != "",
			}
		},
		func() Assertion {
			return Assertion{
				Name: "Load time under 5 seconds",
				Expected: "< 5000ms",
				Actual: fmt.Sprintf("%.2fms", result.LoadTime),
				Passed: result.LoadTime < 5000,
			}
		},
	}
	
	for _, assert := range assertions {
		assertion := assert()
		result.Assertions = append(result.Assertions, assertion)
		if assertion.Passed {
			result.TestsPassed++
		} else {
			result.TestsFailed++
		}
	}
	
	result.Success = result.TestsFailed == 0
	return result
}

func saveResults(result CITestResult) {
	jsonData, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile("ci-test-results.json", jsonData, 0644)
}