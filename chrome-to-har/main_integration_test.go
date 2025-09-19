//go:build integration
// +build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/tmc/misc/chrome-to-har/internal/testutil"
)

func TestIntegration_BasicHARCapture(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
	<h1>Test Page</h1>
	<script src="/script.js"></script>
	<link rel="stylesheet" href="/style.css">
	<img src="/image.png">
</body>
</html>`)
	})

	mux.HandleFunc("/script.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintf(w, "console.log('test script loaded');")
	})

	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		fmt.Fprintf(w, "body { background: white; }")
	})

	mux.HandleFunc("/image.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		// 1x1 transparent PNG
		png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00,
			0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01,
			0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1F,
			0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
			0x54, 0x78, 0x9C, 0x62, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00,
			0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
			0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82}
		w.Write(png)
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	// Create temp file for HAR output
	tmpfile, err := os.CreateTemp("", "test-*.har")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// Run chrome-to-har
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options{
		outputFile: tmpfile.Name(),
		headless:   testutil.RunInHeadless(t),
		startURL:   server.URL(),
		timeout:    20,
	}

	// Use mock profile manager for testing
	mockPM := testutil.NewMockProfileManager()
	runner := NewRunner(mockPM)

	if err := runner.Run(ctx, opts); err != nil {
		t.Fatalf("Failed to run chrome-to-har: %v", err)
	}

	// Verify HAR file was created
	data, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read HAR file: %v", err)
	}

	// Parse HAR file
	var harData har.HAR
	if err := json.Unmarshal(data, &harData); err != nil {
		t.Fatalf("Failed to parse HAR file: %v", err)
	}

	// Verify HAR structure
	if harData.Log == nil {
		t.Fatal("HAR log is nil")
	}

	if len(harData.Log.Entries) < 4 {
		t.Errorf("Expected at least 4 entries (html, js, css, png), got %d", len(harData.Log.Entries))
	}

	// Check for expected resources
	foundHTML := false
	foundJS := false
	foundCSS := false
	foundPNG := false

	for _, entry := range harData.Log.Entries {
		url := entry.Request.URL
		if strings.HasSuffix(url, "/") {
			foundHTML = true
			if entry.Response.Status != 200 {
				t.Errorf("HTML response status = %d, want 200", entry.Response.Status)
			}
		} else if strings.HasSuffix(url, "/script.js") {
			foundJS = true
			if entry.Response.Content.MimeType != "application/javascript" {
				t.Errorf("JS content type = %s, want application/javascript", entry.Response.Content.MimeType)
			}
		} else if strings.HasSuffix(url, "/style.css") {
			foundCSS = true
			if entry.Response.Content.MimeType != "text/css" {
				t.Errorf("CSS content type = %s, want text/css", entry.Response.Content.MimeType)
			}
		} else if strings.HasSuffix(url, "/image.png") {
			foundPNG = true
			if entry.Response.Content.MimeType != "image/png" {
				t.Errorf("PNG content type = %s, want image/png", entry.Response.Content.MimeType)
			}
		}
	}

	if !foundHTML {
		t.Error("HTML request not found in HAR")
	}
	if !foundJS {
		t.Error("JavaScript request not found in HAR")
	}
	if !foundCSS {
		t.Error("CSS request not found in HAR")
	}
	if !foundPNG {
		t.Error("PNG request not found in HAR")
	}
}

func TestIntegration_StreamingMode(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	// Create test server with delayed responses
	mux := http.NewServeMux()
	requestCount := 0

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
	<script>
		// Make multiple requests
		fetch('/api/1');
		setTimeout(() => fetch('/api/2'), 100);
		setTimeout(() => fetch('/api/3'), 200);
	</script>
</body>
</html>`)
	})

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		time.Sleep(50 * time.Millisecond) // Simulate slow API
		fmt.Fprintf(w, `{"path": "%s", "timestamp": %d}`, r.URL.Path, time.Now().Unix())
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	// Capture streaming output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputChan := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		outputChan <- string(data)
	}()

	// Run chrome-to-har in streaming mode
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options{
		streaming: true,
		headless:  testutil.RunInHeadless(t),
		startURL:  server.URL(),
		timeout:   10,
	}

	mockPM := testutil.NewMockProfileManager()
	runner := NewRunner(mockPM)

	if err := runner.Run(ctx, opts); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatalf("Failed to run chrome-to-har: %v", err)
	}

	// Restore stdout and get output
	w.Close()
	os.Stdout = oldStdout
	output := <-outputChan

	// Verify streaming output
	lines := strings.Split(strings.TrimSpace(output), "\n")
	jsonCount := 0

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Try to parse as HAR entry
		var entry har.Entry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			jsonCount++

			// Verify entry has required fields
			if entry.Request.URL == "" {
				t.Error("Entry missing request URL")
			}
			if entry.Response.Status == 0 {
				t.Error("Entry missing response status")
			}
		}
	}

	if jsonCount < 3 {
		t.Errorf("Expected at least 3 JSON entries in streaming mode, got %d", jsonCount)
	}
}

func TestIntegration_FilteredCapture(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
	<script>
		fetch('/api/success').then(r => r.text());
		fetch('/api/error').then(r => r.text());
		fetch('/static/image.png');
	</script>
</body>
</html>`)
	})

	mux.HandleFunc("/api/success", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "OK")
	})

	mux.HandleFunc("/api/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "Internal Server Error")
	})

	mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, "Static content")
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	tests := []struct {
		name          string
		filter        string
		urlPattern    string
		expectedCount int
		expectedURLs  []string
	}{
		{
			name:          "filter_by_status",
			filter:        "select(.response.status >= 400)",
			expectedCount: 1,
			expectedURLs:  []string{"/api/error"},
		},
		{
			name:          "filter_by_url_pattern",
			urlPattern:    "/api/",
			expectedCount: 2,
			expectedURLs:  []string{"/api/success", "/api/error"},
		},
		{
			name:          "filter_by_mime_type",
			filter:        `select(.response.content.mimeType | startswith("text/html"))`,
			expectedCount: 1,
			expectedURLs:  []string{"/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file for HAR output
			tmpfile, err := os.CreateTemp("", "test-*.har")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())
			tmpfile.Close()

			// Run chrome-to-har with filter
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := options{
				outputFile: tmpfile.Name(),
				headless:   testutil.RunInHeadless(t),
				startURL:   server.URL(),
				timeout:    10,
				filter:     tt.filter,
				urlPattern: tt.urlPattern,
			}

			mockPM := testutil.NewMockProfileManager()
			runner := NewRunner(mockPM)

			if err := runner.Run(ctx, opts); err != nil {
				t.Fatalf("Failed to run chrome-to-har: %v", err)
			}

			// Parse HAR file
			data, err := os.ReadFile(tmpfile.Name())
			if err != nil {
				t.Fatalf("Failed to read HAR file: %v", err)
			}

			var harData har.HAR
			if err := json.Unmarshal(data, &harData); err != nil {
				t.Fatalf("Failed to parse HAR file: %v", err)
			}

			// Verify filtered results
			if len(harData.Log.Entries) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(harData.Log.Entries))
			}

			// Check URLs match expected
			for _, expectedURL := range tt.expectedURLs {
				found := false
				for _, entry := range harData.Log.Entries {
					if strings.Contains(entry.Request.URL, expectedURL) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected URL %s not found in filtered results", expectedURL)
				}
			}
		})
	}
}

func TestIntegration_RemoteChrome(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	// Start Chrome with remote debugging
	helper := testutil.NewChromeTestHelper(t)
	ctx := context.Background()
	chromeCtx, cancel, err := helper.StartChrome(ctx, testutil.RunInHeadless(t))
	if err != nil {
		t.Fatalf("Failed to start Chrome: %v", err)
	}
	defer cancel()

	// Wait for Chrome to be ready
	if err := helper.WaitForChrome(10 * time.Second); err != nil {
		t.Fatalf("Chrome failed to start: %v", err)
	}

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><body>Remote Chrome Test</body></html>")
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	// Create temp file for HAR output
	tmpfile, err := os.CreateTemp("", "test-*.har")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// Run chrome-to-har with remote Chrome
	runCtx, runCancel := context.WithTimeout(chromeCtx, 30*time.Second)
	defer runCancel()

	opts := options{
		outputFile: tmpfile.Name(),
		useRemote:  true,
		remoteHost: "localhost",
		remotePort: helper.port,
		startURL:   server.URL(),
		timeout:    10,
	}

	mockPM := testutil.NewMockProfileManager()
	runner := NewRunner(mockPM)

	if err := runner.Run(runCtx, opts); err != nil {
		t.Fatalf("Failed to run chrome-to-har: %v", err)
	}

	// Verify HAR file
	data, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read HAR file: %v", err)
	}

	var harData har.HAR
	if err := json.Unmarshal(data, &harData); err != nil {
		t.Fatalf("Failed to parse HAR file: %v", err)
	}

	if len(harData.Log.Entries) == 0 {
		t.Error("No entries captured from remote Chrome")
	}
}

func TestIntegration_NetworkIdle(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	// Create test server with delayed loading
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
	<div id="content">Initial</div>
	<script>
		// Simulate dynamic content loading
		setTimeout(() => {
			fetch('/api/data1').then(r => r.json()).then(data => {
				document.getElementById('content').textContent = data.message;
			});
		}, 500);
		
		setTimeout(() => {
			fetch('/api/data2').then(r => r.json());
		}, 1000);
		
		setTimeout(() => {
			fetch('/api/data3').then(r => r.json());
		}, 1500);
	</script>
</body>
</html>`)
	})

	apiCallCount := 0
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		apiCallCount++
		w.Header().Set("Content-Type", "application/json")
		time.Sleep(100 * time.Millisecond) // Simulate API latency
		fmt.Fprintf(w, `{"message": "Data %d", "path": "%s"}`, apiCallCount, r.URL.Path)
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	// Create temp file for HAR output
	tmpfile, err := os.CreateTemp("", "test-*.har")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// Run chrome-to-har with network idle waiting
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options{
		outputFile:      tmpfile.Name(),
		headless:        testutil.RunInHeadless(t),
		startURL:        server.URL(),
		timeout:         10,
		waitNetworkIdle: true,
		stableTimeout:   5,
	}

	mockPM := testutil.NewMockProfileManager()
	runner := NewRunner(mockPM)

	if err := runner.Run(ctx, opts); err != nil {
		t.Fatalf("Failed to run chrome-to-har: %v", err)
	}

	// Verify all API calls were captured
	data, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read HAR file: %v", err)
	}

	var harData har.HAR
	if err := json.Unmarshal(data, &harData); err != nil {
		t.Fatalf("Failed to parse HAR file: %v", err)
	}

	// Count API requests
	apiRequests := 0
	for _, entry := range harData.Log.Entries {
		if strings.Contains(entry.Request.URL, "/api/") {
			apiRequests++
		}
	}

	if apiRequests < 3 {
		t.Errorf("Expected at least 3 API requests, got %d", apiRequests)
	}
}

func TestIntegration_LargePayload(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	// Create test server with large responses
	mux := http.NewServeMux()

	// Generate large content
	largeContent := strings.Repeat("Lorem ipsum dolor sit amet. ", 10000) // ~280KB

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
	<h1>Large Content Test</h1>
	<script src="/large.js"></script>
	<div id="content">%s</div>
</body>
</html>`, largeContent)
	})

	mux.HandleFunc("/large.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		// Generate large JavaScript
		fmt.Fprintf(w, "// Large JavaScript file\n")
		for i := 0; i < 1000; i++ {
			fmt.Fprintf(w, "function func%d() { console.log('Function %d'); }\n", i, i)
		}
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	// Create temp file for HAR output
	tmpfile, err := os.CreateTemp("", "test-*.har")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// Run chrome-to-har
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options{
		outputFile: tmpfile.Name(),
		headless:   testutil.RunInHeadless(t),
		startURL:   server.URL(),
		timeout:    10,
	}

	mockPM := testutil.NewMockProfileManager()
	runner := NewRunner(mockPM)

	if err := runner.Run(ctx, opts); err != nil {
		t.Fatalf("Failed to run chrome-to-har: %v", err)
	}

	// Verify HAR file size and content
	info, err := os.Stat(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to stat HAR file: %v", err)
	}

	if info.Size() < 100000 { // Should be at least 100KB
		t.Errorf("HAR file seems too small: %d bytes", info.Size())
	}

	// Parse and verify
	data, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read HAR file: %v", err)
	}

	var harData har.HAR
	if err := json.Unmarshal(data, &harData); err != nil {
		t.Fatalf("Failed to parse HAR file: %v", err)
	}

	// Check response sizes
	for _, entry := range harData.Log.Entries {
		if strings.HasSuffix(entry.Request.URL, "/") {
			if entry.Response.Content.Size < 250000 {
				t.Errorf("HTML response size too small: %d bytes", entry.Response.Content.Size)
			}
		}
	}
}
