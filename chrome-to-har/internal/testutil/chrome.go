package testutil

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

// ChromeTestHelper provides utilities for running Chrome in tests
type ChromeTestHelper struct {
	t          *testing.T
	chromePath string
	tempDir    string
	process    *os.Process
	port       int
	mu         sync.Mutex
}

// NewChromeTestHelper creates a new Chrome test helper
func NewChromeTestHelper(t *testing.T) *ChromeTestHelper {
	t.Helper()

	// Skip browser tests in short mode (go test -short)
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	// Skip if in CI environment
	if os.Getenv("CI") != "" {
		t.Skip("Skipping browser tests in CI environment")
	}

	chromePath := findChrome()
	if chromePath == "" {
		t.Skip("No Chrome-compatible browser found (Chrome, Chromium, Brave, etc.)")
	}

	return &ChromeTestHelper{
		t:          t,
		chromePath: chromePath,
	}
}

// StartChrome launches a Chrome instance for testing
func (h *ChromeTestHelper) StartChrome(ctx context.Context, headless bool) (context.Context, context.CancelFunc, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create temp directory for user data
	tempDir, err := os.MkdirTemp("", "chrome-test-*")
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating temp dir")
	}
	h.tempDir = tempDir

	// Find a free port
	port, err := getFreePort()
	if err != nil {
		return nil, nil, errors.Wrap(err, "getting free port")
	}
	h.port = port

	// Set up Chrome options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.UserDataDir(tempDir),
		chromedp.Flag("remote-debugging-port", fmt.Sprintf("%d", port)),
		// Stability flags for CI
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-features", "TranslateUI"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
	)

	if headless {
		opts = append(opts, chromedp.Headless)
	}

	// Use custom Chrome path if provided
	if h.chromePath != "" {
		opts = append(opts, chromedp.ExecPath(h.chromePath))
	}

	// Create allocator context
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)

	// Create browser context
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	// Ensure Chrome starts successfully
	testCtx, testCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer testCancel()

	if err := chromedp.Run(testCtx, chromedp.Navigate("about:blank")); err != nil {
		browserCancel()
		allocCancel()
		return nil, nil, errors.Wrap(err, "starting Chrome")
	}

	// Return combined cancel function
	cancel := func() {
		browserCancel()
		allocCancel()
		h.Cleanup()
	}

	return browserCtx, cancel, nil
}

// GetDebugURL returns the Chrome DevTools debug URL
func (h *ChromeTestHelper) GetDebugURL() string {
	return fmt.Sprintf("http://localhost:%d", h.port)
}

// WaitForChrome waits for Chrome to be ready
func (h *ChromeTestHelper) WaitForChrome(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(h.GetDebugURL() + "/json/version")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return errors.New("timeout waiting for Chrome to start")
}

// Cleanup cleans up resources
func (h *ChromeTestHelper) Cleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.process != nil {
		h.process.Kill()
		h.process = nil
	}

	if h.tempDir != "" {
		os.RemoveAll(h.tempDir)
		h.tempDir = ""
	}
}

// TestServer creates a test HTTP server
func TestServer(t *testing.T, handler http.Handler) *TestHTTPServer {
	t.Helper()

	server := &TestHTTPServer{
		t:       t,
		handler: handler,
	}

	// Start server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	server.server = &http.Server{Handler: handler}
	server.url = fmt.Sprintf("http://%s", listener.Addr().String())

	go func() {
		if err := server.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Test server error: %v", err)
		}
	}()

	// Wait for server to be ready
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(server.url)
		if err == nil {
			resp.Body.Close()
			return server
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("Test server failed to start")
	return nil
}

// TestHTTPServer represents a test HTTP server
type TestHTTPServer struct {
	t       *testing.T
	server  *http.Server
	handler http.Handler
	url     string
}

// URL returns the server URL
func (s *TestHTTPServer) URL() string {
	return s.url
}

// Close shuts down the server
func (s *TestHTTPServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.t.Logf("Error shutting down test server: %v", err)
	}
}

// FindChrome locates the Chrome executable (public wrapper)
func FindChrome() string {
	return findChrome()
}

// findChrome locates the Chrome executable
func findChrome() string {
	// Check environment variable first
	if path := os.Getenv("CHROME_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Platform-specific paths
	var paths []string

	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "linux":
		paths = []string{
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
			"/usr/bin/brave-browser",
			"/usr/bin/microsoft-edge",
			"/snap/bin/chromium",
			"/snap/bin/brave",
		}
	case "windows":
		paths = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`C:\Program Files (x86)\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("LOCALAPPDATA"), `BraveSoftware\Brave-Browser\Application\brave.exe`),
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Edge\Application\msedge.exe`),
		}
	}

	// Check each path
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try to find in PATH
	pathCommands := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"brave-browser",
		"microsoft-edge",
		"msedge",
		"chrome",
		"brave",
	}

	for _, cmd := range pathCommands {
		if path, err := exec.LookPath(cmd); err == nil {
			return path
		}
	}

	return ""
}

// getFreePort finds a free port
func getFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// SkipIfNoChrome skips the test if Chrome is not available or if running in short mode
func SkipIfNoChrome(t *testing.T) {
	t.Helper()

	// Skip browser tests in short mode (go test -short)
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	// Skip if in CI environment
	if os.Getenv("CI") != "" {
		t.Skip("Skipping browser tests in CI environment")
	}

	if findChrome() == "" {
		t.Skip("Chrome not found, skipping test")
	}
}

// SkipInCI skips the test if running in CI
func SkipInCI(t *testing.T) {
	t.Helper()

	if os.Getenv("CI") != "" {
		t.Skip("Skipping test in CI environment")
	}
}

// RunInHeadless runs the test only in headless mode
func RunInHeadless(t *testing.T) bool {
	t.Helper()

	// Always use headless in CI
	if os.Getenv("CI") != "" {
		return true
	}

	// Check HEADLESS env var
	headless := os.Getenv("HEADLESS")
	return headless == "true" || headless == "1" || headless == "yes"
}

// CreateTestHTML creates a temporary HTML file for testing
func CreateTestHTML(t *testing.T, content string) string {
	t.Helper()

	tmpfile, err := os.CreateTemp("", "test-*.html")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	t.Cleanup(func() {
		os.Remove(tmpfile.Name())
	})

	return tmpfile.Name()
}

// AssertEventually asserts that a condition becomes true within a timeout
func AssertEventually(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Errorf("Condition not met within %v: %s", timeout, msg)
}

// MustStartChrome starts Chrome and fails the test if it can't
func MustStartChrome(t *testing.T, ctx context.Context, headless bool) (context.Context, context.CancelFunc) {
	t.Helper()

	helper := NewChromeTestHelper(t)
	chromeCtx, cancel, err := helper.StartChrome(ctx, headless)
	if err != nil {
		t.Fatalf("Failed to start Chrome: %v", err)
	}

	return chromeCtx, cancel
}

// GetChromeVersion returns the Chrome version
func GetChromeVersion() (string, error) {
	chromePath := findChrome()
	if chromePath == "" {
		return "", errors.New("Chrome not found")
	}

	cmd := exec.Command(chromePath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "getting Chrome version")
	}

	return strings.TrimSpace(string(output)), nil
}
