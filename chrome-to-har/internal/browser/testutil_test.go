package browser_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// TestBrowserPool manages a pool of browser instances for testing
type TestBrowserPool struct {
	browsers []*browser.Browser
	t        testing.TB
}

// NewTestBrowserPool creates a new browser pool
func NewTestBrowserPool(t testing.TB) *TestBrowserPool {
	return &TestBrowserPool{
		browsers: make([]*browser.Browser, 0),
		t:        t,
	}
}

// GetBrowser gets or creates a browser from the pool
func (p *TestBrowserPool) GetBrowser(opts ...browser.Option) *browser.Browser {
	b, cleanup := createTestBrowser(p.t, opts...)
	p.browsers = append(p.browsers, b)
	p.t.Cleanup(cleanup)
	return b
}

// Cleanup closes all browsers in the pool
func (p *TestBrowserPool) Cleanup() {
	for _, b := range p.browsers {
		b.Close()
	}
}

// TestFileServer serves test files over HTTP
type TestFileServer struct {
	*httptest.Server
	Root string
}

// NewTestFileServer creates a new test file server
func NewTestFileServer(t testing.TB) *TestFileServer {
	// Create temp directory for test files
	tempDir, err := os.MkdirTemp("", "browser-test-files")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	fs := &TestFileServer{
		Root: tempDir,
	}

	// Create file server
	fileServer := http.FileServer(http.Dir(tempDir))
	fs.Server = httptest.NewServer(fileServer)

	t.Cleanup(func() {
		fs.Server.Close()
	})

	return fs
}

// WriteFile writes a file to the test server
func (fs *TestFileServer) WriteFile(path, content string) error {
	fullPath := filepath.Join(fs.Root, path)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

// URL returns the URL for a file path
func (fs *TestFileServer) URL(path string) string {
	return fs.Server.URL + "/" + path
}

// WaitForCondition waits for a condition to be true
func WaitForCondition(t testing.TB, timeout time.Duration, check func() bool, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if check() {
			return
		}

		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				t.Fatalf("Timeout waiting for condition: %s", message)
			}
		}
	}
}

// AssertElementText asserts that an element has specific text
func AssertElementText(t testing.TB, page *browser.Page, selector, expected string) {
	t.Helper()

	element, err := page.QuerySelector(selector)
	if err != nil {
		t.Fatalf("Failed to find element %s: %v", selector, err)
	}

	if element == nil {
		t.Fatalf("Element %s not found", selector)
	}

	text, err := element.GetText()
	if err != nil {
		t.Fatalf("Failed to get text from %s: %v", selector, err)
	}

	if text != expected {
		t.Errorf("Element %s has wrong text: got %q, want %q", selector, text, expected)
	}
}

// AssertElementExists asserts that an element exists
func AssertElementExists(t testing.TB, page *browser.Page, selector string) {
	t.Helper()

	exists, err := page.ElementExists(selector)
	if err != nil {
		t.Fatalf("Failed to check element existence %s: %v", selector, err)
	}

	if !exists {
		t.Errorf("Element %s does not exist", selector)
	}
}

// AssertElementNotExists asserts that an element does not exist
func AssertElementNotExists(t testing.TB, page *browser.Page, selector string) {
	t.Helper()

	exists, err := page.ElementExists(selector)
	if err != nil {
		t.Fatalf("Failed to check element existence %s: %v", selector, err)
	}

	if exists {
		t.Errorf("Element %s should not exist", selector)
	}
}

// AssertElementVisible asserts that an element is visible
func AssertElementVisible(t testing.TB, page *browser.Page, selector string) {
	t.Helper()

	visible, err := page.ElementVisible(selector)
	if err != nil {
		t.Fatalf("Failed to check element visibility %s: %v", selector, err)
	}

	if !visible {
		t.Errorf("Element %s is not visible", selector)
	}
}

// AssertElementNotVisible asserts that an element is not visible
func AssertElementNotVisible(t testing.TB, page *browser.Page, selector string) {
	t.Helper()

	visible, err := page.ElementVisible(selector)
	if err != nil {
		t.Fatalf("Failed to check element visibility %s: %v", selector, err)
	}

	if visible {
		t.Errorf("Element %s should not be visible", selector)
	}
}

// AssertPageTitle asserts that the page has a specific title
func AssertPageTitle(t testing.TB, page *browser.Page, expected string) {
	t.Helper()

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Failed to get page title: %v", err)
	}

	if title != expected {
		t.Errorf("Page has wrong title: got %q, want %q", title, expected)
	}
}

// AssertPageURL asserts that the page has a specific URL
func AssertPageURL(t testing.TB, page *browser.Page, expected string) {
	t.Helper()

	url, err := page.URL()
	if err != nil {
		t.Fatalf("Failed to get page URL: %v", err)
	}

	if url != expected {
		t.Errorf("Page has wrong URL: got %q, want %q", url, expected)
	}
}

// CreateTestHTML creates a test HTML page with common elements
func CreateTestHTML(title string, body string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>%s</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		.hidden { display: none; }
		.button { padding: 10px 20px; margin: 5px; cursor: pointer; }
		.input { padding: 5px; margin: 5px; }
	</style>
</head>
<body>
%s
</body>
</html>`, title, body)
}

// NavigateAndWait navigates to a URL and waits for it to load
func NavigateAndWait(t testing.TB, page *browser.Page, url string) {
	t.Helper()

	err := page.Navigate(url, browser.NavigateWithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Failed to navigate to %s: %v", url, err)
	}

	// Wait a bit for any JavaScript to run
	time.Sleep(100 * time.Millisecond)
}

// TakeDebugScreenshot takes a screenshot for debugging purposes
func TakeDebugScreenshot(t testing.TB, page *browser.Page, name string) {
	t.Helper()

	if !testing.Verbose() {
		return
	}

	screenshot, err := page.Screenshot(browser.ScreenshotFullPage(true))
	if err != nil {
		t.Logf("Failed to take debug screenshot: %v", err)
		return
	}

	// Save to temp directory
	tempDir := os.TempDir()
	filename := filepath.Join(tempDir, fmt.Sprintf("browser-test-%s-%d.png", name, time.Now().Unix()))

	if err := os.WriteFile(filename, screenshot, 0644); err != nil {
		t.Logf("Failed to save debug screenshot: %v", err)
		return
	}

	t.Logf("Debug screenshot saved to: %s", filename)
}

// RunWithTimeout runs a function with a timeout
func RunWithTimeout(t testing.TB, timeout time.Duration, fn func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(timeout):
		t.Fatalf("Operation timed out after %v", timeout)
	}
}

// CreateTestContext creates a test context with timeout
func CreateTestContext(t testing.TB, timeout time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(func() {
		cancel()
	})

	return ctx, cancel
}
