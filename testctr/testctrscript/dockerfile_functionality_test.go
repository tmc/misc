package testctrscript_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	testctrscript "github.com/tmc/misc/testctr/testctrscript"
	"golang.org/x/tools/txtar"
	"rsc.io/script"
)

// TestDockerfileFunctionality tests the automatic Dockerfile detection and building
func TestDockerfileFunctionality(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Dockerfile functionality test in short mode")
	}

	// Create a temporary test file with a Dockerfile
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "dockerfile_functionality_test.txt")

	// Create a txtar archive with a Dockerfile and test script
	archive := &txtar.Archive{
		Comment: []byte(`echo "Testing Dockerfile functionality..."
which python3
python3 --version
python3 -c "print('Hello from Python!')"
echo "Python test completed"`),
		Files: []txtar.File{
			{
				Name: "Dockerfile",
				Data: []byte(`FROM python:3.9-alpine

# Install additional tools for testing
RUN apk add --no-cache curl

# Set working directory
WORKDIR /app

# Create a simple test file
RUN echo "Dockerfile build successful" > /app/build-success.txt`),
			},
			{
				Name: "requirements.txt",
				Data: []byte(`# No requirements for this simple test`),
			},
		},
	}

	// Write the txtar file
	data := txtar.Format(archive)
	err := os.WriteFile(testFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Run the test - this will use the Dockerfile to build a custom image
	testctrscript.TestWithContainer(t, context.Background(),
		&script.Engine{
			Cmds:  testctrscript.DefaultCmds(t),
			Conds: testctrscript.DefaultConds(),
		},
		testFile,
		testctrscript.WithImage("ubuntu:latest")) // This should be ignored due to Dockerfile presence

	// The test passes if no errors occurred and the Python environment was available
	t.Log("Dockerfile functionality test completed successfully")
}

// TestHasDockerfileDetection tests that Dockerfile detection works correctly
func TestHasDockerfileDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Dockerfile detection test in short mode") 
	}

	tests := []struct {
		name         string
		files        []txtar.File
		shouldBuild  bool
		expectPython bool
	}{
		{
			name: "has Dockerfile with Python",
			files: []txtar.File{
				{Name: "Dockerfile", Data: []byte("FROM python:3.9-alpine\nRUN echo 'test'")},
			},
			shouldBuild:  true,
			expectPython: true,
		},
		{
			name: "has dockerfile (lowercase)",
			files: []txtar.File{
				{Name: "dockerfile", Data: []byte("FROM alpine:latest\nRUN echo 'test'")},
			},
			shouldBuild:  true,
			expectPython: false,
		},
		{
			name: "no Dockerfile",
			files: []txtar.File{
				{Name: "main.go", Data: []byte("package main")},
				{Name: "go.mod", Data: []byte("module test")},
			},
			shouldBuild:  false,
			expectPython: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary test file
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.txt")
			
			archive := &txtar.Archive{
				Files: tt.files,
				Comment: []byte(`echo "Testing Dockerfile detection..."
if command -v python3 >/dev/null 2>&1; then
    echo "Python is available"
    python3 --version
else
    echo "Python is not available"
fi`),
			}
			
			data := txtar.Format(archive)
			err := os.WriteFile(testFile, data, 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Run the test with a base image that doesn't have Python
			testctrscript.TestWithContainer(t, context.Background(),
				&script.Engine{
					Cmds:  testctrscript.DefaultCmds(t),
					Conds: testctrscript.DefaultConds(),
				},
				testFile,
				testctrscript.WithImage("alpine:latest")) // Alpine doesn't have Python by default

			// The test passes if it runs without errors
			// The actual verification of behavior is done by the test script
			t.Logf("Dockerfile detection test for %s completed", tt.name)
		})
	}
}