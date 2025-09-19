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

// testingBWrapper wraps testing.B to implement testing.TB interface
type testingBWrapper struct {
	*testing.B
}

func (w *testingBWrapper) Helper() {
	w.B.Helper()
}

func (w *testingBWrapper) Fatal(args ...interface{}) {
	w.B.Fatal(args...)
}

func (w *testingBWrapper) Fatalf(format string, args ...interface{}) {
	w.B.Fatalf(format, args...)
}

func (w *testingBWrapper) Error(args ...interface{}) {
	w.B.Error(args...)
}

func (w *testingBWrapper) Errorf(format string, args ...interface{}) {
	w.B.Errorf(format, args...)
}

func (w *testingBWrapper) Skip(args ...interface{}) {
	w.B.Skip(args...)
}

func (w *testingBWrapper) SkipNow() {
	w.B.SkipNow()
}

func (w *testingBWrapper) Skipf(format string, args ...interface{}) {
	w.B.Skipf(format, args...)
}

func (w *testingBWrapper) Log(args ...interface{}) {
	w.B.Log(args...)
}

func (w *testingBWrapper) Logf(format string, args ...interface{}) {
	w.B.Logf(format, args...)
}

// TestPerformanceMultipleNavigations tests rapid sequential navigations
func TestPerformanceMultipleNavigations(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	// Perform many navigations
	navigationCount := 50
	start := time.Now()

	for i := 0; i < navigationCount; i++ {
		url := fmt.Sprintf("%s/?n=%d", ts.URL, i)
		err := b.Navigate(url)
		if err != nil {
			t.Errorf("Navigation %d failed: %v", i, err)
		}
	}

	elapsed := time.Since(start)
	avgTime := elapsed / time.Duration(navigationCount)

	t.Logf("Performed %d navigations in %v (avg: %v per navigation)",
		navigationCount, elapsed, avgTime)

	// Check that average time is reasonable
	if avgTime > 500*time.Millisecond {
		t.Errorf("Navigation too slow: %v average", avgTime)
	}
}

// TestPerformanceConcurrentPages tests multiple pages operating concurrently
func TestPerformanceConcurrentPages(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	pageCount := 5
	operationsPerPage := 10

	start := time.Now()
	var wg sync.WaitGroup
	errors := make(chan error, pageCount*operationsPerPage)

	for i := 0; i < pageCount; i++ {
		wg.Add(1)
		go func(pageNum int) {
			defer wg.Done()

			page, err := b.NewPage()
			if err != nil {
				errors <- fmt.Errorf("failed to create page %d: %v", pageNum, err)
				return
			}
			defer page.Close()

			// Perform operations on each page
			for j := 0; j < operationsPerPage; j++ {
				url := fmt.Sprintf("%s/?page=%d&op=%d", ts.URL, pageNum, j)
				if err := page.Navigate(url); err != nil {
					errors <- fmt.Errorf("page %d navigation %d failed: %v", pageNum, j, err)
					continue
				}

				// Do some work
				if _, err := page.Title(); err != nil {
					errors <- fmt.Errorf("page %d get title failed: %v", pageNum, err)
				}

				if err := page.Evaluate("document.body.textContent.length", nil); err != nil {
					errors <- fmt.Errorf("page %d evaluate failed: %v", pageNum, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	elapsed := time.Since(start)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Error(err)
		errorCount++
	}

	totalOps := pageCount * operationsPerPage
	t.Logf("Performed %d operations across %d pages in %v", totalOps, pageCount, elapsed)

	if errorCount > totalOps/10 { // Allow up to 10% errors
		t.Errorf("Too many errors: %d out of %d operations", errorCount, totalOps)
	}
}

// TestPerformanceLargeDOM tests handling of large DOM trees
func TestPerformanceLargeDOM(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Create a page with a large DOM
	elementCount := 10000
	html := `<!DOCTYPE html><html><body><div id="container">`
	for i := 0; i < elementCount; i++ {
		html += fmt.Sprintf(`<div class="item" data-index="%d">Item %d</div>`, i, i)
	}
	html += `</div></body></html>`

	dataURL := "data:text/html," + html

	start := time.Now()
	err = page.Navigate(dataURL)
	if err != nil {
		t.Fatal(err)
	}
	navTime := time.Since(start)

	// Query all elements
	start = time.Now()
	elements, err := page.QuerySelectorAll(".item")
	if err != nil {
		t.Fatal(err)
	}
	queryTime := time.Since(start)

	if len(elements) != elementCount {
		t.Errorf("Expected %d elements, got %d", elementCount, len(elements))
	}

	t.Logf("Large DOM test: Navigation=%v, Query %d elements=%v",
		navTime, elementCount, queryTime)

	// Performance benchmarks
	if navTime > 5*time.Second {
		t.Errorf("Navigation too slow for large DOM: %v", navTime)
	}
	if queryTime > 2*time.Second {
		t.Errorf("Query too slow for large DOM: %v", queryTime)
	}
}

// TestPerformanceMemoryLeaks tests for memory leaks during repeated operations
func TestPerformanceMemoryLeaks(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ts := newTestServer()
	defer ts.Close()

	// Get initial memory stats
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	// Perform many page create/destroy cycles
	cycles := 20
	for i := 0; i < cycles; i++ {
		func() {
			b, cleanup := createTestBrowser(t)
			defer cleanup()

			// Create and destroy pages
			for j := 0; j < 5; j++ {
				page, err := b.NewPage()
				if err != nil {
					t.Fatal(err)
				}

				err = page.Navigate(ts.URL)
				if err != nil {
					t.Fatal(err)
				}

				// Do some work
				page.GetText("#title")
				page.Evaluate("1+1", nil)

				page.Close()
			}
		}()

		// Force GC every few cycles
		if i%5 == 0 {
			runtime.GC()
			runtime.Gosched()
		}
	}

	// Final GC and memory check
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	heapGrowth := int64(finalMem.HeapAlloc) - int64(initialMem.HeapAlloc)
	heapGrowthMB := heapGrowth / 1024 / 1024

	t.Logf("Memory growth after %d cycles: %d MB", cycles, heapGrowthMB)

	// Allow some growth but flag potential leaks
	if heapGrowthMB > 100 {
		t.Errorf("Excessive memory growth: %d MB", heapGrowthMB)
	}
}

// TestPerformanceScriptExecution tests JavaScript execution performance
func TestPerformanceScriptExecution(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate("about:blank")
	if err != nil {
		t.Fatal(err)
	}

	// Test simple script execution
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		var result float64
		err := page.Evaluate(fmt.Sprintf("%d + %d", i, i), &result)
		if err != nil {
			t.Fatal(err)
		}
		if result != float64(2*i) {
			t.Errorf("Wrong result: got %f, want %d", result, 2*i)
		}
	}

	elapsed := time.Since(start)
	avgTime := elapsed / time.Duration(iterations)

	t.Logf("Executed %d scripts in %v (avg: %v per script)", iterations, elapsed, avgTime)

	if avgTime > 10*time.Millisecond {
		t.Errorf("Script execution too slow: %v average", avgTime)
	}
}

// TestPerformanceNetworkLoad tests performance with heavy network activity
func TestPerformanceNetworkLoad(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Create page that makes many network requests
	requestCount := 50
	html := `<!DOCTYPE html><html><body><div id="status">Loading...</div><script>
		let loaded = 0;
		const total = ` + fmt.Sprintf("%d", requestCount) + `;
		
		for (let i = 0; i < total; i++) {
			fetch('/api/data?n=' + i)
				.then(() => {
					loaded++;
					if (loaded === total) {
						document.getElementById('status').textContent = 'All loaded';
					}
				});
		}
	</script></body></html>`

	start := time.Now()
	err = page.Navigate("data:text/html," + html)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for all requests to complete
	err = page.WaitForFunction(
		`document.getElementById('status').textContent === 'All loaded'`,
		10*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)

	t.Logf("Loaded %d network requests in %v", requestCount, elapsed)

	if elapsed > 5*time.Second {
		t.Errorf("Network loading too slow: %v for %d requests", elapsed, requestCount)
	}
}

// TestPerformanceScreenshots tests screenshot performance
func TestPerformanceScreenshots(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Take multiple screenshots
	screenshotCount := 20
	start := time.Now()
	totalSize := 0

	for i := 0; i < screenshotCount; i++ {
		screenshot, err := page.Screenshot()
		if err != nil {
			t.Fatal(err)
		}
		totalSize += len(screenshot)
	}

	elapsed := time.Since(start)
	avgTime := elapsed / time.Duration(screenshotCount)
	avgSize := totalSize / screenshotCount

	t.Logf("Took %d screenshots in %v (avg: %v per screenshot, avg size: %d KB)",
		screenshotCount, elapsed, avgTime, avgSize/1024)

	if avgTime > 200*time.Millisecond {
		t.Errorf("Screenshot too slow: %v average", avgTime)
	}
}

// BenchmarkBrowserLaunch benchmarks browser launch time
func BenchmarkBrowserLaunch(b *testing.B) {
	skipIfNoChromish(b)

	ctx := context.Background()
	chromePath := findChrome()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		profileMgr, _ := chromeprofiles.NewProfileManager()
		browser, err := browser.New(ctx, profileMgr,
			browser.WithHeadless(true),
			browser.WithChromePath(chromePath),
		)
		if err != nil {
			b.Fatal(err)
		}

		err = browser.Launch(ctx)
		if err != nil {
			b.Fatal(err)
		}

		browser.Close()
	}
}

// BenchmarkPageNavigation benchmarks page navigation
func BenchmarkPageNavigation(b *testing.B) {
	ts := newTestServer()
	defer ts.Close()

	browser, cleanup := createTestBrowser(b)
	defer cleanup()

	page, err := browser.NewPage()
	if err != nil {
		b.Fatal(err)
	}
	defer page.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := page.Navigate(ts.URL)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkElementQuery benchmarks element querying
func BenchmarkElementQuery(b *testing.B) {
	ts := newTestServer()
	defer ts.Close()

	browser, cleanup := createTestBrowser(b)
	defer cleanup()

	page, err := browser.NewPage()
	if err != nil {
		b.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate(ts.URL)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		element, err := page.QuerySelector("#title")
		if err != nil {
			b.Fatal(err)
		}
		if element == nil {
			b.Fatal("Element not found")
		}
	}
}

// BenchmarkScriptExecution benchmarks JavaScript execution
func BenchmarkScriptExecution(b *testing.B) {
	browser, cleanup := createTestBrowser(b)
	defer cleanup()

	page, err := browser.NewPage()
	if err != nil {
		b.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate("about:blank")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result float64
		err := page.Evaluate("1 + 1", &result)
		if err != nil {
			b.Fatal(err)
		}
	}
}
