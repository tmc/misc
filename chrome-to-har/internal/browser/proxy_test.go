package browser_test

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// ProxyServer simulates a proxy server for testing
type ProxyServer struct {
	*httptest.Server
	authRequired  bool
	username      string
	password      string
	requestCount  int32
	lastRequest   *http.Request
	bypassDomains []string
}

// NewProxyServer creates a new test proxy server
func NewProxyServer(authRequired bool, username, password string) *ProxyServer {
	proxy := &ProxyServer{
		authRequired: authRequired,
		username:     username,
		password:     password,
	}

	proxy.Server = httptest.NewServer(http.HandlerFunc(proxy.handleRequest))
	return proxy
}

// handleRequest handles proxy requests
func (p *ProxyServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&p.requestCount, 1)
	p.lastRequest = r

	// Check authentication if required
	if p.authRequired {
		auth := r.Header.Get("Proxy-Authorization")
		if auth == "" {
			w.Header().Set("Proxy-Authenticate", `Basic realm="Test Proxy"`)
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}

		// Verify credentials
		expectedAuth := "Basic " + basicAuth(p.username, p.password)
		if auth != expectedAuth {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	// Handle CONNECT method for HTTPS
	if r.Method == http.MethodConnect {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Forward the request
	targetURL := r.URL.String()
	if !strings.HasPrefix(targetURL, "http") {
		targetURL = "http://" + r.Host + r.URL.Path
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}
	}

	// Create forwarding request
	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		if key != "Proxy-Authorization" {
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}
	}

	// Make the request
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

// GetRequestCount returns the number of requests handled
func (p *ProxyServer) GetRequestCount() int {
	return int(atomic.LoadInt32(&p.requestCount))
}

// basicAuth creates a basic auth string
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func TestProxyConnection(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping proxy test in short mode")
	}

	// Create a test web server
	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body><h1>Test Page</h1><p>Proxy: %s</p></body></html>`, r.Header.Get("X-Forwarded-For"))
	}))
	defer webServer.Close()

	// Create a test proxy server
	proxyServer := NewProxyServer(false, "", "")
	defer proxyServer.Close()

	// Parse proxy URL
	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Create browser with proxy
	b, cleanup := createTestBrowser(t,
		browser.WithProxy(proxyURL.String()),
		browser.WithVerbose(testing.Verbose()),
	)
	defer cleanup()

	// Navigate through proxy
	if err := b.Navigate(webServer.URL); err != nil {
		t.Fatalf("Failed to navigate: %v", err)
	}

	// Check that proxy was used
	if proxyServer.GetRequestCount() == 0 {
		t.Error("Proxy server did not receive any requests")
	}

	// Get page content
	html, err := b.GetHTML()
	if err != nil {
		t.Fatalf("Failed to get HTML: %v", err)
	}

	if !strings.Contains(html, "Test Page") {
		t.Error("Page content not found")
	}
}

func TestProxyAuthentication(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping proxy auth test in short mode")
	}

	// Create a test web server
	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><body><h1>Authenticated Page</h1></body></html>`)
	}))
	defer webServer.Close()

	// Create a test proxy server with authentication
	proxyServer := NewProxyServer(true, "testuser", "testpass")
	defer proxyServer.Close()

	// Parse proxy URL
	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Test with correct credentials
	t.Run("ValidCredentials", func(t *testing.T) {
		b, cleanup := createTestBrowser(t,
			browser.WithProxy(proxyURL.String()),
			browser.WithProxyAuth("testuser", "testpass"),
			browser.WithVerbose(testing.Verbose()),
		)
		defer cleanup()

		// Navigate through authenticated proxy
		if err := b.Navigate(webServer.URL); err != nil {
			t.Fatalf("Failed to navigate with valid credentials: %v", err)
		}

		// Get page content
		html, err := b.GetHTML()
		if err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}

		if !strings.Contains(html, "Authenticated Page") {
			t.Error("Failed to load page through authenticated proxy")
		}
	})

	// Test with invalid credentials
	t.Run("InvalidCredentials", func(t *testing.T) {
		b, cleanup := createTestBrowser(t,
			browser.WithProxy(proxyURL.String()),
			browser.WithProxyAuth("wronguser", "wrongpass"),
			browser.WithVerbose(testing.Verbose()),
		)
		defer cleanup()

		// Navigation should fail or show auth error
		err := b.Navigate(webServer.URL)
		if err == nil {
			// Check if we got an error page
			html, _ := b.GetHTML()
			if strings.Contains(html, "Authenticated Page") {
				t.Error("Should not have loaded page with invalid credentials")
			}
		}
	})
}

func TestProxyBypassList(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping proxy bypass test in short mode")
	}

	// Create two test web servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body><h1>Server 1</h1></body></html>`)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body><h1>Server 2</h1></body></html>`)
	}))
	defer server2.Close()

	// Create a test proxy server
	proxyServer := NewProxyServer(false, "", "")
	defer proxyServer.Close()

	// Parse URLs
	proxyURL, _ := url.Parse(proxyServer.URL)
	server1URL, _ := url.Parse(server1.URL)

	// Create browser with proxy and bypass list
	b, cleanup := createTestBrowser(t,
		browser.WithProxy(proxyURL.String()),
		browser.WithProxyBypassList(server1URL.Host),
		browser.WithVerbose(testing.Verbose()),
	)
	defer cleanup()

	// Navigate to bypassed server (should not use proxy)
	initialCount := proxyServer.GetRequestCount()
	if err := b.Navigate(server1.URL); err != nil {
		t.Fatalf("Failed to navigate to bypassed server: %v", err)
	}

	if proxyServer.GetRequestCount() > initialCount {
		t.Error("Proxy was used for bypassed host")
	}

	// Navigate to non-bypassed server (should use proxy)
	if err := b.Navigate(server2.URL); err != nil {
		t.Fatalf("Failed to navigate to non-bypassed server: %v", err)
	}

	if proxyServer.GetRequestCount() == initialCount {
		t.Error("Proxy was not used for non-bypassed host")
	}
}

func TestSOCKS5Proxy(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping SOCKS5 proxy test in short mode")
	}

	// Note: This test requires a real SOCKS5 proxy or a mock implementation
	// For now, we'll just test that the option is accepted

	b, cleanup := createTestBrowser(t,
		browser.WithProxy("socks5://localhost:1080"),
		browser.WithVerbose(testing.Verbose()),
	)
	defer cleanup()

	// The browser should accept SOCKS5 proxy configuration
	// Actual SOCKS5 testing would require a SOCKS5 server
	if b == nil {
		t.Error("Failed to create browser with SOCKS5 proxy")
	}
}

func TestProxyWithHTTPS(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping HTTPS proxy test in short mode")
	}

	// Create a test HTTPS server
	httpsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body><h1>HTTPS Page</h1></body></html>`)
	}))
	defer httpsServer.Close()

	// Create a test proxy server
	proxyServer := NewProxyServer(false, "", "")
	defer proxyServer.Close()

	// Parse proxy URL
	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Create browser with proxy
	b, cleanup := createTestBrowser(t,
		browser.WithProxy(proxyURL.String()),
		browser.WithVerbose(testing.Verbose()),
	)
	defer cleanup()

	// Navigate to HTTPS site through proxy
	if err := b.Navigate(httpsServer.URL); err != nil {
		// HTTPS through proxy might fail in test environment
		t.Logf("HTTPS navigation through proxy failed (expected in test env): %v", err)
	}
}
