package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestChurl_Build(t *testing.T) {
	// Skip if running in CI or without Chrome
	if os.Getenv("CI") != "" || os.Getenv("SKIP_BROWSER_TESTS") != "" {
		t.Skip("Skipping test in CI environment")
	}

	// Make sure we can build the command
	cmd := exec.Command("go", "build", "-o", "churl_test")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to build churl: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}
	
	// Clean up
	defer os.Remove("churl_test")
}

func TestChurl_ShowHelp(t *testing.T) {
	// Skip if running in CI or without Chrome
	if os.Getenv("CI") != "" || os.Getenv("SKIP_BROWSER_TESTS") != "" {
		t.Skip("Skipping test in CI environment")
	}
	
	// Build churl for testing
	cmd := exec.Command("go", "build", "-o", "churl_test")
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to build churl: %v", err)
	}
	defer os.Remove("churl_test")
	
	// Run with -h flag
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	helpCmd := exec.CommandContext(ctx, "./churl_test", "-h")
	output, err := helpCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run churl -h: %v\nOutput: %s", err, string(output))
	}
	
	// Check that help output contains expected text
	if !bytes.Contains(output, []byte("Chrome-powered curl")) {
		t.Errorf("Help output doesn't contain expected text.\nOutput: %s", string(output))
	}
}

// TestChurl_BasicFetch tests fetching a simple URL
// This is commented out by default as it requires Chrome to be installed
// and launches a browser instance
/*
func TestChurl_BasicFetch(t *testing.T) {
	// Skip if running in CI or without Chrome
	if os.Getenv("CI") != "" || os.Getenv("SKIP_BROWSER_TESTS") != "" {
		t.Skip("Skipping test in CI environment")
	}
	
	// Build churl for testing
	cmd := exec.Command("go", "build", "-o", "churl_test")
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to build churl: %v", err)
	}
	defer os.Remove("churl_test")
	
	// Run with example.com
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	fetchCmd := exec.CommandContext(ctx, "./churl_test", 
		"--headless", 
		"--output-format=text", 
		"https://example.com")
	output, err := fetchCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to fetch example.com: %v\nOutput: %s", err, string(output))
	}
	
	// Check that output contains expected text
	if !bytes.Contains(output, []byte("Example Domain")) {
		t.Errorf("Output doesn't contain expected 'Example Domain' text.\nOutput: %s", string(output))
	}
}
*/