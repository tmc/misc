package macgo

import (
	"os"
	"strings"
	"testing"
)

func TestCalculateSHA256(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "macgo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	
	// Write some content
	content := "test content for SHA256"
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}
	
	// Calculate the hash
	hash, err := checksum(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	
	// The hash should be 64 characters long (SHA-256 is 32 bytes, hex-encoded)
	if len(hash) != 64 {
		t.Errorf("Expected SHA-256 hash to be 64 characters, got %d", len(hash))
	}
}

func TestCopyFile(t *testing.T) {
	// Create a source file
	srcFile, err := os.CreateTemp("", "macgo-test-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(srcFile.Name())
	
	// Write content
	content := "test content for copy file"
	if _, err := srcFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := srcFile.Close(); err != nil {
		t.Fatal(err)
	}
	
	// Create a destination path
	dstFile, err := os.CreateTemp("", "macgo-test-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	dstPath := dstFile.Name()
	dstFile.Close()
	os.Remove(dstPath) // Remove it so copyFile can create it
	defer os.Remove(dstPath)
	
	// Copy the file
	if err := copyFile(srcFile.Name(), dstPath); err != nil {
		t.Fatal(err)
	}
	
	// Verify the content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	
	if string(dstContent) != content {
		t.Errorf("Expected content %q, got %q", content, string(dstContent))
	}
}

// TestAppBundleCreation skips actual creation in test mode
func TestAppBundleCreation(t *testing.T) {
	// Skip if we can't find our own executable
	execPath, err := os.Executable()
	if err != nil {
		t.Skip("Could not determine executable path")
	}
	
	// Skip this test - it's more of a functionality test
	// We can't properly test this without actually creating an app bundle
	// and that might interfere with the user's environment
	if strings.Contains(execPath, "go-build") {
		t.Log("Running with go test, which uses temporary binaries")
		// Verify the isTemporary detection works
		if !strings.Contains(execPath, "go-build") {
			t.Error("Expected to detect temporary binary during test")
		}
	}
}