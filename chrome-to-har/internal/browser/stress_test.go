//go:build stress
// +build stress

// Stress tests for browser package - run with: go test -tags stress

package browser_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

// TestStressLongRunningSession tests browser under prolonged usage
func TestStressLongRunningSession(t *testing.T) {
	skipIfNoChromish(t)

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	// Run for 30 minutes with continuous activity
	testDuration := 30 * time.Minute
	start := time.Now()
	operationCount := 0
	errorCount := 0

	for time.Since(start) < testDuration {
		// Create page
		page, err := b.NewPage()
		if err != nil {
			errorCount++
			continue
		}

		// Navigate
		url := fmt.Sprintf("%s/?stress=%d", ts.URL, operationCount)
		err = page.Navigate(url)
		if err != nil {
			errorCount++
			page.Close()
			continue
		}

		// Perform operations
		page.Title()
		page.Evaluate("1+1", nil)
		page.QuerySelector("#title")

		// Take screenshot occasionally
		if operationCount%10 == 0 {
			page.Screenshot()
		}

		page.Close()
		operationCount++

		// Force GC every 100 operations
		if operationCount%100 == 0 {
			runtime.GC()
			t.Logf("Completed %d operations, %d errors", operationCount, errorCount)
		}

		// Small delay to prevent overwhelming
		time.Sleep(100 * time.Millisecond)
	}

	errorRate := float64(errorCount) / float64(operationCount) * 100
	t.Logf("Stress test completed: %d operations, %d errors (%.2f%% error rate)",
		operationCount, errorCount, errorRate)

	// Allow up to 5% error rate
	if errorRate > 5.0 {
		t.Errorf("Error rate too high: %.2f%%", errorRate)
	}
}

// TestStressMassiveConcurrency tests with many concurrent browser instances
func TestStressMassiveConcurrency(t *testing.T) {
	skipIfNoChromish(t)

	ts := newTestServer()
	defer ts.Close()

	browserCount := 10
	pagesPerBrowser := 5
	operationsPerPage := 20

	var wg sync.WaitGroup
	errors := make(chan error, browserCount*pagesPerBrowser*operationsPerPage)

	start := time.Now()

	for i := 0; i < browserCount; i++ {
		wg.Add(1)
		go func(browserNum int) {
			defer wg.Done()

			// Create browser
			ctx := context.Background()
			profileMgr, err := chromeprofiles.NewProfileManager()
			if err != nil {
				errors <- fmt.Errorf("browser %d: profile manager failed: %v", browserNum, err)
				return
			}

			b, err := browser.New(ctx, profileMgr,
				browser.WithHeadless(true),
				browser.WithChromePath(findChrome()),
				browser.WithTimeout(30),
			)
			if err != nil {
				errors <- fmt.Errorf("browser %d: creation failed: %v", browserNum, err)
				return
			}
			defer b.Close()

			err = b.Launch(ctx)
			if err != nil {
				errors <- fmt.Errorf("browser %d: launch failed: %v", browserNum, err)
				return
			}

			// Create pages
			for j := 0; j < pagesPerBrowser; j++ {
				page, err := b.NewPage()
				if err != nil {
					errors <- fmt.Errorf("browser %d page %d: creation failed: %v", browserNum, j, err)
					continue
				}

				// Perform operations
				for k := 0; k < operationsPerPage; k++ {
					url := fmt.Sprintf("%s/?b=%d&p=%d&op=%d", ts.URL, browserNum, j, k)
					if err := page.Navigate(url); err != nil {
						errors <- fmt.Errorf("browser %d page %d op %d: nav failed: %v", browserNum, j, k, err)
					}
					if _, err := page.Title(); err != nil {
						errors <- fmt.Errorf("browser %d page %d op %d: title failed: %v", browserNum, j, k, err)
					}
				}

				page.Close()
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	elapsed := time.Since(start)
	totalOps := browserCount * pagesPerBrowser * operationsPerPage

	errorCount := 0
	for err := range errors {
		t.Error(err)
		errorCount++
	}

	errorRate := float64(errorCount) / float64(totalOps) * 100
	t.Logf("Massive concurrency test: %d browsers, %d pages each, %d ops per page",
		browserCount, pagesPerBrowser, operationsPerPage)
	t.Logf("Total: %d operations in %v, %d errors (%.2f%% error rate)",
		totalOps, elapsed, errorCount, errorRate)

	// Allow up to 15% error rate for this stress test
	if errorRate > 15.0 {
		t.Errorf("Error rate too high: %.2f%%", errorRate)
	}
}

// TestStressResourceExhaustion tests behavior under resource constraints
func TestStressResourceExhaustion(t *testing.T) {
	skipIfNoChromish(t)

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	// Create many pages without closing them to exhaust resources
	pages := make([]*browser.Page, 0, 100)
	errorCount := 0

	for i := 0; i < 100; i++ {
		page, err := b.NewPage()
		if err != nil {
			errorCount++
			if errorCount > 50 { // Stop if too many errors
				break
			}
			continue
		}

		pages = append(pages, page)

		// Navigate each page
		url := fmt.Sprintf("%s/?exhaust=%d", ts.URL, i)
		err = page.Navigate(url)
		if err != nil {
			errorCount++
		}

		// Create complex DOM on each page
		script := `
			for (let i = 0; i < 1000; i++) {
				let div = document.createElement('div');
				div.textContent = 'Element ' + i;
				document.body.appendChild(div);
			}
		`
		err = page.Evaluate(script, nil)
		if err != nil {
			errorCount++
		}

		if i%10 == 0 {
			t.Logf("Created %d pages, %d errors", i+1, errorCount)
			runtime.GC() // Force garbage collection
		}
	}

	t.Logf("Resource exhaustion test: created %d pages, %d errors", len(pages), errorCount)

	// Clean up pages
	for _, page := range pages {
		page.Close()
	}

	// Final GC
	runtime.GC()
	time.Sleep(1 * time.Second)
	runtime.GC()
}

// TestStressNetworkHeavyLoad tests with intensive network activity
func TestStressNetworkHeavyLoad(t *testing.T) {
	skipIfNoChromish(t)

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Track network requests
	requestCount := 0
	err = page.Route(".*", func(req *browser.Request) error {
		requestCount++
		return req.Continue()
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create page that makes many simultaneous requests
	requestsToMake := 200
	html := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<body>
			<div id="status">Loading %d requests...</div>
			<script>
				let completed = 0;
				const total = %d;
				
				for (let i = 0; i < total; i++) {
					fetch('/api/data?heavy=' + i)
						.then(() => {
							completed++;
							document.getElementById('status').textContent = 
								'Completed ' + completed + '/' + total;
							if (completed === total) {
								document.getElementById('status').textContent = 'All completed';
							}
						})
						.catch(() => {
							completed++;
						});
				}
			</script>
		</body>
		</html>
	`, requestsToMake, requestsToMake)

	start := time.Now()
	err = page.Navigate("data:text/html," + html)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for all requests to complete with generous timeout
	err = page.WaitForFunction(
		`document.getElementById('status').textContent === 'All completed'`,
		60*time.Second,
	)
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("Heavy network load test timed out after %v", elapsed)
		t.Logf("Intercepted %d requests out of expected %d", requestCount, requestsToMake)
	} else {
		t.Logf("Heavy network load test completed in %v", elapsed)
		t.Logf("Successfully handled %d network requests", requestCount)
	}

	// Verify reasonable performance
	if elapsed > 2*time.Minute {
		t.Errorf("Network heavy load took too long: %v", elapsed)
	}
}
