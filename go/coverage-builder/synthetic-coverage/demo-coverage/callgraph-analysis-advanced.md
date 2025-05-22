# Callgraph Analysis for TestScript Coverage: Part 2 - Advanced Techniques

## Introduction

In Part 1, we covered the fundamentals of callgraph analysis for testscript files. We built a basic tool that analyzes your code to construct a callgraph and uses it to determine which functions would be executed by testscript commands.

In this part, we'll explore advanced techniques for more accurate callgraph analysis:

1. Precise inter-procedural analysis
2. Context-sensitive callgraphs
3. Handling dynamic dispatch and reflection
4. Optimizing for large codebases
5. Integration with Go's analysis framework

## 1. Precise Inter-procedural Analysis

The basic callgraph we built in Part 1 uses a simplified approach that doesn't consider control flow within functions. For more precise analysis, we need to track inter-procedural control flow:

```go
// ControlFlowGraph represents a control flow graph for a function
type ControlFlowGraph struct {
	Blocks       []Block
	EntryBlock   int
	ExitBlocks   []int
}

// Block represents a basic block in a control flow graph
type Block struct {
	ID          int
	Statements  []ast.Stmt
	Predecessors []int
	Successors   []int
	Calls        []string
}

// buildControlFlowGraph builds a control flow graph for a function
func buildControlFlowGraph(fn *ast.FuncDecl, fset *token.FileSet) *ControlFlowGraph {
	cfg := &ControlFlowGraph{}
	
	// Create entry block
	entryBlock := Block{
		ID: 0,
		Statements: []ast.Stmt{},
	}
	cfg.Blocks = append(cfg.Blocks, entryBlock)
	cfg.EntryBlock = 0
	
	// Process function body if it exists
	if fn.Body == nil || fn.Body.List == nil {
		return cfg
	}
	
	// Split function body into basic blocks
	blocks := splitIntoBlocks(fn.Body.List)
	
	// Add blocks to CFG
	entryBlockID := len(cfg.Blocks)
	for i, stmts := range blocks {
		blockID := entryBlockID + i
		
		// Create block
		block := Block{
			ID: blockID,
			Statements: stmts,
		}
		
		// Find calls in this block
		for _, stmt := range stmts {
			findCalls(stmt, &block)
		}
		
		cfg.Blocks = append(cfg.Blocks, block)
	}
	
	// Connect entry block to first real block
	if len(cfg.Blocks) > 1 {
		cfg.Blocks[0].Successors = append(cfg.Blocks[0].Successors, 1)
		cfg.Blocks[1].Predecessors = append(cfg.Blocks[1].Predecessors, 0)
	}
	
	// Analyze control flow to connect blocks
	connectBlocks(cfg)
	
	return cfg
}

// splitIntoBlocks splits a list of statements into basic blocks
func splitIntoBlocks(stmts []ast.Stmt) [][]ast.Stmt {
	var blocks [][]ast.Stmt
	var currentBlock []ast.Stmt
	
	for _, stmt := range stmts {
		// Add statement to current block
		currentBlock = append(currentBlock, stmt)
		
		// Check if this statement ends a block
		endsBlock := false
		
		switch s := stmt.(type) {
		case *ast.ReturnStmt, *ast.BranchStmt:
			// Return and branch statements end blocks
			endsBlock = true
		case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt, *ast.SelectStmt:
			// Control structures also end blocks
			endsBlock = true
		}
		
		if endsBlock && len(currentBlock) > 0 {
			blocks = append(blocks, currentBlock)
			currentBlock = []ast.Stmt{}
		}
	}
	
	// Add any remaining statements as a block
	if len(currentBlock) > 0 {
		blocks = append(blocks, currentBlock)
	}
	
	return blocks
}

// findCalls finds all function calls in a statement
func findCalls(stmt ast.Stmt, block *Block) {
	ast.Inspect(stmt, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if callee := getCalleeName(call); callee != "" {
				block.Calls = append(block.Calls, callee)
			}
		}
		return true
	})
}

// connectBlocks connects blocks based on control flow
func connectBlocks(cfg *ControlFlowGraph) {
	for i := 1; i < len(cfg.Blocks); i++ {
		block := &cfg.Blocks[i]
		
		// If block is empty, skip it
		if len(block.Statements) == 0 {
			continue
		}
		
		// Last statement determines outgoing edges
		lastStmt := block.Statements[len(block.Statements)-1]
		
		switch s := lastStmt.(type) {
		case *ast.ReturnStmt:
			// Return statement - no successors, this is an exit block
			cfg.ExitBlocks = append(cfg.ExitBlocks, block.ID)
			
		case *ast.BranchStmt:
			// Branch statement - depends on branch type
			// This is simplified; real implementation would handle labels
			
		case *ast.IfStmt:
			// If statement - two possible successors (then and else)
			if i+1 < len(cfg.Blocks) {
				// Then branch goes to next block
				block.Successors = append(block.Successors, i+1)
				cfg.Blocks[i+1].Predecessors = append(cfg.Blocks[i+1].Predecessors, block.ID)
			}
			
			// Else branch would need additional analysis
			
		default:
			// Default - falls through to next block
			if i+1 < len(cfg.Blocks) {
				block.Successors = append(block.Successors, i+1)
				cfg.Blocks[i+1].Predecessors = append(cfg.Blocks[i+1].Predecessors, block.ID)
			}
		}
	}
}
```

With the control flow graph constructed, we can enhance our callgraph analysis:

```go
// enhancedBuildCallgraph builds a more precise callgraph
func enhancedBuildCallgraph(sourceDir string) (*Callgraph, error) {
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
			continue
		}
		
		// Get relative path
		relPath, err := filepath.Rel(sourceDir, file)
		if err != nil {
			relPath = file
		}
		
		// Find all functions and analyze their control flow
		ast.Inspect(node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				funcName := getFunctionName(fn)
				
				// Get position info
				startPos := fset.Position(fn.Pos())
				endPos := fset.Position(fn.End())
				
				// Build control flow graph
				cfg := buildControlFlowGraph(fn, fset)
				
				// Extract calls from all blocks
				var calls []string
				for _, block := range cfg.Blocks {
					calls = append(calls, block.Calls...)
				}
				
				// Create function entry
				function := &Function{
					Name:       funcName,
					File:       relPath,
					LineStart:  startPos.Line,
					LineEnd:    endPos.Line,
					Calls:      uniqueStrings(calls),
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

// uniqueStrings returns a new slice with duplicate strings removed
func uniqueStrings(strings []string) []string {
	seen := make(map[string]bool)
	var result []string
	
	for _, s := range strings {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	
	return result
}
```

## 2. Context-Sensitive Callgraphs

For more accurate analysis, we can use context-sensitive callgraphs that consider the calling context:

```go
// CallContext represents a calling context
type CallContext struct {
	Caller     string
	CallSite   token.Pos
	Args       []ast.Expr
}

// ContextSensitiveCallgraph represents a context-sensitive callgraph
type ContextSensitiveCallgraph struct {
	Nodes      map[string]*CSGNode
	Edges      []CSGEdge
}

// CSGNode represents a node in the context-sensitive callgraph
type CSGNode struct {
	Function   string
	File       string
	LineStart  int
	LineEnd    int
	Blocks     []*Block
}

// CSGEdge represents an edge in the context-sensitive callgraph
type CSGEdge struct {
	From       string
	To         string
	Context    CallContext
}

// buildContextSensitiveCallgraph builds a context-sensitive callgraph
func buildContextSensitiveCallgraph(sourceDir string) (*ContextSensitiveCallgraph, error) {
	csg := &ContextSensitiveCallgraph{
		Nodes: make(map[string]*CSGNode),
	}
	
	// Find and parse all Go files (similar to basic callgraph)
	// ...
	
	// For each function:
	// 1. Build control flow graph
	// 2. Identify call sites with context
	// 3. Create nodes and edges
	
	return csg, nil
}

// identifyCallSites identifies call sites with context
func identifyCallSites(function string, body *ast.BlockStmt, fset *token.FileSet) []CallContext {
	var callSites []CallContext
	
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			callee := ""
			
			switch expr := call.Fun.(type) {
			case *ast.Ident:
				callee = expr.Name
			case *ast.SelectorExpr:
				if x, ok := expr.X.(*ast.Ident); ok {
					callee = x.Name + "." + expr.Sel.Name
				}
			}
			
			if callee != "" {
				callSites = append(callSites, CallContext{
					Caller:   function,
					CallSite: call.Pos(),
					Args:     call.Args,
				})
			}
		}
		return true
	})
	
	return callSites
}

// analyzeCallPath analyzes a specific call path through the callgraph
func analyzeCallPath(csg *ContextSensitiveCallgraph, startNode string, visited map[string]bool) []string {
	if visited[startNode] {
		return nil // Avoid cycles
	}
	
	visited[startNode] = true
	node := csg.Nodes[startNode]
	
	var path []string
	path = append(path, startNode)
	
	// Find outgoing edges
	for _, edge := range csg.Edges {
		if edge.From == startNode {
			// Follow this edge
			subPath := analyzeCallPath(csg, edge.To, visited)
			if subPath != nil {
				path = append(path, subPath...)
			}
		}
	}
	
	return path
}
```

## 3. Handling Dynamic Dispatch and Reflection

Go's interfaces and reflection make static callgraph analysis challenging. Here's how to handle these cases:

### Interface Method Resolution

```go
// resolveInterfaceCalls resolves potential implementations of interface methods
func resolveInterfaceCalls(callgraph *Callgraph, sourceDir string) error {
	// Build interface -> implementation map
	interfaceImpls, err := buildInterfaceImplementationMap(sourceDir)
	if err != nil {
		return err
	}
	
	// For each function in the callgraph
	for key, function := range callgraph.Functions {
		var newCalls []string
		
		// Check each call to see if it's an interface method
		for _, call := range function.Calls {
			// Check if this is an interface method call
			if impls, ok := interfaceImpls[call]; ok {
				// Add all potential implementations
				newCalls = append(newCalls, impls...)
			} else {
				// Keep the original call
				newCalls = append(newCalls, call)
			}
		}
		
		// Update the function's calls
		function.Calls = uniqueStrings(newCalls)
		callgraph.Functions[key] = function
	}
	
	return nil
}

// buildInterfaceImplementationMap builds a map of interface methods to their implementations
func buildInterfaceImplementationMap(sourceDir string) (map[string][]string, error) {
	impls := make(map[string][]string)
	
	// Build type -> interface satisfaction map (similar to previous code)
	// ...
	
	return impls, nil
}
```

### Reflection Analysis

```go
// analyzeReflection analyzes reflection usage in the code
func analyzeReflection(callgraph *Callgraph, sourceDir string) error {
	// Common reflection patterns
	reflectionPatterns := map[string]struct{}{
		"reflect.ValueOf":       {},
		"reflect.TypeOf":        {},
		"reflect.New":           {},
		"reflect.Value.Method":  {},
		"reflect.Value.Call":    {},
		"reflect.Value.CallSlice": {},
	}
	
	// Find functions using reflection
	reflectionUsers := make(map[string]bool)
	
	for key, function := range callgraph.Functions {
		for _, call := range function.Calls {
			if _, isReflection := reflectionPatterns[call]; isReflection {
				reflectionUsers[key] = true
				break
			}
		}
	}
	
	// For reflection users, perform deeper analysis
	for key := range reflectionUsers {
		function := callgraph.Functions[key]
		
		// Parse the function
		filePath := filepath.Join(sourceDir, function.File)
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			continue
		}
		
		// Find the function declaration
		var funcDecl *ast.FuncDecl
		ast.Inspect(node, func(n ast.Node) bool {
			if fd, ok := n.(*ast.FuncDecl); ok {
				if getFunctionName(fd) == function.Name {
					funcDecl = fd
					return false
				}
			}
			return true
		})
		
		if funcDecl != nil {
			// Analyze reflection calls
			reflectCalls := analyzeReflectionCalls(funcDecl)
			
			// Add potential reflection targets to function calls
			function.Calls = append(function.Calls, reflectCalls...)
			callgraph.Functions[key] = function
		}
	}
	
	return nil
}

// analyzeReflectionCalls analyzes reflection calls in a function
func analyzeReflectionCalls(funcDecl *ast.FuncDecl) []string {
	var potentialTargets []string
	
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		// Look for patterns like:
		// - reflect.ValueOf(x).Method(y).Call(args)
		// - reflect.ValueOf(x).MethodByName("Method").Call(args)
		
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Call" || sel.Sel.Name == "CallSlice" {
					// This is a reflection call, try to determine the target
					if methodCall, ok := sel.X.(*ast.CallExpr); ok {
						if methodSel, ok := methodCall.Fun.(*ast.SelectorExpr); ok {
							if methodSel.Sel.Name == "MethodByName" {
								// Check if we can determine the method name
								if len(methodCall.Args) > 0 {
									if lit, ok := methodCall.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
										// Extract the method name from the string literal
										methodName := strings.Trim(lit.Value, "\"")
										
										// Add as a potential target
										// In a real implementation, you'd try to determine
										// which types this could be called on
										potentialTargets = append(potentialTargets, methodName)
									}
								}
							}
						}
					}
				}
			}
		}
		
		return true
	})
	
	return potentialTargets
}
```

## 4. Optimizing for Large Codebases

For large codebases, the basic approach might be too resource-intensive. Here are some optimization techniques:

### Incremental Analysis

```go
// incrementalCallgraphBuilder builds a callgraph incrementally
type incrementalCallgraphBuilder struct {
	callgraph      *Callgraph
	processedFiles map[string]bool
	sourceDir      string
	workQueue      []string
}

// newIncrementalCallgraphBuilder creates a new incremental callgraph builder
func newIncrementalCallgraphBuilder(sourceDir string) *incrementalCallgraphBuilder {
	return &incrementalCallgraphBuilder{
		callgraph: &Callgraph{
			Functions:   make(map[string]*Function),
			EntryPoints: make(map[string]*EntryPoint),
		},
		processedFiles: make(map[string]bool),
		sourceDir:      sourceDir,
		workQueue:      []string{},
	}
}

// buildCallgraphForFile builds a callgraph for a specific file
func (b *incrementalCallgraphBuilder) buildCallgraphForFile(filePath string) error {
	if b.processedFiles[filePath] {
		return nil // Already processed
	}
	
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	
	// Get relative path
	relPath, err := filepath.Rel(b.sourceDir, filePath)
	if err != nil {
		relPath = filePath
	}
	
	// Process functions in this file
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			// Process function (similar to basic callgraph)
			// ...
			
			// Add any new files to work queue based on imports
			for _, imp := range node.Imports {
				if imp.Path != nil {
					importPath := strings.Trim(imp.Path.Value, "\"")
					// In a real implementation, resolve this import path
					// to actual files and add them to the work queue
				}
			}
		}
		return true
	})
	
	b.processedFiles[filePath] = true
	return nil
}

// buildCallgraphForEntryPoints builds a callgraph starting from entry points
func (b *incrementalCallgraphBuilder) buildCallgraphForEntryPoints(entryPoints []string) (*Callgraph, error) {
	// Find files containing entry points
	// ...
	
	// Add them to work queue
	// ...
	
	// Process work queue
	for len(b.workQueue) > 0 {
		filePath := b.workQueue[0]
		b.workQueue = b.workQueue[1:]
		
		if err := b.buildCallgraphForFile(filePath); err != nil {
			return nil, err
		}
	}
	
	return b.callgraph, nil
}
```

### Parallel Analysis

```go
// parallelCallgraphBuilder builds a callgraph in parallel
type parallelCallgraphBuilder struct {
	callgraph      *Callgraph
	sourceDir      string
	concurrency    int
	mutex          sync.Mutex
}

// buildCallgraphInParallel builds a callgraph in parallel
func (b *parallelCallgraphBuilder) buildCallgraphInParallel(files []string) (*Callgraph, error) {
	// Create a work queue
	workQueue := make(chan string, len(files))
	for _, file := range files {
		workQueue <- file
	}
	close(workQueue)
	
	// Create a wait group
	var wg sync.WaitGroup
	wg.Add(b.concurrency)
	
	// Start worker goroutines
	for i := 0; i < b.concurrency; i++ {
		go func() {
			defer wg.Done()
			
			for filePath := range workQueue {
				// Process file
				if err := b.processFile(filePath); err != nil {
					// Log error, but continue
					fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", filePath, err)
				}
			}
		}()
	}
	
	// Wait for all workers to finish
	wg.Wait()
	
	return b.callgraph, nil
}

// processFile processes a single file
func (b *parallelCallgraphBuilder) processFile(filePath string) error {
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	
	// Get relative path
	relPath, err := filepath.Rel(b.sourceDir, filePath)
	if err != nil {
		relPath = filePath
	}
	
	// Process functions (similar to basic callgraph)
	// ...
	
	// Thread-safe update of callgraph
	b.mutex.Lock()
	// Update callgraph
	b.mutex.Unlock()
	
	return nil
}
```

## 5. Integration with Go's Analysis Framework

Go provides a powerful `go/analysis` framework that we can leverage:

```go
// analysisCallgraphBuilder.go
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// callgraphAnalyzer is an analysis that builds a callgraph
var callgraphAnalyzer = &analysis.Analyzer{
	Name:     "callgraph",
	Doc:      "builds a callgraph for the analyzed package",
	Run:      runCallgraphAnalysis,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	ResultType: reflect.TypeOf((*callgraph.Graph)(nil)),
}

func runCallgraphAnalysis(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	
	// Collect function declarations
	var funcs []*ast.FuncDecl
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}
	
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		funcs = append(funcs, n.(*ast.FuncDecl))
	})
	
	// Build SSA representation
	prog, pkgs := ssautil.AllPackages(pass.Pkg, 0)
	prog.Build()
	
	// Build callgraph using Class Hierarchy Analysis
	cg := cha.CallGraph(prog)
	
	return cg, nil
}

// analyzeWithGoAnalysis uses Go's analysis framework to build a callgraph
func analyzeWithGoAnalysis(packagePatterns []string) (*callgraph.Graph, error) {
	// Configure package loading
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedDeps,
	}
	
	// Load packages
	pkgs, err := packages.Load(cfg, packagePatterns...)
	if err != nil {
		return nil, err
	}
	
	// Create analyzer
	analyzer := callgraphAnalyzer
	
	// Run analyzer
	results, err := analysis.Run([]*analysis.Analyzer{analyzer}, pkgs)
	if err != nil {
		return nil, err
	}
	
	// Extract callgraph from results
	var cg *callgraph.Graph
	for _, pass := range results {
		if result, ok := pass.Result.(*callgraph.Graph); ok {
			cg = result
			break
		}
	}
	
	if cg == nil {
		return nil, fmt.Errorf("callgraph analysis did not produce a result")
	}
	
	return cg, nil
}
```

Now we can adapt this to generate synthetic coverage:

```go
// convertToSyntheticCoverage converts a callgraph to synthetic coverage
func convertToSyntheticCoverage(cg *callgraph.Graph, commands []string, 
	entryPoints map[string]*ssa.Function, packagePrefix string) (string, error) {
	
	// Map of functions that will be executed
	executed := make(map[*ssa.Function]bool)
	
	// Find entry points for commands
	for _, cmd := range commands {
		if entry, ok := entryPoints[cmd]; ok {
			executed[entry] = true
			
			// Follow callgraph edges
			node := cg.Nodes[entry]
			if node != nil {
				followCallgraphEdges(cg, node, executed)
			}
		}
	}
	
	// Generate coverage lines
	var lines []string
	lines = append(lines, "mode: set")
	
	// Add a line for each executed function
	for fn := range executed {
		if fn.Pos().IsValid() {
			// Get file and line information
			pos := fn.Prog.Fset.Position(fn.Pos())
			endPos := fn.Prog.Fset.Position(fn.Body.Lbrace)
			
			// Create coverage line
			line := fmt.Sprintf("%s/%s:%d.1,%d.1 %d 1", 
				packagePrefix, 
				pos.Filename, 
				pos.Line, 
				endPos.Line, 
				endPos.Line - pos.Line + 1)
			
			lines = append(lines, line)
		}
	}
	
	return strings.Join(lines, "\n"), nil
}

// followCallgraphEdges follows edges in the callgraph
func followCallgraphEdges(cg *callgraph.Graph, node *callgraph.Node, executed map[*ssa.Function]bool) {
	// Process outgoing edges
	for _, edge := range node.Out {
		callee := edge.Callee.Func
		if !executed[callee] {
			executed[callee] = true
			followCallgraphEdges(cg, edge.Callee, executed)
		}
	}
}
```

## Complete Example: Advanced Callgraph Analyzer

Let's put it all together in a complete tool:

```go
// advanced-callgraph-analyzer.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var (
	scriptFile   = flag.String("script", "", "Path to testscript file")
	packagePatterns = flag.String("packages", "./...", "Package patterns to analyze")
	outputFile   = flag.String("output", "advanced-coverage.txt", "Output coverage file")
	packagePrefix = flag.String("prefix", "", "Package name prefix for coverage")
	verbose      = flag.Bool("v", false, "Verbose output")
	concurrent   = flag.Int("concurrent", 4, "Number of concurrent workers")
	contextSensitive = flag.Bool("context", false, "Use context-sensitive analysis")
)

func main() {
	flag.Parse()
	
	if *scriptFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -script flag is required\n")
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
	
	// Load packages
	pkgs, err := loadPackages(*packagePatterns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading packages: %v\n", err)
		os.Exit(1)
	}
	
	if *verbose {
		fmt.Printf("Loaded %d packages\n", len(pkgs))
	}
	
	// Build SSA representation
	prog, _ := ssautil.AllPackages(pkgs, ssa.BuilderMode(0))
	prog.Build()
	
	// Build callgraph
	var cg *callgraph.Graph
	if *contextSensitive {
		// Use a context-sensitive algorithm (like RTA)
		// For simplicity, we're using CHA here
		cg = cha.CallGraph(prog)
	} else {
		// Use Class Hierarchy Analysis
		cg = cha.CallGraph(prog)
	}
	
	if *verbose {
		fmt.Printf("Built callgraph with %d nodes\n", len(cg.Nodes))
	}
	
	// Map commands to entry points
	entryPoints, err := mapCommandsToEntryPoints(cg, commands)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error mapping commands: %v\n", err)
		os.Exit(1)
	}
	
	// Generate synthetic coverage
	coverage, err := convertToSyntheticCoverage(cg, commands, entryPoints, *packagePrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating coverage: %v\n", err)
		os.Exit(1)
	}
	
	// Write coverage file
	if err := os.WriteFile(*outputFile, []byte(coverage), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing coverage file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Generated synthetic coverage in %s\n", *outputFile)
}

// loadPackages loads Go packages for analysis
func loadPackages(patterns string) ([]*types.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedDeps,
	}
	
	pkgs, err := packages.Load(cfg, strings.Split(patterns, ",")...)
	if err != nil {
		return nil, err
	}
	
	var typesPkgs []*types.Package
	for _, pkg := range pkgs {
		typesPkgs = append(typesPkgs, pkg.Types)
	}
	
	return typesPkgs, nil
}

// mapCommandsToEntryPoints maps commands to entry points in the callgraph
func mapCommandsToEntryPoints(cg *callgraph.Graph, commands []string) (map[string]*ssa.Function, error) {
	entryPoints := make(map[string]*ssa.Function)
	
	// This is a simplified implementation
	// In a real tool, you would:
	// 1. Analyze the code to find command handlers
	// 2. Map command names to handler functions
	
	// For demonstration, we'll assume a common pattern:
	// Functions named "handle<Command>" or "cmd<Command>" are entry points
	for _, node := range cg.Nodes {
		fn := node.Func
		if fn == nil || fn.Pkg == nil {
			continue
		}
		
		name := fn.Name()
		
		// Check for common handler naming patterns
		isHandler := false
		lowerName := strings.ToLower(name)
		for _, cmd := range commands {
			lowerCmd := strings.ToLower(cmd)
			
			if strings.HasPrefix(lowerName, "handle"+lowerCmd) ||
			   strings.HasPrefix(lowerName, "cmd"+lowerCmd) {
				isHandler = true
				entryPoints[cmd] = fn
				break
			}
		}
		
		if *verbose && isHandler {
			fmt.Printf("Found handler: %s\n", name)
		}
	}
	
	return entryPoints, nil
}

// convertToSyntheticCoverage and followCallgraphEdges as defined earlier
// ...

// extractCommands as defined earlier
// ...
```

## When to Use Advanced Techniques

These advanced techniques are most valuable in specific scenarios:

1. **Large codebases**: The optimizations for large codebases become important when analyzing projects with hundreds of thousands of lines of code.

2. **Heavy interface usage**: Context-sensitive analysis and interface resolution are critical for codebases that make extensive use of interfaces and dependency injection.

3. **Reflection-heavy code**: If your code uses reflection extensively (e.g., for plugins or dynamic dispatch), the reflection analysis becomes essential.

4. **Precise coverage metrics**: When you need line-by-line coverage rather than function-level coverage, the more sophisticated analysis techniques provide greater accuracy.

5. **Integration with existing tools**: The Go analysis framework integration is valuable when you need to combine callgraph analysis with other Go tools.

## Conclusion

This guide has explored advanced techniques for callgraph analysis to generate synthetic coverage from testscript files. By using these approaches, you can generate more accurate and comprehensive coverage data that better represents the code paths exercised by your testscript tests.

Remember that while these techniques can significantly improve the accuracy of synthetic coverage, they also come with increased complexity and resource requirements. Choose the approach that best fits your project's needs, balancing accuracy with performance and maintenance costs.

In the next part, we'll explore ways to integrate these techniques into your CI/CD pipelines and how to maintain synthetic coverage as your codebase evolves.