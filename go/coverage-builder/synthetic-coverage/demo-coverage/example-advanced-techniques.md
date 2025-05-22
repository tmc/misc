# Practical Example: Advanced Synthetic Coverage Techniques

This guide demonstrates advanced techniques for working with synthetic coverage in Go projects.

## 1. Automating Synthetic Coverage Generation

Let's explore multiple approaches to automate the generation of synthetic coverage files.

### Approach 1: Generate from Comments

This tool identifies functions with special comments and generates synthetic coverage:

```go
// synthetic-from-comments.go
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

var (
	sourceDir  = flag.String("source", ".", "Source directory to scan")
	outputFile = flag.String("output", "synthetic.txt", "Output synthetic coverage file")
	moduleName = flag.String("module", "", "Module name (autodetected if not provided)")
)

func main() {
	flag.Parse()

	// Auto-detect module name if not provided
	module := *moduleName
	if module == "" {
		module = detectModuleName(*sourceDir)
	}

	// Find files to process
	var sourceFiles []string
	err := filepath.Walk(*sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			sourceFiles = append(sourceFiles, path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking source directory: %v\n", err)
		os.Exit(1)
	}

	// Process files and find annotated functions
	var entries []string
	for _, file := range sourceFiles {
		fileEntries, err := processFile(file, module, *sourceDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", file, err)
			continue
		}
		entries = append(entries, fileEntries...)
	}

	// Write synthetic coverage file
	if err := writeEntries(*outputFile, entries); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated synthetic coverage for %d functions\n", len(entries))
}

func processFile(filename, module, sourceDir string) ([]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Calculate import path for this file
	relPath, err := filepath.Rel(sourceDir, filename)
	if err != nil {
		return nil, err
	}
	importPath := filepath.Join(module, filepath.Dir(relPath))
	fileName := filepath.Base(filename)

	var entries []string
	for _, decl := range node.Decls {
		// Look for function declarations
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Doc == nil {
			continue
		}

		// Check for synthetic coverage annotations
		shouldCover := false
		for _, comment := range funcDecl.Doc.List {
			if strings.Contains(comment.Text, "@synthetic-coverage") {
				shouldCover = true
				break
			}
		}

		if shouldCover {
			// Get position information
			startPos := fset.Position(funcDecl.Pos())
			endPos := fset.Position(funcDecl.End())
			
			// Estimate number of statements
			numLines := endPos.Line - startPos.Line
			numStatements := max(1, numLines/2) // Rough estimate: 1 statement per 2 lines
			
			// Create coverage entry
			entry := fmt.Sprintf("%s/%s:%d.1,%d.1 %d 1", 
				importPath, fileName, 
				startPos.Line, endPos.Line, 
				numStatements)
			
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func detectModuleName(dir string) string {
	// Try to read go.mod file
	modFile := filepath.Join(dir, "go.mod")
	content, err := os.ReadFile(modFile)
	if err != nil {
		return "example.com/unknown"
	}

	// Extract module name
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}

	return "example.com/unknown"
}

func writeEntries(filename string, entries []string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range entries {
		if _, err := fmt.Fprintln(f, entry); err != nil {
			return err
		}
	}

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

Usage:

```bash
# Find functions with @synthetic-coverage comment
go run synthetic-from-comments.go -source=. -output=synthetic.txt
```

Example of annotated function:

```go
// @synthetic-coverage
// This function is generated and doesn't need testing
func GeneratedMethod() {
    // ...
}
```

### Approach 2: Pattern-Based Detection

This tool automatically generates synthetic coverage for files matching patterns:

```go
// synthetic-from-patterns.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	sourceDir = flag.String("source", ".", "Source directory to scan")
	outputFile = flag.String("output", "synthetic.txt", "Output synthetic coverage file")
	patterns = flag.String("patterns", "generated_,zz_,mock_", "Comma-separated patterns to match")
	modulePrefix = flag.String("module", "", "Module name (autodetected if not provided)")
)

func main() {
	flag.Parse()

	// Parse patterns
	patternList := strings.Split(*patterns, ",")
	
	// Auto-detect module name if not provided
	module := *modulePrefix
	if module == "" {
		module = detectModuleName(*sourceDir)
	}
	
	var entries []string

	// Walk the directory tree
	err := filepath.Walk(*sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip non-Go files and test files
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		
		// Check if file matches any pattern
		shouldCover := false
		fileName := filepath.Base(path)
		for _, pattern := range patternList {
			if strings.Contains(fileName, pattern) {
				shouldCover = true
				break
			}
		}
		
		if shouldCover {
			// Calculate import path
			relPath, err := filepath.Rel(*sourceDir, path)
			if err != nil {
				return err
			}
			
			dirPath := filepath.Dir(relPath)
			importPath := filepath.Join(module, dirPath)
			
			// Count lines in file
			content, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip if can't read
			}
			
			lines := strings.Count(string(content), "\n") + 1
			statements := lines / 3 // Rough estimate: 1 statement per 3 lines
			if statements < 1 {
				statements = 1
			}
			
			// Create coverage entry for whole file
			entry := fmt.Sprintf("%s/%s:1.1,%d.1 %d 1", 
				importPath, fileName, lines, statements)
			
			entries = append(entries, entry)
		}
		
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Write synthetic coverage file
	f, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	
	for _, entry := range entries {
		fmt.Fprintln(f, entry)
	}
	
	fmt.Printf("Generated synthetic coverage for %d files\n", len(entries))
}

func detectModuleName(dir string) string {
	// Try to read go.mod file
	modFile := filepath.Join(dir, "go.mod")
	content, err := os.ReadFile(modFile)
	if err != nil {
		return "example.com/unknown"
	}

	// Extract module name
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}

	return "example.com/unknown"
}
```

Usage:

```bash
# Generate synthetic coverage for files matching patterns
go run synthetic-from-patterns.go -patterns="generated_,mock_,pb.go" -output=synthetic.txt
```

## 2. Directory-Based Coverage Requirements

This example demonstrates how to configure different coverage requirements per directory:

### Create a Configuration File

```bash
cat > .coverage-config.yml << EOF
# Default settings
default:
  threshold: 80
  synthetic: false

# Generated code
generated/:
  threshold: 0
  synthetic: true

# Core business logic
core/:
  threshold: 90
  synthetic: false

# Third-party code
vendor/:
  threshold: 0
  synthetic: true

# Generated protobuf code
**/*.pb.go:
  threshold: 0
  synthetic: true
EOF
```

### Implement a Processor

```go
// coverage-config-processor.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// CoverageConfig represents coverage requirements for a directory pattern
type CoverageConfig struct {
	Threshold int  `yaml:"threshold"`
	Synthetic bool `yaml:"synthetic"`
}

// Config is a map of directory pattern to coverage config
type Config map[string]CoverageConfig

func main() {
	configFile := flag.String("config", ".coverage-config.yml", "Coverage configuration file")
	sourceDir := flag.String("source", ".", "Source directory")
	outputFile := flag.String("output", "synthetic.txt", "Output synthetic coverage file")
	flag.Parse()

	// Read config file
	data, err := os.ReadFile(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
		os.Exit(1)
	}

	// Parse config
	config := make(Config)
	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
		os.Exit(1)
	}

	// Get default config
	defaultConfig, ok := config["default"]
	if !ok {
		defaultConfig = CoverageConfig{
			Threshold: 80,
			Synthetic: false,
		}
	}

	// Process source files
	var entries []string
	err = filepath.Walk(*sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Determine which config applies
		relPath, _ := filepath.Rel(*sourceDir, path)
		cfg := findMatchingConfig(relPath, config, defaultConfig)

		if cfg.Synthetic {
			// Calculate import path
			dir := filepath.Dir(relPath)
			importPath := fmt.Sprintf("example.com/myproject/%s", dir)
			fileName := filepath.Base(path)

			// Count lines
			content, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip if can't read
			}

			lines := strings.Count(string(content), "\n") + 1
			statements := lines / 3 // Rough estimate
			if statements < 1 {
				statements = 1
			}

			// Create coverage entry
			entry := fmt.Sprintf("%s/%s:1.1,%d.1 %d 1",
				importPath, fileName, lines, statements)
			entries = append(entries, entry)
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Write synthetic coverage file
	f, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	for _, entry := range entries {
		fmt.Fprintln(f, entry)
	}

	fmt.Printf("Generated synthetic coverage for %d files based on configuration\n", len(entries))
}

func findMatchingConfig(path string, config Config, defaultConfig CoverageConfig) CoverageConfig {
	bestMatch := ""
	bestConfig := defaultConfig

	for pattern, cfg := range config {
		if pattern == "default" {
			continue
		}

		// Handle glob patterns
		if strings.Contains(pattern, "*") {
			matched, err := filepath.Match(pattern, path)
			if err == nil && matched && len(pattern) > len(bestMatch) {
				bestMatch = pattern
				bestConfig = cfg
			}
			continue
		}

		// Handle directory prefixes
		if strings.HasSuffix(pattern, "/") {
			prefix := strings.TrimSuffix(pattern, "/")
			if strings.HasPrefix(path, prefix) && len(pattern) > len(bestMatch) {
				bestMatch = pattern
				bestConfig = cfg
			}
			continue
		}

		// Exact match
		if path == pattern && len(pattern) > len(bestMatch) {
			bestMatch = pattern
			bestConfig = cfg
		}
	}

	return bestConfig
}
```

Usage:

```bash
go run coverage-config-processor.go -config=.coverage-config.yml -output=synthetic.txt
```

## 3. Including Non-Go Files in Coverage

This example shows how to include non-Go files (templates, SQL, etc.) in coverage reports:

```go
// synthetic-for-non-go.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	sourceDir = flag.String("source", ".", "Source directory")
	outputFile = flag.String("output", "synthetic.txt", "Output file")
	extensions = flag.String("extensions", "tmpl,sql,css,js", "File extensions to include")
	moduleName = flag.String("module", "example.com/myproject", "Module name")
)

func main() {
	flag.Parse()
	
	extList := strings.Split(*extensions, ",")
	
	var entries []string
	
	// Process each extension
	for _, ext := range extList {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		
		// Find files with this extension
		pattern := fmt.Sprintf("**/*.%s", ext)
		matches, err := filepath.Glob(filepath.Join(*sourceDir, pattern))
		if err != nil {
			// Fall back to walking the directory
			filepath.Walk(*sourceDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, "."+ext) {
					matches = append(matches, path)
				}
				return nil
			})
		}
		
		// Process each file
		for _, path := range matches {
			relPath, _ := filepath.Rel(*sourceDir, path)
			
			// Count lines
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			
			lines := strings.Count(string(content), "\n") + 1
			statements := lines / 2 // Rough estimate for non-Go files
			if statements < 1 {
				statements = 1
			}
			
			// Create synthetic coverage entry
			importPath := filepath.Join(*moduleName, filepath.Dir(relPath))
			fileName := filepath.Base(path)
			
			entry := fmt.Sprintf("%s/%s:1.1,%d.1 %d 1",
				importPath, fileName, lines, statements)
			
			entries = append(entries, entry)
		}
	}
	
	// Write the output file
	f, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	
	for _, entry := range entries {
		fmt.Fprintln(f, entry)
	}
	
	fmt.Printf("Generated synthetic coverage for %d non-Go files\n", len(entries))
}
```

Usage:

```bash
go run synthetic-for-non-go.go -extensions="tmpl,graphql,proto" -output=non-go-coverage.txt
```

## 4. Making Synthetic Coverage Visible in Reports

This script post-processes HTML coverage reports to highlight synthetic coverage:

```bash
cat > highlight-synthetic.sh << 'EOF'
#!/bin/bash
# Highlight synthetic coverage in HTML report

if [ $# -lt 3 ]; then
  echo "Usage: $0 coverage.txt synthetic.txt coverage.html"
  exit 1
fi

COVERAGE_FILE=$1
SYNTHETIC_FILE=$2
HTML_FILE=$3

# Generate HTML report
go tool cover -html="$COVERAGE_FILE" -o="$HTML_FILE"

# Extract file paths from synthetic file
grep -v "^mode:" "$SYNTHETIC_FILE" | cut -d':' -f1 | sort | uniq > synthetic-files.txt

# Add a note at the top of the HTML report
sed -i.bak '/<body>/a\
<div style="position:fixed; top:0; left:0; right:0; background-color:#FFC; padding:5px; z-index:1000; text-align:center; font-weight:bold; border-bottom:2px solid #F90;">\
  This report includes synthetic coverage. Files with synthetic coverage are marked with <span style="color:#F90;">★</span>\
</div>' "$HTML_FILE"

# Add markers to files with synthetic coverage
while read -r file; do
  # Escape file path for sed
  escaped_file=$(echo "$file" | sed 's/[\/&]/\\&/g')
  
  # Add marker to file name in HTML
  sed -i.bak "s/<span class=\"cov[0-9]*\" title=\"$escaped_file\">/$& <span style=\"color:#F90;\">★<\/span> /" "$HTML_FILE"
done < synthetic-files.txt

# Clean up
rm synthetic-files.txt
rm "$HTML_FILE.bak"

echo "Enhanced HTML report generated at $HTML_FILE"
EOF

chmod +x highlight-synthetic.sh
```

Usage:

```bash
./highlight-synthetic.sh merged.txt synthetic.txt coverage-highlighted.html
```

## 5. Enforcing Coverage Requirements in CI

This example checks coverage against thresholds defined in the config:

```go
// check-coverage.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

// CoverageConfig represents coverage requirements
type CoverageConfig struct {
	Threshold int  `yaml:"threshold"`
	Synthetic bool `yaml:"synthetic"`
}

func main() {
	coverageFile := flag.String("coverage", "coverage.txt", "Coverage file to check")
	configFile := flag.String("config", ".coverage-config.yml", "Coverage configuration file")
	flag.Parse()

	// Read config
	data, err := os.ReadFile(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}

	config := make(map[string]CoverageConfig)
	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
		os.Exit(1)
	}

	// Get default threshold
	defaultConfig, ok := config["default"]
	if !ok {
		defaultConfig = CoverageConfig{Threshold: 80}
	}

	// Parse coverage data
	fileCoverage, err := parseCoverage(*coverageFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing coverage: %v\n", err)
		os.Exit(1)
	}

	// Check each file against its threshold
	var failures []string
	for file, coverage := range fileCoverage {
		// Find matching config
		fileConfig := findMatchingConfig(file, config, defaultConfig)
		
		if coverage < float64(fileConfig.Threshold) {
			failures = append(failures, fmt.Sprintf("%s: %.1f%% (required: %d%%)",
				file, coverage, fileConfig.Threshold))
		}
	}

	// Report results
	if len(failures) > 0 {
		fmt.Println("The following files do not meet coverage requirements:")
		for _, failure := range failures {
			fmt.Println("  - " + failure)
		}
		os.Exit(1)
	}

	fmt.Println("All files meet coverage requirements!")
}

func parseCoverage(filename string) (map[string]float64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Parse func output
	funcRegex := regexp.MustCompile(`^(.+):\s+\S+\s+(\d+\.\d+)%$`)
	totalRegex := regexp.MustCompile(`^total:\s+.*\s+(\d+\.\d+)%$`)

	// Get func coverage from "go tool cover -func"
	result := make(map[string]float64)
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Check if it's a file coverage line
		if matches := funcRegex.FindStringSubmatch(line); len(matches) >= 3 {
			file := matches[1]
			coverage, _ := strconv.ParseFloat(matches[2], 64)
			result[file] = coverage
			continue
		}
		
		// Check if it's the total line
		if matches := totalRegex.FindStringSubmatch(line); len(matches) >= 2 {
			coverage, _ := strconv.ParseFloat(matches[1], 64)
			result["total"] = coverage
		}
	}

	return result, scanner.Err()
}

func findMatchingConfig(path string, config map[string]CoverageConfig, defaultConfig CoverageConfig) CoverageConfig {
	bestMatch := ""
	bestConfig := defaultConfig

	for pattern, cfg := range config {
		if pattern == "default" {
			continue
		}

		// Handle glob patterns
		if strings.Contains(pattern, "*") {
			matched, err := filepath.Match(pattern, path)
			if err == nil && matched && len(pattern) > len(bestMatch) {
				bestMatch = pattern
				bestConfig = cfg
			}
			continue
		}

		// Handle directory prefixes
		if strings.HasSuffix(pattern, "/") {
			prefix := strings.TrimSuffix(pattern, "/")
			if strings.HasPrefix(path, prefix) && len(pattern) > len(bestMatch) {
				bestMatch = pattern
				bestConfig = cfg
			}
			continue
		}

		// Exact match
		if path == pattern && len(pattern) > len(bestMatch) {
			bestMatch = pattern
			bestConfig = cfg
		}
	}

	return bestConfig
}
```

Usage in CI:

```yaml
# .github/workflows/go.yml
name: Go

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
        
    - name: Test with coverage
      run: go test -coverprofile=coverage.txt ./...
      
    - name: Generate synthetic coverage
      run: go run tools/synthetic-from-patterns.go -output=synthetic.txt
      
    - name: Merge coverage
      run: go run tools/merge-coverage.go -i=coverage.txt -s=synthetic.txt -o=merged.txt
      
    - name: Convert to func format
      run: go tool cover -func=merged.txt > coverage-func.txt
      
    - name: Check coverage requirements
      run: go run tools/check-coverage.go -coverage=coverage-func.txt -config=.coverage-config.yml
      
    - name: Upload coverage to Codecov
      uses: codecov/codecov-action@v3
      with:
        file: ./merged.txt
```

## Key Takeaways

1. **Automation is key** - Use tools to maintain synthetic coverage as your codebase evolves
2. **Configuration-driven** - Define coverage requirements in configuration files
3. **Clear visibility** - Make it obvious which parts are synthetically covered
4. **CI integration** - Enforce coverage requirements automatically
5. **Multiple approaches** - Mix and match techniques to suit your project's needs

These advanced techniques help make synthetic coverage a sustainable part of your testing strategy.