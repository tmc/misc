# Callgraph Analysis for TestScript Coverage: Part 1 - Fundamentals

## Introduction

When generating synthetic coverage for `testscript` tests, a simple line-by-line mapping often falls short. These tests invoke your program as a separate process, making it challenging to know which functions are actually called. Callgraph analysis solves this problem by determining the complete execution path your code would take when running testscript commands.

This guide explores how to build a callgraph analyzer for testscript files to generate more accurate synthetic coverage.

## What is a Callgraph?

A callgraph is a directed graph where:
- Nodes represent functions in your codebase
- Edges represent function calls between those functions

For example, if function `A` calls functions `B` and `C`, and `C` calls `D`, the callgraph would look like:

```
A → B
↓
C → D
```

By analyzing the callgraph, we can determine all functions that would be executed when a particular entry point is called.

## Why Callgraph Analysis for TestScript?

Standard coverage tools only capture execution within a single process. When using testscript, your Go program runs as a separate process, causing conventional coverage tools to miss this execution.

Callgraph analysis allows us to:

1. Parse testscript files to identify which commands are executed
2. Determine the entry points these commands would trigger in your code
3. Follow the callgraph to identify all functions that would be executed
4. Generate synthetic coverage for these functions

## Building a Basic Callgraph Analyzer

Let's build a tool to analyze testscript files and generate synthetic coverage using callgraph analysis:

```go
// callgraph-analyzer.go
package main

import (
	"bufio"
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
	scriptFile   = flag.String("script", "", "Path to testscript file")
	sourceDir    = flag.String("source", ".", "Source directory")
	outputFile   = flag.String("output", "callgraph-coverage.txt", "Output coverage file")
	packageRoot  = flag.String("package", "example.com/myproject", "Package root")
	commandMap   = flag.String("commands", "", "JSON file mapping commands to entry points")
	verbose      = flag.Bool("v", false, "Verbose output")
)

// EntryPoint represents a command entry point in the code
type EntryPoint struct {
	Command    string   // Command pattern (e.g., "run")
	File       string   // Source file containing the entry point
	Function   string   // Function name
	LineStart  int      // Starting line
	LineEnd    int      // Ending line
}

// Function represents a function in the callgraph
type Function struct {
	Name       string
	File       string
	LineStart  int
	LineEnd    int 
	Calls      []string  // Functions this function calls
}

// Callgraph represents the entire callgraph of the project
type Callgraph struct {
	Functions map[string]*Function
	EntryPoints map[string]*EntryPoint
}

func main() {
	flag.Parse()
	
	if *scriptFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -script flag is required\n")
		os.Exit(1)
	}
	
	// Build the callgraph
	callgraph, err := buildCallgraph(*sourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building callgraph: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		fmt.Printf("Built callgraph with %d functions\n", len(callgraph.Functions))
	}
	
	// Load command mapping
	if err := loadCommandMapping(callgraph, *commandMap); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading command mapping: %v\n", err)
		os.Exit(1)
	}
	
	// Extract commands from testscript file
	commands, err := extractCommands(*scriptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting commands: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		fmt.Printf("Extracted %d commands from testscript file\n", len(commands))
	}
	
	// Generate coverage based on callgraph analysis
	coverage := generateCoverage(callgraph, commands, *packageRoot)
	
	// Write coverage file
	if err := os.WriteFile(*outputFile, []byte(coverage), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing coverage file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage in %s\n", *outputFile)
}

// buildCallgraph analyzes source files to build a callgraph
func buildCallgraph(sourceDir string) (*Callgraph, error) {
	callgraph := &Callgraph{
		Functions: make(map[string]*Function),
		EntryPoints: make(map[string]*EntryPoint),
	}
	
	// Find all Go files
	var goFiles []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && 
		   !strings.HasSuffix(path, "_test.go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	
	// Process each Go file
	fset := token.NewFileSet()
	for _, file := range goFiles {
		// Parse the file
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			continue // Skip files that can't be parsed
		}
		
		// Get relative path
		relPath, err := filepath.Rel(sourceDir, file)
		if err != nil {
			relPath = file // Fall back to absolute path
		}
		
		// Find all functions and their calls
		ast.Inspect(node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				// Get function name with potential receiver
				funcName := getFunctionName(fn)
				
				// Get position info
				startPos := fset.Position(fn.Pos())
				endPos := fset.Position(fn.End())
				
				// Create function entry
				function := &Function{
					Name:       funcName,
					File:       relPath,
					LineStart:  startPos.Line,
					LineEnd:    endPos.Line,
					Calls:      []string{},
				}
				
				// Find function calls within this function
				if fn.Body != nil {
					ast.Inspect(fn.Body, func(n ast.Node) bool {
						if call, ok := n.(*ast.CallExpr); ok {
							if callee := getCalleeName(call); callee != "" {
								function.Calls = append(function.Calls, callee)
							}
						}
						return true
					})
				}
				
				// Add to callgraph
				key := relPath + ":" + funcName
				callgraph.Functions[key] = function
			}
			return true
		})
	}
	
	return callgraph, nil
}

// getFunctionName gets the qualified name of a function
func getFunctionName(fn *ast.FuncDecl) string {
	if fn.Recv == nil {
		return fn.Name.Name
	}
	
	// Handle methods (with receiver)
	if len(fn.Recv.List) > 0 {
		recvType := ""
		
		// Extract receiver type
		switch t := fn.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				recvType = ident.Name
			}
		case *ast.Ident:
			recvType = t.Name
		}
		
		if recvType != "" {
			return recvType + "." + fn.Name.Name
		}
	}
	
	return fn.Name.Name
}

// getCalleeName gets the name of the function being called
func getCalleeName(call *ast.CallExpr) string {
	switch expr := call.Fun.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.SelectorExpr:
		if x, ok := expr.X.(*ast.Ident); ok {
			return x.Name + "." + expr.Sel.Name
		}
	}
	return ""
}

// loadCommandMapping loads the mapping from commands to entry points
func loadCommandMapping(callgraph *Callgraph, mapFile string) error {
	// Simple command to function mapping for demo
	// In a real implementation, this would read from a config file
	
	// Sample hardcoded mapping
	entryPoints := []EntryPoint{
		{
			Command:   "run",
			File:      "cmd/tool/main.go",
			Function:  "handleRun",
			LineStart: 30,
			LineEnd:   40,
		},
		{
			Command:   "version",
			File:      "cmd/tool/main.go",
			Function:  "handleVersion",
			LineStart: 50,
			LineEnd:   55,
		},
		{
			Command:   "build",
			File:      "cmd/tool/main.go",
			Function:  "handleBuild",
			LineStart: 60,
			LineEnd:   70,
		},
	}
	
	// Add entry points to callgraph
	for _, ep := range entryPoints {
		callgraph.EntryPoints[ep.Command] = &EntryPoint{
			Command:   ep.Command,
			File:      ep.File,
			Function:  ep.Function,
			LineStart: ep.LineStart,
			LineEnd:   ep.LineEnd,
		}
	}
	
	return nil
}

// extractCommands extracts commands from a testscript file
func extractCommands(scriptFile string) ([]string, error) {
	file, err := os.Open(scriptFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var commands []string
	scanner := bufio.NewScanner(file)
	
	// Match lines like: exec mytool command args...
	cmdRegex := regexp.MustCompile(`^exec\s+(\S+)(.*)$`)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Check for command execution
		if matches := cmdRegex.FindStringSubmatch(line); len(matches) >= 3 {
			tool := matches[1]
			args := strings.TrimSpace(matches[2])
			
			// Only process commands for our tool
			if tool == "mytool" && args != "" {
				// Extract the base command (first word)
				baseCmd := strings.Fields(args)[0]
				commands = append(commands, baseCmd)
			}
		}
	}
	
	return commands, scanner.Err()
}

// generateCoverage generates synthetic coverage based on callgraph analysis
func generateCoverage(callgraph *Callgraph, commands []string, packageRoot string) string {
	// Track functions that would be executed
	executed := make(map[string]bool)
	
	// Process each command
	for _, cmd := range commands {
		// Find matching entry point
		if entryPoint, ok := callgraph.EntryPoints[cmd]; ok {
			// Mark entry point as executed
			entryPointKey := entryPoint.File + ":" + entryPoint.Function
			executed[entryPointKey] = true
			
			// Follow callgraph to mark all called functions
			processCalls(callgraph, entryPointKey, executed)
		}
	}
	
	// Generate coverage lines
	var lines []string
	lines = append(lines, "mode: set")
	
	for key, function := range callgraph.Functions {
		if executed[key] {
			// Add coverage line for this function
			coverageLine := fmt.Sprintf("%s/%s:%d.1,%d.1 %d 1", 
				packageRoot,
				function.File, 
				function.LineStart, 
				function.LineEnd, 
				function.LineEnd - function.LineStart + 1)
			
			lines = append(lines, coverageLine)
		}
	}
	
	return strings.Join(lines, "\n")
}

// processCalls recursively follows the callgraph to mark executed functions
func processCalls(callgraph *Callgraph, functionKey string, executed map[string]bool) {
	function, ok := callgraph.Functions[functionKey]
	if !ok || function == nil {
		return
	}
	
	// Process each call
	for _, callName := range function.Calls {
		// Find all potential matches for this call
		for key, calledFunc := range callgraph.Functions {
			if strings.HasSuffix(key, ":"+callName) || 
			   (strings.Contains(key, ".") && strings.HasSuffix(key, callName)) {
				if !executed[key] {
					executed[key] = true
					processCalls(callgraph, key, executed)
				}
			}
		}
	}
}
```

## Using the Basic Callgraph Analyzer

Let's set up a simple example to demonstrate the callgraph analyzer:

1. Create a simple project structure:

```bash
mkdir -p callgraph-demo/{cmd/tool,pkg/processor,testscripts}
cd callgraph-demo
```

2. Create a main program:

```go
// cmd/tool/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	
	"example.com/callgraph-demo/pkg/processor"
)

func main() {
	flag.Parse()
	
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	
	cmd := os.Args[1]
	args := os.Args[2:]
	
	switch cmd {
	case "version":
		handleVersion()
	case "run":
		handleRun(args)
	case "process":
		handleProcess(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: tool COMMAND [ARGS]")
	fmt.Println("Commands:")
	fmt.Println("  version    Show version information")
	fmt.Println("  run        Run a command")
	fmt.Println("  process    Process a file")
}

func handleVersion() {
	fmt.Println("v1.0.0")
}

func handleRun(args []string) {
	if len(args) == 0 {
		fmt.Println("No command specified to run")
		os.Exit(1)
	}
	
	processor.RunCommand(args[0], args[1:])
}

func handleProcess(args []string) {
	if len(args) == 0 {
		fmt.Println("No file specified to process")
		os.Exit(1)
	}
	
	result, err := processor.ProcessFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println(result)
}
```

3. Create the processor package:

```go
// pkg/processor/processor.go
package processor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunCommand runs a command with arguments
func RunCommand(command string, args []string) error {
	fmt.Printf("Running command: %s %s\n", command, strings.Join(args, " "))
	
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

// ProcessFile processes a file and returns the result
func ProcessFile(filename string) (string, error) {
	content, err := readFile(filename)
	if err != nil {
		return "", err
	}
	
	processed := transformContent(content)
	
	return processed, nil
}

// readFile reads a file and returns its content
func readFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	
	return string(data), nil
}

// transformContent transforms the content
func transformContent(content string) string {
	// Apply some transformation
	result := strings.ToUpper(content)
	
	return "PROCESSED: " + result
}
```

4. Create a testscript file:

```txt
# testscripts/basic.txt

# Test version command
exec mytool version
stdout v1.0.0

# Test processing a file
cp testdata/sample.txt sample.txt
exec mytool process sample.txt
stdout PROCESSED: HELLO WORLD

# Test running a command
exec mytool run echo hello
stdout hello
```

5. Run the callgraph analyzer:

```bash
go run callgraph-analyzer.go \
  -script=testscripts/basic.txt \
  -source=. \
  -output=callgraph-coverage.txt \
  -package=example.com/callgraph-demo \
  -v
```

## Advanced Callgraph Construction

The basic approach works well, but we can improve it with more sophisticated analysis:

### Handling Interface Methods

Go's interfaces make callgraph construction challenging. Here's how to handle interface methods:

```go
// identifyInterfaceMethods analyzes the codebase to find potential
// implementations of interface methods
func identifyInterfaceMethods(sourceDir string) (map[string][]string, error) {
	// Map interface methods to potential implementations
	interfaceMap := make(map[string][]string)
	
	// Find all Go files
	var goFiles []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && 
		   !strings.HasSuffix(path, "_test.go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	
	// First pass: identify all interfaces
	interfaces := make(map[string][]string) // interface name -> methods
	
	fset := token.NewFileSet()
	for _, file := range goFiles {
		node, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			continue
		}
		
		// Find interface declarations
		ast.Inspect(node, func(n ast.Node) bool {
			if typeSpec, ok := n.(*ast.TypeSpec); ok {
				if ifaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
					ifaceName := typeSpec.Name.Name
					
					// Extract method names
					var methods []string
					if ifaceType.Methods != nil {
						for _, field := range ifaceType.Methods.List {
							for _, name := range field.Names {
								methods = append(methods, name.Name)
							}
						}
					}
					
					interfaces[ifaceName] = methods
				}
			}
			return true
		})
	}
	
	// Second pass: find implementations
	for _, file := range goFiles {
		node, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			continue
		}
		
		// Find type declarations that might implement interfaces
		ast.Inspect(node, func(n ast.Node) bool {
			if typeSpec, ok := n.(*ast.TypeSpec); ok {
				typeName := typeSpec.Name.Name
				
				// Skip interfaces
				if _, isInterface := typeSpec.Type.(*ast.InterfaceType); isInterface {
					return true
				}
				
				// Find methods for this type
				for _, decl := range node.Decls {
					if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
						// Check if this is a method for our type
						recvType := ""
						
						switch t := funcDecl.Recv.List[0].Type.(type) {
						case *ast.StarExpr:
							if ident, ok := t.X.(*ast.Ident); ok {
								recvType = ident.Name
							}
						case *ast.Ident:
							recvType = t.Name
						}
						
						if recvType == typeName {
							methodName := funcDecl.Name.Name
							
							// Check if this method is part of any interface
							for ifaceName, methods := range interfaces {
								for _, ifaceMethod := range methods {
									if methodName == ifaceMethod {
										// This type implements this interface method
										key := ifaceName + "." + methodName
										implKey := typeName + "." + methodName
										interfaceMap[key] = append(interfaceMap[key], implKey)
									}
								}
							}
						}
					}
				}
			}
			return true
		})
	}
	
	return interfaceMap, nil
}
```

### Handling Dynamic Dispatch

To handle dynamic dispatch, we need to extend our callgraph processing:

```go
// Enhanced processCalls that handles interface method calls
func processCalls(callgraph *Callgraph, functionKey string, executed map[string]bool, 
	interfaceMap map[string][]string) {
	
	function, ok := callgraph.Functions[functionKey]
	if !ok || function == nil {
		return
	}
	
	// Process each call
	for _, callName := range function.Calls {
		// Check if this is an interface method call
		for ifaceMethod, impls := range interfaceMap {
			if callName == ifaceMethod || strings.HasSuffix(callName, "."+ifaceMethod) {
				// This is an interface method call
				for _, impl := range impls {
					// Find implementation in callgraph
					for key, _ := range callgraph.Functions {
						if strings.HasSuffix(key, ":"+impl) || 
						   strings.HasSuffix(key, "."+impl) {
							if !executed[key] {
								executed[key] = true
								processCalls(callgraph, key, executed, interfaceMap)
							}
						}
					}
				}
			}
		}
		
		// Handle direct calls
		for key, calledFunc := range callgraph.Functions {
			if strings.HasSuffix(key, ":"+callName) || 
			   (strings.Contains(key, ".") && strings.HasSuffix(key, callName)) {
				if !executed[key] {
					executed[key] = true
					processCalls(callgraph, key, executed, interfaceMap)
				}
			}
		}
	}
}
```

## Enhancing Coverage Accuracy

To make our synthetic coverage more accurate, we can add these features:

1. **Line-level coverage** instead of just function-level:

```go
// generateLineLevelCoverage generates coverage at line level
func generateLineLevelCoverage(callgraph *Callgraph, executed map[string]bool, 
	packageRoot string) string {
	
	var lines []string
	lines = append(lines, "mode: set")
	
	// Process each executed function
	for key, function := range callgraph.Functions {
		if executed[key] {
			// Read the source file
			fileContent, err := os.ReadFile(function.File)
			if err != nil {
				continue
			}
			
			// Split into lines
			fileLines := strings.Split(string(fileContent), "\n")
			
			// Process the function body line by line
			lineStart := function.LineStart
			statementCount := 0
			
			for i := function.LineStart; i <= function.LineEnd && i < len(fileLines); i++ {
				line := strings.TrimSpace(fileLines[i-1])
				
				// Skip empty lines and comments
				if line == "" || strings.HasPrefix(line, "//") {
					continue
				}
				
				// Check if this line might contain a statement
				hasStatement := strings.Contains(line, ";") ||
					strings.HasSuffix(line, "{") ||
					strings.HasSuffix(line, "}") ||
					regexp.MustCompile(`\breturn\b`).MatchString(line) ||
					regexp.MustCompile(`\bif\b`).MatchString(line) ||
					regexp.MustCompile(`\bfor\b`).MatchString(line)
				
				if hasStatement {
					statementCount++
				}
				
				// If we've found a statement or reached the end, create a coverage line
				// for this block of code
				if hasStatement && (i == function.LineEnd || 
					strings.HasSuffix(line, "}") || 
					strings.HasSuffix(line, ";")) {
					
					if statementCount > 0 {
						coverageLine := fmt.Sprintf("%s/%s:%d.1,%d.1 %d 1", 
							packageRoot,
							function.File, 
							lineStart, 
							i, 
							statementCount)
						
						lines = append(lines, coverageLine)
					}
					
					// Reset for next block
					lineStart = i + 1
					statementCount = 0
				}
			}
		}
	}
	
	return strings.Join(lines, "\n")
}
```

2. **Branch coverage** for conditional statements:

```go
// analyzeBranchCoverage analyzes branch coverage for a function
func analyzeBranchCoverage(filePath string, function *Function) []BranchInfo {
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil
	}
	
	var branches []BranchInfo
	
	// Find the function declaration
	ast.Inspect(node, func(n ast.Node) bool {
		if fd, ok := n.(*ast.FuncDecl); ok {
			funcName := getFunctionName(fd)
			if funcName == function.Name {
				// Found our function
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					switch stmt := n.(type) {
					case *ast.IfStmt:
						// Handle if statement
						condPos := fset.Position(stmt.Cond.Pos())
						condEnd := fset.Position(stmt.Cond.End())
						
						// If branch
						ifPos := fset.Position(stmt.Body.Pos())
						ifEnd := fset.Position(stmt.Body.End())
						
						branches = append(branches, BranchInfo{
							Type:      "if",
							Condition: fmt.Sprintf("%d.1,%d.1", condPos.Line, condEnd.Line),
							Branch:    fmt.Sprintf("%d.1,%d.1", ifPos.Line, ifEnd.Line),
						})
						
						// Else branch if present
						if stmt.Else != nil {
							elsePos := fset.Position(stmt.Else.Pos())
							elseEnd := fset.Position(stmt.Else.End())
							
							branches = append(branches, BranchInfo{
								Type:      "else",
								Condition: fmt.Sprintf("%d.1,%d.1", condPos.Line, condEnd.Line),
								Branch:    fmt.Sprintf("%d.1,%d.1", elsePos.Line, elseEnd.Line),
							})
						}
						
					case *ast.SwitchStmt:
						// Handle switch statement
						switchPos := fset.Position(stmt.Pos())
						
						for _, s := range stmt.Body.List {
							if caseClause, ok := s.(*ast.CaseClause); ok {
								casePos := fset.Position(caseClause.Pos())
								caseEnd := fset.Position(caseClause.End())
								
								branches = append(branches, BranchInfo{
									Type:      "case",
									Condition: fmt.Sprintf("%d.1,%d.1", switchPos.Line, switchPos.Line),
									Branch:    fmt.Sprintf("%d.1,%d.1", casePos.Line, caseEnd.Line),
								})
							}
						}
					}
					return true
				})
				return false // Stop once we've found and processed our function
			}
		}
		return true
	})
	
	return branches
}
```

## Conclusion

This guide has introduced the fundamentals of callgraph analysis for generating synthetic coverage from testscript files. By constructing a callgraph and analyzing testscript files, we can generate more accurate synthetic coverage data that better represents the code paths that would be executed during script tests.

In the next part, we'll explore advanced callgraph analysis techniques, including:

- Inter-procedural control flow analysis
- Context-sensitive callgraph construction
- Handling reflection and dynamic calls
- Optimizing for large codebases
- Integrating with go/analysis framework

Stay tuned for "Part 2: Advanced Callgraph Analysis Techniques"!