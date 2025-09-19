package browser_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/testutil"
)

// TestMain adds global test setup and teardown for browser cleanup
func TestMain(m *testing.M) {
	// Clean up before tests
	testutil.CleanupOrphanedBrowsers(&testing.T{})

	// Run tests
	code := m.Run()

	// Clean up after tests
	testutil.CleanupOrphanedBrowsers(&testing.T{})

	os.Exit(code)
}

// TestServer provides a test HTTP server with various endpoints
type TestServer struct {
	*httptest.Server
	requestCount map[string]int
}

func newTestServer() *TestServer {
	ts := &TestServer{
		requestCount: make(map[string]int),
	}

	mux := http.NewServeMux()

	// Basic HTML page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
</head>
<body>
	<h1 id="title">Test Page</h1>
	<p id="content">This is a test page.</p>
	<button id="clickme">Click Me</button>
	<input id="textinput" type="text" value="" />
	<div id="result" style="display:none;">Button clicked!</div>
	<script>
		document.getElementById('clickme').addEventListener('click', function() {
			document.getElementById('result').style.display = 'block';
		});
	</script>
</body>
</html>`)
	})

	// Page with delayed content
	mux.HandleFunc("/delayed", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
	<div id="initial">Loading...</div>
	<div id="delayed" style="display:none;">Loaded!</div>
	<script>
		setTimeout(function() {
			document.getElementById('initial').style.display = 'none';
			document.getElementById('delayed').style.display = 'block';
		}, 1000);
	</script>
</body>
</html>`)
	})

	// JSON API endpoint
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","data":{"value":42}}`)
	})

	// Page with network requests
	mux.HandleFunc("/network-test", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
	<div id="status">Loading resources...</div>
	<script>
		// Make some network requests
		fetch('/api/data')
			.then(r => r.json())
			.then(data => {
				document.getElementById('status').textContent = 'Resources loaded';
			});
	</script>
</body>
</html>`)
	})

	// Page with forms
	mux.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		if r.Method == "POST" {
			r.ParseForm()
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<div id="result">Form submitted: %s</div>`, r.FormValue("name"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
	<form id="testform" method="POST">
		<input id="name" name="name" type="text" />
		<select id="option" name="option">
			<option value="opt1">Option 1</option>
			<option value="opt2">Option 2</option>
		</select>
		<button id="submit" type="submit">Submit</button>
	</form>
</body>
</html>`)
	})

	// Page that sets cookies
	mux.HandleFunc("/cookies", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		http.SetCookie(w, &http.Cookie{
			Name:  "test-cookie",
			Value: "test-value",
			Path:  "/",
		})
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div id="status">Cookie set</div>`)
	})

	// Redirect page
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		http.Redirect(w, r, "/", http.StatusFound)
	})

	// Basic auth protected page
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testpass" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Test"`)
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Unauthorized")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div id="status">Authenticated</div>`)
	})

	// POST endpoint for testing
	mux.HandleFunc("/api/post", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "Method not allowed")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error reading body")
			return
		}

		contentType := r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"method": "%s",
			"contentType": "%s",
			"body": "%s",
			"success": true
		}`, r.Method, contentType, string(body))
	})

	// PUT endpoint for testing
	mux.HandleFunc("/api/put", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++
		if r.Method != "PUT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "Method not allowed")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error reading body")
			return
		}

		contentType := r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"method": "%s",
			"contentType": "%s",
			"body": "%s",
			"success": true
		}`, r.Method, contentType, string(body))
	})

	// Generic endpoint that echoes request info
	mux.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		ts.requestCount[r.URL.Path]++

		var body string
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil {
				body = string(bodyBytes)
			}
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Echo Response</title></head>
<body>
	<div id="method">%s</div>
	<div id="content-type">%s</div>
	<div id="body">%s</div>
	<div id="success">true</div>
</body>
</html>`, r.Method, r.Header.Get("Content-Type"), body)
	})

	ts.Server = httptest.NewServer(mux)
	return ts
}

// Helper function to find Chrome executable - use discovery package
func findChrome() string {
	return testutil.FindChrome()
}

// Test utilities
func skipIfNoChromish(t testing.TB) {
	t.Helper()

	// Skip browser tests in CI only
	if os.Getenv("CI") != "" {
		t.Skip("Skipping browser test in CI environment")
	}

	if os.Getenv("CI") == "true" && runtime.GOOS != "linux" {
		t.Skip("Skipping browser test in CI on non-Linux platform")
	}

	chromePath := findChrome()
	if chromePath == "" {
		t.Skip("No Chromium-based browser found (Chrome, Brave, Chromium, etc.), skipping browser tests")
	}
}

func createTestBrowser(t testing.TB, opts ...browser.Option) (*browser.Browser, func()) {
	t.Helper()
	skipIfNoChromish(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	// Create temp profile manager
	profileMgr, err := chromeprofiles.NewProfileManager()
	if err != nil {
		t.Fatal(err)
	}

	// Default options for tests
	defaultOpts := []browser.Option{
		browser.WithHeadless(true),
		browser.WithTimeout(30),
		browser.WithChromePath(findChrome()),
		browser.WithVerbose(testing.Verbose()),
	}

	// Append custom options
	defaultOpts = append(defaultOpts, opts...)

	b, err := browser.New(ctx, profileMgr, defaultOpts...)
	if err != nil {
		cancel()
		t.Fatal(err)
	}

	if err := b.Launch(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}

	cleanup := func() {
		b.Close()
		cancel()
	}

	return b, cleanup
}

// TestBrowserLaunch tests basic browser launching
func TestBrowserLaunch(t *testing.T) {
	t.Parallel()
	b, cleanup := createTestBrowser(t)
	defer cleanup()

	// Browser should be launched
	if b.Context() == nil {
		t.Error("Browser context is nil after launch")
	}

	// Should be able to navigate to about:blank
	err := b.Navigate("about:blank")
	if err != nil {
		t.Errorf("Failed to navigate to about:blank: %v", err)
	}
}

// TestBrowserNavigation tests page navigation
func TestBrowserNavigation(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"Homepage", ts.URL + "/", "Test Page"},
		{"Delayed page", ts.URL + "/delayed", "Test Page"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := b.Navigate(tc.url)
			if err != nil {
				t.Errorf("Failed to navigate: %v", err)
			}

			// Get current URL
			currentURL, err := b.GetURL()
			if err != nil {
				t.Errorf("Failed to get URL: %v", err)
			}

			if !strings.HasPrefix(currentURL, ts.URL) {
				t.Errorf("Unexpected URL: got %s, want prefix %s", currentURL, ts.URL)
			}
		})
	}
}

// TestBrowserGetHTML tests HTML retrieval
func TestBrowserGetHTML(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	err := b.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	html, err := b.GetHTML()
	if err != nil {
		t.Errorf("Failed to get HTML: %v", err)
	}

	// Check for expected content
	if !strings.Contains(html, "Test Page") {
		t.Error("HTML does not contain expected content")
	}

	if !strings.Contains(html, `id="title"`) {
		t.Error("HTML does not contain expected title element")
	}
}

// TestBrowserTitle tests title retrieval
func TestBrowserTitle(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	err := b.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	title, err := b.GetTitle()
	if err != nil {
		t.Errorf("Failed to get title: %v", err)
	}

	if title != "Test Page" {
		t.Errorf("Unexpected title: got %s, want Test Page", title)
	}
}

// TestBrowserWaitForSelector tests waiting for elements
func TestBrowserWaitForSelector(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	err := b.Navigate(ts.URL + "/delayed")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for delayed element
	err = b.WaitForSelector("#delayed", 3*time.Second)
	if err != nil {
		t.Errorf("Failed to wait for selector: %v", err)
	}
}

// TestBrowserExecuteScript tests JavaScript execution
func TestBrowserExecuteScript(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	err := b.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Execute script to get element text
	result, err := b.ExecuteScript(`document.getElementById('title').textContent`)
	if err != nil {
		t.Errorf("Failed to execute script: %v", err)
	}

	if result != "Test Page" {
		t.Errorf("Unexpected script result: got %v, want Test Page", result)
	}

	// Execute script that returns a number
	result, err = b.ExecuteScript(`1 + 2`)
	if err != nil {
		t.Errorf("Failed to execute math script: %v", err)
	}

	// Type assertion for float64 (JavaScript numbers)
	if num, ok := result.(float64); !ok || num != 3 {
		t.Errorf("Unexpected math result: got %v (type %T), want 3", result, result)
	}
}

// TestBrowserHeaders tests custom header setting
func TestBrowserHeaders(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	// Set custom headers
	headers := map[string]string{
		"X-Test-Header": "test-value",
		"User-Agent":    "TestBrowser/1.0",
	}

	err := b.SetRequestHeaders(headers)
	if err != nil {
		t.Errorf("Failed to set headers: %v", err)
	}

	// Navigate with headers set
	err = b.Navigate(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
}

// TestBrowserBasicAuth tests basic authentication
func TestBrowserBasicAuth(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	// Set basic auth
	err := b.SetBasicAuth("testuser", "testpass")
	if err != nil {
		t.Errorf("Failed to set basic auth: %v", err)
	}

	// Navigate to protected page
	err = b.Navigate(ts.URL + "/auth")
	if err != nil {
		t.Fatal(err)
	}

	// Check for authenticated content
	html, err := b.GetHTML()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(html, "Authenticated") {
		t.Error("Failed to authenticate")
	}
}

// TestBrowserWithProfile tests browser with profile
func TestBrowserWithProfile(t *testing.T) {
	t.Parallel()

	// Skip in CI only
	if os.Getenv("CI") != "" {
		t.Skip("Skipping browser test in CI environment")
	}

	// Create temp dir for profile
	tempDir, err := os.MkdirTemp("", "browser-test-profile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a minimal profile structure
	profileDir := filepath.Join(tempDir, "Default")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create mock profile manager
	profileMgr := testutil.NewMockProfileManager()
	profileMgr.WorkDirPath = tempDir

	ctx := context.Background()
	b, err := browser.New(ctx, profileMgr,
		browser.WithHeadless(true),
		browser.WithProfile("Default"),
		browser.WithChromePath(findChrome()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	err = b.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Test navigation with profile
	ts := newTestServer()
	defer ts.Close()

	err = b.Navigate(ts.URL + "/cookies")
	if err != nil {
		t.Fatal(err)
	}

	// Verify cookie was set
	html, err := b.GetHTML()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(html, "Cookie set") {
		t.Error("Cookie page did not load correctly")
	}
}

// TestBrowserMultipleNavigations tests multiple page navigations
func TestBrowserMultipleNavigations(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t)
	defer cleanup()

	urls := []string{
		ts.URL + "/",
		ts.URL + "/delayed",
		ts.URL + "/form",
		ts.URL + "/network-test",
	}

	for _, url := range urls {
		err := b.Navigate(url)
		if err != nil {
			t.Errorf("Failed to navigate to %s: %v", url, err)
		}

		// Give page time to load
		time.Sleep(100 * time.Millisecond)

		// Verify we're on the right page
		currentURL, err := b.GetURL()
		if err != nil {
			t.Errorf("Failed to get URL: %v", err)
		}

		if currentURL != url && currentURL != url+"/" {
			t.Errorf("Wrong URL after navigation: got %s, want %s", currentURL, url)
		}
	}
}

// TestBrowserNetworkIdle tests network idle waiting
func TestBrowserNetworkIdle(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	defer ts.Close()

	b, cleanup := createTestBrowser(t,
		browser.WithWaitForNetworkIdle(true),
		browser.WithStableTimeout(5),
	)
	defer cleanup()

	start := time.Now()
	err := b.Navigate(ts.URL + "/network-test")
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)

	// Should have waited for network
	if elapsed < 500*time.Millisecond {
		t.Error("Navigation completed too quickly, network idle may not be working")
	}

	// Check content loaded
	html, err := b.GetHTML()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(html, "Resources loaded") {
		t.Error("Network resources did not load")
	}
}

// TestBrowserTimeout tests operation timeouts
func TestBrowserTimeout(t *testing.T) {
	t.Parallel()
	b, cleanup := createTestBrowser(t,
		browser.WithTimeout(2),           // 2 second timeout
		browser.WithNavigationTimeout(1), // 1 second navigation timeout
	)
	defer cleanup()

	// Create a server that never responds
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never respond
		<-r.Context().Done()
	}))
	defer slowServer.Close()

	// Navigation should timeout
	err := b.Navigate(slowServer.URL)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestBrowserHTTPRequestPOST(t *testing.T) {
	t.Parallel()
	skipIfNoChromish(t)
	ts := newTestServer()
	defer ts.Close()

	ctx := context.Background()
	pm := testutil.NewMockProfileManager()

	chromePath := findChrome()
	if chromePath == "" {
		t.Skip("Chrome not found")
	}

	b, err := browser.New(ctx, pm,
		browser.WithHeadless(true),
		browser.WithVerbose(true),
		browser.WithChromePath(chromePath),
		browser.WithTimeout(30),
	)
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	// Test JSON POST request
	jsonData := `{"name": "test", "value": 42}`
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	err = b.HTTPRequest("POST", ts.URL+"/api/echo", jsonData, headers)
	if err != nil {
		t.Fatalf("Failed to make POST request: %v", err)
	}

	// Verify the response content
	html, err := b.GetHTML()
	if err != nil {
		t.Fatalf("Failed to get HTML: %v", err)
	}

	if !strings.Contains(html, "POST") {
		t.Errorf("Expected method 'POST' in response, got: %s", html)
	}

	if !strings.Contains(html, "application/json") {
		t.Errorf("Expected content-type 'application/json' in response, got: %s", html)
	}

	if !strings.Contains(html, jsonData) {
		t.Errorf("Expected body data in response, got: %s", html)
	}
}

func TestBrowserHTTPRequestPUT(t *testing.T) {
	t.Parallel()
	skipIfNoChromish(t)
	ts := newTestServer()
	defer ts.Close()

	ctx := context.Background()
	pm := testutil.NewMockProfileManager()

	chromePath := findChrome()
	if chromePath == "" {
		t.Skip("Chrome not found")
	}

	b, err := browser.New(ctx, pm,
		browser.WithHeadless(true),
		browser.WithVerbose(true),
		browser.WithChromePath(chromePath),
		browser.WithTimeout(30),
	)
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	// Test form data PUT request
	formData := "name=test&value=42"
	headers := map[string]string{}

	err = b.HTTPRequest("PUT", ts.URL+"/api/echo", formData, headers)
	if err != nil {
		t.Fatalf("Failed to make PUT request: %v", err)
	}

	// Verify the response content
	html, err := b.GetHTML()
	if err != nil {
		t.Fatalf("Failed to get HTML: %v", err)
	}

	if !strings.Contains(html, "PUT") {
		t.Errorf("Expected method 'PUT' in response, got: %s", html)
	}

	if !strings.Contains(html, "application/x-www-form-urlencoded") {
		t.Errorf("Expected content-type 'application/x-www-form-urlencoded' in response, got: %s", html)
	}

	if !strings.Contains(html, formData) {
		t.Errorf("Expected body data in response, got: %s", html)
	}
}

func TestBrowserHTTPRequestContentTypeDetection(t *testing.T) {
	t.Parallel()
	skipIfNoChromish(t)
	ts := newTestServer()
	defer ts.Close()

	ctx := context.Background()
	pm := testutil.NewMockProfileManager()

	chromePath := findChrome()
	if chromePath == "" {
		t.Skip("Chrome not found")
	}

	b, err := browser.New(ctx, pm,
		browser.WithHeadless(true),
		browser.WithVerbose(true),
		browser.WithChromePath(chromePath),
		browser.WithTimeout(30),
	)
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	testCases := []struct {
		name       string
		data       string
		expectedCT string
	}{
		{
			name:       "JSON object",
			data:       `{"key": "value"}`,
			expectedCT: "application/json",
		},
		{
			name:       "JSON array",
			data:       `[1, 2, 3]`,
			expectedCT: "application/json",
		},
		{
			name:       "Form data",
			data:       "key=value&another=test",
			expectedCT: "application/x-www-form-urlencoded",
		},
		{
			name:       "Plain text",
			data:       "simple text data",
			expectedCT: "text/plain",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err = b.HTTPRequest("POST", ts.URL+"/api/echo", tc.data, map[string]string{})
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}

			html, err := b.GetHTML()
			if err != nil {
				t.Fatalf("Failed to get HTML: %v", err)
			}

			if !strings.Contains(html, tc.expectedCT) {
				t.Errorf("Expected content-type '%s' in response, got: %s", tc.expectedCT, html)
			}
		})
	}
}
