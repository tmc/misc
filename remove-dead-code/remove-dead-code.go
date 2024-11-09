// Command remove-dead-code removes or comments out dead code in Go files based on the dead code JSON file.
//
// BUGS:
// - missing topo sort to process files/func in the correct order
// - misses function comments
// TODO:
// - Add tests
// - Add support for removing dead code from test files
// - Add support for removing dead code from other types of files (e.g. .proto files)
// - Make it not confused by temporal, if possible.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/imports"
)

type Package struct {
	Name  string     `json:"Name"`
	Path  string     `json:"Path"`
	Funcs []Function `json:"Funcs"`
}

type Function struct {
	Name      string   `json:"Name"`
	Position  Position `json:"Position"`
	Generated bool     `json:"Generated"`
}

type Position struct {
	File string `json:"File"`
	Line int    `json:"Line"`
	Col  int    `json:"Col"`
}

type FileProcessor struct {
	fset      *token.FileSet
	mode      string
	filePath  string
	functions []Function
	modified  bool
	notFound  map[string]string
}

var (
	flagWarnOnlyOnCycles = flag.Bool("warn-only-on-cycles", true, "Only warn on cycles instead of failing")
	flagMode             = flag.String("mode", "print", "one of print, comment, or remove")
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()
	if err := validateArgs(); err != nil {
		return err
	}

	jsonFile := flag.Args()[0]
	mode := *flagMode
	fmt.Fprintln(os.Stderr, "Processing dead code in", jsonFile, "with mode", mode)

	packages, err := loadPackages(jsonFile)
	if err != nil {
		return fmt.Errorf("loading packages: %w", err)
	}

	fileToFuncs := buildFileMap(packages)
	return processFiles(fileToFuncs, mode)
}

func validateArgs() error {
	if len(flag.Args()) != 1 {
		return fmt.Errorf("usage: <deadcode_json_file> -mode=print|comment|remove")
	}
	mode := *flagMode
	if mode != "comment" && mode != "remove" && mode != "print" {
		return fmt.Errorf("invalid mode. Use 'comment', 'print', or 'remove', got %s", mode)
	}
	return nil
}

func loadPackages(jsonFile string) ([]Package, error) {
	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("reading JSON file: %w", err)
	}

	var packages []Package
	if err := json.Unmarshal(jsonData, &packages); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	return packages, nil
}

func buildFileMap(packages []Package) map[string][]Function {
	fileToFuncs := make(map[string][]Function)
	for _, pkg := range packages {
		for _, function := range pkg.Funcs {
			fileToFuncs[function.Position.File] = append(fileToFuncs[function.Position.File], function)
		}
	}
	return fileToFuncs
}

// Node represents a node in a dependency graph
// we are doing to to topo-sort to removal order
type Node struct {
	Name     string
	Edges    map[string]bool
	Visited  bool
	Visiting bool
}

func buildDependencyGraph(packages []Package) map[string]*Node {
	graph := make(map[string]*Node)

	// Create nodes for all functions
	for _, pkg := range packages {
		for _, fn := range pkg.Funcs {
			if _, exists := graph[fn.Name]; !exists {
				graph[fn.Name] = &Node{
					Name:  fn.Name,
					Edges: make(map[string]bool),
				}
			}
		}
	}

	// Add edges based on function dependencies
	// This would require analyzing function calls within each function
	// For now, we'll just use file-level dependencies
	for _, pkg := range packages {
		for _, fn := range pkg.Funcs {
			file := fn.Position.File
			for _, otherFn := range pkg.Funcs {
				if otherFn.Position.File == file && otherFn.Name != fn.Name {
					graph[fn.Name].Edges[otherFn.Name] = true
				}
			}
		}
	}

	return graph
}

// topoSort performs a topological sort on the given graph
// If warnOnlyOnCycles is true, it will only warn on cycles instead of failing
// Cycles are detected by the presence of a node that is currently being visited,
// but not yet visited, while traversing the graph.
// This is a naive implementation and may not work in all cases.
func topoSort(graph map[string]*Node, warnOnlyOnCycles bool) ([]string, error) {
	var result []string

	var visit func(name string) error
	visit = func(name string) error {
		node := graph[name]
		if node.Visiting {
			if warnOnlyOnCycles {
				fmt.Printf("Warning: cycle detected at %s\n", name)
				return nil
			} else {
				return fmt.Errorf("cycle detected at %s", name)
			}
		}
		if node.Visited {
			return nil
		}

		node.Visiting = true
		for dep := range node.Edges {
			if err := visit(dep); err != nil {
				return err
			}
		}
		node.Visiting = false
		node.Visited = true
		result = append(result, name)
		return nil
	}

	for name := range graph {
		if !graph[name].Visited {
			if err := visit(name); err != nil {
				return nil, err
			}
		}
	}

	// Reverse the result to get correct order
	for i := 0; i < len(result)/2; i++ {
		result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
	}

	return result, nil
}

// topoSortDumb is a simpler version of topoSort that doesn't handle cycles
func topoSortDumb(graph map[string]*Node) ([]string, error) {
	var result []string
	visited := make(map[string]bool)

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		for dep := range graph[name].Edges {
			visit(dep)
		}
		result = append(result, name)
	}

	for name := range graph {
		visit(name)
	}

	return result, nil
}

func processFiles(fileToFuncs map[string][]Function, mode string) error {
	// Build dependency graph
	packages := []Package{{Funcs: []Function{}}}
	for _, funcs := range fileToFuncs {
		packages[0].Funcs = append(packages[0].Funcs, funcs...)
	}

	graph := buildDependencyGraph(packages)
	sortedFuncs, err := topoSortDumb(graph)
	//sortedFuncs, err := topoSort(graph, *flagWarnOnlyOnCycles)
	if err != nil {
		return fmt.Errorf("topological sort failed: %w", err)
	}

	// Create a map of function names to their files
	funcToFile := make(map[string]string)
	for file, funcs := range fileToFuncs {
		for _, f := range funcs {
			funcToFile[f.Name] = file
		}
	}

	// Process files in order
	processedFiles := make(map[string]bool)
	for _, funcName := range sortedFuncs {
		file := funcToFile[funcName]
		if !processedFiles[file] {
			processor := NewFileProcessor(file, fileToFuncs[file], mode)
			if err := processor.Process(); err != nil {
				return fmt.Errorf("processing %s: %w", file, err)
			}
			processedFiles[file] = true
		}
	}

	return nil
}

func NewFileProcessor(filePath string, functions []Function, mode string) *FileProcessor {
	return &FileProcessor{
		fset:      token.NewFileSet(),
		mode:      mode,
		filePath:  filePath,
		functions: functions,
		notFound:  make(map[string]string),
	}
}

func (fp *FileProcessor) Process() error {
	node, err := parser.ParseFile(fp.fset, fp.filePath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	fp.processNode(node)

	if !fp.modified && len(fp.notFound) == 0 {
		fmt.Printf("no functions processed in file %s\n", fp.filePath)
		return nil
	}

	if fp.modified {
		if err := fp.writeModifiedFile(node); err != nil {
			return err
		}
	}

	fp.reportUnprocessedFunctions()
	return nil
}

func (fp *FileProcessor) processNode(node *ast.File) {
	ast.Inspect(node, func(n ast.Node) bool {
		if fd, ok := n.(*ast.FuncDecl); ok {
			fp.processFuncDecl(node, fd)
		}
		return true
	})
}

func (fp *FileProcessor) processFuncDecl(node *ast.File, fd *ast.FuncDecl) {
	for i, function := range fp.functions {
		if fp.matchesFunction(fd, function) {
			if fp.mode == "comment" {
				fp.commentOutFunction(fd, function)
			} else if fp.mode == "remove" {
				removeNode(node, fd)
			} else {
				fmt.Printf("function %s found at line %d\n", function.Name, fp.fset.Position(fd.Pos()).Line)
			}
			fp.modified = true
			fp.functions[i] = fp.functions[len(fp.functions)-1]
			fp.functions = fp.functions[:len(fp.functions)-1]
			break
		}
	}
}

func (fp *FileProcessor) matchesFunction(fd *ast.FuncDecl, function Function) bool {
	funcName := extractFuncName(function.Name)
	if fd.Name.Name != funcName {
		return false
	}

	if fd.Recv != nil && strings.Contains(function.Name, ".") {
		recvName := getReceiverTypeName(fd.Recv.List[0].Type)
		if recvName != strings.Split(function.Name, ".")[0] {
			return false
		}
	}

	if fp.fset.Position(fd.Pos()).Line != function.Position.Line {
		fp.notFound[function.Name] = fmt.Sprintf("Found at different line: %d", fp.fset.Position(fd.Pos()).Line)
		return false
	}

	return true
}

func (fp *FileProcessor) writeModifiedFile(node *ast.File) error {
	tempFile, err := os.CreateTemp(filepath.Dir(fp.filePath), "temp_*.go")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if err := printer.Fprint(tempFile, fp.fset, node); err != nil {
		return fmt.Errorf("writing to temporary file: %w", err)
	}
	tempFile.Close()

	processedSource, err := formatFile(tempFile.Name())
	if err != nil {
		return err
	}

	if err := os.WriteFile(fp.filePath, processedSource, 0644); err != nil {
		return fmt.Errorf("writing to original file: %w", err)
	}

	mode := fmt.Sprintf("%sed", fp.mode)
	if fp.mode != "print" {
		mode = fmt.Sprintf("%s and formatted", mode)
	}
	fmt.Printf("processed file %s %s\n", fp.filePath, fp.mode)
	return nil
}

func formatFile(path string) ([]byte, error) {
	options := &imports.Options{
		Fragment:  true,
		AllErrors: true,
		Comments:  true,
		TabIndent: true,
		TabWidth:  8,
	}
	return imports.Process(path, nil, options)
}

func (fp *FileProcessor) reportUnprocessedFunctions() {
	if len(fp.functions) > 0 || len(fp.notFound) > 0 {
		fmt.Printf("Warning: The following functions were not processed in %s:\n", fp.filePath)
		for _, f := range fp.functions {
			reason := fp.notFound[f.Name]
			if reason == "" {
				reason = "Not found"
			}
			fmt.Printf("  - %s (Line: %d, Col: %d): %s\n", f.Name, f.Position.Line, f.Position.Col, reason)
		}
	}
}

func (fp *FileProcessor) commentOutFunction(fd *ast.FuncDecl, function Function) {
	fd.Doc = &ast.CommentGroup{
		List: []*ast.Comment{{
			Text: fmt.Sprintf("// Commented out as dead code (Line: %d, Col: %d)",
				function.Position.Line, function.Position.Col),
		}},
	}
	fd.Body.List = nil
}

func extractFuncName(fullName string) string {
	if strings.Contains(fullName, ".") {
		parts := strings.Split(fullName, ".")
		return parts[len(parts)-1]
	}
	return fullName
}

func removeNode(file *ast.File, node ast.Node) {
	var newDecls []ast.Decl
	for _, decl := range file.Decls {
		if decl != node {
			newDecls = append(newDecls, decl)
		}
	}
	file.Decls = newDecls
}

func getReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}
