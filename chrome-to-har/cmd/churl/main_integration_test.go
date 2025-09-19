//go:build integration
// +build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/tmc/misc/chrome-to-har/internal/testutil"
)

func TestIntegrationChurl_BasicFetch(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	// Build churl binary
	churlBinary := buildChurlIntegration(t)

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Churl Test</title></head>
<body>
	<h1>Hello from Churl!</h1>
	<div id="content">Test content</div>
</body>
</html>`)
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	tests := []struct {
		name       string
		args       []string
		wantOutput []string
		wantError  bool
	}{
		{
			name: "basic_html_fetch",
			args: []string{"--headless", server.URL()},
			wantOutput: []string{
				"Hello from Churl!",
				"Test content",
			},
		},
		{
			name: "text_output_format",
			args: []string{"--headless", "--output-format=text", server.URL()},
			wantOutput: []string{
				"Churl Test",
				"Hello from Churl!",
				"Test content",
			},
		},
		{
			name: "json_output_format",
			args: []string{"--headless", "--output-format=json", server.URL()},
			wantOutput: []string{
				`"title"`,
				`"Churl Test"`,
				`"url"`,
				server.URL(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, churlBinary, tt.args...)
			output, err := cmd.CombinedOutput()

			if (err != nil) != tt.wantError {
				t.Errorf("churl error = %v, wantError %v\nOutput: %s", err, tt.wantError, output)
			}

			outputStr := string(output)
			for _, want := range tt.wantOutput {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Output missing expected content %q\nGot: %s", want, outputStr)
				}
			}
		})
	}
}

func TestIntegrationChurl_Headers(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	churlBinary := buildChurlIntegration(t)

	// Create test server that echoes headers
	mux := http.NewServeMux()
	mux.HandleFunc("/headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		headers := make(map[string]string)
		for k, v := range r.Header {
			if strings.HasPrefix(k, "X-") || k == "User-Agent" || k == "Cookie" {
				headers[k] = v[0]
			}
		}
		json.NewEncoder(w).Encode(headers)
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	tests := []struct {
		name       string
		args       []string
		wantInJSON map[string]string
	}{
		{
			name: "custom_headers",
			args: []string{
				"--headless",
				"-H", "X-Custom-Header: test-value",
				"-H", "X-Another: another-value",
				"--output-format=html",
				server.URL() + "/headers",
			},
			wantInJSON: map[string]string{
				"X-Custom-Header": "test-value",
				"X-Another":       "another-value",
			},
		},
		{
			name: "user_agent",
			args: []string{
				"--headless",
				"-H", "User-Agent: Churl/1.0",
				"--output-format=html",
				server.URL() + "/headers",
			},
			wantInJSON: map[string]string{
				"User-Agent": "Churl/1.0",
			},
		},
		{
			name: "cookies",
			args: []string{
				"--headless",
				"-H", "Cookie: session=abc123; user=test",
				"--output-format=html",
				server.URL() + "/headers",
			},
			wantInJSON: map[string]string{
				"Cookie": "session=abc123; user=test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, churlBinary, tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("churl failed: %v\nOutput: %s", err, output)
			}

			// Extract JSON from HTML output
			outputStr := string(output)
			start := strings.Index(outputStr, "{")
			end := strings.LastIndex(outputStr, "}") + 1
			if start < 0 || end <= start {
				t.Fatalf("Could not find JSON in output: %s", outputStr)
			}

			jsonStr := outputStr[start:end]
			var headers map[string]string
			if err := json.Unmarshal([]byte(jsonStr), &headers); err != nil {
				t.Fatalf("Failed to parse JSON response: %v\nJSON: %s", err, jsonStr)
			}

			for k, v := range tt.wantInJSON {
				if headers[k] != v {
					t.Errorf("Header %s = %q, want %q", k, headers[k], v)
				}
			}
		})
	}
}

func TestIntegrationChurl_PostData(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	churlBinary := buildChurlIntegration(t)

	// Create test server that echoes POST data
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			fmt.Fprintf(w, "Method not allowed: %s", r.Method)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		var body bytes.Buffer
		body.ReadFrom(r.Body)

		response := map[string]interface{}{
			"method":       r.Method,
			"content-type": r.Header.Get("Content-Type"),
			"body":         body.String(),
		}

		json.NewEncoder(w).Encode(response)
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	tests := []struct {
		name         string
		args         []string
		wantMethod   string
		wantBody     string
		wantContType string
	}{
		{
			name: "json_post",
			args: []string{
				"--headless",
				"-X", "POST",
				"-d", `{"test": "data", "number": 123}`,
				"-H", "Content-Type: application/json",
				"--output-format=html",
				server.URL() + "/echo",
			},
			wantMethod:   "POST",
			wantBody:     `{"test": "data", "number": 123}`,
			wantContType: "application/json",
		},
		{
			name: "form_post",
			args: []string{
				"--headless",
				"-X", "POST",
				"-d", "field1=value1&field2=value2",
				"-H", "Content-Type: application/x-www-form-urlencoded",
				"--output-format=html",
				server.URL() + "/echo",
			},
			wantMethod:   "POST",
			wantBody:     "field1=value1&field2=value2",
			wantContType: "application/x-www-form-urlencoded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, churlBinary, tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("churl failed: %v\nOutput: %s", err, output)
			}

			// Parse response
			outputStr := string(output)
			if !strings.Contains(outputStr, tt.wantMethod) {
				t.Errorf("Output missing method %s", tt.wantMethod)
			}
			if !strings.Contains(outputStr, tt.wantBody) {
				t.Errorf("Output missing body %s", tt.wantBody)
			}
			if !strings.Contains(outputStr, tt.wantContType) {
				t.Errorf("Output missing content-type %s", tt.wantContType)
			}
		})
	}
}

func TestIntegrationChurl_WaitForSelector(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	churlBinary := buildChurlIntegration(t)

	// Create test server with dynamic content
	mux := http.NewServeMux()
	mux.HandleFunc("/dynamic", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
	<div id="initial">Initial content</div>
	<script>
		setTimeout(() => {
			const div = document.createElement('div');
			div.id = 'delayed-content';
			div.textContent = 'This content appears after 1 second';
			document.body.appendChild(div);
		}, 1000);
		
		setTimeout(() => {
			const div = document.createElement('div');
			div.id = 'very-delayed-content';
			div.textContent = 'This content appears after 2 seconds';
			document.body.appendChild(div);
		}, 2000);
	</script>
</body>
</html>`)
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	tests := []struct {
		name         string
		waitSelector string
		shouldFind   bool
		content      string
	}{
		{
			name:         "wait_for_immediate",
			waitSelector: "#initial",
			shouldFind:   true,
			content:      "Initial content",
		},
		{
			name:         "wait_for_delayed",
			waitSelector: "#delayed-content",
			shouldFind:   true,
			content:      "This content appears after 1 second",
		},
		{
			name:         "wait_for_very_delayed",
			waitSelector: "#very-delayed-content",
			shouldFind:   true,
			content:      "This content appears after 2 seconds",
		},
		{
			name:         "wait_for_nonexistent",
			waitSelector: "#never-appears",
			shouldFind:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			args := []string{
				"--headless",
				"--wait-for", tt.waitSelector,
				"--output-format=html",
				server.URL() + "/dynamic",
			}

			// For non-existent selector, we expect timeout
			if !tt.shouldFind {
				args = append(args, "--timeout", "3")
			}

			cmd := exec.CommandContext(ctx, churlBinary, args...)
			output, err := cmd.CombinedOutput()

			if tt.shouldFind {
				if err != nil {
					t.Errorf("churl failed: %v\nOutput: %s", err, output)
				}
				if !strings.Contains(string(output), tt.content) {
					t.Errorf("Output missing expected content %q", tt.content)
				}
			} else {
				if err == nil {
					t.Error("Expected error for non-existent selector, but got none")
				}
			}
		})
	}
}

func TestIntegrationChurl_HAR_Output(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	churlBinary := buildChurlIntegration(t)

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Header", "test-value")
		fmt.Fprint(w, `<html><body>HAR test</body></html>`)
	})

	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status": "ok"}`)
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	// Create temp file for output
	tmpfile, err := os.CreateTemp("", "churl-test-*.har")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlBinary,
		"--headless",
		"--output-format=har",
		"-o", tmpfile.Name(),
		server.URL(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("churl failed: %v\nOutput: %s", err, output)
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

	// Verify HAR structure
	if harData.Log == nil || len(harData.Log.Entries) == 0 {
		t.Fatal("HAR file has no entries")
	}

	// Check first entry
	entry := harData.Log.Entries[0]
	if !strings.Contains(entry.Request.URL, server.URL()) {
		t.Errorf("Request URL = %s, want to contain %s", entry.Request.URL, server.URL())
	}

	// Check response headers
	foundHeader := false
	for _, header := range entry.Response.Headers {
		if header.Name == "X-Test-Header" && header.Value == "test-value" {
			foundHeader = true
			break
		}
	}
	if !foundHeader {
		t.Error("Custom header X-Test-Header not found in HAR")
	}
}

func TestIntegrationChurl_Timeout(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	churlBinary := buildChurlIntegration(t)

	// Create test server with slow response
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		fmt.Fprint(w, "This should not be seen")
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlBinary,
		"--headless",
		"--timeout", "2", // 2 second timeout
		server.URL()+"/slow",
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected timeout error, got none. Output: %s", output)
	}
}

func TestIntegrationChurl_ExtractSelector(t *testing.T) {
	t.Parallel()
	testutil.SkipIfNoChrome(t)

	churlBinary := buildChurlIntegration(t)

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<body>
	<div id="header">
		<h1>Main Title</h1>
		<p class="subtitle">Subtitle text</p>
	</div>
	<div id="content">
		<p>First paragraph</p>
		<p>Second paragraph</p>
		<ul>
			<li>Item 1</li>
			<li>Item 2</li>
			<li>Item 3</li>
		</ul>
	</div>
	<div id="footer">Footer content</div>
</body>
</html>`)
	})

	server := testutil.TestServer(t, mux)
	defer server.Close()

	tests := []struct {
		name     string
		selector string
		want     []string
		notWant  []string
	}{
		{
			name:     "extract_by_id",
			selector: "#content",
			want:     []string{"First paragraph", "Second paragraph", "Item 1"},
			notWant:  []string{"Main Title", "Footer content"},
		},
		{
			name:     "extract_by_class",
			selector: ".subtitle",
			want:     []string{"Subtitle text"},
			notWant:  []string{"Main Title", "First paragraph"},
		},
		{
			name:     "extract_multiple",
			selector: "p",
			want:     []string{"Subtitle text", "First paragraph", "Second paragraph"},
			notWant:  []string{"Item 1", "Footer content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, churlBinary,
				"--headless",
				"--extract", tt.selector,
				"--output-format=text",
				server.URL(),
			)

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("churl failed: %v\nOutput: %s", err, output)
			}

			outputStr := string(output)
			for _, want := range tt.want {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Output missing expected content %q", want)
				}
			}

			for _, notWant := range tt.notWant {
				if strings.Contains(outputStr, notWant) {
					t.Errorf("Output contains unexpected content %q", notWant)
				}
			}
		})
	}
}

// Helper function to build churl binary for integration tests
func buildChurlIntegration(t *testing.T) string {
	t.Helper()

	// Build in temp directory
	tmpDir := t.TempDir()
	churlBinary := filepath.Join(tmpDir, "churl")
	if runtime.GOOS == "windows" {
		churlBinary += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", churlBinary, ".")
	cmd.Dir = "/Volumes/tmc/go/src/github.com/tmc/misc/chrome-to-har/cmd/churl"

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build churl: %v\nOutput: %s", err, output)
	}

	return churlBinary
}
