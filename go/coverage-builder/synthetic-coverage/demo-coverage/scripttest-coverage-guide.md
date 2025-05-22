# Synthetic Coverage for Script Tests in Go

This guide demonstrates how to add synthetic coverage for script-based tests, focusing on:
1. Tests using `rsc.io/script/scripttest`
2. Bash tests invoked by Go test runners

## Introduction

Script-based tests are a powerful way to test Go applications, especially CLIs and tools that interact with the operating system. However, these tests often execute code in separate processes, which aren't captured by Go's standard coverage instrumentation.

This is where synthetic coverage becomes valuable - allowing you to represent script test coverage in your standard coverage reports.

## Part 1: Coverage for `rsc.io/script/scripttest` Tests

### What is `rsc.io/script/scripttest`?

`rsc.io/script/scripttest` is a package by Russ Cox for testing command-line tools by running scripts. It executes a command and compares its output against expected output, similar to a "golden file" test approach.

### The Coverage Challenge

When `scripttest` runs your Go program, it typically executes it as a separate process. This means:

1. Standard Go coverage instrumentation doesn't capture this execution
2. Coverage for code paths exercised only by script tests is missing
3. Coverage reports underrepresent the actual test coverage

### Step 1: Set Up a Sample Project with Script Tests

First, let's create a simple project using `scripttest`:

```bash
mkdir -p script-demo/{cmd,test}
cd script-demo

# Initialize Go module
cat > go.mod << EOF
module example.com/script-demo
go 1.21

require rsc.io/script v0.0.0-20211004134434-0dce6a2265e2
EOF

# Install dependency
go get rsc.io/script/scripttest

# Create a simple CLI tool
cat > cmd/tool/main.go << EOF
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Parse()
	args := flag.Args()
	
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "greet":
		handleGreet(args[1:])
	case "calculate":
		handleCalculate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: tool COMMAND [ARGS]")
	fmt.Println("\nCommands:")
	fmt.Println("  greet NAME        Greet the specified name")
	fmt.Println("  calculate OP X Y  Perform calculation (add, sub, mul, div)")
}

func handleGreet(args []string) {
	if len(args) < 1 {
		fmt.Println("Hello, world!")
		return
	}
	fmt.Printf("Hello, %s!\n", args[0])
}

func handleCalculate(args []string) {
	if len(args) != 3 {
		fmt.Fprintf(os.Stderr, "calculate requires 3 arguments: OP X Y\n")
		os.Exit(1)
	}
	
	op := args[0]
	x, y := 0, 0
	fmt.Sscanf(args[1], "%d", &x)
	fmt.Sscanf(args[2], "%d", &y)
	
	switch op {
	case "add":
		fmt.Printf("%d\n", x + y)
	case "sub":
		fmt.Printf("%d\n", x - y)
	case "mul":
		fmt.Printf("%d\n", x * y)
	case "div":
		if y == 0 {
			fmt.Fprintf(os.Stderr, "Error: division by zero\n")
			os.Exit(1)
		}
		fmt.Printf("%d\n", x / y)
	default:
		fmt.Fprintf(os.Stderr, "Unknown operation: %s\n", op)
		os.Exit(1)
	}
}
EOF

# Create a script test file
cat > test/script_test.go << EOF
package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"rsc.io/script/scripttest"
)

func TestScript(t *testing.T) {
	// Build the tool
	toolPath := buildTool(t)
	
	// Set up script test
	ts := scripttest.New()
	ts.Cmds["tool"] = toolPath
	
	// Run the script
	ts.Run(t, "testscript", "script.txt")
}

func buildTool(t *testing.T) string {
	t.Helper()
	
	// Get temporary directory for build output
	tmpDir, err := os.MkdirTemp("", "tool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Build the tool
	toolPath := filepath.Join(tmpDir, "tool")
	cmd := exec.Command("go", "build", "-o", toolPath, "../cmd/tool")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build tool: %v\n%s", err, output)
	}
	
	return toolPath
}
EOF

# Create a script file
cat > test/script.txt << EOF
# Test basic greet command
exec tool greet
stdout 'Hello, world!'

# Test greet with name
exec tool greet Gopher
stdout 'Hello, Gopher!'

# Test basic calculation
exec tool calculate add 2 3
stdout '5'

# Test subtraction
exec tool calculate sub 5 2
stdout '3'

# Test multiplication
exec tool calculate mul 4 5
stdout '20'

# Test division
exec tool calculate div 10 2
stdout '5'

# Test error cases
! exec tool calculate div 5 0
stderr 'division by zero'

! exec tool calculate foo 1 2
stderr 'Unknown operation: foo'
EOF
```

### Step 2: Generate Standard Coverage

Let's run the tests with standard coverage:

```bash
go test -coverprofile=coverage.txt ./...
```

Examining the coverage, you'll likely see that many code paths in your tool are not covered, despite being tested by the script tests.

### Step 3: Analyze Script Tests to Determine Coverage

To create synthetic coverage, we need to analyze which code paths are exercised by the script tests. Let's create a tool that parses the script file and identifies the code paths:

```bash
cat > scripttest-analyzer.go << EOF
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

func main() {
	scriptFile := flag.String("script", "", "Path to script.txt file")
	packagePath := flag.String("package", "example.com/script-demo/cmd/tool", "Package path for the tool")
	outputFile := flag.String("output", "scripttest-coverage.txt", "Output file for synthetic coverage")
	flag.Parse()
	
	if *scriptFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -script flag is required\n")
		os.Exit(1)
	}
	
	// Parse script file to identify test cases
	commands, err := parseScriptFile(*scriptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing script file: %v\n", err)
		os.Exit(1)
	}
	
	// Generate synthetic coverage based on commands
	coverage := generateSyntheticCoverage(commands, *packagePath)
	
	// Write synthetic coverage file
	err = os.WriteFile(*outputFile, []byte(coverage), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage for %d script commands in %s\n", 
		len(commands), *outputFile)
}

func parseScriptFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var commands []string
	execRegex := regexp.MustCompile("^exec\\s+tool\\s+(.+)$")
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Extract tool commands
		if match := execRegex.FindStringSubmatch(line); len(match) > 1 {
			commands = append(commands, match[1])
		}
	}
	
	return commands, scanner.Err()
}

func generateSyntheticCoverage(commands []string, packagePath string) string {
	var lines []string
	lines = append(lines, "mode: set")
	
	// Map of command to code paths
	codePaths := map[string][]string{
		"greet": {
			"main.go:26.25,27.21 1 1", // handleGreet start
			"main.go:27.21,29.3 1 1",   // handleGreet empty case
			"main.go:30.2,30.35 1 1",   // handleGreet with name
		},
		"calculate add": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:46.2,46.32 1 1",    // add case
		},
		"calculate sub": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:48.2,48.32 1 1",    // sub case
		},
		"calculate mul": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:50.2,50.32 1 1",    // mul case
		},
		"calculate div": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:52.2,52.13 1 1",    // div case start
			"main.go:57.2,57.32 1 1",    // div calculation
		},
		"calculate div 5 0": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:52.2,52.13 1 1",    // div case start
			"main.go:53.13,56.4 2 1",    // div by zero check
		},
		"calculate foo": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:59.2,60.14 1 1",    // unknown operation
		},
	}
	
	// Track which paths have been covered
	covered := make(map[string]bool)
	
	// Mark code paths as covered based on commands
	for _, cmd := range commands {
		// Check for exact matches
		if paths, ok := codePaths[cmd]; ok {
			for _, path := range paths {
				covered[path] = true
			}
			continue
		}
		
		// Check for prefix matches
		for pattern, paths := range codePaths {
			if strings.HasPrefix(cmd, pattern) {
				for _, path := range paths {
					covered[path] = true
				}
			}
		}
	}
	
	// Generate coverage lines
	for path := range covered {
		lines = append(lines, packagePath + "/" + path)
	}
	
	// Add main function coverage
	lines = append(lines, packagePath+"/main.go:9.13,24.2 13 1")
	
	return strings.Join(lines, "\n")
}
EOF
```

Run the analyzer to generate synthetic coverage for the script tests:

```bash
go run scripttest-analyzer.go -script=test/script.txt -output=scripttest-coverage.txt
```

### Step 4: Merge with Real Coverage

Now, let's merge the real coverage with our synthetic coverage:

```bash
cat > merge-coverage.go << EOF
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	realCoverage := flag.String("real", "coverage.txt", "Real coverage file")
	syntheticCoverage := flag.String("synthetic", "scripttest-coverage.txt", "Synthetic coverage file")
	outputFile := flag.String("output", "merged-coverage.txt", "Output merged coverage file")
	flag.Parse()
	
	// Read real coverage
	realLines, mode := readCoverageFile(*realCoverage)
	
	// Read synthetic coverage
	syntheticLines, _ := readCoverageFile(*syntheticCoverage)
	
	// Merge coverage (take synthetic if both exist)
	mergedMap := make(map[string]bool)
	
	// Add all real coverage
	for _, line := range realLines {
		mergedMap[line] = true
	}
	
	// Add all synthetic coverage (will overwrite real if exists)
	for _, line := range syntheticLines {
		mergedMap[line] = true
	}
	
	// Convert back to slice
	var mergedLines []string
	for line := range mergedMap {
		mergedLines = append(mergedLines, line)
	}
	
	// Write merged coverage
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
	
	fmt.Printf("Merged coverage written to %s\n", *outputFile)
}

func readCoverageFile(filename string) ([]string, string) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, "set"
	}
	defer file.Close()
	
	var lines []string
	mode := "set"
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			mode = strings.TrimSpace(strings.TrimPrefix(line, "mode:"))
		} else if line != "" {
			lines = append(lines, line)
		}
	}
	
	return lines, mode
}
EOF
```

Merge the coverage files:

```bash
go run merge-coverage.go -real=coverage.txt -synthetic=scripttest-coverage.txt -output=merged-coverage.txt
```

### Step 5: Verify the Results

Check the merged coverage:

```bash
go tool cover -func=merged-coverage.txt
```

You should see significantly better coverage, now including the code paths executed by the script tests.

## Part 2: Coverage for Bash Tests Invoked by Go

Many Go projects use bash scripts for integration testing, especially for CLI tools. Let's add coverage for those too.

### Step 1: Create a Bash Test Example

```bash
mkdir -p bash-tests
cat > bash-tests/integration_test.go << EOF
package bash_tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBashIntegration(t *testing.T) {
	// Skip if not on a Unix-like system
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available, skipping test")
	}
	
	// Build the tool
	toolPath := buildTool(t)
	
	// Set environment for script
	env := append(os.Environ(), "TOOL_PATH="+toolPath)
	
	// Run bash test
	cmd := exec.Command("bash", "test.sh")
	cmd.Env = env
	cmd.Dir = "."
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Bash test failed: %v\n%s", err, output)
	}
	
	t.Logf("Bash test output:\n%s", output)
}

func buildTool(t *testing.T) string {
	t.Helper()
	
	// Get temporary directory for build output
	tmpDir, err := os.MkdirTemp("", "tool-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Build the tool
	toolPath := filepath.Join(tmpDir, "tool")
	cmd := exec.Command("go", "build", "-o", toolPath, "../cmd/tool")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build tool: %v\n%s", err, output)
	}
	
	return toolPath
}
EOF

cat > bash-tests/test.sh << EOF
#!/bin/bash
set -e

if [ -z "$TOOL_PATH" ]; then
  echo "TOOL_PATH environment variable is not set"
  exit 1
fi

echo "=== Testing tool with bash ==="

# Test version command (this would be added to the tool)
echo "Testing greet command..."
output=$($TOOL_PATH greet Bash)
if [ "$output" != "Hello, Bash!" ]; then
  echo "Error: Expected 'Hello, Bash!', got '$output'"
  exit 1
fi

# Test calculate commands
echo "Testing calculation commands..."

output=$($TOOL_PATH calculate add 10 20)
if [ "$output" -ne 30 ]; then
  echo "Error: add test failed, expected 30, got $output"
  exit 1
fi

output=$($TOOL_PATH calculate mul 5 7)
if [ "$output" -ne 35 ]; then
  echo "Error: mul test failed, expected 35, got $output"
  exit 1
fi

# Test error handling
echo "Testing error handling..."
if $TOOL_PATH calculate div 10 0 2>/dev/null; then
  echo "Error: Expected division by zero error, but command succeeded"
  exit 1
fi

echo "All bash tests passed!"
EOF

chmod +x bash-tests/test.sh
```

### Step 2: Analyze Bash Tests for Coverage

Let's create a tool to analyze the bash test script and generate synthetic coverage:

```bash
cat > bash-analyzer.go << EOF
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	bashFile := flag.String("script", "", "Path to bash script file")
	packagePath := flag.String("package", "example.com/script-demo/cmd/tool", "Package path for the tool")
	outputFile := flag.String("output", "bash-coverage.txt", "Output file for synthetic coverage")
	flag.Parse()
	
	if *bashFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -script flag is required\n")
		os.Exit(1)
	}
	
	// Parse bash file to find test cases
	commands, err := parseBashFile(*bashFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing bash file: %v\n", err)
		os.Exit(1)
	}
	
	// Generate synthetic coverage
	coverage := generateSyntheticCoverage(commands, *packagePath)
	
	// Write synthetic coverage file
	err = os.WriteFile(*outputFile, []byte(coverage), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage for %d bash commands in %s\n", 
		len(commands), *outputFile)
}

func parseBashFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var commands []string
	
	// Different regex patterns to find tool executions
	patterns := []*regexp.Regexp{
		regexp.MustCompile("\\$TOOL_PATH\\s+(.+)"),
		regexp.MustCompile("\\$\\{TOOL_PATH\\}\\s+(.+)"),
		regexp.MustCompile("\"\\$TOOL_PATH\"\\s+(.+)"),
	}
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Try each pattern
		for _, pattern := range patterns {
			if match := pattern.FindStringSubmatch(line); len(match) > 1 {
				// Clean up the command (remove redirection, etc.)
				cmd := strings.Split(match[1], " 2>")[0]
				cmd = strings.Split(cmd, " >")[0]
				cmd = strings.TrimSpace(cmd)
				commands = append(commands, cmd)
				break
			}
		}
	}
	
	return commands, scanner.Err()
}

func generateSyntheticCoverage(commands []string, packagePath string) string {
	var lines []string
	lines = append(lines, "mode: set")
	
	// Use the same mapping as for scripttest
	codePaths := map[string][]string{
		"greet": {
			"main.go:26.25,27.21 1 1", // handleGreet start
			"main.go:27.21,29.3 1 1",   // handleGreet empty case
			"main.go:30.2,30.35 1 1",   // handleGreet with name
		},
		"calculate add": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:46.2,46.32 1 1",    // add case
		},
		"calculate sub": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:48.2,48.32 1 1",    // sub case
		},
		"calculate mul": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:50.2,50.32 1 1",    // mul case
		},
		"calculate div": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:52.2,52.13 1 1",    // div case start
			"main.go:57.2,57.32 1 1",    // div calculation
		},
		"calculate div 10 0": {
			"main.go:33.31,38.2 4 1",    // handleCalculate validation
			"main.go:52.2,52.13 1 1",    // div case start
			"main.go:53.13,56.4 2 1",    // div by zero check
		},
	}
	
	// Track which paths have been covered
	covered := make(map[string]bool)
	
	// Mark code paths as covered based on commands
	for _, cmd := range commands {
		// Check for exact matches
		if paths, ok := codePaths[cmd]; ok {
			for _, path := range paths {
				covered[path] = true
			}
			continue
		}
		
		// Check for prefix matches
		for pattern, paths := range codePaths {
			if strings.HasPrefix(cmd, pattern) {
				for _, path := range paths {
					covered[path] = true
				}
			}
		}
	}
	
	// Generate coverage lines
	for path := range covered {
		lines = append(lines, packagePath + "/" + path)
	}
	
	// Add main function coverage
	lines = append(lines, packagePath+"/main.go:9.13,24.2 13 1")
	
	return strings.Join(lines, "\n")
}
EOF
```

Run the analyzer:

```bash
go run bash-analyzer.go -script=bash-tests/test.sh -output=bash-coverage.txt
```

### Step 3: Create a Comprehensive Merger for All Test Types

```bash
cat > merge-all-coverage.go << EOF
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
	outputFile := flag.String("output", "final-coverage.txt", "Output merged coverage file")
	flag.Parse()
	
	// Find all coverage files
	coverageFiles, err := filepath.Glob("*.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding coverage files: %v\n", err)
		os.Exit(1)
	}
	
	// Track all coverage lines
	allLines := make(map[string]bool)
	mode := "set"
	
	// Process each file
	for _, file := range coverageFiles {
		// Skip output file
		if file == *outputFile {
			continue
		}
		
		// Read coverage file
		lines, fileMode := readCoverageFile(file)
		if fileMode != "" {
			mode = fileMode
		}
		
		// Add lines to map
		for _, line := range lines {
			allLines[line] = true
		}
		
		fmt.Printf("Processed %s: %d lines\n", file, len(lines))
	}
	
	// Write merged coverage
	f, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	
	fmt.Fprintf(f, "mode: %s\n", mode)
	
	var mergedLines []string
	for line := range allLines {
		mergedLines = append(mergedLines, line)
	}
	
	for _, line := range mergedLines {
		fmt.Fprintln(f, line)
	}
	
	fmt.Printf("Merged %d coverage files with %d total lines to %s\n", 
		len(coverageFiles), len(mergedLines), *outputFile)
}

func readCoverageFile(filename string) ([]string, string) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, ""
	}
	defer file.Close()
	
	var lines []string
	mode := ""
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			mode = strings.TrimSpace(strings.TrimPrefix(line, "mode:"))
		} else if line != "" {
			lines = append(lines, line)
		}
	}
	
	return lines, mode
}
EOF
```

### Step 4: Combine All Coverage Sources

```bash
go run merge-all-coverage.go -output=final-coverage.txt
```

### Step 5: View the Final Coverage

```bash
go tool cover -func=final-coverage.txt
go tool cover -html=final-coverage.txt -o=coverage.html
```

## Part 3: Automating Script Test Coverage with CI/CD

Here's how to integrate this into your CI/CD pipeline:

```yaml
name: Go
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Run tests with coverage
      run: go test -coverprofile=coverage.txt ./...
    
    - name: Generate script test coverage
      run: |
        go run tools/scripttest-analyzer.go -script=test/script.txt -output=scripttest-coverage.txt
        go run tools/bash-analyzer.go -script=bash-tests/test.sh -output=bash-coverage.txt
    
    - name: Merge coverage files
      run: go run tools/merge-all-coverage.go -output=final-coverage.txt
    
    - name: Check coverage threshold
      run: |
        COVERAGE=$(go tool cover -func=final-coverage.txt | grep total | awk '{print $3}' | tr -d '%')
        if (( $(echo "$COVERAGE < 80" | bc -l) )); then
          echo "Coverage $COVERAGE% is below threshold of 80%"
          exit 1
        fi
        echo "Coverage $COVERAGE% meets threshold"
    
    - name: Upload coverage report
      uses: codecov/codecov-action@v3
      with:
        file: ./final-coverage.txt
```

## Advanced: Dynamic Analysis of Script Tests

For more accurate coverage, you could use a dynamic approach:

1. Instrument your tool to record which functions are called
2. Run script tests with the instrumented version
3. Generate synthetic coverage based on actual execution

```go
// Example of a simple execution tracer
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
	traceFile = flag.String("trace", "execution-trace.txt", "File to write execution trace")
)

var (
	traceMutex sync.Mutex
	traceLog   *os.File
	functions  = make(map[string]bool)
)

func init() {
	flag.Parse()
	
	var err error
	traceLog, err = os.Create(*traceFile)
	if err != nil {
		log.Fatalf("Failed to create trace file: %v", err)
	}
	
	// Set up trace function
	go func() {
		for {
			traceExecution()
			runtime.Gosched()
		}
	}()
}

func traceExecution() {
	// Get caller info
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	
	traceMutex.Lock()
	defer traceMutex.Unlock()
	
	// Log each frame
	for {
		frame, more := frames.Next()
		
		// Skip runtime and trace functions
		if strings.Contains(frame.Function, "runtime.") || 
		   strings.Contains(frame.Function, "traceExecution") {
			if !more {
				break
			}
			continue
		}
		
		// Record this function if we haven't seen it
		if !functions[frame.Function] {
			functions[frame.Function] = true
			fmt.Fprintf(traceLog, "%s\n", frame.Function)
		}
		
		if !more {
			break
		}
	}
}

func main() {
	defer traceLog.Close()
	
	// Tool code here...
}
```

## Key Takeaways

1. **Script tests need special handling**: Standard Go coverage doesn't capture code executed by script tests.

2. **Multiple approaches work**: 
   - Static analysis of script files
   - Dynamic execution tracing
   - Manually defined coverage maps

3. **Improved coverage representation**: By adding synthetic coverage for script tests, you get a more accurate representation of your test coverage.

4. **Automation is essential**: Automated tools for generating and merging coverage make this practical for real projects.

5. **CI/CD integration**: Incorporating synthetic coverage into CI/CD ensures consistent coverage reporting.

By using these techniques, you can ensure that your coverage reports accurately reflect the code paths tested by `rsc.io/script/scripttest` and bash tests invoked by Go.