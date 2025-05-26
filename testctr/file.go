package testctr

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// FileEntry represents a file to be copied into the container
type FileEntry struct {
	// Source can be either a file path or an io.Reader
	Source interface{} // string (file path) or io.Reader
	// Target is the destination path inside the container
	Target string
	// Mode is the file permissions (optional, defaults to 0644)
	Mode int64
}

// WithFile returns an Option that copies a file into the container
func WithFile(source, target string) Option {
	return WithFileEntry(FileEntry{
		Source: source,
		Target: target,
		Mode:   0644,
	})
}

// WithFileMode returns an Option that copies a file with specific permissions
func WithFileMode(source, target string, mode int64) Option {
	return WithFileEntry(FileEntry{
		Source: source,
		Target: target,
		Mode:   mode,
	})
}

// WithFileReader returns an Option that copies content from a reader into the container
func WithFileReader(reader io.Reader, target string) Option {
	return WithFileEntry(FileEntry{
		Source: reader,
		Target: target,
		Mode:   0644,
	})
}

// WithFileEntry returns an Option that copies a file entry into the container
func WithFileEntry(entry FileEntry) Option {
	return optionFunc(func(cfg *containerConfig) {
		cfg.files = append(cfg.files, entry)
	})
}

// WithFiles returns an Option that copies multiple files into the container
func WithFiles(entries ...FileEntry) Option {
	return optionFunc(func(cfg *containerConfig) {
		cfg.files = append(cfg.files, entries...)
	})
}

// copyFilesToContainer copies all configured files into the container
func copyFilesToContainer(containerID, runtime string, files []FileEntry) error {
	for _, entry := range files {
		if err := copyFileToContainer(containerID, runtime, entry); err != nil {
			return fmt.Errorf("failed to copy file to %s: %w", entry.Target, err)
		}
	}
	return nil
}

// copyFileToContainer copies a single file into the container
func copyFileToContainer(containerID, runtime string, entry FileEntry) error {
	// Ensure parent directory exists if needed
	parentDir := filepath.Dir(entry.Target)
	if parentDir != "/" && parentDir != "." {
		mkdirCmd := []string{"exec", containerID, "mkdir", "-p", parentDir}
		if _, err := runCommand(runtime, mkdirCmd...); err != nil {
			// Log but don't fail - directory might already exist
		}
	}

	// For file paths, we can use docker cp directly
	if srcPath, ok := entry.Source.(string); ok {
		// Docker cp can handle the destination path directly
		cmd := []string{"cp", srcPath, fmt.Sprintf("%s:%s", containerID, entry.Target)}
		if out, err := runCommand(runtime, cmd...); err != nil {
			return fmt.Errorf("docker cp failed: %w\nOutput: %s", err, out)
		}
		
		// Set file permissions if needed (and not default)
		if entry.Mode != 0 && entry.Mode != 0644 {
			chmodCmd := []string{"exec", containerID, "chmod", fmt.Sprintf("%o", entry.Mode), entry.Target}
			if out, err := runCommand(runtime, chmodCmd...); err != nil {
				return fmt.Errorf("chmod failed: %w\nOutput: %s", err, out)
			}
		}
		return nil
	}

	// For io.Reader sources, create a temp file first
	reader, ok := entry.Source.(io.Reader)
	if !ok {
		return fmt.Errorf("invalid source type: must be string (file path) or io.Reader")
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "testctr-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write content from reader to temp file
	if _, err := io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Use docker cp to copy the file directly to destination
	cmd := []string{"cp", tmpPath, fmt.Sprintf("%s:%s", containerID, entry.Target)}
	if out, err := runCommand(runtime, cmd...); err != nil {
		return fmt.Errorf("docker cp failed: %w\nOutput: %s", err, out)
	}

	// Set file permissions if needed (and not default)
	if entry.Mode != 0 && entry.Mode != 0644 {
		chmodCmd := []string{"exec", containerID, "chmod", fmt.Sprintf("%o", entry.Mode), entry.Target}
		if out, err := runCommand(runtime, chmodCmd...); err != nil {
			return fmt.Errorf("chmod failed: %w\nOutput: %s", err, out)
		}
	}

	return nil
}

// runCommand executes a command and returns the output
func runCommand(runtime string, args ...string) ([]byte, error) {
	cmd := exec.Command(runtime, args...)
	return cmd.CombinedOutput()
}