// demo.go - Demonstrates synthetic coverage usage
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	fmt.Println("=== Go Synthetic Coverage Demo ===")
	
	// Create temporary directory for demo
	tmpDir := "/tmp/coverage-demo"
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	
	// Step 1: Create a real Go program
	fmt.Println("\n1. Creating sample Go program...")
	realProgram := `package main

import "fmt"

func main() {
	fmt.Println("Real coverage")
	if true {
		fmt.Println("This is covered")
	}
}

func uncovered() {
	fmt.Println("Not covered")
}
`
	progFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(progFile, []byte(realProgram), 0644); err != nil {
		log.Fatal(err)
	}
	
	// Step 2: Run with GOCOVERDIR to generate real coverage
	fmt.Println("\n2. Running program with coverage...")
	coverDir := filepath.Join(tmpDir, "coverage")
	os.MkdirAll(coverDir, 0755)
	
	cmd := exec.Command("go", "run", progFile)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOCOVERDIR=%s", coverDir))
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Failed to run: %v\n%s", err, output)
	}
	
	// Step 3: Convert to text format
	fmt.Println("\n3. Converting to text format...")
	textFile := filepath.Join(tmpDir, "coverage.txt")
	cmd = exec.Command("go", "tool", "covdata", "textfmt",
		fmt.Sprintf("-i=%s", coverDir),
		fmt.Sprintf("-o=%s", textFile))
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Failed to convert: %v\n%s", err, output)
	}
	
	// Step 4: Create synthetic coverage
	fmt.Println("\n4. Creating synthetic coverage...")
	synthetic := `github.com/example/fake/database/mock.go:1.1,50.1 25 1
github.com/example/fake/database/mock.go:52.1,100.1 20 1
github.com/example/fake/api/generated.go:1.1,200.1 100 1
github.com/example/fake/api/generated.go:201.1,300.1 50 0
github.com/vendor/external/lib.go:1.1,150.1 75 1
`
	syntheticFile := filepath.Join(tmpDir, "synthetic.txt")
	if err := os.WriteFile(syntheticFile, []byte(synthetic), 0644); err != nil {
		log.Fatal(err)
	}
	
	// Step 5: Merge coverage
	fmt.Println("\n5. Merging real and synthetic coverage...")
	mergedFile := filepath.Join(tmpDir, "merged.txt")
	toolPath := filepath.Join("..", "text-format", "main.go")
	cmd = exec.Command("go", "run", toolPath,
		"-i", textFile,
		"-s", syntheticFile,
		"-o", mergedFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Failed to merge: %v\n%s", err, output)
	}
	
	// Step 6: Display results
	fmt.Println("\n6. Coverage Results:")
	fmt.Println("\nOriginal coverage:")
	original, _ := os.ReadFile(textFile)
	fmt.Println(string(original))
	
	fmt.Println("\nSynthetic coverage added:")
	fmt.Println(synthetic)
	
	fmt.Println("\nMerged coverage:")
	merged, _ := os.ReadFile(mergedFile)
	fmt.Println(string(merged))
	
	// Step 7: Generate HTML report
	fmt.Println("\n7. Generating HTML report...")
	htmlFile := filepath.Join(tmpDir, "coverage.html")
	cmd = exec.Command("go", "tool", "cover",
		fmt.Sprintf("-html=%s", mergedFile),
		fmt.Sprintf("-o=%s", htmlFile))
	if _, err := cmd.CombinedOutput(); err != nil {
		// This may fail if files don't exist, which is expected for synthetic files
		fmt.Printf("Note: HTML generation may fail for non-existent files: %v\n", err)
	} else {
		fmt.Printf("HTML report generated at: %s\n", htmlFile)
	}
	
	fmt.Printf("\nDemo complete! Files created in: %s\n", tmpDir)
	fmt.Println("\nKey takeaways:")
	fmt.Println("- Real coverage was generated using GOCOVERDIR")
	fmt.Println("- Synthetic coverage was added for:")
	fmt.Println("  * Mock database files")  
	fmt.Println("  * Generated API code")
	fmt.Println("  * Vendored libraries")
	fmt.Println("- Coverage data was merged using text format")
	fmt.Println("\nThis is useful for:")
	fmt.Println("- Including generated code in coverage reports")
	fmt.Println("- Adding coverage for mocks and test doubles")
	fmt.Println("- Accounting for vendored dependencies")
}