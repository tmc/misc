package secureio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSecureTempDir(t *testing.T) {
	// Test basic functionality
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("CreateSecureTempDir() error = %v", err)
	}
	defer os.RemoveAll(dir)

	// Check that directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Created directory does not exist: %s", dir)
	}

	// Check permissions
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	if info.Mode().Perm() != SecureDirPerms {
		t.Errorf("Directory permissions = %o, expected %o", info.Mode().Perm(), SecureDirPerms)
	}

	// Check that directory name contains prefix
	if !strings.HasPrefix(filepath.Base(dir), "test-") {
		t.Errorf("Directory name does not contain prefix: %s", dir)
	}
}

func TestSecureWriteFile(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Test data
	testData := []byte("test data")
	testFile := filepath.Join(dir, "test.txt")

	// Write file
	if err := SecureWriteFile(testFile, testData); err != nil {
		t.Fatalf("SecureWriteFile() error = %v", err)
	}

	// Check file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("File does not exist: %s", testFile)
	}

	// Check permissions
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Mode().Perm() != SecureFilePerms {
		t.Errorf("File permissions = %o, expected %o", info.Mode().Perm(), SecureFilePerms)
	}

	// Check content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != string(testData) {
		t.Errorf("File content = %s, expected %s", string(content), string(testData))
	}
}

func TestSecureWriteFileTooBig(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Test data that's too big
	testData := make([]byte, MaxFileSize+1)
	testFile := filepath.Join(dir, "test.txt")

	// Write file should fail
	if err := SecureWriteFile(testFile, testData); err == nil {
		t.Errorf("SecureWriteFile() should have failed for oversized file")
	}
}

func TestSecureCopyFile(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create source file
	srcFile := filepath.Join(dir, "source.txt")
	testData := []byte("test data for copy")
	if err := os.WriteFile(srcFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy file
	dstFile := filepath.Join(dir, "dest.txt")
	if err := SecureCopyFile(srcFile, dstFile, MaxFileSize); err != nil {
		t.Fatalf("SecureCopyFile() error = %v", err)
	}

	// Check destination file exists
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Errorf("Destination file does not exist: %s", dstFile)
	}

	// Check permissions
	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("Failed to stat destination file: %v", err)
	}

	if info.Mode().Perm() != SecureFilePerms {
		t.Errorf("File permissions = %o, expected %o", info.Mode().Perm(), SecureFilePerms)
	}

	// Check content
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(content) != string(testData) {
		t.Errorf("File content = %s, expected %s", string(content), string(testData))
	}
}

func TestSecureCopyFileTooBig(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create large source file
	srcFile := filepath.Join(dir, "source.txt")
	testData := make([]byte, 1024)
	if err := os.WriteFile(srcFile, testData, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy file with small size limit should fail
	dstFile := filepath.Join(dir, "dest.txt")
	if err := SecureCopyFile(srcFile, dstFile, 512); err == nil {
		t.Errorf("SecureCopyFile() should have failed for oversized file")
	}
}

func TestSecureCopyDir(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create source directory structure
	srcDir := filepath.Join(dir, "source")
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(srcDir, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", relPath, err)
		}
	}

	// Copy directory
	dstDir := filepath.Join(dir, "dest")
	if err := SecureCopyDir(srcDir, dstDir, MaxTotalSize); err != nil {
		t.Fatalf("SecureCopyDir() error = %v", err)
	}

	// Check destination directory exists
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		t.Errorf("Destination directory does not exist: %s", dstDir)
	}

	// Check all files were copied
	for relPath, expectedContent := range testFiles {
		fullPath := filepath.Join(dstDir, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("Failed to read copied file %s: %v", relPath, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("File %s content = %s, expected %s", relPath, string(content), expectedContent)
		}

		// Check permissions
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Errorf("Failed to stat copied file %s: %v", relPath, err)
			continue
		}

		if info.Mode().Perm() != SecureFilePerms {
			t.Errorf("File %s permissions = %o, expected %o", relPath, info.Mode().Perm(), SecureFilePerms)
		}
	}
}

func TestSecureRemoveAll(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create some content
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Remove directory
	if err := SecureRemoveAll(dir); err != nil {
		t.Fatalf("SecureRemoveAll() error = %v", err)
	}

	// Check directory is gone
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Directory still exists after removal: %s", dir)
	}
}

func TestLockFile(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	lockPath := filepath.Join(dir, "test.lock")

	// Create lock
	lock1, err := NewLockFile(lockPath)
	if err != nil {
		t.Fatalf("NewLockFile() error = %v", err)
	}

	// Try to create another lock on same file (should fail)
	lock2, err := NewLockFile(lockPath)
	if err == nil {
		lock2.Unlock()
		t.Errorf("NewLockFile() should have failed for already locked file")
	}

	// Release first lock
	if err := lock1.Unlock(); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	// Now second lock should succeed
	lock3, err := NewLockFile(lockPath)
	if err != nil {
		t.Fatalf("NewLockFile() after unlock error = %v", err)
	}
	defer lock3.Unlock()
}

func TestBuildDomainFilterQuery(t *testing.T) {
	tests := []struct {
		name    string
		domains []string
		wantNil bool
	}{
		{"Empty domains", []string{}, true},
		{"Single domain", []string{"example.com"}, false},
		{"Multiple domains", []string{"example.com", "test.com"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := BuildDomainFilterQuery(tt.domains)

			if tt.wantNil {
				if query != "" {
					t.Errorf("Expected empty query for empty domains")
				}
				return
			}

			if query == "" {
				t.Errorf("Expected non-empty query for domains %v", tt.domains)
			}

			if len(args) != len(tt.domains) {
				t.Errorf("Expected %d args, got %d", len(tt.domains), len(args))
			}

			// Check that query contains proper placeholders
			expectedPlaceholders := len(tt.domains)
			actualPlaceholders := 0
			for _, char := range query {
				if char == '?' {
					actualPlaceholders++
				}
			}

			if actualPlaceholders != expectedPlaceholders {
				t.Errorf("Expected %d placeholders, got %d", expectedPlaceholders, actualPlaceholders)
			}
		})
	}
}

func TestIsSecurePermissions(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Test directory permissions
	isSecure, err := IsSecurePermissions(dir)
	if err != nil {
		t.Fatalf("IsSecurePermissions() error = %v", err)
	}

	if !isSecure {
		t.Errorf("Directory should have secure permissions")
	}

	// Create test file
	testFile := filepath.Join(dir, "test.txt")
	if err := SecureWriteFile(testFile, []byte("test")); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test file permissions
	isSecure, err = IsSecurePermissions(testFile)
	if err != nil {
		t.Fatalf("IsSecurePermissions() error = %v", err)
	}

	if !isSecure {
		t.Errorf("File should have secure permissions")
	}

	// Create insecure file
	insecureFile := filepath.Join(dir, "insecure.txt")
	if err := os.WriteFile(insecureFile, []byte("test"), 0666); err != nil {
		t.Fatalf("Failed to create insecure file: %v", err)
	}

	// Test insecure file permissions
	isSecure, err = IsSecurePermissions(insecureFile)
	if err != nil {
		t.Fatalf("IsSecurePermissions() error = %v", err)
	}

	if isSecure {
		t.Errorf("File should not have secure permissions")
	}
}

func TestEnsureSecurePermissions(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create insecure file
	insecureFile := filepath.Join(dir, "insecure.txt")
	if err := os.WriteFile(insecureFile, []byte("test"), 0666); err != nil {
		t.Fatalf("Failed to create insecure file: %v", err)
	}

	// Fix permissions
	if err := EnsureSecurePermissions(insecureFile); err != nil {
		t.Fatalf("EnsureSecurePermissions() error = %v", err)
	}

	// Check permissions are now secure
	isSecure, err := IsSecurePermissions(insecureFile)
	if err != nil {
		t.Fatalf("IsSecurePermissions() error = %v", err)
	}

	if !isSecure {
		t.Errorf("File should have secure permissions after fix")
	}
}

func TestCleanupHandler(t *testing.T) {
	// Create test directory
	dir, err := CreateSecureTempDir("test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test file
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create cleanup handler
	cleanup := NewCleanupHandler()
	cleanup.AddPath(dir)

	// Create lock file
	lockFile := filepath.Join(dir, "test.lock")
	lock, err := NewLockFile(lockFile)
	if err != nil {
		t.Fatalf("Failed to create lock: %v", err)
	}
	cleanup.AddLock(lock)

	// Run cleanup
	if err := cleanup.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// Check directory is gone
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Directory still exists after cleanup: %s", dir)
	}
}

// Benchmarks
func BenchmarkCreateSecureTempDir(b *testing.B) {
	for i := 0; i < b.N; i++ {
		dir, err := CreateSecureTempDir("bench-")
		if err != nil {
			b.Fatalf("CreateSecureTempDir() error = %v", err)
		}
		os.RemoveAll(dir)
	}
}

func BenchmarkSecureWriteFile(b *testing.B) {
	dir, err := CreateSecureTempDir("bench-")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	data := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(dir, fmt.Sprintf("bench-%d.txt", i))
		if err := SecureWriteFile(testFile, data); err != nil {
			b.Fatalf("SecureWriteFile() error = %v", err)
		}
	}
}

func BenchmarkSecureCopyFile(b *testing.B) {
	dir, err := CreateSecureTempDir("bench-")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create source file
	srcFile := filepath.Join(dir, "source.txt")
	testData := []byte("benchmark data for copy")
	if err := os.WriteFile(srcFile, testData, 0644); err != nil {
		b.Fatalf("Failed to create source file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dstFile := filepath.Join(dir, fmt.Sprintf("dest-%d.txt", i))
		if err := SecureCopyFile(srcFile, dstFile, MaxFileSize); err != nil {
			b.Fatalf("SecureCopyFile() error = %v", err)
		}
	}
}
