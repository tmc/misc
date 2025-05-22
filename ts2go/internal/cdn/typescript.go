package cdn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	TypeScriptCDNURL = "https://cdnjs.cloudflare.com/ajax/libs/typescript/5.8.2/typescript.js"
	CacheDir         = ".ts2go-cache"
	HttpTimeout      = 30 * time.Second
)

// verifyDirectoryWritable checks if the directory exists and is writable
func verifyDirectoryWritable(dir string) error {
	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("cannot access directory: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	// Create a temporary file to verify write permissions
	tempFile := filepath.Join(dir, ".write-test-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	fd, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	fd.Close()
	os.Remove(tempFile)

	return nil
}

// validateCacheFile checks if the cached file is valid
func validateCacheFile(path string) error {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Check if it's a file
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a file", path)
	}

	// Check if file is not empty
	if info.Size() == 0 {
		return fmt.Errorf("cache file is empty")
	}

	return nil
}

// calculateFileHash computes sha256 hash of file contents
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// FetchTypeScript downloads the TypeScript compiler from CDN if not already cached
// and returns the path to the local file
func FetchTypeScript() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, CacheDir)

	// Check if directory exists first
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		// Directory doesn't exist, attempt to create it
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			// Check for specific errors
			if os.IsPermission(err) {
				return "", fmt.Errorf("permission denied when creating cache directory %s: %w", cacheDir, err)
			}
			if os.IsExist(err) {
				// Race condition - another process created it between our check and creation
				// This is fine, we can continue
			} else {
				return "", fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
			}
		}
	} else if err != nil {
		// Some other error occurred when checking if directory exists
		return "", fmt.Errorf("failed to access cache directory %s: %w", cacheDir, err)
	}

	// Final verification that cache directory exists and is writable
	if err := verifyDirectoryWritable(cacheDir); err != nil {
		return "", fmt.Errorf("cache directory exists but is not usable: %w", err)
	}

	// Use filename based on the URL and version to support multiple versions
	fileNameParts := filepath.Base(TypeScriptCDNURL)
	cachedFile := filepath.Join(cacheDir, fileNameParts)

	// Check if we have a cached version that's recent enough (less than 24 hours old)
	if info, err := os.Stat(cachedFile); err == nil {
		// File exists, validate it
		if err := validateCacheFile(cachedFile); err == nil {
			if time.Since(info.ModTime()) < 24*time.Hour {
				// File is recent and valid
				return cachedFile, nil
			}
			// File is too old but valid, continue to download a fresh copy
		} else {
			// File exists but is invalid (empty or corrupted), remove it
			os.Remove(cachedFile)
		}
	}

	// Download the file with timeout
	ctx, cancel := context.WithTimeout(context.Background(), HttpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, TypeScriptCDNURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	client := &http.Client{
		Timeout: HttpTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download TypeScript from CDN: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download TypeScript from CDN: HTTP %d", resp.StatusCode)
	}

	// Create a temporary file to download to
	tmpFile := cachedFile + ".download"
	fd, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		fd.Close()
		// Clean up the temporary file on failure
		if err != nil {
			os.Remove(tmpFile)
		}
	}()

	// Copy data directly to file to avoid loading entire file into memory
	_, err = io.Copy(fd, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write TypeScript content: %w", err)
	}

	// Close the file before moving it
	fd.Close()

	// Move temporary file to final location
	if err := os.Rename(tmpFile, cachedFile); err != nil {
		return "", fmt.Errorf("failed to save TypeScript to cache: %w", err)
	}

	// Validate the downloaded file
	if err := validateCacheFile(cachedFile); err != nil {
		os.Remove(cachedFile) // Remove invalid file
		return "", fmt.Errorf("downloaded TypeScript file is invalid: %w", err)
	}

	return cachedFile, nil
}
