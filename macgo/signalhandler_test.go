package macgo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestSignalHandling tests the signal handling functionality
func TestSignalHandling(t *testing.T) {
	if !isMacOS() {
		t.Skip("Skipping test on non-macOS platform")
	}

	if IsInAppBundle() {
		t.Skip("Skipping test when already running in app bundle")
	}

	// Set up environment
	os.Setenv("MACGO_DEBUG", "1")
	
	// Build test binary
	testBinaryPath := buildTestBinary(t)
	defer os.Remove(testBinaryPath)

	// Run tests
	t.Run("SignalTest_SIGINT", func(t *testing.T) {
		testSignalPropagation(t, testBinaryPath, syscall.SIGINT)
	})

	t.Run("SignalTest_SIGTERM", func(t *testing.T) {
		testSignalPropagation(t, testBinaryPath, syscall.SIGTERM)
	})

	t.Run("SignalTest_CleanupOnSignal", func(t *testing.T) {
		testCleanupOnSignal(t, testBinaryPath)
	})
}

// Helper function to build a test binary that uses macgo
func buildTestBinary(t *testing.T) string {
	t.Helper()
	
	// Create temp directory for test binary
	tempDir, err := os.MkdirTemp("", "macgo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	// Create test binary path
	testBinaryPath := filepath.Join(tempDir, "signal_test")
	
	// Build the test binary
	cmd := exec.Command("go", "build", "-o", testBinaryPath, "./examples/signal-test/main.go")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build test binary: %v\nOutput: %s", err, output)
	}
	
	return testBinaryPath
}

// Helper to run a test binary and send it a signal
func testSignalPropagation(t *testing.T, binaryPath string, sig syscall.Signal) {
	t.Helper()
	
	// Build the signal capture utility
	captureDir, err := os.MkdirTemp("", "macgo-signal-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(captureDir)
	
	capturePath := filepath.Join(captureDir, "signal-capture")
	buildCmd := exec.Command("go", "build", "-o", capturePath, "./examples/signal-capture/main.go")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build signal capture utility: %v\nOutput: %s", err, output)
	}
	
	// Start the signal capture utility
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, capturePath)
	cmd.Env = append(os.Environ(), "MACGO_DEBUG=1")
	
	var outputBuf strings.Builder
	var errBuf strings.Builder
	cmd.Stdout = &outputBuf
	cmd.Stderr = &errBuf
	
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start signal capture utility: %v", err)
	}
	
	// Give the process time to initialize
	time.Sleep(2 * time.Second)
	
	// Find the log file
	tempDir := os.TempDir()
	logFilePattern := filepath.Join(tempDir, fmt.Sprintf("macgo-signal-log-%d.txt", cmd.Process.Pid))
	
	// Send the signal
	t.Logf("Sending signal %v to process %d", sig, cmd.Process.Pid)
	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}
	
	// Wait for the process to exit
	err = cmd.Wait()
	
	// Read stdout and stderr
	stdout := outputBuf.String()
	stderr := errBuf.String()
	t.Logf("Process stdout: %s", stdout)
	t.Logf("Process stderr: %s", stderr)
	
	// Check log file
	time.Sleep(500 * time.Millisecond) // Give time for file to be written
	logContent, err := os.ReadFile(logFilePattern)
	if err != nil {
		t.Logf("Warning: Could not read log file: %v", err)
	} else {
		t.Logf("Log file content:\n%s", string(logContent))
		
		// Verify signal was received
		if !strings.Contains(string(logContent), "Received signal:") {
			t.Errorf("Process did not report receiving the signal in log file")
		}
	}
	
	// Verify signal was received
	signalReceived := strings.Contains(stdout, "Received signal:") || 
		strings.Contains(stderr, "Received signal:") ||
		(logContent != nil && strings.Contains(string(logContent), "Received signal:"))
	
	if !signalReceived {
		t.Errorf("Process did not report receiving the signal")
	}
}

// Test that resources are cleaned up properly when a signal is received
func testCleanupOnSignal(t *testing.T, binaryPath string) {
	t.Helper()
	
	// Build the signal capture utility
	captureDir, err := os.MkdirTemp("", "macgo-signal-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(captureDir)
	
	capturePath := filepath.Join(captureDir, "signal-capture")
	buildCmd := exec.Command("go", "build", "-o", capturePath, "./examples/signal-capture/main.go")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build signal capture utility: %v\nOutput: %s", err, output)
	}

	// Get the temp directory used by macgo
	tempDir := os.TempDir()
	
	// Get initial state of temp directory
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}
	
	initialFiles := make(map[string]struct{})
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "macgo-") {
			initialFiles[entry.Name()] = struct{}{}
		}
	}
	
	// Start the signal capture utility
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, capturePath)
	cmd.Env = append(os.Environ(), "MACGO_DEBUG=1")
	
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start signal capture utility: %v", err)
	}
	
	// Give the process time to initialize
	time.Sleep(2 * time.Second)
	
	// Send SIGINT
	t.Logf("Sending SIGINT to process %d", cmd.Process.Pid)
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}
	
	// Wait for process to exit
	cmd.Wait()
	
	// Check for leaked files in temp directory
	time.Sleep(1 * time.Second) // Give time for cleanup
	
	// Skip any log files created by our signal-capture utility
	logFilePrefix := fmt.Sprintf("macgo-signal-log-%d", cmd.Process.Pid)
	
	// Look for any new macgo-* temp files that weren't cleaned up
	entries, err = os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}
	
	leakedFiles := []string{}
	for _, entry := range entries {
		name := entry.Name()
		// Skip if it was already there
		if _, existed := initialFiles[name]; existed {
			continue
		}
		
		// Skip our signal capture log files
		if strings.Contains(name, logFilePrefix) {
			continue
		}
		
		// Check if it's a new macgo-* file created recently
		if strings.Contains(name, "macgo-") && 
		   time.Since(getFileModTime(t, filepath.Join(tempDir, name))) < 1*time.Minute {
			leakedFiles = append(leakedFiles, name)
		}
	}
	
	if len(leakedFiles) > 0 {
		t.Errorf("Found leaked temp files after signal: %v", leakedFiles)
	}
}

// Helper to check if running on macOS
func isMacOS() bool {
	return runtime.GOOS == "darwin"
}

// Helper to get file modification time
func getFileModTime(t *testing.T, path string) time.Time {
	t.Helper()
	
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}
	
	return info.ModTime()
}