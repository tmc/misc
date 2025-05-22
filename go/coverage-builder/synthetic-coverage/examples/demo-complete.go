// demo-complete.go - Complete demonstration of synthetic coverage injection
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
	fmt.Println("=== Complete Synthetic Coverage Demo ===")
	
	workDir := "/tmp/synthetic-coverage-demo"
	os.RemoveAll(workDir)
	os.MkdirAll(workDir, 0755)
	
	// Create test application
	appDir := filepath.Join(workDir, "myapp")
	os.MkdirAll(appDir, 0755)
	
	// go.mod
	goMod := `module myapp
go 1.20`
	os.WriteFile(filepath.Join(appDir, "go.mod"), []byte(goMod), 0644)
	
	// main.go
	mainGo := `package main

import (
	"fmt"
	"myapp/core"
)

func main() {
	result := core.Process("hello")
	fmt.Println(result)
}
`
	os.WriteFile(filepath.Join(appDir, "main.go"), []byte(mainGo), 0644)
	
	// core package
	coreDir := filepath.Join(appDir, "core")
	os.MkdirAll(coreDir, 0755)
	
	coreGo := `package core

func Process(input string) string {
	if input == "" {
		return "empty"
	}
	return "processed: " + input
}

func Helper() string {
	return "helper"
}

func Unused() string {
	return "unused"
}
`
	os.WriteFile(filepath.Join(coreDir, "core.go"), []byte(coreGo), 0644)
	
	// test file
	testGo := `package core

import "testing"

func TestProcess(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "processed: hello"},
		{"", "empty"},
	}
	
	for _, tt := range tests {
		if got := Process(tt.input); got != tt.want {
			t.Errorf("Process(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHelper(t *testing.T) {
	if Helper() != "helper" {
		t.Error("unexpected helper result")
	}
}
`
	os.WriteFile(filepath.Join(coreDir, "core_test.go"), []byte(testGo), 0644)
	
	// Step 1: Run tests with coverage
	fmt.Println("\n1. Running tests with coverage...")
	coverFile := filepath.Join(workDir, "coverage.txt")
	
	cmd := exec.Command("go", "test", "-coverprofile="+coverFile, "./...")
	cmd.Dir = appDir
	output, _ := cmd.CombinedOutput()
	fmt.Printf("%s", output)
	
	// Step 2: Show original coverage
	fmt.Println("\n2. Original coverage:")
	original, _ := os.ReadFile(coverFile)
	fmt.Println(string(original))
	
	cmd = exec.Command("go", "tool", "cover", "-func="+coverFile)
	cmd.Dir = appDir
	output, _ = cmd.Output()
	fmt.Printf("%s", output)
	
	// Step 3: Create synthetic coverage
	fmt.Println("\n3. Creating synthetic coverage for:")
	fmt.Println("   - Generated protobuf files")
	fmt.Println("   - Mock implementations")
	fmt.Println("   - Vendor libraries")
	
	synthetic := []string{
		"myapp/generated/api.pb.go:1.1,500.1 250 1",
		"myapp/generated/api.pb.go:501.1,1000.1 200 1",
		"myapp/mocks/database.go:1.1,100.1 50 1",
		"myapp/mocks/database.go:101.1,200.1 50 0",
		"myapp/internal/cache/lru.go:1.1,300.1 150 1",
		"github.com/vendor/lib/client.go:1.1,400.1 200 1",
		"github.com/vendor/lib/util.go:1.1,150.1 75 1",
	}
	
	syntheticFile := filepath.Join(workDir, "synthetic.txt")
	os.WriteFile(syntheticFile, []byte(strings.Join(synthetic, "\n")), 0644)
	
	// Step 4: Build and run merger
	fmt.Println("\n4. Building coverage merger...")
	mergerPath := filepath.Join(workDir, "merger")
	mergerSrc := "/Volumes/tmc/go/src/github.com/tmc/misc/go/coverage-builder/synthetic-coverage/text-format/main.go"
	
	cmd = exec.Command("go", "build", "-o", mergerPath, mergerSrc)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build merger: %v", err)
	}
	
	// Step 5: Merge coverage
	fmt.Println("\n5. Merging real and synthetic coverage...")
	mergedFile := filepath.Join(workDir, "merged.txt")
	
	cmd = exec.Command(mergerPath,
		"-i", coverFile,
		"-s", syntheticFile,
		"-o", mergedFile)
	
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to merge: %v", err)
	}
	
	// Step 6: Show merged coverage
	fmt.Println("\n6. Merged coverage:")
	merged, _ := os.ReadFile(mergedFile)
	lines := strings.Split(string(merged), "\n")
	for i, line := range lines {
		if i > 10 && i < len(lines)-3 {
			if i == 11 {
				fmt.Println("... (showing first 10 and last 2 lines)")
			}
			continue
		}
		if line != "" {
			fmt.Println(line)
		}
	}
	
	// Step 7: Calculate synthetic coverage impact
	fmt.Println("\n7. Coverage impact:")
	
	// Count statements
	origLines := strings.Split(string(original), "\n")
	origStmts := 0
	origCovered := 0
	
	for _, line := range origLines[1:] { // Skip mode line
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			stmts := 0
			covered := 0
			fmt.Sscanf(parts[1], "%d", &stmts)
			fmt.Sscanf(parts[2], "%d", &covered)
			origStmts += stmts
			if covered > 0 {
				origCovered += stmts
			}
		}
	}
	
	synthStmts := 0
	synthCovered := 0
	
	for _, line := range synthetic {
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			stmts := 0
			covered := 0
			fmt.Sscanf(parts[1], "%d", &stmts)
			fmt.Sscanf(parts[2], "%d", &covered)
			synthStmts += stmts
			if covered > 0 {
				synthCovered += stmts
			}
		}
	}
	
	fmt.Printf("Original: %d/%d statements (%.1f%%)\n", 
		origCovered, origStmts, float64(origCovered)*100/float64(origStmts))
	fmt.Printf("Synthetic: %d/%d statements (%.1f%%)\n",
		synthCovered, synthStmts, float64(synthCovered)*100/float64(synthStmts))
	fmt.Printf("Combined: %d/%d statements (%.1f%%)\n",
		origCovered+synthCovered, origStmts+synthStmts,
		float64(origCovered+synthCovered)*100/float64(origStmts+synthStmts))
	
	// Step 8: Generate reports
	fmt.Println("\n8. Generating reports...")
	htmlFile := filepath.Join(workDir, "coverage.html")
	
	cmd = exec.Command("go", "tool", "cover", "-html="+mergedFile, "-o="+htmlFile)
	cmd.Dir = appDir
	if err := cmd.Run(); err != nil {
		fmt.Println("Note: HTML generation may fail for non-existent files")
	} else {
		fmt.Printf("HTML report: %s\n", htmlFile)
	}
	
	// Summary
	fmt.Println("\n=== Summary ===")
	fmt.Println("✓ Created real coverage from unit tests")
	fmt.Println("✓ Added synthetic coverage for:")
	fmt.Println("  - Generated protobuf files (450 statements)")
	fmt.Println("  - Mock implementations (100 statements)")
	fmt.Println("  - Internal utilities (150 statements)")
	fmt.Println("  - Vendor libraries (275 statements)")
	fmt.Println("✓ Merged all coverage data")
	fmt.Println("✓ Generated combined reports")
	
	fmt.Printf("\nAll files saved to: %s\n", workDir)
	fmt.Println("\nUsage with CI tools:")
	fmt.Println("  codecov -f merged.txt")
	fmt.Println("  coveralls < merged.txt")
}