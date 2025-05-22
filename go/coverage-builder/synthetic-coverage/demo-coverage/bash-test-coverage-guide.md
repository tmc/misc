# Synthetic Coverage for Bash Tests in Go Projects

This guide focuses specifically on adding synthetic coverage for bash-based tests in Go projects. It provides practical techniques to ensure your coverage reports accurately represent the code paths exercised by bash tests.

## Introduction

Bash tests are common in Go projects for:
- Integration testing of CLI tools
- Testing system interactions
- End-to-end workflow testing
- Testing behavior that's difficult to simulate with standard Go tests

However, these tests run your code in a separate process, outside of Go's standard coverage instrumentation. This means code paths only exercised by bash tests are not represented in your coverage reports.

## Step 1: Understanding the Problem

Let's examine a typical bash test setup in a Go project:

```go
// integration_test.go
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBashIntegration(t *testing.T) {
	// Build the tool
	toolPath := buildTool(t)
	
	// Run bash test script
	cmd := exec.Command("bash", "./test.sh")
	cmd.Env = append(os.Environ(), "TOOL_PATH="+toolPath)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Bash test failed: %v\n%s", err, output)
	}
	
	t.Logf("Test output: %s", output)
}

func buildTool(t *testing.T) string {
	// Build tool for testing
	tmpDir, _ := os.MkdirTemp("", "test-bin")
	binPath := filepath.Join(tmpDir, "mytool")
	
	cmd := exec.Command("go", "build", "-o", binPath, "../cmd/mytool")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build tool: %v", err)
	}
	
	return binPath
}
```

```bash
# test.sh
#!/bin/bash
set -e

if [ -z "$TOOL_PATH" ]; then
  echo "TOOL_PATH not set"
  exit 1
fi

# Test basic functionality
echo "Testing basic functionality..."
output=$($TOOL_PATH --version)
if [[ ! "$output" =~ v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "Version command failed, got: $output"
  exit 1
fi

# Test file processing
echo "Testing file processing..."
echo "test content" > test_file.txt
$TOOL_PATH process test_file.txt > result.txt
if ! grep -q "PROCESSED: test content" result.txt; then
  echo "File processing failed"
  exit 1
fi

# Test error handling
echo "Testing error handling..."
if $TOOL_PATH process nonexistent.txt 2>/dev/null; then
  echo "Error handling failed - should have errored on nonexistent file"
  exit 1
fi

echo "All tests passed!"
```

When we run `go test -cover ./...`, the coverage report won't include code executed by the bash script, even though it's testing important functionality.

## Step 2: Analyzing Bash Tests

The first step in adding synthetic coverage is to analyze what your bash tests are actually testing. There are several approaches:

### Approach 1: Static Analysis of Bash Scripts

We can parse the bash script to identify which commands are being tested:

```go
// bash-coverage-analyzer.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	scriptDir   = flag.String("scripts", ".", "Directory containing bash scripts")
	packagePath = flag.String("package", "example.com/myproject/cmd/mytool", "Package path for the tool")
	outputFile  = flag.String("output", "bash-coverage.txt", "Output synthetic coverage file")
)

func main() {
	flag.Parse()
	
	// Find all bash scripts
	scripts, err := filepath.Glob(filepath.Join(*scriptDir, "*.sh"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding scripts: %v\n", err)
		os.Exit(1)
	}
	
	// Process each script
	var allCommands []string
	for _, script := range scripts {
		commands, err := extractCommands(script)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", script, err)
			continue
		}
		
		allCommands = append(allCommands, commands...)
		fmt.Printf("Found %d commands in %s\n", len(commands), script)
	}
	
	// Generate synthetic coverage
	coverage := generateSyntheticCoverage(allCommands, *packagePath)
	
	// Write output file
	if err := os.WriteFile(*outputFile, []byte(coverage), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage for %d commands in %s\n", 
		len(allCommands), *outputFile)
}

func extractCommands(scriptPath string) ([]string, error) {
	file, err := os.Open(scriptPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var commands []string
	
	// Patterns to find tool invocations
	toolPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\$TOOL_PATH\s+([^\s>|&;]+(?:\s+[^>|&;]+)*)`),
		regexp.MustCompile(`"\$TOOL_PATH"\s+([^\s>|&;]+(?:\s+[^>|&;]+)*)`),
		regexp.MustCompile(`'\$TOOL_PATH'\s+([^\s>|&;]+(?:\s+[^>|&;]+)*)`),
		// Add more patterns as needed
	}
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		
		// Skip comments and empty lines
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Check for tool invocations
		for _, pattern := range toolPatterns {
			matches := pattern.FindStringSubmatch(line)
			if len(matches) > 1 {
				command := strings.TrimSpace(matches[1])
				commands = append(commands, command)
			}
		}
	}
	
	return commands, scanner.Err()
}

func generateSyntheticCoverage(commands []string, packagePath string) string {
	// Map of commands to code paths (simplified example)
	// In a real implementation, you would:
	// 1. Analyze your codebase to map commands to functions
	// 2. Generate coverage lines for each function
	codePaths := map[string][]string{
		"--version": {
			"main.go:25.13,35.2 4 1",      // version command
		},
		"process": {
			"main.go:40.32,50.2 8 1",      // process command
			"processor/file.go:15.40,25.2 9 1", // file processing
		},
	}
	
	// Build coverage set
	coverageSet := make(map[string]bool)
	
	// Add main function coverage
	coverageSet[packagePath+"/main.go:10.13,20.2 8 1"] = true
	
	// Process commands
	for _, cmd := range commands {
		// Extract the base command (first word)
		baseCmd := strings.Split(cmd, " ")[0]
		
		// Check for exact match
		if paths, ok := codePaths[cmd]; ok {
			for _, path := range paths {
				coverageSet[packagePath+"/"+path] = true
			}
			continue
		}
		
		// Check for base command match
		if paths, ok := codePaths[baseCmd]; ok {
			for _, path := range paths {
				coverageSet[packagePath+"/"+path] = true
			}
		}
		
		// Check for special cases like flags
		for pattern, paths := range codePaths {
			if strings.HasPrefix(cmd, pattern) {
				for _, path := range paths {
					coverageSet[packagePath+"/"+path] = true
				}
			}
		}
	}
	
	// Generate coverage file content
	var lines []string
	lines = append(lines, "mode: set")
	
	for line := range coverageSet {
		lines = append(lines, line)
	}
	
	return strings.Join(lines, "\n")
}
```

### Approach 2: Dynamic Execution Tracing

For more accurate coverage, we can modify our tool to record which functions are called during bash tests:

```go
// cmd/mytool/main.go with tracing
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
)

var (
	// Add a flag to enable tracing
	traceEnabled = flag.Bool("trace", false, "Enable function tracing")
	traceFile    = flag.String("trace-output", "trace.txt", "Trace output file")
)

var (
	traceMutex sync.Mutex
	traceLog   *os.File
	functions  = make(map[string]bool)
)

func setupTracing() {
	var err error
	traceLog, err = os.Create(*traceFile)
	if err != nil {
		log.Fatalf("Failed to create trace file: %v", err)
	}
	
	// Start periodic trace collection
	go func() {
		for {
			collectTrace()
			runtime.Gosched() // Allow other goroutines to run
		}
	}()
}

func collectTrace() {
	// Collect stack trace
	buf := make([]byte, 10240)
	n := runtime.Stack(buf, true)
	
	// Parse stack trace to extract functions
	traceMutex.Lock()
	defer traceMutex.Unlock()
	
	lines := strings.Split(string(buf[:n]), "\n")
	for _, line := range lines {
		// Extract function names from stack trace
		if strings.Contains(line, "example.com/myproject") && 
		   !strings.Contains(line, "collectTrace") {
			// Clean up the line to get just the function
			parts := strings.Split(line, "(")
			if len(parts) > 0 {
				funcName := strings.TrimSpace(parts[0])
				if !functions[funcName] {
					functions[funcName] = true
					fmt.Fprintln(traceLog, funcName)
				}
			}
		}
	}
}

func main() {
	flag.Parse()
	
	if *traceEnabled {
		setupTracing()
		defer traceLog.Close()
	}
	
	// Rest of your tool's code...
}
```

Then modify your test to enable tracing:

```go
func TestBashIntegration(t *testing.T) {
	// Build the tool with tracing enabled
	toolPath := buildToolWithTracing(t)
	
	// Run bash test script
	cmd := exec.Command("bash", "./test.sh")
	cmd.Env = append(os.Environ(), "TOOL_PATH="+toolPath)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Bash test failed: %v\n%s", err, output)
	}
	
	t.Logf("Test output: %s", output)
	
	// Process the trace file
	generateSyntheticCoverageFromTrace(t, "trace.txt")
}

func buildToolWithTracing(t *testing.T) string {
	tmpDir, _ := os.MkdirTemp("", "test-bin")
	binPath := filepath.Join(tmpDir, "mytool")
	
	cmd := exec.Command("go", "build", 
		"-o", binPath, 
		"-ldflags", "-X main.traceEnabled=true",
		"../cmd/mytool")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build tool: %v", err)
	}
	
	return binPath
}
```

### Approach 3: Map-Based Coverage

For a more structured approach, create a mapping file that defines which bash commands exercise which code paths:

```yaml
# coverage-map.yaml
bash_commands:
  - pattern: "--version"
    files:
      - path: "cmd/mytool/main.go"
        lines:
          - start: 25
            end: 35
  
  - pattern: "process"
    files:
      - path: "cmd/mytool/main.go"
        lines:
          - start: 40
            end: 50
      - path: "internal/processor/file.go"
        lines:
          - start: 15
            end: 25
          - start: 30
            end: 45
```

Then use this mapping to generate synthetic coverage:

```go
// map-based-coverage.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	
	"gopkg.in/yaml.v2"
)

type LineRange struct {
	Start int `yaml:"start"`
	End   int `yaml:"end"`
}

type FileCoverage struct {
	Path  string      `yaml:"path"`
	Lines []LineRange `yaml:"lines"`
}

type BashCommand struct {
	Pattern string        `yaml:"pattern"`
	Files   []FileCoverage `yaml:"files"`
}

type CoverageMap struct {
	BashCommands []BashCommand `yaml:"bash_commands"`
}

func main() {
	mapFile := flag.String("map", "coverage-map.yaml", "Coverage mapping file")
	scriptFile := flag.String("script", "test.sh", "Bash script to analyze")
	outputFile := flag.String("output", "bash-coverage.txt", "Output coverage file")
	packagePrefix := flag.String("package", "example.com/myproject", "Package prefix")
	flag.Parse()
	
	// Load coverage map
	coverageMap, err := loadCoverageMap(*mapFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading coverage map: %v\n", err)
		os.Exit(1)
	}
	
	// Extract commands from script
	commands, err := extractCommands(*scriptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting commands: %v\n", err)
		os.Exit(1)
	}
	
	// Generate synthetic coverage
	coverage := generateCoverage(commands, coverageMap, *packagePrefix)
	
	// Write output
	if err := os.WriteFile(*outputFile, []byte(coverage), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage in %s\n", *outputFile)
}

func loadCoverageMap(filename string) (*CoverageMap, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	var coverageMap CoverageMap
	if err := yaml.Unmarshal(data, &coverageMap); err != nil {
		return nil, err
	}
	
	return &coverageMap, nil
}

func extractCommands(filename string) ([]string, error) {
	// Same as in previous examples
	// ...
}

func generateCoverage(commands []string, coverageMap *CoverageMap, packagePrefix string) string {
	var lines []string
	lines = append(lines, "mode: set")
	
	// Track covered lines to avoid duplicates
	covered := make(map[string]bool)
	
	// Process each command
	for _, cmd := range commands {
		// Find matching patterns
		for _, bashCmd := range coverageMap.BashCommands {
			// Check if command matches pattern
			matched, _ := regexp.MatchString(bashCmd.Pattern, cmd)
			
			if matched {
				// Add coverage for all files
				for _, file := range bashCmd.Files {
					for _, lineRange := range file.Lines {
						// Add coverage line
						// Format: packagePath/filePath:startLine.startCol,endLine.endCol numStmts count
						coverageLine := fmt.Sprintf("%s/%s:%d.1,%d.1 %d 1",
							packagePrefix,
							file.Path,
							lineRange.Start,
							lineRange.End,
							lineRange.End - lineRange.Start + 1)
						
						covered[coverageLine] = true
					}
				}
			}
		}
	}
	
	// Add all covered lines
	for line := range covered {
		lines = append(lines, line)
	}
	
	return strings.Join(lines, "\n")
}
```

## Step 3: Integrating with Go's Coverage System

Now that we have generated synthetic coverage, we need to merge it with the standard Go coverage:

```go
// merge-coverage.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	realCoverage := flag.String("real", "", "Path to real coverage file")
	syntheticCoverage := flag.String("synthetic", "", "Path to synthetic coverage file")
	outputFile := flag.String("output", "merged-coverage.txt", "Output merged coverage file")
	flag.Parse()
	
	if *realCoverage == "" || *syntheticCoverage == "" {
		fmt.Fprintf(os.Stderr, "Both -real and -synthetic flags are required\n")
		flag.Usage()
		os.Exit(1)
	}
	
	// Read real coverage
	realLines, mode := readCoverageFile(*realCoverage)
	
	// Read synthetic coverage
	syntheticLines, _ := readCoverageFile(*syntheticCoverage)
	
	// Merge coverage (taking synthetic if both exist)
	mergedMap := make(map[string]bool)
	
	// Add all real coverage
	for _, line := range realLines {
		mergedMap[line] = true
	}
	
	// Add all synthetic coverage
	for _, line := range syntheticLines {
		mergedMap[line] = true
	}
	
	// Convert back to slice
	var mergedLines []string
	for line := range mergedMap {
		mergedLines = append(mergedLines, line)
	}
	
	// Create output directory if needed
	if dir := filepath.Dir(*outputFile); dir != "" {
		os.MkdirAll(dir, 0755)
	}
	
	// Write the merged coverage
	f, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	
	fmt.Fprintf(f, "mode: %s\n", mode)
	for _, line := range mergedLines {
		fmt.Fprintln(f, line)
	}
	
	fmt.Printf("Merged %d real and %d synthetic coverage lines to %s\n",
		len(realLines), len(syntheticLines), *outputFile)
}

func readCoverageFile(filename string) ([]string, string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: couldn't open %s: %v\n", filename, err)
		return nil, "set"
	}
	defer file.Close()
	
	var lines []string
	mode := "set"
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" {
			continue
		}
		
		if strings.HasPrefix(line, "mode:") {
			mode = strings.TrimSpace(strings.TrimPrefix(line, "mode:"))
		} else {
			lines = append(lines, line)
		}
	}
	
	return lines, mode
}
```

## Step 4: Complete Example Workflow

Let's put everything together in a complete workflow:

```bash
#!/bin/bash
# run-with-coverage.sh

set -e

# Step 1: Run standard Go tests with coverage
go test -coverprofile=go-coverage.txt ./...

# Step 2: Analyze bash scripts and generate synthetic coverage
go run tools/bash-coverage-analyzer.go \
  --scripts=./scripts \
  --package=example.com/myproject \
  --output=bash-coverage.txt

# Step 3: Merge coverage files
go run tools/merge-coverage.go \
  --real=go-coverage.txt \
  --synthetic=bash-coverage.txt \
  --output=merged-coverage.txt

# Step 4: Generate coverage reports
go tool cover -func=merged-coverage.txt > coverage-summary.txt
go tool cover -html=merged-coverage.txt -o=coverage.html

# Print summary
echo "Coverage report generated: coverage.html"
cat coverage-summary.txt | grep total
```

## Step 5: Visualization Enhancement

To clearly distinguish synthetic coverage in HTML reports, you can post-process the HTML report:

```go
// enhance-coverage-html.go
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	htmlFile := flag.String("html", "coverage.html", "HTML coverage report")
	syntheticFile := flag.String("synthetic", "bash-coverage.txt", "Synthetic coverage file")
	outputFile := flag.String("output", "enhanced-coverage.html", "Enhanced output file")
	flag.Parse()
	
	// Read synthetic coverage to extract paths
	syntheticPaths := extractPathsFromCoverage(*syntheticFile)
	
	// Read HTML file
	html, err := os.ReadFile(*htmlFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading HTML file: %v\n", err)
		os.Exit(1)
	}
	
	// Add CSS for synthetic coverage highlighting
	htmlStr := string(html)
	cssInjection := `
<style>
.synthetic-coverage {
    background-color: #FFC !important;
    position: relative;
}
.synthetic-coverage:after {
    content: "🔧";
    position: absolute;
    right: 5px;
    font-size: 12px;
}
.synthetic-notice {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    background-color: #FFC;
    padding: 5px;
    text-align: center;
    font-weight: bold;
    z-index: 1000;
    border-bottom: 1px solid #F90;
}
</style>
`
	
	htmlStr = strings.Replace(htmlStr, "</head>", cssInjection+"</head>", 1)
	
	// Add notice
	noticeDiv := `<div class="synthetic-notice">
  This report includes synthetic coverage for bash tests. Lines marked with 🔧 represent coverage from bash tests.
</div>`
	
	htmlStr = strings.Replace(htmlStr, "<body>", "<body>"+noticeDiv, 1)
	
	// Mark synthetic coverage
	for _, path := range syntheticPaths {
		escapedPath := regexp.QuoteMeta(path)
		re := regexp.MustCompile(`(class="cov[0-9]+"[^>]*>)([^<]*)` + escapedPath)
		htmlStr = re.ReplaceAllString(htmlStr, `$1<span class="synthetic-coverage">$2`+path+`</span>`)
	}
	
	// Write enhanced HTML
	if err := os.WriteFile(*outputFile, []byte(htmlStr), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing enhanced HTML: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Enhanced coverage report written to %s\n", *outputFile)
}

func extractPathsFromCoverage(filename string) []string {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading synthetic coverage: %v\n", err)
		os.Exit(1)
	}
	
	var paths []string
	pathSet := make(map[string]bool)
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "mode:") || line == "" {
			continue
		}
		
		// Extract file path
		parts := strings.Split(line, ":")
		if len(parts) > 1 {
			path := parts[0]
			if !pathSet[path] {
				pathSet[path] = true
				paths = append(paths, path)
			}
		}
	}
	
	return paths
}
```

## Step 6: Integration with CI/CD

Add the synthetic coverage process to your CI/CD pipeline:

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
        
    - name: Run Go tests with coverage
      run: go test -coverprofile=go-coverage.txt ./...
      
    - name: Generate synthetic coverage for bash tests
      run: |
        go run tools/bash-coverage-analyzer.go \
          --scripts=./scripts \
          --package=example.com/myproject \
          --output=bash-coverage.txt
      
    - name: Merge coverage data
      run: |
        go run tools/merge-coverage.go \
          --real=go-coverage.txt \
          --synthetic=bash-coverage.txt \
          --output=merged-coverage.txt
      
    - name: Check coverage thresholds
      run: |
        total_coverage=$(go tool cover -func=merged-coverage.txt | grep total | awk '{print $3}' | tr -d '%')
        if (( $(echo "$total_coverage < 80" | bc -l) )); then
          echo "Total coverage is $total_coverage%, which is below the required 80%"
          exit 1
        fi
        echo "Total coverage: $total_coverage%"
      
    - name: Upload coverage report
      uses: codecov/codecov-action@v3
      with:
        file: ./merged-coverage.txt
```

## Advanced Techniques

### Technique 1: Command-Based Mapping

For larger projects, you can create a detailed mapping from commands to code paths:

```go
// Create a mapping of commands to code paths
var commandMap = map[string][]string{
	"--help": {
		"main.go:20.20,30.2 8 1",
		"cmd/help.go:10.40,20.2 5 1",
	},
	"--version": {
		"main.go:35.25,38.2 3 1",
		"version/version.go:5.40,10.2 4 1",
	},
	"run": {
		"main.go:45.19,60.2 12 1",
		"cmd/run.go:12.35,45.2 25 1",
	},
	// Add more mappings
}
```

### Technique 2: Source Code Analysis

For automatic mapping generation, analyze your source code to find command handlers:

```go
// Find command handlers in source code
func findCommandHandlers(sourceDir string) (map[string][]string, error) {
	handlers := make(map[string][]string)
	
	// Find all Go files
	goFiles, err := filepath.Glob(filepath.Join(sourceDir, "**/*.go"))
	if err != nil {
		return nil, err
	}
	
	// Look for command registration patterns
	cmdRegex := regexp.MustCompile(`cmd\.Register(?:Command)?\(["']([^"']+)["']`)
	
	for _, file := range goFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		// Find command registrations
		matches := cmdRegex.FindAllSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				command := string(match[1])
				
				// Find the handler function
				// This is a simplification - you'd need more sophisticated
				// analysis to find the actual handler function
				handlers[command] = append(handlers[command], 
					fmt.Sprintf("%s:1.1,100.1 50 1", 
						filepath.Base(file)))
			}
		}
	}
	
	return handlers, nil
}
```

### Technique 3: Custom Coverage Format

For complex projects, consider creating a custom coverage format that explicitly marks synthetic data:

```go
// Write coverage with metadata
func writeCoverageWithMetadata(filename string, realLines, syntheticLines []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// Write metadata
	fmt.Fprintf(file, "// COVERAGE_METADATA\n")
	fmt.Fprintf(file, "// Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "// Real: %d lines\n", len(realLines))
	fmt.Fprintf(file, "// Synthetic: %d lines\n", len(syntheticLines))
	fmt.Fprintf(file, "// COVERAGE_DATA\n")
	
	// Write mode line
	fmt.Fprintln(file, "mode: set")
	
	// Write real coverage with metadata comments
	for _, line := range realLines {
		fmt.Fprintln(file, line)
	}
	
	// Write synthetic coverage with metadata comments
	fmt.Fprintln(file, "// BEGIN_SYNTHETIC_COVERAGE")
	for _, line := range syntheticLines {
		fmt.Fprintln(file, line)
	}
	fmt.Fprintln(file, "// END_SYNTHETIC_COVERAGE")
	
	return nil
}
```

## Key Takeaways

1. **Command Analysis**: Extract and analyze commands from bash scripts
2. **Code Path Mapping**: Map commands to the code paths they exercise
3. **Systematic Approach**: Use a consistent method for generating synthetic coverage
4. **Merge and Visualize**: Combine real and synthetic coverage for complete reports
5. **CI Integration**: Automate the process in your continuous integration pipeline

By following these techniques, you can ensure your coverage reports accurately reflect the code paths exercised by your bash tests, giving you a more complete picture of your test coverage.

## Benefits of This Approach

- More accurate representation of your actual test coverage
- Better visibility into tested code paths, regardless of test method
- Improved confidence in refactoring and changes
- More realistic coverage metrics for CI/CD enforcement
- Encourages comprehensive testing approach

This synthetic coverage approach for bash tests complements your Go tests and gives you a holistic view of your testing strategy.

## Further Reading

- [Go's Coverage Tools Documentation](https://pkg.go.dev/cmd/cover)
- [Bash Testing Best Practices](https://google.github.io/styleguide/shellguide.html)
- [Integrating Coverage with CI/CD Systems](https://about.codecov.io/blog/getting-started-with-code-coverage-for-golang/)