# Advanced Synthetic Coverage for `rsc.io/script/scripttest` Tests

This guide explores advanced techniques for generating and maintaining synthetic coverage for tests using the `rsc.io/script/scripttest` package developed by Russ Cox.

## Introduction to `scripttest`

`rsc.io/script/scripttest` is a package that enables script-based testing for Go programs. It works by:

1. Running your executable in a controlled environment
2. Feeding it script-defined inputs
3. Verifying outputs against expected values
4. Checking exit codes and error conditions

While ideal for testing command-line tools, `scripttest` tests aren't captured by Go's standard coverage instrumentation because they execute your code in a separate process.

## Advanced Coverage Techniques

### 1. Compiler-Assisted Coverage Tracking

This approach modifies your program at compile time to record execution paths during script tests:

```go
// cmd/mytool/main.go with coverage hooks
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

// Coverage instrumentation
var (
	coverageEnabled = flag.Bool("coverage-track", false, "Enable coverage tracking")
	coverageFile    = flag.String("coverage-output", "", "Coverage output file")
	
	covMutex sync.Mutex
	covData  = make(map[string]bool)
)

// Register a file/line as covered
func markCovered(file string, line int) {
	if !*coverageEnabled {
		return
	}
	
	covMutex.Lock()
	defer covMutex.Unlock()
	
	covData[fmt.Sprintf("%s:%d", file, line)] = true
}

// Coverage hook that goes at strategic points in your code
func cover() {
	if !*coverageEnabled {
		return
	}
	
	// Get caller info
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return
	}
	
	// Only track coverage for our own packages
	if !strings.Contains(file, "example.com/myproject") {
		return
	}
	
	// Record coverage
	markCovered(file, line)
}

// Write coverage data when program exits
func writeCoverageData() {
	if !*coverageEnabled || *coverageFile == "" {
		return
	}
	
	covMutex.Lock()
	defer covMutex.Unlock()
	
	// Convert to Go coverage format
	var lines []string
	lines = append(lines, "mode: set")
	
	for location := range covData {
		parts := strings.Split(location, ":")
		if len(parts) != 2 {
			continue
		}
		
		file := parts[0]
		line, _ := strconv.Atoi(parts[1])
		
		// Create synthetic coverage line
		// In a real implementation, you'd gather more data about
		// the code structure to get proper column and statement info
		coverageLine := fmt.Sprintf("%s:%d.1,%d.1 1 1", file, line, line)
		lines = append(lines, coverageLine)
	}
	
	// Write coverage file
	os.WriteFile(*coverageFile, []byte(strings.Join(lines, "\n")), 0644)
}

func main() {
	flag.Parse()
	
	if *coverageEnabled {
		defer writeCoverageData()
	}
	
	// Insert coverage hooks in key functions
	cover() // Cover main function entry
	
	// Rest of your program...
}

// Example of coverage hooks in other functions
func handleCommand(cmd string, args []string) {
	cover() // Cover function entry
	
	switch cmd {
	case "version":
		cover() // Cover this branch
		showVersion()
	case "run":
		cover() // Cover this branch
		runCommand(args)
	default:
		cover() // Cover this branch
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}
```

Modify your script test to enable coverage tracking:

```go
func TestScript(t *testing.T) {
	// Build the tool with coverage tracking enabled
	tmpDir, err := os.MkdirTemp("", "coverage")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	
	coverageFile := filepath.Join(tmpDir, "script-coverage.txt")
	
	cmd := exec.Command("go", "build", 
		"-o", filepath.Join(tmpDir, "mytool"),
		"../cmd/mytool")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	
	// Set up script test with coverage options
	ts := scripttest.New()
	ts.Cmds["mytool"] = fmt.Sprintf("%s/mytool -coverage-track -coverage-output=%s", 
		tmpDir, coverageFile)
	
	// Run the script
	ts.Run(t, "testscript", "script.txt")
	
	// Read and process coverage data
	if _, err := os.Stat(coverageFile); err == nil {
		processCoverageFile(t, coverageFile)
	}
}

func processCoverageFile(t *testing.T, coverageFile string) {
	// Read the coverage file generated during script test
	data, err := os.ReadFile(coverageFile)
	if err != nil {
		t.Logf("Failed to read coverage file: %v", err)
		return
	}
	
	// Append to the main coverage file or process as needed
	mainCoverage := "coverage.txt"
	f, err := os.OpenFile(mainCoverage, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Logf("Failed to open main coverage file: %v", err)
		return
	}
	defer f.Close()
	
	// Skip mode line when appending
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 && strings.HasPrefix(line, "mode:") {
			continue
		}
		if line != "" {
			fmt.Fprintln(f, line)
		}
	}
	
	t.Logf("Added script test coverage to %s", mainCoverage)
}
```

### 2. Script Analysis with AST Parsing

For more accurate coverage generation, parse your script files using the Go AST (Abstract Syntax Tree) to determine which functions are exercised:

```go
// scripttest-ast-analyzer.go
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	scriptFile = flag.String("script", "", "Path to scripttest file")
	sourceDir  = flag.String("source", ".", "Source directory")
	outputFile = flag.String("output", "script-coverage.txt", "Output coverage file")
	packagePrefix = flag.String("package", "example.com/myproject", "Package prefix")
)

// Command Pattern represents a command pattern to search for in the script
type CommandPattern struct {
	Regex   *regexp.Regexp
	Handler string // The function that handles this command
}

func main() {
	flag.Parse()
	
	if *scriptFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -script flag is required\n")
		os.Exit(1)
	}
	
	// Extract commands from script
	commands, err := extractCommands(*scriptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting commands: %v\n", err)
		os.Exit(1)
	}
	
	// Build command patterns
	patterns := buildCommandPatterns()
	
	// Find matching handlers for each command
	handlers := findMatchingHandlers(commands, patterns)
	
	// Analyze source code to find function implementations
	coverage, err := generateCoverageFromHandlers(handlers, *sourceDir, *packagePrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating coverage: %v\n", err)
		os.Exit(1)
	}
	
	// Write coverage file
	if err := os.WriteFile(*outputFile, []byte(coverage), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing coverage file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage for %d commands in %s\n", 
		len(commands), *outputFile)
}

func extractCommands(scriptFile string) ([]string, error) {
	data, err := os.ReadFile(scriptFile)
	if err != nil {
		return nil, err
	}
	
	var commands []string
	cmdRegex := regexp.MustCompile(`(?m)^exec\s+(\S+)(.*)$`)
	
	matches := cmdRegex.FindAllStringSubmatch(string(data), -1)
	for _, match := range matches {
		if len(match) >= 3 {
			// Check if it's our tool
			cmd := strings.TrimSpace(match[1])
			if cmd == "mytool" {
				args := strings.TrimSpace(match[2])
				commands = append(commands, args)
			}
		}
	}
	
	return commands, nil
}

func buildCommandPatterns() []CommandPattern {
	return []CommandPattern{
		{
			Regex:   regexp.MustCompile(`^version`),
			Handler: "handleVersion",
		},
		{
			Regex:   regexp.MustCompile(`^run\s`),
			Handler: "handleRun",
		},
		{
			Regex:   regexp.MustCompile(`^process\s+(\S+)`),
			Handler: "handleProcess",
		},
		// Add more patterns as needed
	}
}

func findMatchingHandlers(commands []string, patterns []CommandPattern) []string {
	var handlers []string
	uniqueHandlers := make(map[string]bool)
	
	// Always include main function
	uniqueHandlers["main"] = true
	
	for _, cmd := range commands {
		for _, pattern := range patterns {
			if pattern.Regex.MatchString(cmd) {
				if !uniqueHandlers[pattern.Handler] {
					uniqueHandlers[pattern.Handler] = true
					handlers = append(handlers, pattern.Handler)
				}
			}
		}
	}
	
	return handlers
}

func generateCoverageFromHandlers(handlers []string, sourceDir, packagePrefix string) (string, error) {
	// Find all Go files
	var goFiles []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	
	// Store coverage lines
	coverageLines := []string{"mode: set"}
	
	// Track functions we've found
	foundHandlers := make(map[string]bool)
	
	// Process each file
	for _, file := range goFiles {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			continue
		}
		
		// Process functions
		ast.Inspect(node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				// Check if this is one of our handlers
				if contains(handlers, fn.Name.Name) {
					// Mark as found
					foundHandlers[fn.Name.Name] = true
					
					// Get position info
					startPos := fset.Position(fn.Pos())
					endPos := fset.Position(fn.End())
					
					// Create relative path from source dir
					relPath, err := filepath.Rel(sourceDir, file)
					if err != nil {
						relPath = file
					}
					
					// Generate coverage line
					coverageLine := fmt.Sprintf("%s/%s:%d.1,%d.1 %d 1", 
						packagePrefix, 
						relPath,
						startPos.Line,
						endPos.Line,
						endPos.Line - startPos.Line + 1)
					
					coverageLines = append(coverageLines, coverageLine)
					
					// Process call expressions within this function
					findCallExpressions(fn, func(callee string) {
						if !foundHandlers[callee] && !contains(handlers, callee) {
							handlers = append(handlers, callee)
						}
					})
				}
			}
			return true
		})
	}
	
	// Warn about handlers we couldn't find
	for _, handler := range handlers {
		if !foundHandlers[handler] {
			fmt.Fprintf(os.Stderr, "Warning: Could not find implementation of %s\n", handler)
		}
	}
	
	return strings.Join(coverageLines, "\n"), nil
}

func findCallExpressions(fn *ast.FuncDecl, callback func(string)) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok {
				callback(ident.Name)
			}
		}
		return true
	})
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
```

### 3. Binary Instrumentation & Call Tracing

For the most accurate coverage data, use binary instrumentation to trace function calls during script tests:

```go
// Install the library first:
// go get github.com/google/gops

// cmd/mytool/trace.go
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

var (
	// Tracing configuration
	traceEnabled = os.Getenv("TRACE_ENABLED") == "1"
	tracePath    = os.Getenv("TRACE_PATH")
	
	// Tracing state
	traceStarted  bool
	traceMutex    sync.Mutex
	traceFile     *os.File
	traceFunctions = make(map[string]bool)
	traceTicker    *time.Ticker
)

// StartTracing begins function call tracing
func StartTracing() error {
	if !traceEnabled || tracePath == "" {
		return nil
	}
	
	traceMutex.Lock()
	defer traceMutex.Unlock()
	
	if traceStarted {
		return nil
	}
	
	var err error
	traceFile, err = os.Create(tracePath)
	if err != nil {
		return fmt.Errorf("failed to create trace file: %w", err)
	}
	
	// Start trace ticker
	traceTicker = time.NewTicker(10 * time.Millisecond)
	go func() {
		for range traceTicker.C {
			captureTrace()
		}
	}()
	
	traceStarted = true
	return nil
}

// StopTracing ends function call tracing
func StopTracing() error {
	if !traceEnabled || !traceStarted {
		return nil
	}
	
	traceMutex.Lock()
	defer traceMutex.Unlock()
	
	if traceTicker != nil {
		traceTicker.Stop()
	}
	
	// Capture final trace
	captureTrace()
	
	// Write coverage file in Go format
	fmt.Fprintln(traceFile, "mode: set")
	for fn := range traceFunctions {
		fmt.Fprintln(traceFile, fn+":1.1,100.1 1 1")
	}
	
	if err := traceFile.Close(); err != nil {
		return fmt.Errorf("error closing trace file: %w", err)
	}
	
	traceStarted = false
	return nil
}

// captureTrace grabs the current goroutine stack and records function calls
func captureTrace() {
	// Capture goroutine stacks
	err := pprof.Lookup("goroutine").WriteTo(os.Stderr, 1)
	if err != nil {
		return
	}
	
	// Get all goroutines
	buf := make([]byte, 10240)
	n := runtime.Stack(buf, true)
	
	// Parse stack trace to find functions
	traceMutex.Lock()
	defer traceMutex.Unlock()
	
	lines := strings.Split(string(buf[:n]), "\n")
	for _, line := range lines {
		// Extract function names
		if strings.Contains(line, "example.com/myproject") {
			// Extract just the function name
			parts := strings.Fields(line)
			if len(parts) > 0 {
				funcName := parts[0]
				if !strings.Contains(funcName, "captureTrace") &&
				   !strings.Contains(funcName, "StartTracing") &&
				   !strings.Contains(funcName, "StopTracing") {
					traceFunctions[funcName] = true
				}
			}
		}
	}
}
```

Modify your script test:

```go
func TestScript(t *testing.T) {
	// Build tool
	toolPath := buildTool(t)
	
	// Create trace directory
	traceDir, err := os.MkdirTemp("", "trace")
	if err != nil {
		t.Fatalf("Failed to create trace dir: %v", err)
	}
	defer os.RemoveAll(traceDir)
	
	tracePath := filepath.Join(traceDir, "trace.txt")
	
	// Set up script test
	ts := scripttest.New()
	ts.Setenv("TRACE_ENABLED", "1")
	ts.Setenv("TRACE_PATH", tracePath)
	ts.Cmds["mytool"] = toolPath
	
	// Run the script
	ts.Run(t, "testscript", "script.txt")
	
	// Process trace file
	if _, err := os.Stat(tracePath); err == nil {
		convertTraceToSyntheticCoverage(t, tracePath)
	}
}

func convertTraceToSyntheticCoverage(t *testing.T, tracePath string) {
	// Read trace file
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Logf("Failed to read trace file: %v", err)
		return
	}
	
	// Parse trace data and convert to coverage format
	// Implementation details would depend on your trace format
	
	// Merge with existing coverage
	// ...
}
```

### 4. Cross-Reference Script Files with AST

This advanced technique cross-references script commands with your program's AST to identify the exact lines of code that should be covered:

```go
// script-ast-xref.go
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CommandHandler maps a command pattern to a handler path
type CommandHandler struct {
	Regex  *regexp.Regexp
	Paths  []string
}

func main() {
	scriptFile := flag.String("script", "", "Script file to analyze")
	sourceDir := flag.String("source", ".", "Source directory")
	outputFile := flag.String("output", "scripttest-coverage.txt", "Output coverage file")
	packagePrefix := flag.String("package", "example.com/myproject", "Package prefix")
	flag.Parse()
	
	if *scriptFile == "" {
		fmt.Fprintf(os.Stderr, "Script file is required\n")
		os.Exit(1)
	}
	
	// Extract commands from script
	commands, err := extractCommands(*scriptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting commands: %v\n", err)
		os.Exit(1)
	}
	
	// Build command-to-handler map
	handlerMap := buildHandlerMap()
	
	// Build AST map of the source code
	sourceMap, err := buildSourceMap(*sourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building source map: %v\n", err)
		os.Exit(1)
	}
	
	// Map commands to source code locations
	coverLines := mapCommandsToSource(commands, handlerMap, sourceMap, *packagePrefix)
	
	// Write coverage file
	coverage := strings.Join(append([]string{"mode: set"}, coverLines...), "\n")
	if err := os.WriteFile(*outputFile, []byte(coverage), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage for %d script commands in %s\n",
		len(commands), *outputFile)
}

func extractCommands(scriptFile string) ([]string, error) {
	// Same implementation as before
	// ...
}

func buildHandlerMap() map[string][]string {
	// Map commands to handlers and their dependencies
	// This could be read from a configuration file for more flexibility
	return map[string][]string{
		"version": {
			"main.go:handleVersion", 
			"version/version.go:GetVersion",
		},
		"run": {
			"main.go:handleRun",
			"run/runner.go:RunCommand",
			"run/executor.go:Execute",
		},
		"process": {
			"main.go:handleProcess",
			"processor/processor.go:Process", 
			"processor/file.go:ReadFile",
			"processor/file.go:WriteFile",
		},
		// Add more mappings as needed
	}
}

type SourceInfo struct {
	FilePath string
	StartLine int
	EndLine int
	Functions map[string]*FunctionInfo
}

type FunctionInfo struct {
	Name string
	StartLine int
	EndLine int
	Calls []string
}

func buildSourceMap(sourceDir string) (map[string]*SourceInfo, error) {
	sourceMap := make(map[string]*SourceInfo)
	
	// Find all Go files
	var goFiles []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	
	// Process each file
	for _, file := range goFiles {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			continue
		}
		
		// Get relative path
		relPath, err := filepath.Rel(sourceDir, file)
		if err != nil {
			relPath = file
		}
		
		// Create source info
		info := &SourceInfo{
			FilePath: relPath,
			Functions: make(map[string]*FunctionInfo),
		}
		sourceMap[relPath] = info
		
		// Process functions
		ast.Inspect(node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				// Get function name
				name := fn.Name.Name
				if fn.Recv != nil {
					// For methods, include the receiver type
					if len(fn.Recv.List) > 0 {
						if t, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
							if ident, ok := t.X.(*ast.Ident); ok {
								name = ident.Name + "." + name
							}
						} else if ident, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
							name = ident.Name + "." + name
						}
					}
				}
				
				// Get position info
				startPos := fset.Position(fn.Pos())
				endPos := fset.Position(fn.End())
				
				// Create function info
				fnInfo := &FunctionInfo{
					Name: name,
					StartLine: startPos.Line,
					EndLine: endPos.Line,
				}
				info.Functions[name] = fnInfo
				
				// Find function calls
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						if ident, ok := call.Fun.(*ast.Ident); ok {
							fnInfo.Calls = append(fnInfo.Calls, ident.Name)
						} else if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if x, ok := sel.X.(*ast.Ident); ok {
								fnInfo.Calls = append(fnInfo.Calls, x.Name+"."+sel.Sel.Name)
							}
						}
					}
					return true
				})
			}
			return true
		})
	}
	
	return sourceMap, nil
}

func mapCommandsToSource(commands []string, handlerMap map[string][]string, 
	sourceMap map[string]*SourceInfo, packagePrefix string) []string {
		
	// Track coverage lines
	coverageSet := make(map[string]bool)
	
	// Process each command
	for _, cmd := range commands {
		// Extract base command
		baseCmd := strings.Fields(cmd)[0]
		
		// Find matching handlers
		for pattern, handlers := range handlerMap {
			if strings.HasPrefix(baseCmd, pattern) {
				// Process each handler
				for _, handler := range handlers {
					// Split file and function
					parts := strings.Split(handler, ":")
					if len(parts) != 2 {
						continue
					}
					
					file := parts[0]
					funcName := parts[1]
					
					// Find in source map
					if sourceInfo, ok := sourceMap[file]; ok {
						if fnInfo, ok := sourceInfo.Functions[funcName]; ok {
							// Generate coverage line
							coverageLine := fmt.Sprintf("%s/%s:%d.1,%d.1 %d 1",
								packagePrefix,
								file,
								fnInfo.StartLine,
								fnInfo.EndLine,
								fnInfo.EndLine - fnInfo.StartLine + 1)
							
							coverageSet[coverageLine] = true
							
							// Process function calls recursively
							processFunctionCalls(fnInfo, sourceMap, packagePrefix, coverageSet)
						}
					}
				}
			}
		}
	}
	
	// Convert to slice
	var coverLines []string
	for line := range coverageSet {
		coverLines = append(coverLines, line)
	}
	
	return coverLines
}

func processFunctionCalls(fnInfo *FunctionInfo, sourceMap map[string]*SourceInfo,
	packagePrefix string, coverageSet map[string]bool) {
		
	for _, call := range fnInfo.Calls {
		// Find called function in source map
		for file, sourceInfo := range sourceMap {
			if called, ok := sourceInfo.Functions[call]; ok {
				// Generate coverage line
				coverageLine := fmt.Sprintf("%s/%s:%d.1,%d.1 %d 1",
					packagePrefix,
					file,
					called.StartLine,
					called.EndLine,
					called.EndLine - called.StartLine + 1)
				
				// Add to set if not already present
				if !coverageSet[coverageLine] {
					coverageSet[coverageLine] = true
					
					// Process this function's calls recursively
					processFunctionCalls(called, sourceMap, packagePrefix, coverageSet)
				}
			}
		}
	}
}
```

## Integration with Production Systems

### CI/CD Pipeline Integration

Here's how to integrate these advanced techniques into your CI/CD pipeline:

```yaml
# .github/workflows/coverage.yml
name: Coverage

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
    
    - name: Run standard tests with coverage
      run: go test -coverprofile=unit-coverage.txt ./...
    
    - name: Run script tests
      run: |
        mkdir -p coverage
        TRACE_ENABLED=1 TRACE_PATH=$(pwd)/coverage/trace.txt \
          go test -v ./scripttest/...
    
    - name: Process script test coverage
      run: |
        go run tools/script-ast-xref.go \
          -script=./scripttest/script.txt \
          -source=. \
          -output=script-coverage.txt \
          -package=example.com/myproject
    
    - name: Merge coverage profiles
      run: |
        # Combine unit test coverage and script test coverage
        go run tools/merge-coverage.go \
          -standard=unit-coverage.txt \
          -synthetic=script-coverage.txt \
          -output=coverage.txt
    
    - name: Generate coverage report
      run: |
        # Generate summary
        go tool cover -func=coverage.txt > coverage-summary.txt
        
        # Generate HTML report
        go tool cover -html=coverage.txt -o=coverage.html
    
    - name: Check coverage thresholds
      run: |
        # Extract total coverage percentage
        COVERAGE=$(grep -oP 'total:.*?(\d+\.\d+)%' coverage-summary.txt | grep -oP '\d+\.\d+')
        
        # Check against threshold
        if (( $(echo "$COVERAGE < 80" | bc -l) )); then
          echo "Coverage is below threshold: $COVERAGE% < 80%"
          exit 1
        fi
        
        echo "Coverage meets threshold: $COVERAGE% >= 80%"
    
    - name: Upload coverage artifacts
      uses: actions/upload-artifact@v3
      with:
        name: coverage-reports
        path: |
          coverage.txt
          coverage-summary.txt
          coverage.html
```

### Advanced Coverage Reporting Portal

For larger projects, consider setting up a coverage reporting portal:

```go
// coverage-server.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CoverageFile struct {
	Path       string
	Name       string
	Date       time.Time
	Percentage float64
	Synthetic  bool
}

func main() {
	port := flag.Int("port", 8080, "Server port")
	coverageDir := flag.String("dir", "./coverage", "Coverage directory")
	flag.Parse()
	
	// Create handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// List coverage files
		files, err := findCoverageFiles(*coverageDir)
		if err != nil {
			http.Error(w, "Failed to list coverage files", http.StatusInternalServerError)
			return
		}
		
		// Render index page
		renderIndex(w, files)
	})
	
	// API endpoint for coverage data
	http.HandleFunc("/api/coverage", func(w http.ResponseWriter, r *http.Request) {
		// List coverage files
		files, err := findCoverageFiles(*coverageDir)
		if err != nil {
			http.Error(w, "Failed to list coverage files", http.StatusInternalServerError)
			return
		}
		
		// Return as JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	})
	
	// Serve coverage files
	http.HandleFunc("/view/", func(w http.ResponseWriter, r *http.Request) {
		// Extract filename
		path := strings.TrimPrefix(r.URL.Path, "/view/")
		if path == "" {
			http.Error(w, "Missing file path", http.StatusBadRequest)
			return
		}
		
		// Construct full path
		fullPath := filepath.Join(*coverageDir, path)
		
		// Check if HTML or text
		if strings.HasSuffix(fullPath, ".html") {
			// Serve HTML file
			http.ServeFile(w, r, fullPath)
		} else {
			// Read text file
			data, err := os.ReadFile(fullPath)
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			
			// Format as text
			w.Header().Set("Content-Type", "text/plain")
			w.Write(data)
		}
	})
	
	// Static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	
	// Start server
	fmt.Printf("Starting coverage server on port %d...\n", *port)
	http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
}

func findCoverageFiles(dir string) ([]CoverageFile, error) {
	var files []CoverageFile
	
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		path := filepath.Join(dir, name)
		
		if strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".html") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			
			// Determine if synthetic
			synthetic := strings.Contains(name, "synthetic") || 
				       strings.Contains(name, "script")
			
			// Extract coverage percentage if available
			percentage := 0.0
			if strings.HasSuffix(name, ".txt") {
				percentage = extractCoveragePercentage(path)
			}
			
			files = append(files, CoverageFile{
				Path:       name,
				Name:       name,
				Date:       info.ModTime(),
				Percentage: percentage,
				Synthetic:  synthetic,
			})
		}
	}
	
	return files, nil
}

func extractCoveragePercentage(path string) float64 {
	file, err := os.Open(path)
	if err != nil {
		return 0.0
	}
	defer file.Close()
	
	// Read the file
	data, err := io.ReadAll(file)
	if err != nil {
		return 0.0
	}
	
	// Look for total percentage
	totalRegex := regexp.MustCompile(`total:\s+.*?\s+(\d+\.\d+)%`)
	matches := totalRegex.FindSubmatch(data)
	if len(matches) >= 2 {
		percentage, err := strconv.ParseFloat(string(matches[1]), 64)
		if err != nil {
			return 0.0
		}
		return percentage
	}
	
	return 0.0
}

func renderIndex(w http.ResponseWriter, files []CoverageFile) {
	// HTML template
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Coverage Report</title>
    <style>
        body { font-family: sans-serif; margin: 0; padding: 20px; }
        h1 { color: #333; }
        table { border-collapse: collapse; width: 100%; }
        th, td { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
        tr:hover { background-color: #f5f5f5; }
        .synthetic { background-color: #fff8e1; }
        .percentage { font-weight: bold; }
        .good { color: green; }
        .warn { color: orange; }
        .bad { color: red; }
    </style>
</head>
<body>
    <h1>Coverage Reports</h1>
    <table>
        <tr>
            <th>File</th>
            <th>Date</th>
            <th>Coverage</th>
            <th>Type</th>
        </tr>
        {{range .}}
        <tr class="{{if .Synthetic}}synthetic{{end}}">
            <td><a href="/view/{{.Path}}">{{.Name}}</a></td>
            <td>{{.Date.Format "2006-01-02 15:04:05"}}</td>
            <td class="percentage {{if ge .Percentage 80.0}}good{{else if ge .Percentage 60.0}}warn{{else}}bad{{end}}">
                {{if gt .Percentage 0.0}}{{printf "%.1f" .Percentage}}%{{else}}-{{end}}
            </td>
            <td>{{if .Synthetic}}Synthetic{{else}}Standard{{end}}</td>
        </tr>
        {{end}}
    </table>
</body>
</html>
`
	
	// Parse and execute template
	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, files)
}
```

## Best Practices

1. **Code Organization**: Structure your project to make script test synthetic coverage easier:
   ```
   project/
   ├── cmd/
   │   └── mytool/
   │       ├── main.go      // Main entry point
   │       └── handlers.go  // Command handlers
   ├── internal/
   │   ├── commands/        // Command implementations
   │   └── processors/      // Business logic
   ├── scripttest/          // Script tests
   │   ├── script_test.go   // Test implementation
   │   └── script.txt       // Test script
   └── tools/               // Coverage tooling
       └── script-coverage/ // Script coverage tools
   ```

2. **Command-Handler Pattern**: Standardize command handling to make coverage mapping easier:
   ```go
   // cmd/mytool/handlers.go
   package main
   
   // RegisterHandlers sets up all command handlers
   func RegisterHandlers() {
       // Each handler is in a predictable place
       commands.Register("version", handleVersion)
       commands.Register("run", handleRun)
       commands.Register("process", handleProcess)
   }
   
   // Standard handler signature
   func handleVersion(args []string) error {
       // Implementation
   }
   ```

3. **Automated Coverage Validation**: Create a validation script to ensure your synthetic coverage is accurate:
   ```go
   // tools/validate-coverage.go
   package main
   
   import (
       "flag"
       "fmt"
       "os"
       "regexp"
       "strings"
   )
   
   func main() {
       coverageFile := flag.String("coverage", "coverage.txt", "Coverage file to validate")
       commandMap := flag.String("commands", "commands.txt", "Command mapping file")
       flag.Parse()
       
       // Read command mapping
       commands, err := readCommandMapping(*commandMap)
       if err != nil {
           fmt.Fprintf(os.Stderr, "Error reading command mapping: %v\n", err)
           os.Exit(1)
       }
       
       // Read coverage file
       coverage, err := readCoverageFile(*coverageFile)
       if err != nil {
           fmt.Fprintf(os.Stderr, "Error reading coverage file: %v\n", err)
           os.Exit(1)
       }
       
       // Validate coverage
       validateCoverage(commands, coverage)
   }
   
   // Implementation details...
   ```

4. **Continuous Evolution**: Set up a system to update synthetic coverage when your code changes:
   ```yaml
   # .github/workflows/update-synthetic-coverage.yml
   name: Update Synthetic Coverage
   
   on:
     push:
       paths:
         - 'cmd/**/*.go'
         - 'internal/**/*.go'
         - 'scripttest/script.txt'
   
   jobs:
     update-coverage:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v3
         
         - name: Set up Go
           uses: actions/setup-go@v4
           with:
             go-version: '1.21'
             
         - name: Update synthetic coverage
           run: |
             go run tools/script-ast-xref.go \
               -script=./scripttest/script.txt \
               -source=. \
               -output=synthetic-coverage.txt
             
         - name: Create PR if changed
           run: |
             if git diff --quiet synthetic-coverage.txt; then
               echo "No changes to synthetic coverage"
             else
               git config --local user.email "bot@example.com"
               git config --local user.name "Coverage Bot"
               git checkout -b update-synthetic-coverage
               git add synthetic-coverage.txt
               git commit -m "Update synthetic coverage"
               git push -u origin update-synthetic-coverage
               gh pr create --title "Update synthetic coverage" \
                            --body "Automatic update of synthetic coverage"
             fi
   ```

5. **Granular Coverage**: Generate coverage for specific code blocks, not just entire functions:
   ```go
   // More precise coverage generator
   func generatePreciseCoverage(file string, startLine, endLine int) string {
       // Read the source file
       data, err := os.ReadFile(file)
       if err != nil {
           return ""
       }
       
       // Split into lines
       lines := strings.Split(string(data), "\n")
       
       // Count statements in each block
       var blocks []string
       currentBlock := struct {
           Start, End, Statements int
       }{startLine, startLine, 0}
       
       // Simple heuristic: count statements based on semicolons and braces
       for i := startLine - 1; i < endLine && i < len(lines); i++ {
           line := strings.TrimSpace(lines[i])
           if line == "" || strings.HasPrefix(line, "//") {
               continue // Skip empty lines and comments
           }
           
           // Count statements
           hasStatement := strings.Contains(line, ";") ||
                        strings.HasSuffix(line, "{") ||
                        strings.HasSuffix(line, "}") ||
                        regexp.MustCompile(`\breturn\b`).MatchString(line)
           
           if hasStatement {
               currentBlock.Statements++
           }
           
           // End of block
           if strings.Contains(line, "}") {
               if currentBlock.Statements > 0 {
                   blocks = append(blocks, fmt.Sprintf("%s:%d.1,%d.1 %d 1",
                       file, currentBlock.Start, i+1, currentBlock.Statements))
               }
               currentBlock.Start = i + 2 // Next line after block
               currentBlock.Statements = 0
           }
       }
       
       // Handle any remaining block
       if currentBlock.Statements > 0 {
           blocks = append(blocks, fmt.Sprintf("%s:%d.1,%d.1 %d 1",
               file, currentBlock.Start, endLine, currentBlock.Statements))
       }
       
       return strings.Join(blocks, "\n")
   }
   ```

## Conclusion

These advanced techniques provide comprehensive synthetic coverage for `rsc.io/script/scripttest` tests, ensuring your coverage reports accurately reflect your testing efforts. By combining static analysis, AST parsing, and runtime tracing, you can create sophisticated synthetic coverage that closely matches what actual instrumentation would produce.

The methods described here can be adapted to other script-based testing approaches, including bash scripts executed by Go tests. With these tools, you can ensure your coverage metrics remain accurate even when testing complex command-line applications with script-based tests.