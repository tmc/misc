# Callgraph Analysis for TestScript Coverage: Part 3 - Precise TestScript Command Mapping

## Introduction

In the previous parts, we explored building callgraphs and using them to generate synthetic coverage. But we still need to solve a critical challenge: accurately mapping testscript commands to their corresponding entry points in the code.

This guide focuses on precise testscript command mapping techniques that connect script commands to the specific functions they would trigger in your codebase.

## The Command Mapping Challenge

Testscript files contain commands like:

```
exec mytool run --verbose path/to/file
exec mytool process --format=json data.txt
```

To generate accurate synthetic coverage, we need to map these commands to the specific code paths they would execute. This mapping is challenging because:

1. Command structures vary between applications
2. There's no standard way to route commands to handlers
3. Applications may use different CLI frameworks (e.g., cobra, urfave/cli, flag)
4. Arguments and flags further customize execution paths

## Approach 1: Heuristic-Based Mapping

The simplest approach uses naming conventions and patterns to map commands:

```go
// heuristicCommandMapper maps commands based on naming patterns
func heuristicCommandMapper(commands []string, callgraph *Callgraph) map[string][]string {
	mapping := make(map[string][]string)
	
	// Common naming patterns
	patterns := []struct {
		Prefix string
		Suffix string
	}{
		{"handle", ""},
		{"cmd", ""},
		{"", "Command"},
		{"", "Cmd"},
		{"run", ""},
		{"exec", ""},
	}
	
	// For each command, look for matching functions
	for _, cmd := range commands {
		cmdName := strings.Split(cmd, " ")[0] // Extract base command
		cmdName = strings.TrimSpace(cmdName)
		
		var matches []string
		
		// Try different naming patterns
		for _, pattern := range patterns {
			possibleName := pattern.Prefix + strings.Title(cmdName) + pattern.Suffix
			
			// Look for matching functions in callgraph
			for funcKey, _ := range callgraph.Functions {
				if strings.Contains(funcKey, ":"+possibleName) || 
				   strings.HasSuffix(funcKey, "."+possibleName) {
					matches = append(matches, funcKey)
				}
			}
		}
		
		mapping[cmd] = matches
	}
	
	return mapping
}
```

While simple, this approach is prone to false positives and misses. We can do better.

## Approach 2: CLI Framework Analysis

Most Go CLI applications use frameworks like Cobra, urfave/cli, or the standard flag package. By analyzing how these frameworks register commands, we can build more accurate mappings:

### Cobra Command Analysis

```go
// cobraCommandAnalyzer analyzes Cobra command structures
type cobraCommandAnalyzer struct {
	callgraph *Callgraph
	commands  map[string]string // Command path to handler function
}

// newCobraCommandAnalyzer creates a new Cobra command analyzer
func newCobraCommandAnalyzer(callgraph *Callgraph) *cobraCommandAnalyzer {
	return &cobraCommandAnalyzer{
		callgraph: callgraph,
		commands:  make(map[string]string),
	}
}

// analyzeFile analyzes a file for Cobra command registrations
func (a *cobraCommandAnalyzer) analyzeFile(filePath string) error {
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	
	// Look for Cobra command definitions
	ast.Inspect(node, func(n ast.Node) bool {
		// Look for variable assignments with cobra.Command
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, rhs := range assign.Rhs {
				if comp, ok := rhs.(*ast.CompositeLit); ok {
					if sel, ok := comp.Type.(*ast.SelectorExpr); ok {
						if x, ok := sel.X.(*ast.Ident); ok && x.Name == "cobra" && sel.Sel.Name == "Command" {
							// Found a cobra.Command
							cmdName := extractCobraCommandName(comp)
							runFunc := extractCobraRunFunc(comp)
							
							if cmdName != "" && runFunc != "" {
								a.commands[cmdName] = runFunc
							}
						}
					}
				}
			}
		}
		
		// Look for AddCommand calls
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "AddCommand" {
					// This is an AddCommand call, extract parent and child
					if len(call.Args) > 0 {
						// Process parent-child relationship
						// In a real implementation, you'd parse this to build
						// the command hierarchy
					}
				}
			}
		}
		
		return true
	})
	
	return nil
}

// extractCobraCommandName extracts the command name from a cobra.Command composite literal
func extractCobraCommandName(comp *ast.CompositeLit) string {
	for _, elt := range comp.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Use" {
				if lit, ok := kv.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					// Extract command name from "use" value
					cmdName := strings.Trim(lit.Value, "\"")
					// Extract just the first word (command name)
					return strings.Fields(cmdName)[0]
				}
			}
		}
	}
	return ""
}

// extractCobraRunFunc extracts the Run function from a cobra.Command composite literal
func extractCobraRunFunc(comp *ast.CompositeLit) string {
	for _, elt := range comp.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && 
				(key.Name == "Run" || key.Name == "RunE") {
				if fun, ok := kv.Value.(*ast.Ident); ok {
					return fun.Name
				} else if funLit, ok := kv.Value.(*ast.FuncLit); ok {
					// This is an inline function
					// In a real implementation, you'd need to analyze the function body
					return "inline-function"
				}
			}
		}
	}
	return ""
}

// mapCobraCommandsToHandlers maps Cobra commands to their handlers
func (a *cobraCommandAnalyzer) mapToHandlers() map[string][]string {
	mapping := make(map[string][]string)
	
	for cmd, handler := range a.commands {
		// Find handler function in callgraph
		var handlers []string
		
		for funcKey, _ := range a.callgraph.Functions {
			if strings.HasSuffix(funcKey, ":"+handler) || 
			   strings.HasSuffix(funcKey, "."+handler) {
				handlers = append(handlers, funcKey)
			}
		}
		
		mapping[cmd] = handlers
	}
	
	return mapping
}
```

### Urfave/cli Analysis

```go
// urfaveCliAnalyzer analyzes urfave/cli command structures
type urfaveCliAnalyzer struct {
	callgraph *Callgraph
	commands  map[string]string // Command path to handler function
}

// analyzeFile analyzes a file for urfave/cli command registrations
func (a *urfaveCliAnalyzer) analyzeFile(filePath string) error {
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	
	// Look for urfave/cli command definitions
	ast.Inspect(node, func(n ast.Node) bool {
		// Look for cli.Command literals
		if comp, ok := n.(*ast.CompositeLit); ok {
			if sel, ok := comp.Type.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "cli" && sel.Sel.Name == "Command" {
					// Found a cli.Command
					cmdName := extractUrfaveCommandName(comp)
					actionFunc := extractUrfaveActionFunc(comp)
					
					if cmdName != "" && actionFunc != "" {
						a.commands[cmdName] = actionFunc
					}
				}
			}
		}
		
		return true
	})
	
	return nil
}

// extractUrfaveCommandName extracts the command name from a cli.Command composite literal
func extractUrfaveCommandName(comp *ast.CompositeLit) string {
	for _, elt := range comp.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Name" {
				if lit, ok := kv.Value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					return strings.Trim(lit.Value, "\"")
				}
			}
		}
	}
	return ""
}

// extractUrfaveActionFunc extracts the Action function from a cli.Command composite literal
func extractUrfaveActionFunc(comp *ast.CompositeLit) string {
	for _, elt := range comp.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Action" {
				if fun, ok := kv.Value.(*ast.Ident); ok {
					return fun.Name
				} else if funLit, ok := kv.Value.(*ast.FuncLit); ok {
					return "inline-function"
				}
			}
		}
	}
	return ""
}
```

### Standard Flag Package Analysis

```go
// flagCommandAnalyzer analyzes standard flag package usage
type flagCommandAnalyzer struct {
	callgraph *Callgraph
	subcommands map[string]string // Subcommand name to handler function
	flagHandlers map[string][]string // Flag names to handler functions
}

// analyzeFile analyzes a file for flag package usages
func (a *flagCommandAnalyzer) analyzeFile(filePath string) error {
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	
	// Find main function
	var mainFunc *ast.FuncDecl
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "main" {
			mainFunc = fn
			return false
		}
		return true
	})
	
	if mainFunc == nil {
		return nil // No main function
	}
	
	// Analyze main function for flag parsing and command routing
	ast.Inspect(mainFunc.Body, func(n ast.Node) bool {
		// Look for flag.Parse() calls
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "flag" && sel.Sel.Name == "Parse" {
					// Found flag.Parse(), now look for command routing
					a.analyzeFlagCommandRouting(mainFunc.Body)
					return false
				}
			}
		}
		return true
	})
	
	return nil
}

// analyzeFlagCommandRouting analyzes command routing in a function body
func (a *flagCommandAnalyzer) analyzeFlagCommandRouting(body *ast.BlockStmt) {
	// Look for common command routing patterns:
	// 1. Switch on os.Args[1]
	// 2. If/else chains comparing against command names
	// 3. Subcommand handling with flag.NewFlagSet
	
	ast.Inspect(body, func(n ast.Node) bool {
		// Check for switch statements
		if sw, ok := n.(*ast.SwitchStmt); ok {
			// Check if this is switching on os.Args[1] or similar
			if isCommandSwitch(sw) {
				a.analyzeCommandSwitch(sw)
				return false
			}
		}
		
		// Check for flag.NewFlagSet calls
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "flag" && sel.Sel.Name == "NewFlagSet" {
					// This is likely a subcommand
					if len(call.Args) > 0 {
						if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							cmdName := strings.Trim(lit.Value, "\"")
							// Find where this flagset is used
							// This requires more sophisticated analysis
						}
					}
				}
			}
		}
		
		return true
	})
}

// isCommandSwitch checks if a switch statement is switching on a command argument
func isCommandSwitch(sw *ast.SwitchStmt) bool {
	if indexExpr, ok := sw.Tag.(*ast.IndexExpr); ok {
		if sel, ok := indexExpr.X.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "os" && sel.Sel.Name == "Args" {
				// This is os.Args[something]
				if lit, ok := indexExpr.Index.(*ast.BasicLit); ok && lit.Kind == token.INT {
					if lit.Value == "1" {
						// This is os.Args[1], which is typically the command
						return true
					}
				}
			}
		}
	}
	return false
}

// analyzeCommandSwitch analyzes a switch statement for command routing
func (a *flagCommandAnalyzer) analyzeCommandSwitch(sw *ast.SwitchStmt) {
	// Process each case
	for _, stmt := range sw.Body.List {
		if caseClause, ok := stmt.(*ast.CaseClause); ok {
			// Get command name(s)
			var cmds []string
			for _, expr := range caseClause.List {
				if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					cmds = append(cmds, strings.Trim(lit.Value, "\""))
				}
			}
			
			// Find handler function call
			ast.Inspect(caseClause.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok {
						// This might be the handler function
						for _, cmd := range cmds {
							a.subcommands[cmd] = ident.Name
						}
						return false
					}
				}
				return true
			})
		}
	}
}
```

## Approach 3: Dynamic Command Registration Tracing

Some applications register commands dynamically. For these, we need a more sophisticated approach:

```go
// dynamicCommandTracer traces dynamic command registrations
type dynamicCommandTracer struct {
	callgraph *Callgraph
	commandRegistry map[string]string // Command to handler mapping
}

// traceCommandRegistrations traces command registrations
func (t *dynamicCommandTracer) traceCommandRegistrations(sourceDir string) error {
	// Find potential command registration patterns
	registrationPatterns := []struct {
		FuncName string
		CommandArg int
		HandlerArg int
	}{
		{"RegisterCommand", 0, 1},
		{"AddCommand", 0, -1},  // -1 means the handler is the receiver
		{"Register", 0, 1},
		{"NewCommand", 0, 1},
	}
	
	// Scan files for these patterns
	var files []string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	
	// Process each file
	for _, file := range files {
		// Parse the file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			continue
		}
		
		// Look for registration calls
		ast.Inspect(node, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				// Check if this is a function call
				funcName := ""
				
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					funcName = fun.Name
				case *ast.SelectorExpr:
					funcName = fun.Sel.Name
				}
				
				// Check if this matches a registration pattern
				for _, pattern := range registrationPatterns {
					if funcName == pattern.FuncName && len(call.Args) > pattern.CommandArg {
						// Extract command name
						cmdName := extractStringArg(call.Args[pattern.CommandArg])
						
						if cmdName != "" {
							// Extract handler
							handlerName := ""
							
							if pattern.HandlerArg >= 0 && len(call.Args) > pattern.HandlerArg {
								handlerName = extractHandlerArg(call.Args[pattern.HandlerArg])
							} else if pattern.HandlerArg == -1 {
								// Handler is the receiver
								if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
									if x, ok := sel.X.(*ast.Ident); ok {
										handlerName = x.Name
									}
								}
							}
							
							if handlerName != "" {
								t.commandRegistry[cmdName] = handlerName
							}
						}
					}
				}
			}
			return true
		})
	}
	
	return nil
}

// extractStringArg extracts a string argument from an expression
func extractStringArg(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strings.Trim(e.Value, "\"")
		}
	case *ast.Ident:
		// This is a variable, would need data flow analysis
		return e.Name + "_variable"
	}
	return ""
}

// extractHandlerArg extracts a handler function from an expression
func extractHandlerArg(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			return x.Name + "." + e.Sel.Name
		}
	case *ast.FuncLit:
		// Inline function
		return "inline_function"
	}
	return ""
}
```

## Approach 4: Testscript Command Parser

To parse testscript commands with high precision, we need a dedicated parser:

```go
// testscriptCommandParser parses commands from testscript files
type testscriptCommandParser struct {
	filename string
	commands []TestscriptCommand
}

// TestscriptCommand represents a parsed command from a testscript file
type TestscriptCommand struct {
	Line       int
	Command    string // The command type (exec, cd, etc.)
	Tool       string // The tool being executed
	BaseCmd    string // The base command (first argument)
	Args       []string // Additional arguments
	Flags      map[string]string // Parsed flags
}

// parseFile parses a testscript file
func (p *testscriptCommandParser) parseFile() error {
	file, err := os.Open(p.filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Extract command
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		
		// Parse the command
		cmdType := fields[0]
		
		// Only process exec commands for now
		if cmdType == "exec" && len(fields) >= 2 {
			cmd := TestscriptCommand{
				Line:    lineNum,
				Command: cmdType,
				Flags:   make(map[string]string),
			}
			
			// Extract tool name and command
			if len(fields) >= 2 {
				cmd.Tool = fields[1]
				
				// Extract base command and args
				if len(fields) >= 3 {
					cmd.BaseCmd = fields[2]
					cmd.Args = fields[3:]
					
					// Parse flags
					for i, arg := range cmd.Args {
						if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
							flag := strings.TrimLeft(arg, "-")
							value := ""
							
							// Check if it's a --flag=value format
							if strings.Contains(flag, "=") {
								parts := strings.SplitN(flag, "=", 2)
								flag = parts[0]
								value = parts[1]
							} else if i+1 < len(cmd.Args) && !strings.HasPrefix(cmd.Args[i+1], "-") {
								// This could be a --flag value format
								value = cmd.Args[i+1]
								// We'd need to remove this from args, but for simplicity, we're not doing that here
							}
							
							cmd.Flags[flag] = value
						}
					}
				}
			}
			
			p.commands = append(p.commands, cmd)
		}
	}
	
	return scanner.Err()
}

// getCommands returns the parsed commands
func (p *testscriptCommandParser) getCommands() []TestscriptCommand {
	return p.commands
}
```

## Putting It All Together: Comprehensive Command Mapping

Now let's combine these approaches into a comprehensive command mapping system:

```go
// CommandMapping represents a mapping from testscript commands to code paths
type CommandMapping struct {
	TestscriptCmd  TestscriptCommand
	EntryPoints    []string    // Functions that directly handle this command
	CallPath       []string    // Complete call path including dependencies
	Coverage       []CoverageLine // Generated coverage lines
}

// CoverageLine represents a line of coverage data
type CoverageLine struct {
	File      string
	StartLine int
	EndLine   int
	Count     int
}

// comprehensiveCommandMapper performs comprehensive command mapping
type comprehensiveCommandMapper struct {
	sourceDir       string
	callgraph       *Callgraph
	frameworkMappers []frameworkMapper
	commands        []TestscriptCommand
	mapping         []CommandMapping
}

// frameworkMapper is an interface for CLI framework-specific mappers
type frameworkMapper interface {
	analyzeSource(sourceDir string) error
	mapCommand(cmd TestscriptCommand) []string
}

// mapCommands maps testscript commands to code paths
func (m *comprehensiveCommandMapper) mapCommands() ([]CommandMapping, error) {
	// Initialize framework mappers
	m.frameworkMappers = []frameworkMapper{
		newCobraMapper(m.callgraph),
		newUrfaveMapper(m.callgraph),
		newFlagMapper(m.callgraph),
	}
	
	// Analyze source code with each mapper
	for _, mapper := range m.frameworkMappers {
		if err := mapper.analyzeSource(m.sourceDir); err != nil {
			return nil, err
		}
	}
	
	// Map each command
	for _, cmd := range m.commands {
		mapping := CommandMapping{
			TestscriptCmd: cmd,
		}
		
		// Try each mapper
		for _, mapper := range m.frameworkMappers {
			entryPoints := mapper.mapCommand(cmd)
			if len(entryPoints) > 0 {
				mapping.EntryPoints = append(mapping.EntryPoints, entryPoints...)
			}
		}
		
		// If no mapper found anything, try heuristic approach
		if len(mapping.EntryPoints) == 0 {
			entryPoints := m.heuristicMap(cmd)
			mapping.EntryPoints = entryPoints
		}
		
		// Generate call paths
		if len(mapping.EntryPoints) > 0 {
			mapping.CallPath = m.generateCallPath(mapping.EntryPoints)
			
			// Generate coverage
			mapping.Coverage = m.generateCoverage(mapping.CallPath)
		}
		
		m.mapping = append(m.mapping, mapping)
	}
	
	return m.mapping, nil
}

// heuristicMap is a fallback heuristic mapping approach
func (m *comprehensiveCommandMapper) heuristicMap(cmd TestscriptCommand) []string {
	// Implementation similar to heuristicCommandMapper
	// ...
	return nil
}

// generateCallPath generates a complete call path for entry points
func (m *comprehensiveCommandMapper) generateCallPath(entryPoints []string) []string {
	visited := make(map[string]bool)
	var callPath []string
	
	for _, entry := range entryPoints {
		callPath = append(callPath, entry)
		visited[entry] = true
		
		// Follow the callgraph
		function, ok := m.callgraph.Functions[entry]
		if !ok {
			continue
		}
		
		for _, call := range function.Calls {
			m.followCallPath(call, visited, &callPath)
		}
	}
	
	return callPath
}

// followCallPath recursively follows a call path
func (m *comprehensiveCommandMapper) followCallPath(call string, visited map[string]bool, callPath *[]string) {
	// Find matching functions in callgraph
	for key, function := range m.callgraph.Functions {
		if strings.HasSuffix(key, ":"+call) || 
		   strings.HasSuffix(key, "."+call) {
			
			if visited[key] {
				continue
			}
			
			visited[key] = true
			*callPath = append(*callPath, key)
			
			// Follow its calls
			for _, subcall := range function.Calls {
				m.followCallPath(subcall, visited, callPath)
			}
		}
	}
}

// generateCoverage generates coverage lines for a call path
func (m *comprehensiveCommandMapper) generateCoverage(callPath []string) []CoverageLine {
	var coverage []CoverageLine
	
	for _, funcKey := range callPath {
		function, ok := m.callgraph.Functions[funcKey]
		if !ok {
			continue
		}
		
		coverage = append(coverage, CoverageLine{
			File:      function.File,
			StartLine: function.LineStart,
			EndLine:   function.LineEnd,
			Count:     1,
		})
	}
	
	return coverage
}

// writeCoverageFile writes coverage to a file
func (m *comprehensiveCommandMapper) writeCoverageFile(outputFile, packagePrefix string) error {
	var lines []string
	lines = append(lines, "mode: set")
	
	// Add lines for each function in the call paths
	seen := make(map[string]bool)
	
	for _, mapping := range m.mapping {
		for _, cov := range mapping.Coverage {
			line := fmt.Sprintf("%s/%s:%d.1,%d.1 %d 1", 
				packagePrefix,
				cov.File,
				cov.StartLine,
				cov.EndLine,
				cov.EndLine - cov.StartLine + 1)
			
			if !seen[line] {
				seen[line] = true
				lines = append(lines, line)
			}
		}
	}
	
	return os.WriteFile(outputFile, []byte(strings.Join(lines, "\n")), 0644)
}
```

## Practical Example: Comprehensive Command Analyzer

Let's create a comprehensive command analyzer that combines all these techniques:

```go
// comprehensive-command-analyzer.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var (
	scriptFile    = flag.String("script", "", "Testscript file to analyze")
	sourceDir     = flag.String("source", ".", "Source directory")
	outputFile    = flag.String("output", "comprehensive-coverage.txt", "Output coverage file")
	packagePrefix = flag.String("package", "example.com/myproject", "Package prefix for coverage")
	verbose       = flag.Bool("v", false, "Verbose output")
	toolName      = flag.String("tool", "mytool", "Tool name in testscript file")
)

func main() {
	flag.Parse()
	
	if *scriptFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -script flag is required\n")
		os.Exit(1)
	}
	
	// Parse testscript file
	parser := &testscriptCommandParser{
		filename: *scriptFile,
	}
	if err := parser.parseFile(); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing testscript file: %v\n", err)
		os.Exit(1)
	}
	
	// Filter commands for our tool
	var toolCommands []TestscriptCommand
	for _, cmd := range parser.getCommands() {
		if cmd.Tool == *toolName {
			toolCommands = append(toolCommands, cmd)
		}
	}
	
	if *verbose {
		fmt.Printf("Found %d commands for tool %s\n", len(toolCommands), *toolName)
	}
	
	// Build callgraph
	callgraph, err := buildCallgraph(*sourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building callgraph: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		fmt.Printf("Built callgraph with %d functions\n", len(callgraph.Functions))
	}
	
	// Create comprehensive mapper
	mapper := &comprehensiveCommandMapper{
		sourceDir: *sourceDir,
		callgraph: callgraph,
		commands:  toolCommands,
	}
	
	// Map commands
	mappings, err := mapper.mapCommands()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error mapping commands: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		for i, mapping := range mappings {
			fmt.Printf("Command %d: %s %s\n", i+1, mapping.TestscriptCmd.Tool, mapping.TestscriptCmd.BaseCmd)
			fmt.Printf("  Entry points: %v\n", mapping.EntryPoints)
			fmt.Printf("  Call path length: %d\n", len(mapping.CallPath))
			fmt.Printf("  Coverage lines: %d\n", len(mapping.Coverage))
		}
	}
	
	// Write coverage file
	if err := mapper.writeCoverageFile(*outputFile, *packagePrefix); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing coverage file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage in %s\n", *outputFile)
}
```

## Advanced Techniques for Specific Scenarios

### Handling Plugins and Dynamic Loading

```go
// pluginAnalyzer analyzes plugin loading and usage
type pluginAnalyzer struct {
	callgraph *Callgraph
	pluginMap map[string][]string // Plugin name to potential entry points
}

// analyzePluginUsage analyzes plugin loading and usage
func (a *pluginAnalyzer) analyzePluginUsage(sourceDir string) error {
	// Look for plugin.Open calls
	// ...
	
	// Look for symbol lookups
	// ...
	
	// Look for plugin registration
	// ...
	
	return nil
}
```

### Command Dispatch via Interfaces

```go
// interfaceDispatchAnalyzer analyzes command dispatch via interfaces
type interfaceDispatchAnalyzer struct {
	callgraph *Callgraph
	interfaces map[string][]string // Interface name to implementations
}

// analyzeInterfaceDispatch analyzes command dispatch via interfaces
func (a *interfaceDispatchAnalyzer) analyzeInterfaceDispatch(sourceDir string) error {
	// Build interface -> implementation map
	// ...
	
	// Look for command dispatch via interfaces
	// ...
	
	return nil
}
```

### Configuration-Driven Command Registration

```go
// configDrivenCommandAnalyzer analyzes configuration-driven command registration
type configDrivenCommandAnalyzer struct {
	callgraph *Callgraph
	configMap map[string]string // Command to handler mapping from config
}

// analyzeConfigFiles analyzes configuration files
func (a *configDrivenCommandAnalyzer) analyzeConfigFiles(sourceDir string) error {
	// Look for JSON/YAML configuration files
	// ...
	
	// Parse command registrations
	// ...
	
	return nil
}
```

## Best Practices for Command Mapping

1. **Combine multiple approaches**: No single approach works for all codebases. Use a combination of techniques.

2. **Prefer framework-specific analysis**: If you know which CLI framework your application uses, prioritize that analyzer.

3. **Use heuristics as a fallback**: Heuristic mapping is less precise but provides a safety net.

4. **Verbose mapping output**: Generate detailed mapping information to help debug and improve the system.

5. **Incremental refinement**: Start with basic mapping and refine as needed.

6. **Custom mappings**: For complex applications, consider maintaining a manual mapping file.

## Example: Custom Mapping Configuration

For very complex applications, a custom mapping configuration can be helpful:

```yaml
# command-mapping.yaml
commands:
  - pattern: "run *"
    entry_points:
      - "cmd/root.go:handleRun"
      
  - pattern: "process --json *"
    entry_points:
      - "cmd/process.go:processJSON"
      
  - pattern: "process --xml *"
    entry_points:
      - "cmd/process.go:processXML"
      
  - pattern: "version"
    entry_points:
      - "cmd/root.go:showVersion"
```

```go
// customMappingLoader loads custom command mappings
func loadCustomMapping(filename string) (map[string][]string, error) {
	// Read and parse YAML
	// ...
	
	return nil, nil
}
```

## Conclusion

This guide has explored advanced techniques for precise testscript command mapping. By combining static analysis, framework-specific parsers, and heuristic approaches, you can generate highly accurate synthetic coverage for testscript files.

The methods described here help solve one of the most challenging aspects of synthetic coverage: accurately mapping script commands to the specific code paths they would exercise. With these techniques, you can generate synthetic coverage that closely matches what would be recorded if the code were executed with standard coverage instrumentation.

In the next part, we'll explore maintaining and evolving synthetic coverage over time, integrating it into CI/CD pipelines, and handling complex edge cases.