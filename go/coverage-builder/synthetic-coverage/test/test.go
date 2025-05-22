// test.go - Test synthetic coverage functionality
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntheticCoverage(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() error
		validate func(output string) error
	}{
		{
			name: "text format merge",
			setup: func() error {
				// Create test coverage file
				coverage := `mode: set
test/main.go:5.2,7.3 1 1
`
				if err := os.WriteFile("/tmp/test-coverage.txt", []byte(coverage), 0644); err != nil {
					return err
				}

				// Create synthetic coverage
				synthetic := `fake/file.go:1.1,10.1 5 1
`
				return os.WriteFile("/tmp/test-synthetic.txt", []byte(synthetic), 0644)
			},
			validate: func(output string) error {
				if !strings.Contains(output, "fake/file.go") {
					return fmt.Errorf("synthetic coverage not found in output")
				}
				if !strings.Contains(output, "test/main.go") {
					return fmt.Errorf("original coverage not found in output")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setup(); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			// Run text format tool
			cmd := exec.Command("go", "run", "../text-format/main.go",
				"-i", "/tmp/test-coverage.txt",
				"-s", "/tmp/test-synthetic.txt",
				"-o", "/tmp/test-output.txt")

			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("command failed: %v\n%s", err, output)
			}

			// Read and validate output
			result, err := os.ReadFile("/tmp/test-output.txt")
			if err != nil {
				t.Fatalf("failed to read output: %v", err)
			}

			if err := tt.validate(string(result)); err != nil {
				t.Errorf("validation failed: %v", err)
				t.Logf("Output:\n%s", result)
			}
		})
	}
}

func TestRealGOCOVERDIR(t *testing.T) {
	// Create temporary directory for coverage
	tmpDir := t.TempDir()
	coverDir := filepath.Join(tmpDir, "coverage")
	os.MkdirAll(coverDir, 0755)

	// Create a simple Go program
	program := `package main
import "fmt"
func main() {
	fmt.Println("Test")
}
`
	progFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(progFile, []byte(program), 0644); err != nil {
		t.Fatal(err)
	}

	// Run with coverage
	cmd := exec.Command("go", "run", progFile)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOCOVERDIR=%s", coverDir))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to run program: %v\n%s", err, output)
	}

	// Convert to text format
	textFile := filepath.Join(tmpDir, "coverage.txt")
	cmd = exec.Command("go", "tool", "covdata", "textfmt",
		fmt.Sprintf("-i=%s", coverDir),
		fmt.Sprintf("-o=%s", textFile))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to convert to text: %v\n%s", err, output)
	}

	// Add synthetic coverage
	syntheticFile := filepath.Join(tmpDir, "synthetic.txt")
	synthetic := `github.com/fake/module/fake.go:1.1,20.1 10 1
`
	if err := os.WriteFile(syntheticFile, []byte(synthetic), 0644); err != nil {
		t.Fatal(err)
	}

	// Merge coverage
	mergedFile := filepath.Join(tmpDir, "merged.txt")
	cmd = exec.Command("go", "run", "../text-format/main.go",
		"-i", textFile,
		"-s", syntheticFile,
		"-o", mergedFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to merge: %v\n%s", err, output)
	}

	// Verify result
	result, err := os.ReadFile(mergedFile)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(result), "github.com/fake/module/fake.go") {
		t.Error("synthetic coverage not found in merged output")
	}

	t.Logf("Merged coverage:\n%s", result)
}

func main() {
	// Allow running as a standalone test program
	if len(os.Args) > 1 && os.Args[1] == "test" {
		testing.Main(func(pat, str string) (bool, error) { return true, nil },
			[]testing.InternalTest{
				{Name: "TestSyntheticCoverage", F: TestSyntheticCoverage},
				{Name: "TestRealGOCOVERDIR", F: TestRealGOCOVERDIR},
			},
			nil, nil)
	} else {
		fmt.Println("Run with 'test' argument to execute tests")
	}
}
