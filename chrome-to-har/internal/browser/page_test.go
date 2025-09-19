package browser_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// TestPageBasicOperations tests basic page operations
func TestPageBasicOperations(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	// Create a new page
	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Navigate
	err = page.Navigate(ts.URL, browser.NavigateWithTimeout(10*time.Second))
	if err != nil {
		t.Errorf("Failed to navigate: %v", err)
	}

	// Get title
	title, err := page.Title()
	if err != nil {
		t.Errorf("Failed to get title: %v", err)
	}
	if title != "Test Page" {
		t.Errorf("Unexpected title: got %s, want Test Page", title)
	}

	// Get URL
	url, err := page.URL()
	if err != nil {
		t.Errorf("Failed to get URL: %v", err)
	}
	if !strings.HasPrefix(url, ts.URL) {
		t.Errorf("Unexpected URL: got %s, want prefix %s", url, ts.URL)
	}

	// Get content
	content, err := page.Content()
	if err != nil {
		t.Errorf("Failed to get content: %v", err)
	}
	if !strings.Contains(content, "Test Page") {
		t.Error("Content does not contain expected text")
	}
}

// TestPageInteractions tests page interaction methods
func TestPageInteractions(t *testing.T) {
	t.Parallel()
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

	// Test clicking
	err = page.Click("#clickme", browser.ClickWithTimeout(5*time.Second))
	if err != nil {
		t.Errorf("Failed to click button: %v", err)
	}

	// Verify click result
	visible, err := page.ElementVisible("#result")
	if err != nil {
		t.Errorf("Failed to check element visibility: %v", err)
	}
	if !visible {
		t.Error("Result div should be visible after click")
	}

	// Test typing
	testText := "Hello, World!"
	err = page.Type("#textinput", testText, browser.TypeWithTimeout(5*time.Second))
	if err != nil {
		t.Errorf("Failed to type text: %v", err)
	}

	// Verify typed text
	value, err := page.GetAttribute("#textinput", "value")
	if err != nil {
		t.Errorf("Failed to get input value: %v", err)
	}
	if value != testText {
		t.Errorf("Unexpected input value: got %s, want %s", value, testText)
	}
}

// TestPageWaitForSelector tests waiting for elements
func TestPageWaitForSelector(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate(ts.URL + "/delayed")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for delayed element to appear
	err = page.WaitForSelector("#delayed",
		browser.WaitWithTimeout(5*time.Second),
		browser.WaitWithState("visible"))
	if err != nil {
		t.Errorf("Failed to wait for selector: %v", err)
	}

	// Verify element is visible
	visible, err := page.ElementVisible("#delayed")
	if err != nil {
		t.Errorf("Failed to check visibility: %v", err)
	}
	if !visible {
		t.Error("Delayed element should be visible")
	}
}

// TestPageEvaluate tests JavaScript evaluation
func TestPageEvaluate(t *testing.T) {
	t.Parallel()
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

	// Evaluate simple expression
	var result float64
	err = page.Evaluate("1 + 2", &result)
	if err != nil {
		t.Errorf("Failed to evaluate expression: %v", err)
	}
	if result != 3 {
		t.Errorf("Unexpected result: got %f, want 3", result)
	}

	// Evaluate DOM query
	var text string
	err = page.Evaluate(`document.getElementById('title').textContent`, &text)
	if err != nil {
		t.Errorf("Failed to evaluate DOM query: %v", err)
	}
	if text != "Test Page" {
		t.Errorf("Unexpected text: got %s, want Test Page", text)
	}

	// Evaluate without result
	err = page.Evaluate(`console.log('test')`, nil)
	if err != nil {
		t.Errorf("Failed to evaluate without result: %v", err)
	}
}

// TestPageScreenshot tests screenshot functionality
func TestPageScreenshot(t *testing.T) {
	t.Parallel()
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

	// Take full page screenshot
	screenshot, err := page.Screenshot(browser.ScreenshotFullPage(true))
	if err != nil {
		t.Errorf("Failed to take screenshot: %v", err)
	}
	if len(screenshot) == 0 {
		t.Error("Screenshot is empty")
	}

	// PNG magic number
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47}
	if !bytes.HasPrefix(screenshot, pngHeader) {
		t.Error("Screenshot is not a valid PNG")
	}

	// Take element screenshot
	elementShot, err := page.Screenshot(browser.ScreenshotSelector("#title"))
	if err != nil {
		t.Errorf("Failed to take element screenshot: %v", err)
	}
	if len(elementShot) == 0 {
		t.Error("Element screenshot is empty")
	}
}

// TestPageFormInteraction tests form interactions
func TestPageFormInteraction(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	err = page.Navigate(ts.URL + "/form")
	if err != nil {
		t.Fatal(err)
	}

	// Type in form field
	err = page.Type("#name", "Test User")
	if err != nil {
		t.Errorf("Failed to type in form: %v", err)
	}

	// Select option
	err = page.SelectOption("#option", "opt2")
	if err != nil {
		t.Errorf("Failed to select option: %v", err)
	}

	// Submit form
	err = page.Click("#submit")
	if err != nil {
		t.Errorf("Failed to submit form: %v", err)
	}

	// Wait for result
	err = page.WaitForSelector("#result", browser.WaitWithTimeout(5*time.Second))
	if err != nil {
		t.Errorf("Failed to wait for form result: %v", err)
	}

	// Check result
	resultText, err := page.GetText("#result")
	if err != nil {
		t.Errorf("Failed to get result text: %v", err)
	}
	if !strings.Contains(resultText, "Test User") {
		t.Errorf("Form submission result incorrect: %s", resultText)
	}
}

// TestPageViewport tests viewport manipulation
func TestPageViewport(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Set viewport size
	err = page.SetViewport(800, 600)
	if err != nil {
		t.Errorf("Failed to set viewport: %v", err)
	}

	err = page.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Verify viewport size via JavaScript
	var width, height float64
	err = page.Evaluate(`window.innerWidth`, &width)
	if err != nil {
		t.Errorf("Failed to get window width: %v", err)
	}
	err = page.Evaluate(`window.innerHeight`, &height)
	if err != nil {
		t.Errorf("Failed to get window height: %v", err)
	}

	// Allow some tolerance for browser chrome
	if width != 800 {
		t.Errorf("Unexpected viewport width: got %f, want 800", width)
	}
	if height < 550 || height > 600 {
		t.Errorf("Unexpected viewport height: got %f, want ~600", height)
	}
}

// TestPageFocus tests element focus
func TestPageFocus(t *testing.T) {
	t.Parallel()
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

	// Focus on input
	err = page.Focus("#textinput")
	if err != nil {
		t.Errorf("Failed to focus element: %v", err)
	}

	// Verify focus
	var focused bool
	err = page.Evaluate(`document.activeElement.id === 'textinput'`, &focused)
	if err != nil {
		t.Errorf("Failed to check focus: %v", err)
	}
	if !focused {
		t.Error("Input should be focused")
	}
}

// TestPagePress tests key press functionality
func TestPagePress(t *testing.T) {
	t.Parallel()
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

	// Focus input
	err = page.Focus("#textinput")
	if err != nil {
		t.Fatal(err)
	}

	// Type using key presses
	keys := []string{"H", "e", "l", "l", "o"}
	for _, key := range keys {
		err = page.Press(key)
		if err != nil {
			t.Errorf("Failed to press key %s: %v", key, err)
		}
	}

	// Verify typed text
	value, err := page.GetAttribute("#textinput", "value")
	if err != nil {
		t.Errorf("Failed to get input value: %v", err)
	}
	if value != "Hello" {
		t.Errorf("Unexpected value after key presses: got %s, want Hello", value)
	}
}

// TestPageWaitForFunction tests waiting for JavaScript conditions
func TestPageWaitForFunction(t *testing.T) {
	t.Parallel()
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

	// Set a value after delay
	err = page.Evaluate(`
		setTimeout(() => {
			window.testValue = 'ready';
		}, 500);
	`, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for value to be set
	err = page.WaitForFunction(`window.testValue === 'ready'`, 2*time.Second)
	if err != nil {
		t.Errorf("Failed to wait for function: %v", err)
	}

	// Verify value is set
	var value string
	err = page.Evaluate(`window.testValue`, &value)
	if err != nil {
		t.Errorf("Failed to get test value: %v", err)
	}
	if value != "ready" {
		t.Errorf("Unexpected value: got %s, want ready", value)
	}
}

// TestPageMultipleTabs tests managing multiple pages/tabs
func TestPageMultipleTabs(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	// Create multiple pages
	pages := make([]*browser.Page, 3)
	for i := range pages {
		page, err := b.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		pages[i] = page
		defer page.Close()

		// Navigate each to different content
		url := fmt.Sprintf("%s/?page=%d", ts.URL, i)
		err = page.Navigate(url)
		if err != nil {
			t.Errorf("Failed to navigate page %d: %v", i, err)
		}
	}

	// Verify each page has correct URL
	for i, page := range pages {
		url, err := page.URL()
		if err != nil {
			t.Errorf("Failed to get URL for page %d: %v", i, err)
		}

		expectedURL := fmt.Sprintf("%s/?page=%d", ts.URL, i)
		if url != expectedURL {
			t.Errorf("Page %d has wrong URL: got %s, want %s", i, url, expectedURL)
		}
	}

	// Get all pages
	allPages, err := b.Pages()
	if err != nil {
		t.Errorf("Failed to get all pages: %v", err)
	}

	// Should have at least our created pages
	if len(allPages) < len(pages) {
		t.Errorf("Expected at least %d pages, got %d", len(pages), len(allPages))
	}
}

// TestPageHover tests hover functionality
func TestPageHover(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	// Create page with hover effect
	err = page.Navigate("data:text/html,<div id='hover' style='width:100px;height:100px;background:red;' onmouseover='this.style.background=\"blue\"'>Hover me</div>")
	if err != nil {
		t.Fatal(err)
	}

	// Get initial color
	var initialColor string
	err = page.Evaluate(`getComputedStyle(document.getElementById('hover')).backgroundColor`, &initialColor)
	if err != nil {
		t.Fatal(err)
	}

	// Hover over element
	err = page.Hover("#hover")
	if err != nil {
		t.Errorf("Failed to hover: %v", err)
	}

	// Small delay for hover effect
	time.Sleep(100 * time.Millisecond)

	// Get color after hover
	var hoverColor string
	err = page.Evaluate(`getComputedStyle(document.getElementById('hover')).backgroundColor`, &hoverColor)
	if err != nil {
		t.Fatal(err)
	}

	// Colors should be different
	if initialColor == hoverColor {
		t.Error("Hover did not change element color")
	}
}
