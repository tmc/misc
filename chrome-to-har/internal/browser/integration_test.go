package browser_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/testutil"
)

// Integration tests for the complete browser package functionality
// These tests verify end-to-end browser automation capabilities

// TestBrowserFullWorkflow tests complete browser lifecycle
func TestBrowserFullWorkflow(t *testing.T) {
	skipIfNoChrome(t)

	ts := newTestServer()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create real profile manager
	profileMgr, err := chromeprofiles.NewProfileManager()
	if err != nil {
		t.Fatal(err)
	}

	b, err := browser.New(ctx, profileMgr,
		browser.WithHeadless(true),
		browser.WithTimeout(30),
		browser.WithChromePath(findChrome()),
		browser.WithVerbose(testing.Verbose()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Launch browser
	err = b.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Test basic navigation
	err = b.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Verify page loaded
	title, err := b.GetTitle()
	if err != nil {
		t.Fatal(err)
	}
	if title != "Test Page" {
		t.Errorf("Expected title 'Test Page', got %s", title)
	}

	// Test JavaScript execution
	result, err := b.ExecuteScript(`document.getElementById('title').textContent`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Test Page" {
		t.Errorf("Expected 'Test Page', got %v", result)
	}

	// Test getting HTML
	html, err := b.GetHTML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Test Page") {
		t.Error("HTML does not contain expected content")
	}

	// Test URL retrieval
	currentURL, err := b.GetURL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(currentURL, ts.URL) {
		t.Errorf("Unexpected URL: %s", currentURL)
	}
}

// TestPageCompleteWorkflow tests complete page interaction workflow
func TestPageCompleteWorkflow(t *testing.T) {
	skipIfNoChrome(t)

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	// Create page
	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Navigate with options
	err = page.Navigate(ts.URL, NavigateWithTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	// Test basic page operations
	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "Test Page" {
		t.Errorf("Expected title 'Test Page', got %s", title)
	}

	// Test element querying
	titleElement, err := page.QuerySelector("#title")
	if err != nil {
		t.Fatal(err)
	}
	if titleElement == nil {
		t.Fatal("Title element not found")
	}

	// Test element text
	text, err := titleElement.GetText()
	if err != nil {
		t.Fatal(err)
	}
	if text != "Test Page" {
		t.Errorf("Expected 'Test Page', got %s", text)
	}

	// Test element interaction
	button, err := page.QuerySelector("#clickme")
	if err != nil {
		t.Fatal(err)
	}

	err = button.Click()
	if err != nil {
		t.Fatal(err)
	}

	// Wait for result
	err = page.WaitForSelector("#result", WaitWithTimeout(5*time.Second), WaitWithState("visible"))
	if err != nil {
		t.Fatal(err)
	}

	// Verify result visible
	visible, err := page.ElementVisible("#result")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("Result should be visible after button click")
	}

	// Test input interaction
	input, err := page.QuerySelector("#textinput")
	if err != nil {
		t.Fatal(err)
	}

	err = input.Clear()
	if err != nil {
		t.Fatal(err)
	}

	testText := "Integration Test"
	err = input.Type(testText)
	if err != nil {
		t.Fatal(err)
	}

	value, err := input.GetAttribute("value")
	if err != nil {
		t.Fatal(err)
	}
	if value != testText {
		t.Errorf("Expected '%s', got '%s'", testText, value)
	}
}

// TestNetworkInterceptionWorkflow tests complete network interception
func TestNetworkInterceptionWorkflow(t *testing.T) {
	skipIfNoChrome(t)

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Track intercepted requests
	var interceptedRequests []*browser.Request
	var mu sync.Mutex

	// Set up route to intercept all requests
	err = page.Route(".*", func(req *browser.Request) error {
		mu.Lock()
		interceptedRequests = append(interceptedRequests, req)
		mu.Unlock()
		return req.Continue()
	})
	if err != nil {
		t.Fatal(err)
	}

	// Navigate to page with network activity
	err = page.Navigate(ts.URL + "/network-test")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for network activity
	time.Sleep(2 * time.Second)

	// Verify requests were intercepted
	mu.Lock()
	requestCount := len(interceptedRequests)
	mu.Unlock()

	if requestCount == 0 {
		t.Error("No requests were intercepted")
	}

	// Check for main page request
	found := false
	mu.Lock()
	for _, req := range interceptedRequests {
		if strings.Contains(req.URL, "/network-test") {
			found = true
			break
		}
	}
	mu.Unlock()

	if !found {
		t.Error("Main page request not found in intercepted requests")
	}
}

// TestScreenshotWorkflow tests screenshot functionality
func TestScreenshotWorkflow(t *testing.T) {
	skipIfNoChrome(t)

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

	// Test full page screenshot
	fullScreenshot, err := page.Screenshot(ScreenshotFullPage(true))
	if err != nil {
		t.Fatal(err)
	}

	if len(fullScreenshot) == 0 {
		t.Error("Full page screenshot is empty")
	}

	// Verify PNG format
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47}
	if len(fullScreenshot) < 4 || string(fullScreenshot[:4]) != string(pngHeader) {
		t.Error("Screenshot is not a valid PNG")
	}

	// Test element screenshot
	titleElement, err := page.QuerySelector("#title")
	if err != nil {
		t.Fatal(err)
	}

	elementScreenshot, err := titleElement.Screenshot()
	if err != nil {
		t.Fatal(err)
	}

	if len(elementScreenshot) == 0 {
		t.Error("Element screenshot is empty")
	}

	// Element screenshot should be smaller than full page
	if len(elementScreenshot) >= len(fullScreenshot) {
		t.Error("Element screenshot should be smaller than full page screenshot")
	}
}

// TestRemoteConnectionWorkflow tests remote Chrome connection
func TestRemoteConnectionWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping remote connection test in short mode")
	}

	skipIfNoChrome(t)

	// Start remote Chrome
	port := 9230
	cmd, cleanup := startRemoteChrome(t, port)
	defer cleanup()

	// Wait for startup
	time.Sleep(1 * time.Second)

	// Get debugging info
	info, err := browser.GetRemoteDebuggingInfo("localhost", port)
	if err != nil {
		t.Fatal(err)
	}

	if info.Browser == "" {
		t.Error("Browser info is empty")
	}

	// List tabs
	tabs, err := browser.ListTabs("localhost", port)
	if err != nil {
		t.Fatal(err)
	}

	if len(tabs) == 0 {
		t.Error("No tabs found")
	}

	// Connect to remote Chrome
	ctx := context.Background()
	profileMgr := testutil.NewMockProfileManager()

	b, err := browser.New(ctx, profileMgr,
		browser.WithRemoteChrome("localhost", port),
		browser.WithVerbose(testing.Verbose()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	err = b.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Test operations on remote browser
	ts := newTestServer()
	defer ts.Close()

	err = b.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	title, err := b.GetTitle()
	if err != nil {
		t.Fatal(err)
	}
	if title != "Test Page" {
		t.Errorf("Expected 'Test Page', got %s", title)
	}

	// Clean up remote Chrome
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// TestMultiPageConcurrency tests concurrent page operations
func TestMultiPageConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	skipIfNoChrome(t)

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	pageCount := 3
	operationsPerPage := 5

	var wg sync.WaitGroup
	errors := make(chan error, pageCount*operationsPerPage)

	for i := 0; i < pageCount; i++ {
		wg.Add(1)
		go func(pageNum int) {
			defer wg.Done()

			page, err := b.NewPage()
			if err != nil {
				errors <- fmt.Errorf("page %d: failed to create: %v", pageNum, err)
				return
			}
			defer page.Close()

			for j := 0; j < operationsPerPage; j++ {
				// Navigate
				url := fmt.Sprintf("%s/?page=%d&op=%d", ts.URL, pageNum, j)
				if err := page.Navigate(url); err != nil {
					errors <- fmt.Errorf("page %d op %d: nav failed: %v", pageNum, j, err)
					continue
				}

				// Get title
				if _, err := page.Title(); err != nil {
					errors <- fmt.Errorf("page %d op %d: title failed: %v", pageNum, j, err)
				}

				// Execute script
				var result float64
				if err := page.Evaluate("1+1", &result); err != nil {
					errors <- fmt.Errorf("page %d op %d: eval failed: %v", pageNum, j, err)
				}

				// Small delay
				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Error(err)
		errorCount++
	}

	totalOps := pageCount * operationsPerPage
	if errorCount > totalOps/10 { // Allow up to 10% errors
		t.Errorf("Too many errors: %d out of %d operations", errorCount, totalOps)
	}
}

// TestFormInteractionWorkflow tests complete form interaction
func TestFormInteractionWorkflow(t *testing.T) {
	skipIfNoChrome(t)

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Navigate to form page
	err = page.Navigate(ts.URL + "/form")
	if err != nil {
		t.Fatal(err)
	}

	// Fill out form
	nameInput, err := page.QuerySelector("#name")
	if err != nil {
		t.Fatal(err)
	}

	err = nameInput.Type("Integration Test User")
	if err != nil {
		t.Fatal(err)
	}

	// Select option
	selectElement, err := page.QuerySelector("#option")
	if err != nil {
		t.Fatal(err)
	}

	err = selectElement.SetAttribute("value", "opt2")
	if err != nil {
		t.Fatal(err)
	}

	// Submit form
	submitBtn, err := page.QuerySelector("#submit")
	if err != nil {
		t.Fatal(err)
	}

	err = submitBtn.Click()
	if err != nil {
		t.Fatal(err)
	}

	// Wait for form result
	err = page.WaitForSelector("#result", WaitWithTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	// Verify form submission
	result, err := page.GetText("#result")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Integration Test User") {
		t.Errorf("Form result does not contain expected text: %s", result)
	}
}

// TestErrorHandlingWorkflow tests error handling scenarios
func TestErrorHandlingWorkflow(t *testing.T) {
	skipIfNoChrome(t)

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Test navigation to invalid URL
	err = page.Navigate("http://invalid-domain-that-does-not-exist.test")
	if err == nil {
		t.Error("Expected error for invalid domain")
	}

	// Navigate to valid page first
	ts := newTestServer()
	defer ts.Close()

	err = page.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Test querying non-existent element
	element, err := page.QuerySelector("#does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if element != nil {
		t.Error("Expected nil element for non-existent selector")
	}

	// Test invalid JavaScript
	var result interface{}
	err = page.Evaluate("invalid javascript syntax!!!", &result)
	if err == nil {
		t.Error("Expected error for invalid JavaScript")
	}

	// Test timeout on slow operations
	err = page.WaitForSelector("#never-appears", WaitWithTimeout(100*time.Millisecond))
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// TestViewportAndResponsiveDesign tests viewport manipulation
func TestViewportAndResponsiveDesign(t *testing.T) {
	skipIfNoChrome(t)

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Test different viewport sizes
	viewports := []struct {
		width, height int
		name          string
	}{
		{1920, 1080, "Desktop"},
		{768, 1024, "Tablet"},
		{375, 667, "Mobile"},
	}

	for _, vp := range viewports {
		t.Run(vp.name, func(t *testing.T) {
			err := page.SetViewport(vp.width, vp.height)
			if err != nil {
				t.Fatal(err)
			}

			// Navigate to responsive test page
			responsiveHTML := fmt.Sprintf(`
				<!DOCTYPE html>
				<html>
				<head>
					<meta name="viewport" content="width=device-width, initial-scale=1">
					<style>
						body { margin: 0; font-family: Arial; }
						.container { padding: 20px; }
						@media (max-width: 600px) {
							.mobile-only { display: block; }
							.desktop-only { display: none; }
						}
						@media (min-width: 601px) {
							.mobile-only { display: none; }
							.desktop-only { display: block; }
						}
					</style>
				</head>
				<body>
					<div class="container">
						<div class="mobile-only" id="mobile">Mobile View</div>
						<div class="desktop-only" id="desktop">Desktop View</div>
					</div>
				</body>
				</html>
			`)

			err = page.Navigate("data:text/html," + responsiveHTML)
			if err != nil {
				t.Fatal(err)
			}

			// Verify viewport size
			var width, height float64
			err = page.Evaluate("window.innerWidth", &width)
			if err != nil {
				t.Fatal(err)
			}
			err = page.Evaluate("window.innerHeight", &height)
			if err != nil {
				t.Fatal(err)
			}

			if int(width) != vp.width {
				t.Errorf("Width mismatch: expected %d, got %d", vp.width, int(width))
			}

			// Verify responsive behavior
			if vp.width <= 600 {
				// Mobile view
				mobileVisible, err := page.ElementVisible("#mobile")
				if err != nil {
					t.Fatal(err)
				}
				if !mobileVisible {
					t.Error("Mobile element should be visible on mobile viewport")
				}
			} else {
				// Desktop view
				desktopVisible, err := page.ElementVisible("#desktop")
				if err != nil {
					t.Fatal(err)
				}
				if !desktopVisible {
					t.Error("Desktop element should be visible on desktop viewport")
				}
			}
		})
	}
}

// TestStressScenarios tests browser under stress conditions
func TestStressScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	skipIfNoChrome(t)

	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Rapid navigation test
	t.Run("RapidNavigation", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			url := fmt.Sprintf("%s/?rapid=%d", ts.URL, i)
			err := page.Navigate(url)
			if err != nil {
				t.Errorf("Navigation %d failed: %v", i, err)
			}
		}
	})

	// Heavy DOM manipulation
	t.Run("HeavyDOMManipulation", func(t *testing.T) {
		err := page.Navigate("about:blank")
		if err != nil {
			t.Fatal(err)
		}

		// Create many elements
		for i := 0; i < 100; i++ {
			script := fmt.Sprintf(`
				var div = document.createElement('div');
				div.id = 'element%d';
				div.textContent = 'Element %d';
				document.body.appendChild(div);
			`, i, i)

			err := page.Evaluate(script, nil)
			if err != nil {
				t.Errorf("DOM manipulation %d failed: %v", i, err)
			}
		}

		// Verify elements were created
		elements, err := page.QuerySelectorAll("div")
		if err != nil {
			t.Fatal(err)
		}
		if len(elements) != 100 {
			t.Errorf("Expected 100 elements, got %d", len(elements))
		}
	})

	// Multiple script executions
	t.Run("MultipleScriptExecutions", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			var result float64
			err := page.Evaluate(fmt.Sprintf("%d * 2", i), &result)
			if err != nil {
				t.Errorf("Script execution %d failed: %v", i, err)
			}
			if result != float64(i*2) {
				t.Errorf("Wrong result for %d: got %f, want %d", i, result, i*2)
			}
		}
	})
}

// Helper functions for option compatibility

// NavigateWithTimeout creates a navigate option with timeout
func NavigateWithTimeout(timeout time.Duration) browser.NavigateOption {
	return browser.WithNavigateTimeout(timeout)
}

// ClickWithTimeout creates a click option with timeout
func ClickWithTimeout(timeout time.Duration) browser.ClickOption {
	return browser.WithClickTimeout(timeout)
}

// TypeWithTimeout creates a type option with timeout
func TypeWithTimeout(timeout time.Duration) browser.TypeOption {
	return browser.WithTypeTimeout(timeout)
}

// WaitWithTimeout creates a wait option with timeout
func WaitWithTimeout(timeout time.Duration) browser.WaitOption {
	return browser.WithWaitTimeout(timeout)
}

// WaitWithState creates a wait option with state
func WaitWithState(state string) browser.WaitOption {
	return browser.WithWaitState(state)
}

// ScreenshotFullPage creates a screenshot option for full page
func ScreenshotFullPage(fullPage bool) browser.ScreenshotOption {
	if fullPage {
		return browser.WithFullPage()
	}
	return func(*browser.ScreenshotOptions) {} // No-op if false
}
