package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// skipIfNoBrowser skips the test if Chrome is not available or if running in short mode
func skipIfNoBrowser(t testing.TB) {
	t.Helper()

	// Skip browser tests in short mode (go test -short)
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	// Skip in CI environment unless browser is explicitly available
	if os.Getenv("CI") != "" {
		t.Skip("Skipping browser test in CI environment")
	}

	// Skip tests due to known context cancellation issue with chromedp on this system
	if os.Getenv("CHROMEDP_CONTEXT_ISSUE") != "" {
		t.Skip("Skipping browser test due to chromedp context cancellation issue")
	}

	// Skip if SKIP_BROWSER_TESTS is set
	if os.Getenv("SKIP_BROWSER_TESTS") != "" {
		t.Skip("Skipping browser tests (SKIP_BROWSER_TESTS is set)")
	}

	// Check if Chrome is available
	chromePath, found := detectChromePath()
	if !found {
		t.Skip("Chrome/Chromium not found, skipping browser test")
	}

	// Verify the Chrome executable actually exists and is executable
	info, err := os.Stat(chromePath)
	if err != nil {
		t.Skipf("Chrome found at %s but cannot stat: %v", chromePath, err)
	}

	if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
		t.Skipf("Chrome found at %s but is not executable", chromePath)
	}
}

// buildChurl builds the churl binary for testing
func buildChurl(t testing.TB) string {
	t.Helper()

	// Create a temporary directory for the binary
	tempDir := t.TempDir()
	churlPath := filepath.Join(tempDir, "churl_test")
	if runtime.GOOS == "windows" {
		churlPath += ".exe"
	}

	// Build the binary
	cmd := exec.Command("go", "build", "-o", churlPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build churl: %v\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
	}

	return churlPath
}

func TestChurl_Build(t *testing.T) {
	t.Parallel()
	churlPath := buildChurl(t)

	// Verify the binary exists
	if _, err := os.Stat(churlPath); err != nil {
		t.Fatalf("Built binary not found at %s: %v", churlPath, err)
	}
}

func TestChurl_ShowHelp(t *testing.T) {
	t.Parallel()
	churlPath := buildChurl(t)

	tests := []struct {
		name     string
		args     []string
		contains []string
	}{
		{
			name: "help flag",
			args: []string{"-h"},
			contains: []string{
				"Chrome-powered curl",
				"Usage:",
				"Options:",
			},
		},
		{
			name: "no args shows help",
			args: []string{},
			contains: []string{
				"Error: URL is required",
				"Chrome-powered curl",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, churlPath, tt.args...)
			output, _ := cmd.CombinedOutput()

			outputStr := string(output)
			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Output missing expected text %q.\nFull output:\n%s",
						expected, outputStr)
				}
			}
		})
	}
}

func TestChurl_BasicFetch(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	tests := []struct {
		name        string
		args        []string
		contains    []string
		notContains []string
	}{
		{
			name: "basic HTML fetch",
			args: []string{
				"--headless",
				"--timeout", "60",
				"--output-format=html",
				ts.URL,
			},
			contains: []string{
				"<title>Test Page</title>",
				"<h1>Test Page</h1>",
				"<div id=\"content\">Main content here</div>",
			},
		},
		{
			name: "text output format",
			args: []string{
				"--headless",
				"--timeout", "60",
				"--output-format=text",
				ts.URL,
			},
			contains: []string{
				"Test Page",
				"Main content here",
			},
			notContains: []string{
				"<html>",
				"<title>",
			},
		},
		{
			name: "JSON output format",
			args: []string{
				"--headless",
				"--timeout", "60",
				"--output-format=json",
				ts.URL,
			},
			contains: []string{
				`"url"`,
				`"title"`,
				`"content"`,
				"Test Page",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, churlPath, tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Failed to run churl: %v\nOutput: %s", err, string(output))
			}

			outputStr := string(output)
			for _, expected := range tt.contains {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Output missing expected text %q.\nFull output:\n%s",
						expected, outputStr)
				}
			}

			for _, unexpected := range tt.notContains {
				if strings.Contains(outputStr, unexpected) {
					t.Errorf("Output contains unexpected text %q.\nFull output:\n%s",
						unexpected, outputStr)
				}
			}
		})
	}
}

func TestChurl_Headers(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlPath,
		"--headless",
		"--timeout", "60",
		"--output-format=html",
		"-H", "X-Custom-Header: test-value",
		"-H", "X-Another-Header: another-value",
		ts.URL+"/headers",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run churl: %v\nOutput: %s", err, string(output))
	}

	// The response should contain our custom headers
	outputStr := string(output)
	if !strings.Contains(outputStr, "X-Custom-Header") {
		t.Errorf("Custom header X-Custom-Header not found in response")
	}
	if !strings.Contains(outputStr, "X-Another-Header") {
		t.Errorf("Custom header X-Another-Header not found in response")
	}
}

func TestChurl_Authentication(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	tests := []struct {
		name     string
		args     []string
		success  bool
		contains string
	}{
		{
			name: "valid credentials",
			args: []string{
				"--headless",
				"--timeout", "60",
				"--output-format=html",
				"-u", "testuser:testpass",
				ts.URL + "/auth",
			},
			success:  true,
			contains: "Authenticated as: testuser",
		},
		{
			name: "invalid credentials",
			args: []string{
				"--headless",
				"--timeout", "60",
				"--output-format=html",
				"-u", "wronguser:wrongpass",
				ts.URL + "/auth",
			},
			success:  true, // Command should succeed even if auth fails
			contains: "Unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, churlPath, tt.args...)
			output, err := cmd.CombinedOutput()

			if tt.success && err != nil {
				t.Fatalf("Expected success but got error: %v\nOutput: %s", err, string(output))
			}

			if !strings.Contains(string(output), tt.contains) {
				t.Errorf("Output missing expected text %q.\nFull output:\n%s",
					tt.contains, string(output))
			}
		})
	}
}

func TestChurl_OutputFile(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	// Create a temporary output file
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.html")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlPath,
		"--headless",
		"--timeout", "60",
		"--output-format=html",
		"-o", outputFile,
		ts.URL,
	)

	// Separate stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run churl: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify the output file was created
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "Test Page") {
		t.Errorf("Output file missing expected content. Content:\n%s", string(content))
	}

	// Verify stdout is empty (output went to file)
	if stdout.Len() > 0 {
		t.Errorf("Expected empty stdout when using -o, but got:\n%s", stdout.String())
	}
}

func TestChurl_DynamicContent(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlPath,
		"--headless",
		"--timeout", "60",
		"--output-format=html",
		"--wait-network-idle",
		ts.URL+"/dynamic",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run churl: %v\nOutput: %s", err, string(output))
	}

	// Should contain the JavaScript-generated content
	if !strings.Contains(string(output), "Dynamic content loaded") {
		t.Errorf("Dynamic content not found. Output should contain JavaScript-generated content")
	}
}

func TestChurl_WaitSelector(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlPath,
		"--headless",
		"--timeout", "60",
		"--output-format=html",
		"--wait-for", "#content",
		ts.URL,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run churl: %v\nOutput: %s", err, string(output))
	}

	// Should successfully wait for and find the content
	if !strings.Contains(string(output), "Main content here") {
		t.Errorf("Content selector not found or not waited for properly")
	}
}

func TestChurl_APIEndpoint(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlPath,
		"--headless",
		"--timeout", "60",
		"--output-format=json",
		ts.URL+"/api/data",
	)

	// Separate stdout and stderr to avoid contamination
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run churl: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Parse the JSON output from stdout only
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify the content contains the API response
	content, ok := result["content"].(string)
	if !ok {
		t.Fatalf("No content field in JSON output")
	}

	if !strings.Contains(content, "Hello from API") {
		t.Errorf("API response not found in content")
	}
}

func TestChurl_Timeout(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	// Use a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlPath,
		"--headless",
		"--output-format=html",
		"--timeout", "2", // 2 second timeout
		ts.URL+"/slow", // This endpoint takes 5 seconds
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected timeout error, but command succeeded")
	}

	// Should contain timeout error message
	if !strings.Contains(string(output), "timed out") {
		t.Errorf("Expected timeout error message, got: %s", string(output))
	}
}

func TestChurl_HAR_Output(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlPath,
		"--headless",
		"--timeout", "60",
		"--output-format=har",
		ts.URL,
	)

	// Separate stdout and stderr to avoid contamination
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run churl: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Parse the HAR output from stdout only
	var har map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &har); err != nil {
		t.Fatalf("Failed to parse HAR output: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify HAR structure
	if _, ok := har["log"]; !ok {
		t.Errorf("HAR output missing 'log' field")
	}

	log, ok := har["log"].(map[string]interface{})
	if !ok {
		t.Fatalf("HAR log field is not a map")
	}

	if _, ok := log["entries"]; !ok {
		t.Errorf("HAR log missing 'entries' field")
	}
}

// TestChurl_VerboseMode tests verbose logging
func TestChurl_VerboseMode(t *testing.T) {
	t.Parallel()
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, churlPath,
		"--headless",
		"--timeout", "60",
		"--verbose",
		"--output-format=text",
		ts.URL,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run churl: %v\nOutput: %s", err, string(output))
	}

	// Verbose mode should include debug messages
	outputStr := string(output)
	verboseIndicators := []string{
		"Auto-detected Chrome path",
		"Creating browser",
		"Launching browser",
		"Navigating to URL",
	}

	foundVerbose := false
	for _, indicator := range verboseIndicators {
		if strings.Contains(outputStr, indicator) {
			foundVerbose = true
			break
		}
	}

	if !foundVerbose {
		t.Errorf("Verbose mode enabled but no debug messages found in output")
	}
}

// Benchmark test
func BenchmarkChurl_BasicFetch(b *testing.B) {
	skipIfNoBrowser(b)

	churlPath := buildChurl(b)
	ts := createTestServer()
	defer ts.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		cmd := exec.CommandContext(ctx, churlPath,
			"--headless",
			"--timeout", "60",
			"--output-format=text",
			ts.URL,
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			b.Fatalf("Failed to run churl: %v\nOutput: %s", err, string(output))
		}

		cancel()
	}
}

// TestMain allows us to do setup/teardown for all tests
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Exit with the test result code
	os.Exit(code)
}
