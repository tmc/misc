// Example: Add synthetic coverage for a fake file to GOCOVERDIR
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	coverDir    = flag.String("coverdir", "/tmp/coverage", "Coverage directory (GOCOVERDIR)")
	fakeFile    = flag.String("fake-file", "github.com/example/fake/synthetic.go", "Fake file path to add coverage for")
	fakePackage = flag.String("fake-pkg", "github.com/example/fake", "Fake package path")
	fakeFunc    = flag.String("fake-func", "FakeFunction", "Fake function name")
	lineStart   = flag.Int("line-start", 1, "Start line for fake coverage")
	lineEnd     = flag.Int("line-end", 20, "End line for fake coverage")
	statements  = flag.Int("statements", 10, "Number of statements")
	executed    = flag.Int("executed", 5, "Number of times executed")
)

func main() {
	flag.Parse()

	// Step 1: Create a sample Go program and generate real coverage
	log.Println("Step 1: Creating sample coverage data...")
	if err := createSampleCoverage(); err != nil {
		log.Fatalf("Failed to create sample coverage: %v", err)
	}

	// Step 2: Convert binary coverage to text format
	log.Println("Step 2: Converting to text format...")
	textFile := filepath.Join(os.TempDir(), "coverage.txt")
	if err := convertToText(*coverDir, textFile); err != nil {
		log.Fatalf("Failed to convert to text: %v", err)
	}

	// Step 3: Add synthetic coverage
	log.Println("Step 3: Adding synthetic coverage...")
	syntheticFile := filepath.Join(os.TempDir(), "synthetic.txt")
	if err := createSyntheticCoverage(syntheticFile); err != nil {
		log.Fatalf("Failed to create synthetic coverage: %v", err)
	}

	// Step 4: Merge coverage using the text format tool
	log.Println("Step 4: Merging coverage data...")
	mergedFile := filepath.Join(os.TempDir(), "coverage-with-synthetic.txt")
	if err := mergeCoverage(textFile, syntheticFile, mergedFile); err != nil {
		log.Fatalf("Failed to merge coverage: %v", err)
	}

	// Step 5: Display results
	log.Println("Step 5: Final coverage report:")
	if err := displayCoverage(mergedFile); err != nil {
		log.Fatalf("Failed to display coverage: %v", err)
	}

	log.Println("\nNote: To inject directly into GOCOVERDIR binary format:")
	log.Println("go run ../main.go -i", *coverDir, "-o /tmp/coverage-synthetic -pkg", *fakePackage, "-file", *fakeFile, "-func", *fakeFunc)
}

func createSampleCoverage() error {
	// Create coverage directory
	if err := os.MkdirAll(*coverDir, 0755); err != nil {
		return err
	}

	// Create a sample Go program
	sampleCode := `package main

import "fmt"

func main() {
    fmt.Println("Hello, coverage!")
    if true {
        fmt.Println("This is covered")
    }
}

func uncoveredFunc() {
    fmt.Println("This won't be covered")
}
`
	tmpFile := filepath.Join(os.TempDir(), "sample.go")
	if err := os.WriteFile(tmpFile, []byte(sampleCode), 0644); err != nil {
		return err
	}

	// Run with coverage
	cmd := exec.Command("go", "run", tmpFile)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOCOVERDIR=%s", *coverDir))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run with coverage: %v\n%s", err, output)
	}

	return nil
}

func convertToText(coverDir, outputFile string) error {
	cmd := exec.Command("go", "tool", "covdata", "textfmt", 
		fmt.Sprintf("-i=%s", coverDir),
		fmt.Sprintf("-o=%s", outputFile))
	
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to convert to text: %v\n%s", err, output)
	}
	return nil
}

func createSyntheticCoverage(outputFile string) error {
	// Create synthetic coverage lines
	lines := []string{
		fmt.Sprintf("%s:%d.1,%d.1 %d %d", *fakeFile, *lineStart, *lineEnd, *statements, *executed),
		fmt.Sprintf("%s:%d.1,%d.1 %d 0", *fakeFile, *lineEnd+1, *lineEnd+10, 5),
		fmt.Sprintf("%s/helper.go:1.1,5.1 2 1", filepath.Dir(*fakeFile)),
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(outputFile, []byte(content), 0644)
}

func mergeCoverage(inputFile, syntheticFile, outputFile string) error {
	// Use the text format tool to merge
	textFormatTool := filepath.Join(filepath.Dir(os.Args[0]), "..", "text-format", "main.go")
	
	cmd := exec.Command("go", "run", textFormatTool,
		"-i", inputFile,
		"-s", syntheticFile,
		"-o", outputFile)
	
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to merge coverage: %v\n%s", err, output)
	}
	return nil
}

func displayCoverage(coverageFile string) error {
	content, err := os.ReadFile(coverageFile)
	if err != nil {
		return err
	}
	fmt.Println(string(content))
	return nil
}
