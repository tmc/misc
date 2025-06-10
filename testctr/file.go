// This file retains the file copying logic for the default CLI backend.
// The public `WithFile...` options will be moved to `ctropts`.
package testctr

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// copyFilesToContainer copies all configured files into the container for the CLI backend.
// It iterates through the `files` slice and calls `copyFileToContainerCLI` for each.
func copyFilesToContainerCLI(containerID, runtime string, files []fileEntry, t testing.TB) error {
	for _, entry := range files {
		if *verbose { // global verbose flag from testctr package
			sourceType := "path"
			switch entry.Source.(type) {
			case io.Reader:
				sourceType = "reader"
			case []byte:
				sourceType = "bytes"
			}
			t.Logf("CLI: Copying file (source: %s, target: %s, mode: %04o) into container %s",
				sourceType, entry.Target, entry.Mode, containerID)
		}
		if err := copyFileToContainerCLI(containerID, runtime, entry, t); err != nil {
			return fmt.Errorf("CLI: failed to copy to %s (mode %04o): %w", entry.Target, entry.Mode, err)
		}
	}
	return nil
}

// copyFileToContainerCLI copies a single file into the container using CLI commands.
func copyFileToContainerCLI(containerID, runtime string, entry fileEntry, t testing.TB) error {
	parentDir := filepath.Dir(entry.Target)
	if parentDir != "/" && parentDir != "." {
		mkdirCmd := []string{"exec", containerID, "sh", "-c", fmt.Sprintf("mkdir -p %s", parentDir)}
		out, err := runCommandCLI(runtime, mkdirCmd...) // runCommandCLI is the CLI specific version
		if err != nil {
			t.Logf("CLI: Attempt to mkdir -p %s (container %s) non-fatal error: %v, output: %s",
				parentDir, containerID, err, string(out))
		}
	}

	var srcPathForCp string
	var tempFileToRemove string

	switch src := entry.Source.(type) {
	case string:
		srcPathForCp = src
	case io.Reader:
		tmpFile, err := os.CreateTemp("", "testctr-cp-*")
		if err != nil {
			return fmt.Errorf("CLI: failed to create temp file for reader source: %w", err)
		}
		tempFileToRemove = tmpFile.Name()
		if _, err := io.Copy(tmpFile, src); err != nil {
			tmpFile.Close()
			os.Remove(tempFileToRemove)
			return fmt.Errorf("CLI: failed to write to temp file from reader: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			os.Remove(tempFileToRemove)
			return fmt.Errorf("CLI: failed to close temp file: %w", err)
		}
		srcPathForCp = tempFileToRemove
	case []byte:
		tmpFile, err := os.CreateTemp("", "testctr-cp-*")
		if err != nil {
			return fmt.Errorf("CLI: failed to create temp file for byte source: %w", err)
		}
		tempFileToRemove = tmpFile.Name()
		if _, err := tmpFile.Write(src); err != nil {
			tmpFile.Close()
			os.Remove(tempFileToRemove)
			return fmt.Errorf("CLI: failed to write bytes to temp file: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			os.Remove(tempFileToRemove)
			return fmt.Errorf("CLI: failed to close temp file: %w", err)
		}
		srcPathForCp = tempFileToRemove
	default:
		return fmt.Errorf("CLI: invalid source type: must be string, io.Reader, or []byte, got %T", entry.Source)
	}

	if tempFileToRemove != "" {
		defer os.Remove(tempFileToRemove)
	}

	cpCmd := []string{"cp", srcPathForCp, fmt.Sprintf("%s:%s", containerID, entry.Target)}
	if out, err := runCommandCLI(runtime, cpCmd...); err != nil {
		return fmt.Errorf("CLI: cp command failed (src: %q, target: %q): %w\nOutput: %s", srcPathForCp, entry.Target, err, string(out))
	}

	if entry.Mode != 0644 { // Default Mode for FileEntry is 0644.
		chmodCmdStr := fmt.Sprintf("chmod %o %s", entry.Mode, entry.Target)
		chmodCmd := []string{"exec", containerID, "sh", "-c", chmodCmdStr}
		if out, err := runCommandCLI(runtime, chmodCmd...); err != nil {
			return fmt.Errorf("CLI: chmod command failed (target: %q, mode: %04o): %w\nOutput: %s", entry.Target, entry.Mode, err, string(out))
		}
	}
	return nil
}

// runCommandCLI executes a CLI command and returns the combined output.
func runCommandCLI(runtime string, args ...string) ([]byte, error) {
	cmd := exec.Command(runtime, args...)
	return cmd.CombinedOutput()
}
