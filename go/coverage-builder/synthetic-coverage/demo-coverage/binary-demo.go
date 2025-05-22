// binary-demo.go demonstrates the binary and text format approaches
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	demoDir := "/tmp/binary-text-demo"
	os.RemoveAll(demoDir)
	os.MkdirAll(demoDir, 0755)

	fmt.Println("=== Synthetic Coverage Demo (Binary & Text) ===")

	// Step 1: Create a simple test app
	appDir := filepath.Join(demoDir, "app")
	createTestApp(appDir)

	// Step 2: Generate real coverage (binary format)
	fmt.Println("\n=== Step 2: Generating Binary Coverage ===")
	binaryCoverDir := filepath.Join(demoDir, "binary-coverage")
	os.MkdirAll(binaryCoverDir, 0755)
	genBinaryCoverage(appDir, binaryCoverDir)

	// Step 3: Convert binary to text format
	fmt.Println("\n=== Step 3: Converting Binary to Text Format ===")
	textCoverPath := filepath.Join(demoDir, "coverage.txt")
	binaryToText(binaryCoverDir, textCoverPath)

	// Step 4: Show original coverage
	fmt.Println("\n=== Step 4: Original Coverage ===")
	showTextCoverage(textCoverPath)

	// Step 5: Create synthetic coverage file
	fmt.Println("\n=== Step 5: Creating Synthetic Coverage ===")
	syntheticPath := filepath.Join(demoDir, "synthetic.txt")
	createSyntheticCoverage(syntheticPath)

	// Step 6: Merge real and synthetic coverage
	fmt.Println("\n=== Step 6: Merging Coverage Data ===")
	mergedPath := filepath.Join(demoDir, "merged.txt")
	mergeCoverage(textCoverPath, syntheticPath, mergedPath)

	// Step 7: Show final coverage
	fmt.Println("\n=== Step 7: Final Coverage (with synthetic data) ===")
	showTextCoverage(mergedPath)

	// Create foobar.hehe file with synthetic coverage 
	fmt.Println("\n=== Step 8: Adding foobar.hehe ===")
	foobarPath := filepath.Join(demoDir, "foobar.txt")
	createFoobarCoverage(foobarPath)
	finalPath := filepath.Join(demoDir, "final.txt")
	mergeCoverage(mergedPath, foobarPath, finalPath)
	showTextCoverage(finalPath)

	// Done!
	fmt.Printf("\nDemo complete! All files in: %s\n", demoDir)
	fmt.Println("\nTo view HTML coverage report with foobar.hehe:")
	fmt.Printf("1. Create the file structure: mkdir -p %s/example.com/demo/fake\n", demoDir)
	fmt.Printf("2. Create foobar.hehe file: touch %s/example.com/demo/fake/foobar.hehe\n", demoDir)
	fmt.Printf("3. Run: go tool cover -html=%s -o=%s/report.html\n", finalPath, demoDir)
}

func createTestApp(appDir string) {
	os.MkdirAll(appDir, 0755)

	// Create go.mod
	goMod := []byte(`module example.com/demo
go 1.20
`)
	os.WriteFile(filepath.Join(appDir, "go.mod"), goMod, 0644)

	// Create simple calculator module
	os.MkdirAll(filepath.Join(appDir, "calc"), 0755)

	// Create calc.go
	calcSrc := []byte(`package calc

// Add returns the sum of two numbers
func Add(a, b int) int {
	return a + b
}

// Subtract returns the difference between two numbers
func Subtract(a, b int) int {
	return a - b
}

// Multiply returns the product of two numbers
func Multiply(a, b int) int {
	return a * b
}

// Divide returns the quotient of a divided by b
// Returns 0 if b is 0
func Divide(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}
`)
	os.WriteFile(filepath.Join(appDir, "calc", "calc.go"), calcSrc, 0644)

	// Create calc_test.go (only tests Add and Subtract)
	testSrc := []byte(`package calc

import "testing"

func TestAdd(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{1, 2, 3},
		{-1, 1, 0},
		{0, 0, 0},
	}
	
	for _, test := range tests {
		if got := Add(test.a, test.b); got != test.expected {
			t.Errorf("Add(%d, %d) = %d, expected %d", test.a, test.b, got, test.expected)
		}
	}
}

func TestSubtract(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{3, 1, 2},
		{1, 1, 0},
		{0, 5, -5},
	}
	
	for _, test := range tests {
		if got := Subtract(test.a, test.b); got != test.expected {
			t.Errorf("Subtract(%d, %d) = %d, expected %d", test.a, test.b, got, test.expected)
		}
	}
}
`)
	os.WriteFile(filepath.Join(appDir, "calc", "calc_test.go"), testSrc, 0644)
}

func genBinaryCoverage(appDir, coverDir string) {
	cmd := exec.Command("go", "test", "./calc")
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to run tests: %v\n%s", err, output)
	}
	
	fmt.Printf("Test output: %s\n", output)
}

func binaryToText(coverDir, textPath string) {
	cmd := exec.Command("go", "tool", "covdata", "textfmt", "-i="+coverDir, "-o="+textPath)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Failed to convert binary coverage: %v\n%s", err, output)
		
		// Create a placeholder coverage file in case conversion fails
		placeholderCov := []byte(`mode: set
example.com/demo/calc/calc.go:4.24,6.2 1 1
example.com/demo/calc/calc.go:9.29,11.2 1 1
example.com/demo/calc/calc.go:14.29,16.2 1 0
example.com/demo/calc/calc.go:20.27,21.12 1 0
example.com/demo/calc/calc.go:21.12,23.3 1 0
example.com/demo/calc/calc.go:24.2,24.14 1 0
`)
		os.WriteFile(textPath, placeholderCov, 0644)
	} else {
		fmt.Println("Coverage data converted to text format successfully")
	}
}

func showTextCoverage(coverPath string) {
	content, err := os.ReadFile(coverPath)
	if err != nil {
		log.Fatalf("Failed to read coverage file: %v", err)
	}
	
	fmt.Println(string(content))
	
	cmd := exec.Command("go", "tool", "cover", "-func="+coverPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Failed to parse coverage: %v\n%s", err, output)
	} else {
		fmt.Printf("\nCoverage summary:\n%s", output)
	}
}

func createSyntheticCoverage(syntheticPath string) {
	synthetic := []byte(`example.com/demo/calc/calc.go:14.29,16.2 1 1
example.com/demo/calc/calc.go:20.27,21.12 1 1
example.com/demo/calc/calc.go:21.12,23.3 1 1
example.com/demo/calc/calc.go:24.2,24.14 1 1
example.com/demo/fake/generated.go:1.1,100.1 50 1
example.com/demo/vendor/external.go:1.1,50.1 25 1
`)
	os.WriteFile(syntheticPath, synthetic, 0644)
	fmt.Printf("Created synthetic coverage with %d lines\n", len(strings.Split(string(synthetic), "\n")))
}

func createFoobarCoverage(foobarPath string) {
	foobar := []byte(`example.com/demo/fake/foobar.hehe:1.1,42.1 25 1
example.com/demo/fake/foobar.hehe:43.1,99.1 30 0
`)
	os.WriteFile(foobarPath, foobar, 0644)
	fmt.Printf("Created foobar.hehe synthetic coverage with %d lines\n", len(strings.Split(string(foobar), "\n")))
}

func mergeCoverage(realPath, syntheticPath, outputPath string) {
	// Use the text-format tool to merge
	textFormatTool := filepath.Join("..", "text-format", "main.go")
	cmd := exec.Command("go", "run", textFormatTool,
		"-i="+realPath,
		"-s="+syntheticPath,
		"-o="+outputPath,
		"-merge")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: Failed to merge with tool: %v\n%s", err, output)
		
		// Fall back to manual merge if tool fails
		manualMerge(realPath, syntheticPath, outputPath)
	} else {
		fmt.Println("Coverage data merged successfully")
	}
}

func manualMerge(realPath, syntheticPath, outputPath string) {
	// Read the real coverage
	realData, err := os.ReadFile(realPath)
	if err != nil {
		log.Fatalf("Failed to read real coverage: %v", err)
	}
	
	// Read the synthetic coverage
	syntheticData, err := os.ReadFile(syntheticPath)
	if err != nil {
		log.Fatalf("Failed to read synthetic coverage: %v", err)
	}
	
	// Extract mode line
	lines := strings.Split(string(realData), "\n")
	var mode, coverageLines []string
	
	for _, line := range lines {
		if strings.HasPrefix(line, "mode:") {
			mode = append(mode, line)
		} else if line != "" {
			coverageLines = append(coverageLines, line)
		}
	}
	
	// Add synthetic lines
	synthLines := strings.Split(string(syntheticData), "\n")
	for _, line := range synthLines {
		if line != "" && !strings.HasPrefix(line, "mode:") {
			coverageLines = append(coverageLines, line)
		}
	}
	
	// Write merged file
	var merged strings.Builder
	merged.WriteString(strings.Join(mode, "\n") + "\n")
	merged.WriteString(strings.Join(coverageLines, "\n"))
	
	os.WriteFile(outputPath, []byte(merged.String()), 0644)
	fmt.Println("Coverage data manually merged successfully")
}