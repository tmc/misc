// Package secureio provides secure file system operations with proper validation and limits.
package secureio

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	// MaxFileSize is the maximum size for individual files (100MB)
	MaxFileSize = 100 * 1024 * 1024
	// MaxTotalSize is the maximum total size for all operations (1GB)
	MaxTotalSize = 1024 * 1024 * 1024
	// SecureFilePerms are the permissions for secure files (owner read/write only)
	SecureFilePerms = 0600
	// SecureDirPerms are the permissions for secure directories (owner read/write/execute only)
	SecureDirPerms = 0700
	// TempDirPrefix is the prefix for temporary directories
	TempDirPrefix = "chrome-to-har-"
)

// CreateSecureTempDir creates a temporary directory with secure permissions and random name
func CreateSecureTempDir(prefix string) (string, error) {
	// Generate random suffix for security
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}

	if prefix == "" {
		prefix = TempDirPrefix
	}

	dirName := fmt.Sprintf("%s%s", prefix, hex.EncodeToString(randomBytes))
	tempDir := filepath.Join(os.TempDir(), dirName)

	// Create directory with secure permissions
	if err := os.MkdirAll(tempDir, SecureDirPerms); err != nil {
		return "", fmt.Errorf("creating secure directory: %w", err)
	}

	// Double-check permissions were set correctly
	if err := os.Chmod(tempDir, SecureDirPerms); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("setting secure permissions: %w", err)
	}

	return tempDir, nil
}

// SecureCopyFile copies a file with size limits and secure permissions
func SecureCopyFile(src, dst string, maxSize int64) (retErr error) {
	if maxSize <= 0 {
		maxSize = MaxFileSize
	}

	// Validate source file exists and get info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("accessing source file: %w", err)
	}

	// Check file size
	if srcInfo.Size() > maxSize {
		return fmt.Errorf("file too large: %d bytes (max: %d)", srcInfo.Size(), maxSize)
	}

	// Check if source is a regular file
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file")
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file with secure permissions
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, SecureFilePerms)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer func() {
		dstFile.Close()
		if retErr != nil {
			os.Remove(dst) // Clean up on error
		}
	}()

	// Copy with size limit
	written, err := io.CopyN(dstFile, srcFile, maxSize)
	if err != nil && err != io.EOF {
		return fmt.Errorf("copying file: %w", err)
	}

	// Verify we didn't exceed the limit
	if written == maxSize {
		// Check if there's more data
		buf := make([]byte, 1)
		if n, readErr := srcFile.Read(buf); n > 0 || (readErr != nil && readErr != io.EOF) {
			return fmt.Errorf("file exceeds size limit")
		}
	}

	// Ensure proper permissions
	if err := dstFile.Chmod(SecureFilePerms); err != nil {
		return fmt.Errorf("setting file permissions: %w", err)
	}

	return nil
}

// SecureWriteFile writes data to a file with secure permissions using atomic operations
func SecureWriteFile(filename string, data []byte) error {
	if len(data) > MaxFileSize {
		return fmt.Errorf("data too large: %d bytes (max: %d)", len(data), MaxFileSize)
	}

	// Create temporary file in same directory for atomic operation
	dir := filepath.Dir(filename)
	base := filepath.Base(filename)

	tempFile, err := os.CreateTemp(dir, base+".tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tempName := tempFile.Name()

	// Ensure cleanup on failure
	defer func() {
		if err != nil {
			os.Remove(tempName)
		}
	}()

	// Set secure permissions on temp file
	if err := tempFile.Chmod(SecureFilePerms); err != nil {
		tempFile.Close()
		return fmt.Errorf("setting temp file permissions: %w", err)
	}

	// Write data
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("writing data: %w", err)
	}

	// Ensure data is written to disk
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("syncing data: %w", err)
	}

	// Close temp file
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomically rename temp file to final name
	if err := os.Rename(tempName, filename); err != nil {
		return fmt.Errorf("renaming file: %w", err)
	}

	return nil
}

// SecureCopyDir copies a directory with size limits and secure permissions
func SecureCopyDir(src, dst string, maxTotalSize int64) error {
	if maxTotalSize <= 0 {
		maxTotalSize = MaxTotalSize
	}

	var totalSize int64

	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("accessing source directory: %w", err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	// Create destination directory with secure permissions
	if err := os.MkdirAll(dst, SecureDirPerms); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// Walk through source directory
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("calculating relative path: %w", err)
		}

		destPath := filepath.Join(dst, relPath)

		// Check total size limit
		if info.Size() > 0 {
			totalSize += info.Size()
			if totalSize > maxTotalSize {
				return fmt.Errorf("total size exceeds limit: %d bytes (max: %d)", totalSize, maxTotalSize)
			}
		}

		if info.IsDir() {
			// Create directory
			if err := os.MkdirAll(destPath, SecureDirPerms); err != nil {
				return fmt.Errorf("creating directory %s: %w", destPath, err)
			}
		} else if info.Mode().IsRegular() {
			// Copy regular file
			if err := SecureCopyFile(path, destPath, MaxFileSize); err != nil {
				return fmt.Errorf("copying file %s: %w", path, err)
			}
		}
		// Skip other file types (symlinks, devices, etc.)

		return nil
	})

	if err != nil {
		os.RemoveAll(dst) // Clean up on error
		return err
	}

	return nil
}

// SecureRemoveAll removes files and directories securely
func SecureRemoveAll(path string) error {
	// First, try to remove normally
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("removing path: %w", err)
	}

	// Verify removal was successful
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("failed to remove path: %s", path)
	}

	return nil
}

// LockFile provides advisory file locking for preventing concurrent access
type LockFile struct {
	file *os.File
	path string
}

// NewLockFile creates a new lock file with exclusive lock
func NewLockFile(path string) (*LockFile, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, SecureFilePerms)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("acquiring lock: %w", err)
	}

	return &LockFile{file: file, path: path}, nil
}

// Unlock releases the lock and removes the lock file
func (l *LockFile) Unlock() error {
	if l.file == nil {
		return nil
	}

	// Release the lock
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		l.file.Close()
		return fmt.Errorf("releasing lock: %w", err)
	}

	// Close the file
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("closing lock file: %w", err)
	}

	// Remove the lock file
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing lock file: %w", err)
	}

	l.file = nil
	return nil
}

// SecureSQLQuery executes SQL queries with proper parameterization to prevent SQL injection
func SecureSQLQuery(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {
	// Prepare statement to prevent SQL injection
	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	// Execute with parameters
	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}

	return rows, nil
}

// SecureSQLExec executes SQL statements with proper parameterization
func SecureSQLExec(db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	// Prepare statement to prevent SQL injection
	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	// Execute with parameters
	result, err := stmt.Exec(args...)
	if err != nil {
		return nil, fmt.Errorf("executing statement: %w", err)
	}

	return result, nil
}

// BuildDomainFilterQuery builds a secure SQL query for domain filtering
func BuildDomainFilterQuery(domains []string) (string, []interface{}) {
	if len(domains) == 0 {
		return "", nil
	}

	placeholders := make([]string, len(domains))
	args := make([]interface{}, len(domains))

	for i, domain := range domains {
		placeholders[i] = "host_key LIKE ?"
		args[i] = "%" + domain + "%"
	}

	query := "DELETE FROM cookies WHERE NOT (" + strings.Join(placeholders, " OR ") + ")"

	return query, args
}

// FileInfo represents secure file information
type FileInfo struct {
	Path        string
	Size        int64
	ModTime     time.Time
	IsDir       bool
	Permissions os.FileMode
}

// SecureFileInfo gets file information securely
func SecureFileInfo(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("getting file info: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path: %w", err)
	}

	return &FileInfo{
		Path:        absPath,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		IsDir:       info.IsDir(),
		Permissions: info.Mode(),
	}, nil
}

// IsSecurePermissions checks if file/directory has secure permissions
func IsSecurePermissions(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("getting file info: %w", err)
	}

	mode := info.Mode()
	perm := mode.Perm()

	if info.IsDir() {
		return perm == SecureDirPerms, nil
	}

	return perm == SecureFilePerms, nil
}

// EnsureSecurePermissions ensures a file or directory has secure permissions
func EnsureSecurePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("getting file info: %w", err)
	}

	var targetPerm os.FileMode
	if info.IsDir() {
		targetPerm = SecureDirPerms
	} else {
		targetPerm = SecureFilePerms
	}

	if err := os.Chmod(path, targetPerm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	return nil
}

// CleanupHandler provides cleanup functionality for temporary resources
type CleanupHandler struct {
	paths []string
	locks []*LockFile
}

// NewCleanupHandler creates a new cleanup handler
func NewCleanupHandler() *CleanupHandler {
	return &CleanupHandler{}
}

// AddPath adds a path to be cleaned up
func (c *CleanupHandler) AddPath(path string) {
	c.paths = append(c.paths, path)
}

// AddLock adds a lock to be released
func (c *CleanupHandler) AddLock(lock *LockFile) {
	c.locks = append(c.locks, lock)
}

// Cleanup removes all registered paths and releases locks
func (c *CleanupHandler) Cleanup() error {
	var errors []string

	// Release locks first
	for _, lock := range c.locks {
		if err := lock.Unlock(); err != nil {
			errors = append(errors, fmt.Sprintf("releasing lock: %v", err))
		}
	}

	// Remove paths
	for _, path := range c.paths {
		if err := SecureRemoveAll(path); err != nil {
			errors = append(errors, fmt.Sprintf("removing %s: %v", path, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	return nil
}
