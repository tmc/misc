package browser_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

// startRemoteChrome starts a Chrome instance with remote debugging enabled
func startRemoteChrome(t *testing.T, port int) (*exec.Cmd, func()) {
	t.Helper()

	chromePath := findChrome()
	if chromePath == "" {
		t.Skip("Chrome not found")
	}

	// Create temp user data dir
	tempDir, err := os.MkdirTemp("", "remote-chrome-test")
	if err != nil {
		t.Fatal(err)
	}

	args := []string{
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--disable-dev-shm-usage",
		fmt.Sprintf("--remote-debugging-port=%d", port),
		fmt.Sprintf("--user-data-dir=%s", tempDir),
		"about:blank",
	}

	cmd := exec.Command(chromePath, args...)

	// Start Chrome
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatal(err)
	}

	// Wait for Chrome to start
	started := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/json/version", port))
		if err == nil {
			resp.Body.Close()
			started = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !started {
		cmd.Process.Kill()
		os.RemoveAll(tempDir)
		t.Fatal("Chrome failed to start")
	}

	cleanup := func() {
		cmd.Process.Kill()
		cmd.Wait()
		os.RemoveAll(tempDir)
	}

	return cmd, cleanup
}

// TestRemoteDebuggingInfo tests getting remote debugging info
func TestRemoteDebuggingInfo(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Remote Chrome tests unreliable on Windows")
	}

	skipIfNoChromish(t)

	// Start remote Chrome
	port := 9222
	_, cleanup := startRemoteChrome(t, port)
	defer cleanup()

	// Get debugging info
	info, err := browser.GetRemoteDebuggingInfo("localhost", port)
	if err != nil {
		t.Errorf("Failed to get debugging info: %v", err)
	}

	// Verify info
	if info.Browser == "" {
		t.Error("Browser info is empty")
	}
	if info.ProtocolVersion == "" {
		t.Error("Protocol version is empty")
	}
	if info.WebSocketDebuggerURL == "" {
		t.Error("WebSocket URL is empty")
	}
}

// TestListTabs tests listing Chrome tabs
func TestListTabs(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Remote Chrome tests unreliable on Windows")
	}

	skipIfNoChromish(t)

	// Start remote Chrome
	port := 9223
	_, cleanup := startRemoteChrome(t, port)
	defer cleanup()

	// List tabs
	tabs, err := browser.ListTabs("localhost", port)
	if err != nil {
		t.Errorf("Failed to list tabs: %v", err)
	}

	// Should have at least one tab
	if len(tabs) == 0 {
		t.Error("No tabs found")
	}

	// Check first tab
	if len(tabs) > 0 {
		tab := tabs[0]
		if tab.ID == "" {
			t.Error("Tab ID is empty")
		}
		if tab.Type != "page" && tab.Type != "background_page" {
			t.Errorf("Unexpected tab type: %s", tab.Type)
		}
		if tab.WebSocketDebuggerURL == "" {
			t.Error("Tab WebSocket URL is empty")
		}
	}
}

// TestConnectToRunningChrome tests connecting to an already running Chrome
func TestConnectToRunningChrome(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Remote Chrome tests unreliable on Windows")
	}

	skipIfNoChromish(t)

	// Start remote Chrome
	port := 9224
	_, cleanup := startRemoteChrome(t, port)
	defer cleanup()

	// Create browser that connects to running Chrome
	ctx := context.Background()
	profileMgr, _ := chromeprofiles.NewProfileManager()

	b, err := browser.New(ctx, profileMgr,
		browser.WithRemoteChrome("localhost", port),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Launch (which should connect to remote)
	err = b.Launch(ctx)
	if err != nil {
		t.Errorf("Failed to connect to running Chrome: %v", err)
	}

	// Test navigation
	ts := newTestServer()
	defer ts.Close()

	err = b.Navigate(ts.URL)
	if err != nil {
		t.Errorf("Failed to navigate: %v", err)
	}

	// Verify we can interact with the page
	title, err := b.GetTitle()
	if err != nil {
		t.Errorf("Failed to get title: %v", err)
	}
	if title != "Test Page" {
		t.Errorf("Unexpected title: got %s, want Test Page", title)
	}
}

// TestConnectToTab tests connecting to a specific tab
func TestConnectToTab(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Remote Chrome tests unreliable on Windows")
	}

	skipIfNoChromish(t)

	// Start remote Chrome
	port := 9225
	_, cleanup := startRemoteChrome(t, port)
	defer cleanup()

	// First, list tabs to get a tab ID
	tabs, err := browser.ListTabs("localhost", port)
	if err != nil || len(tabs) == 0 {
		t.Skip("No tabs available")
	}

	tabID := tabs[0].ID

	// Create browser that connects to specific tab
	ctx := context.Background()
	profileMgr, _ := chromeprofiles.NewProfileManager()

	b, err := browser.New(ctx, profileMgr,
		browser.WithRemoteTabConnection("localhost", port, tabID),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Launch (which should connect to the tab)
	err = b.Launch(ctx)
	if err != nil {
		t.Errorf("Failed to connect to tab: %v", err)
	}

	// Get current page to interact with the tab
	page := b.GetCurrentPage()
	if page == nil {
		t.Fatal("Failed to get current page")
	}

	// Navigate the tab
	ts := newTestServer()
	defer ts.Close()

	err = page.Navigate(ts.URL)
	if err != nil {
		t.Errorf("Failed to navigate tab: %v", err)
	}

	// Verify navigation
	url, err := page.URL()
	if err != nil {
		t.Errorf("Failed to get URL: %v", err)
	}
	if !strings.HasPrefix(url, ts.URL) {
		t.Errorf("Unexpected URL: got %s, want prefix %s", url, ts.URL)
	}
}

// TestRemoteBrowserOperations tests various operations on remote browser
func TestRemoteBrowserOperations(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Remote Chrome tests unreliable on Windows")
	}

	skipIfNoChromish(t)

	// Start remote Chrome
	port := 9226
	_, cleanup := startRemoteChrome(t, port)
	defer cleanup()

	// Connect to remote Chrome
	ctx := context.Background()
	profileMgr, _ := chromeprofiles.NewProfileManager()

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

	// Test server
	ts := newTestServer()
	defer ts.Close()

	// Test navigation
	err = b.Navigate(ts.URL)
	if err != nil {
		t.Errorf("Failed to navigate: %v", err)
	}

	// Test JavaScript execution
	result, err := b.ExecuteScript("1 + 1")
	if err != nil {
		t.Errorf("Failed to execute script: %v", err)
	}
	if num, ok := result.(float64); !ok || num != 2 {
		t.Errorf("Unexpected script result: %v", result)
	}

	// Test waiting for selector
	err = b.WaitForSelector("#title", 5*time.Second)
	if err != nil {
		t.Errorf("Failed to wait for selector: %v", err)
	}

	// Test getting HTML
	html, err := b.GetHTML()
	if err != nil {
		t.Errorf("Failed to get HTML: %v", err)
	}
	if !strings.Contains(html, "Test Page") {
		t.Error("HTML does not contain expected content")
	}
}

// TestRemoteMultipleTabs tests working with multiple tabs in remote Chrome
func TestRemoteMultipleTabs(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Remote Chrome tests unreliable on Windows")
	}

	skipIfNoChromish(t)

	// Start remote Chrome
	port := 9227
	_, cleanup := startRemoteChrome(t, port)
	defer cleanup()

	// Connect to remote Chrome
	ctx := context.Background()
	profileMgr, _ := chromeprofiles.NewProfileManager()

	b, err := browser.New(ctx, profileMgr,
		browser.WithRemoteChrome("localhost", port),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	err = b.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Create multiple pages
	pages := make([]*browser.Page, 3)
	for i := range pages {
		page, err := b.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		pages[i] = page
		defer page.Close()
	}

	// Navigate each page
	ts := newTestServer()
	defer ts.Close()

	for i, page := range pages {
		url := fmt.Sprintf("%s/?tab=%d", ts.URL, i)
		err := page.Navigate(url)
		if err != nil {
			t.Errorf("Failed to navigate page %d: %v", i, err)
		}
	}

	// Verify each page
	for i, page := range pages {
		url, err := page.URL()
		if err != nil {
			t.Errorf("Failed to get URL for page %d: %v", i, err)
		}

		expectedURL := fmt.Sprintf("%s/?tab=%d", ts.URL, i)
		if url != expectedURL {
			t.Errorf("Page %d has wrong URL: got %s, want %s", i, url, expectedURL)
		}
	}

	// List all tabs via API
	tabs, err := browser.ListTabs("localhost", port)
	if err != nil {
		t.Errorf("Failed to list tabs: %v", err)
	}

	// Should have at least our created tabs
	if len(tabs) < len(pages) {
		t.Errorf("Expected at least %d tabs, got %d", len(pages), len(tabs))
	}
}

// TestRemoteErrorHandling tests error handling for remote connections
func TestRemoteErrorHandling(t *testing.T) {
	t.Parallel()
	skipIfNoChromish(t)

	ctx := context.Background()
	profileMgr, _ := chromeprofiles.NewProfileManager()

	// Test connecting to non-existent Chrome
	b, err := browser.New(ctx, profileMgr,
		browser.WithRemoteChrome("localhost", 9999), // Port with no Chrome
	)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Launch should fail
	err = b.Launch(ctx)
	if err == nil {
		t.Error("Expected error connecting to non-existent Chrome")
	}

	// Test connecting to invalid tab ID
	port := 9228
	_, cleanup := startRemoteChrome(t, port)
	defer cleanup()

	b2, err := browser.New(ctx, profileMgr,
		browser.WithRemoteTabConnection("localhost", port, "invalid-tab-id"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()

	// Launch should fail
	err = b2.Launch(ctx)
	if err == nil {
		t.Error("Expected error connecting to invalid tab")
	}
}
