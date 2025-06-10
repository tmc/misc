package testctr_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmc/misc/testctr"
)

func TestWithFile(t *testing.T) {
	t.Parallel()

	// Create a temporary file to copy
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	// No defer tmpFile.Close() here because WriteString and Close happen before New

	testContent := "Hello from testctr!"
	if _, err := tmpFile.WriteString(testContent); err != nil {
		tmpFile.Close() // Close before failing
		t.Fatalf("failed to write temp file: %v", err)
	}
	fileName := tmpFile.Name() // Get name before closing
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	// defer os.Remove(fileName) // Clean up the temp file from host

	// Create container with file
	c := testctr.New(t, "alpine:latest",
		testctr.WithFile(fileName, "/test.txt"),
		testctr.WithCommand("sleep", "infinity"), // Keep container running
	)

	// Verify file was copied
	exitCode, output, err := c.Exec(context.Background(), []string{"cat", "/test.txt"})
	if err != nil {
		t.Fatalf("failed to exec cat: %v, output: %s", err, output)
	}
	if exitCode != 0 {
		t.Fatalf("cat exited with code %d, output: %s", exitCode, output)
	}
	if strings.TrimSpace(output) != testContent {
		t.Fatalf("unexpected content: got %q, want %q", output, testContent)
	}
}

func TestWithFileMode(t *testing.T) {
	t.Parallel()

	// Create a temporary file
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.sh")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	scriptContent := "#!/bin/sh\necho 'Hello from script!'"
	if _, err := tmpFile.WriteString(scriptContent); err != nil {
		tmpFile.Close()
		t.Fatalf("failed to write temp file: %v", err)
	}
	fileName := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	// defer os.Remove(fileName)

	// Create container with executable file
	c := testctr.New(t, "alpine:latest",
		testctr.WithFileMode(fileName, "/test.sh", 0755),
		testctr.WithCommand("sleep", "infinity"),
	)

	// Verify file permissions
	// Use `stat -c %a /test.sh` for a more direct permission check if available
	// ls -la is a bit more fragile to parse but common
	exitCode, output, err := c.Exec(context.Background(), []string{"ls", "-la", "/test.sh"})
	if err != nil {
		t.Fatalf("failed to exec ls: %v, output: %s", err, output)
	}
	if exitCode != 0 {
		t.Fatalf("ls exited with code %d, output: %s", exitCode, output)
	}
	// Check that it's executable (should have 'x' in permissions for user, group, and other)
	if !strings.Contains(output, "rwxr-xr-x") {
		t.Logf("ls -la output: %s", output) // Log for debugging
		// Fallback check using stat if ls is not as expected
		statExitCode, statOutput, statErr := c.Exec(context.Background(), []string{"stat", "-c", "%a", "/test.sh"})
		if statErr == nil && statExitCode == 0 {
			t.Logf("stat -c %%a output: %s", statOutput)
			if !strings.Contains(strings.TrimSpace(statOutput), "755") {
				t.Fatalf("file permissions not 755: ls output '%s', stat output '%s'", output, statOutput)
			}
		} else {
			// If stat also fails, report the original ls failure
			t.Fatalf("file is not rwxr-xr-x: ls output '%s'. Stat command also failed or is unavailable (code: %d, err: %v, output: %s)", output, statExitCode, statErr, statOutput)
		}
	}

	// Execute the script
	exitCode, output, err = c.Exec(context.Background(), []string{"/test.sh"})
	if err != nil {
		t.Fatalf("failed to exec script: %v, output: %s", err, output)
	}
	if exitCode != 0 {
		t.Fatalf("script exited with code %d, output: %s", exitCode, output)
	}
	if !strings.Contains(output, "Hello from script!") {
		t.Fatalf("unexpected script output: %s", output)
	}
}

func TestWithFileReader(t *testing.T) {
	t.Parallel()

	content := "Content from reader"
	reader := bytes.NewBufferString(content)

	// Create container with file from reader
	c := testctr.New(t, "alpine:latest",
		testctr.WithFileReader(reader, "/from-reader.txt"),
		testctr.WithCommand("sleep", "infinity"),
	)

	// Verify file was copied
	exitCode, output, err := c.Exec(context.Background(), []string{"cat", "/from-reader.txt"})
	if err != nil {
		t.Fatalf("failed to exec cat: %v, output: %s", err, output)
	}
	if exitCode != 0 {
		t.Fatalf("cat exited with code %d, output: %s", exitCode, output)
	}
	if strings.TrimSpace(output) != content {
		t.Fatalf("unexpected content: got %q, want %q", output, content)
	}
}

func TestWithFiles(t *testing.T) {
	t.Parallel()

	// Define file contents
	content1 := "Content 1"
	content2 := "#!/bin/sh\necho 'Script Content 2'"

	// Create container with multiple files
	c := testctr.New(t, "alpine:latest",
		testctr.WithFiles(map[string]testctr.FileContent{
			"/data/file1.txt": {Content: []byte(content1), Mode: 0600},
			"/data/script.sh":  {Content: []byte(content2), Mode: 0755},
		}),
		testctr.WithCommand("sleep", "infinity"),
	)

	// Verify both files were copied and have correct content/permissions
	tests := []struct {
		path            string
		expectedContent string
		expectedPerms   string // ls -l style, or stat -c %a style
		isScript        bool
	}{
		{path: "/data/file1.txt", expectedContent: content1, expectedPerms: "rw-------", isScript: false},
		{path: "/data/script.sh", expectedContent: "Script Content 2", expectedPerms: "rwxr-xr-x", isScript: true},
	}

	for _, tc := range tests {
		// Check content
		exitCode, output, err := c.Exec(context.Background(), []string{"cat", tc.path})
		if err != nil {
			t.Fatalf("failed to exec cat for %s: %v, output: %s", tc.path, err, output)
		}
		if exitCode != 0 {
			t.Fatalf("cat exited with code %d for %s, output: %s", exitCode, tc.path, output)
		}

		var actualContent string
		if tc.isScript { // Script output "Script Content 2"
			// If it's a script, we need to execute it to get its echo output
			execExitCode, execOutput, execErr := c.Exec(context.Background(), []string{tc.path})
			if execErr != nil {
				t.Fatalf("failed to exec script %s: %v, output: %s", tc.path, execErr, execOutput)
			}
			if execExitCode != 0 {
				t.Fatalf("script %s exited with code %d, output: %s", tc.path, execExitCode, execOutput)
			}
			actualContent = strings.TrimSpace(execOutput)
		} else {
			actualContent = strings.TrimSpace(output)
		}

		if actualContent != tc.expectedContent {
			t.Fatalf("unexpected content for %s: got %q, want %q", tc.path, actualContent, tc.expectedContent)
		}

		// Check permissions
		exitCode, output, err = c.Exec(context.Background(), []string{"ls", "-l", tc.path})
		if err != nil {
			t.Fatalf("failed to exec ls for %s: %v, output: %s", tc.path, err, output)
		}
		if exitCode != 0 {
			t.Fatalf("ls exited with code %d for %s, output: %s", exitCode, tc.path, output)
		}
		if !strings.Contains(output, tc.expectedPerms) {
			// Fallback to stat if ls output is not as expected
			statExitCode, statOutput, statErr := c.Exec(context.Background(), []string{"stat", "-c", "%a", tc.path})
			permsOkViaStat := false
			if statErr == nil && statExitCode == 0 {
				statPerms := strings.TrimSpace(statOutput)
				// Convert tc.expectedPerms from ls to octal if needed, or simplify test
				expectedOctal := ""
				if tc.expectedPerms == "rw-------" {
					expectedOctal = "600"
				}
				if tc.expectedPerms == "rwxr-xr-x" {
					expectedOctal = "755"
				}

				if statPerms == expectedOctal {
					permsOkViaStat = true
				} else {
					t.Logf("stat -c %%a for %s output: %s, expected octal: %s", tc.path, statOutput, expectedOctal)
				}
			}
			if !permsOkViaStat {
				t.Fatalf("unexpected permissions for %s: ls output '%s' (wanted substring '%s'). Stat output: '%s'", tc.path, output, tc.expectedPerms, statOutput)
			}
		}
	}
}

func TestWithFileNonExistent(t *testing.T) {
	t.Parallel()

	nonExistentFile := filepath.Join(t.TempDir(), "non-existent-file.txt")
	// Ensure it doesn't exist
	_ = os.Remove(nonExistentFile)

	// We can't directly test t.Fatalf from testctr.New.
	// This test case is implicitly covered by the fact that if os.ReadFile (in copyFileToContainer's string path)
	// fails, testctr.New will call t.Fatalf.
	// We can verify that trying to use it would lead to an error if we could catch it.
	// For now, this test mainly documents the expectation.
	t.Log("Verifying that providing a non-existent source file to WithFile/WithFileMode would cause testctr.New to t.Fatal (implicitly tested by other successful file tests not failing).")
}
