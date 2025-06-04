package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var (
	moduleName = flag.String("module", "", "Module name to parse (e.g., mysql, postgres)")
	modulePath = flag.String("path", "", "Path to module source (if not in standard location)")
	outputDir  = flag.String("out", "", "Output directory for generated module")
	verbose    = flag.Bool("v", false, "Verbose output")
)

// ModuleData holds all extracted information from parsing
type ModuleData struct {
	ModuleName     string
	PackageName    string
	DefaultImage   string
	DefaultPort    string
	ExposedPorts   []string
	EnvVars        map[string]string
	WaitStrategies []WaitStrategy
	Options        []Option
	Commands       []string
	Entrypoint     []string
	Mounts         []Mount
	Networks       []string
	HasDSNSupport  bool
}

// WaitStrategy represents different wait strategies found
type WaitStrategy struct {
	Type     string // log, http, exec, etc.
	Value    string
	Timeout  string
}

// Option represents a With* function
type Option struct {
	Name        string
	Type        string
	ParamName   string
	ParamType   string
	EnvVar      string
	Description string
	Effect      string // what it does to the container
}

// Mount represents volume/bind mounts
type Mount struct {
	Source string
	Target string
	Type   string
}

func main() {
	flag.Parse()

	if *moduleName == "" {
		log.Fatal("Please specify a module name with -module")
	}

	// Find module path
	modPath := *modulePath
	if modPath == "" {
		modPath = findModulePath(*moduleName)
		if modPath == "" {
			log.Fatalf("Could not find module %s", *moduleName)
		}
	}

	log.Printf("Parsing module %s from %s", *moduleName, modPath)

	// Parse the module
	moduleData, err := parseModule(*moduleName, modPath)
	if err != nil {
		log.Fatalf("Failed to parse module: %v", err)
	}

	// Generate testctr module
	outputPath := *outputDir
	if outputPath == "" {
		outputPath = filepath.Join(".", moduleData.PackageName)
	}

	err = generateTestctrModule(moduleData, outputPath)
	if err != nil {
		log.Fatalf("Failed to generate module: %v", err)
	}

	log.Printf("Successfully generated testctr module in %s", outputPath)
}

func findModulePath(moduleName string) string {
	// Try to find in GOPATH module cache
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		if output, err := os.ReadFile("/tmp/gopath"); err == nil {
			gopath = strings.TrimSpace(string(output))
		}
	}

	if gopath != "" {
		pattern := filepath.Join(gopath, "pkg", "mod", "github.com", "testcontainers", "testcontainers-go", "modules", moduleName+"@*")
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return matches[len(matches)-1] // Return latest version
		}
	}

	return ""
}

func parseModule(moduleName, modulePath string) (*ModuleData, error) {
	data := &ModuleData{
		ModuleName:  moduleName,
		PackageName: moduleName,
		EnvVars:     make(map[string]string),
		Options:     []Option{},
	}

	// Parse all Go files in the module
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, modulePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Analyze each file
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			analyzeFile(fset, file, data)
		}
	}

	// Post-process to infer additional data
	inferDefaults(data)

	return data, nil
}

func analyzeFile(fset *token.FileSet, file *ast.File, data *ModuleData) {
	// Walk the AST
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			analyzeGenDecl(node, data)
		case *ast.FuncDecl:
			analyzeFuncDecl(node, data)
		case *ast.CallExpr:
			analyzeCallExpr(node, data)
		}
		return true
	})
}

func analyzeGenDecl(decl *ast.GenDecl, data *ModuleData) {
	// Look for constants that define defaults
	if decl.Tok == token.CONST {
		for _, spec := range decl.Specs {
			if valueSpec, ok := spec.(*ast.ValueSpec); ok {
				for i, name := range valueSpec.Names {
					if i < len(valueSpec.Values) {
						if lit, ok := valueSpec.Values[i].(*ast.BasicLit); ok {
							val := strings.Trim(lit.Value, `"`)
							nameStr := name.Name
							
							// Extract default values
							switch {
							case strings.Contains(strings.ToLower(nameStr), "image"):
								if strings.Contains(val, ":") && !strings.Contains(val, "//") {
									data.DefaultImage = val
								}
							case strings.Contains(strings.ToLower(nameStr), "port"):
								if isPort(val) {
									data.DefaultPort = val
								}
							case strings.Contains(strings.ToLower(nameStr), "user"):
								data.EnvVars[data.ModuleName+"_USER"] = val
							case strings.Contains(strings.ToLower(nameStr), "password"):
								data.EnvVars[data.ModuleName+"_PASSWORD"] = val
							case strings.Contains(strings.ToLower(nameStr), "database"):
								data.EnvVars[data.ModuleName+"_DATABASE"] = val
							}
						}
					}
				}
			}
		}
	}
}

func analyzeFuncDecl(fn *ast.FuncDecl, data *ModuleData) {
	if fn.Name == nil {
		return
	}

	name := fn.Name.Name

	// Analyze With* functions
	if strings.HasPrefix(name, "With") && len(name) > 4 {
		option := analyzeOptionFunction(fn, data)
		if option != nil {
			data.Options = append(data.Options, *option)
		}
	}

	// Look for Run/RunContainer to extract container setup
	if name == "Run" || name == "RunContainer" {
		analyzeRunFunction(fn, data)
	}
}

func analyzeOptionFunction(fn *ast.FuncDecl, data *ModuleData) *Option {
	option := &Option{
		Name: fn.Name.Name,
		Type: "CustomizeRequestOption",
	}

	// Extract parameter info
	if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
		param := fn.Type.Params.List[0]
		if len(param.Names) > 0 {
			option.ParamName = param.Names[0].Name
		}
		option.ParamType = formatType(param.Type)
	}

	// Extract documentation
	if fn.Doc != nil {
		var docs []string
		for _, comment := range fn.Doc.List {
			text := strings.TrimPrefix(comment.Text, "//")
			text = strings.TrimSpace(text)
			if text != "" && !strings.HasPrefix(text, "Deprecated") {
				docs = append(docs, text)
			}
		}
		option.Description = strings.Join(docs, " ")
	}

	// Analyze function body to understand what it does
	if fn.Body != nil {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				// Look for env var assignments
				if sel, ok := node.Lhs[0].(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "req" {
						if sel.Sel.Name == "Env" {
							// This is setting an environment variable
							if idx, ok := node.Lhs[0].(*ast.IndexExpr); ok {
								if lit, ok := idx.Index.(*ast.BasicLit); ok {
									envVar := strings.Trim(lit.Value, `"`)
									option.EnvVar = envVar
									option.Effect = fmt.Sprintf("Sets %s environment variable", envVar)
								}
							}
						}
					}
				}
			}
			return true
		})
	}

	// Infer env var from name if not found
	if option.EnvVar == "" {
		optionName := strings.TrimPrefix(option.Name, "With")
		option.EnvVar = inferEnvVar(data.ModuleName, optionName)
	}

	return option
}

func analyzeRunFunction(fn *ast.FuncDecl, data *ModuleData) {
	// Look for ContainerRequest initialization
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			if isContainerRequest(node.Type) {
				analyzeContainerRequest(node, data)
			}
		}
		return true
	})
}

func analyzeContainerRequest(lit *ast.CompositeLit, data *ModuleData) {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok {
				switch ident.Name {
				case "Image":
					if lit, ok := kv.Value.(*ast.BasicLit); ok {
						data.DefaultImage = strings.Trim(lit.Value, `"`)
					}
				case "ExposedPorts":
					analyzePorts(kv.Value, data)
				case "Env":
					analyzeEnv(kv.Value, data)
				case "WaitingFor":
					analyzeWaitStrategy(kv.Value, data)
				case "Cmd":
					analyzeStringSlice(kv.Value, &data.Commands)
				case "Entrypoint":
					analyzeStringSlice(kv.Value, &data.Entrypoint)
				}
			}
		}
	}
}

func analyzeCallExpr(call *ast.CallExpr, data *ModuleData) {
	// Look for wait strategy calls
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "wait" {
			strategy := WaitStrategy{Type: sel.Sel.Name}
			
			// Extract arguments
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok {
					strategy.Value = strings.Trim(lit.Value, `"`)
				}
			}
			
			data.WaitStrategies = append(data.WaitStrategies, strategy)
		}
	}
}

func analyzePorts(expr ast.Expr, data *ModuleData) {
	if comp, ok := expr.(*ast.CompositeLit); ok {
		for _, elt := range comp.Elts {
			if lit, ok := elt.(*ast.BasicLit); ok {
				port := strings.Trim(lit.Value, `"`)
				data.ExposedPorts = append(data.ExposedPorts, port)
				
				// Extract just the port number as default
				if data.DefaultPort == "" {
					parts := strings.Split(port, "/")
					if len(parts) > 0 {
						data.DefaultPort = parts[0]
					}
				}
			}
		}
	}
}

func analyzeEnv(expr ast.Expr, data *ModuleData) {
	if comp, ok := expr.(*ast.CompositeLit); ok {
		for _, elt := range comp.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if keyLit, ok := kv.Key.(*ast.BasicLit); ok {
					if valLit, ok := kv.Value.(*ast.BasicLit); ok {
						key := strings.Trim(keyLit.Value, `"`)
						val := strings.Trim(valLit.Value, `"`)
						data.EnvVars[key] = val
					}
				}
			}
		}
	}
}

func analyzeWaitStrategy(expr ast.Expr, data *ModuleData) {
	// This is complex - wait strategies can be chained function calls
	// For now, just mark that we found one
	if data.WaitStrategies == nil {
		data.WaitStrategies = []WaitStrategy{}
	}
}

func analyzeStringSlice(expr ast.Expr, target *[]string) {
	if comp, ok := expr.(*ast.CompositeLit); ok {
		for _, elt := range comp.Elts {
			if lit, ok := elt.(*ast.BasicLit); ok {
				*target = append(*target, strings.Trim(lit.Value, `"`))
			}
		}
	}
}

func isContainerRequest(expr ast.Expr) bool {
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		return sel.Sel.Name == "ContainerRequest"
	}
	return false
}

func isPort(s string) bool {
	// Simple check if string looks like a port
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0 && len(s) <= 5
}

func formatType(expr ast.Expr) string {
	var buf bytes.Buffer
	format.Node(&buf, token.NewFileSet(), expr)
	return buf.String()
}

func inferEnvVar(moduleName, optionName string) string {
	module := strings.ToUpper(moduleName)
	option := strings.ToUpper(optionName)
	
	// Handle common patterns
	switch optionName {
	case "Database":
		return module + "_DATABASE"
	case "Username", "User":
		return module + "_USER"
	case "Password":
		return module + "_PASSWORD"
	case "RootPassword":
		return module + "_ROOT_PASSWORD"
	default:
		return module + "_" + option
	}
}

func inferDefaults(data *ModuleData) {
	// Infer DSN support
	for k := range data.EnvVars {
		if strings.Contains(k, "DATABASE") || strings.Contains(k, "USER") || strings.Contains(k, "PASSWORD") {
			data.HasDSNSupport = true
			break
		}
	}

	// Infer wait strategy from module type
	if len(data.WaitStrategies) == 0 {
		switch data.ModuleName {
		case "mysql":
			data.WaitStrategies = append(data.WaitStrategies, WaitStrategy{
				Type:  "log",
				Value: "ready for connections",
			})
		case "postgres":
			data.WaitStrategies = append(data.WaitStrategies, WaitStrategy{
				Type:  "log",
				Value: "database system is ready to accept connections",
			})
		case "redis":
			data.WaitStrategies = append(data.WaitStrategies, WaitStrategy{
				Type:  "log",
				Value: "Ready to accept connections",
			})
		}
	}
}

func generateTestctrModule(data *ModuleData, outputPath string) error {
	// Create output directory
	err := os.MkdirAll(outputPath, 0755)
	if err != nil {
		return err
	}

	// Generate main module file
	err = generateMainFile(data, outputPath)
	if err != nil {
		return err
	}

	// Generate doc.go
	err = generateDocFile(data, outputPath)
	if err != nil {
		return err
	}

	// Generate test file
	err = generateTestFile(data, outputPath)
	if err != nil {
		return err
	}

	return nil
}

const moduleTemplate = `// Code generated by parse-tc-module. DO NOT EDIT.

package {{.PackageName}}

import (
{{if or .HasDSNSupport (gt (len .Options) 0)}}	"fmt"
{{end}}	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for {{.ModuleName}} containers.
func Default() testctr.Option {
	return testctr.Options(
{{if .DefaultPort}}		testctr.WithPort("{{.DefaultPort}}"),
{{end}}{{range .EnvVars}}		testctr.WithEnv("{{.}}", "{{.}}"),
{{end}}{{range .WaitStrategies}}		ctropts.WithWaitForLog("{{.Value}}", 30*time.Second),
{{end}}{{if .HasDSNSupport}}		// TODO: Add DSN provider when fully implemented
{{end}}	)
}

{{range .Options}}
{{if .Description}}// {{.Name}} {{.Description}}
{{end}}func {{.Name}}({{.ParamName}} {{.ParamType}}) testctr.Option {
{{if .EnvVar}}	return testctr.WithEnv("{{.EnvVar}}", {{if eq .ParamType "string"}}{{.ParamName}}{{else}}fmt.Sprintf("%v", {{.ParamName}}){{end}})
{{else}}	// TODO: Implement {{.Name}} option ({{.Effect}})
	return testctr.OptionFunc(func(interface{}) {})
{{end}}}

{{end}}`

const docTemplate = `// Code generated by parse-tc-module. DO NOT EDIT.

/*
Package {{.PackageName}} provides testctr support for {{.ModuleName}} containers.

This package was generated by parsing testcontainers-go/modules/{{.ModuleName}}.

# Default Configuration

{{if .DefaultImage}}Image: {{.DefaultImage}}
{{end}}{{if .DefaultPort}}Port: {{.DefaultPort}}
{{end}}{{if .ExposedPorts}}Exposed Ports: {{range .ExposedPorts}}{{.}} {{end}}
{{end}}{{if .EnvVars}}Environment Variables:
{{range $k, $v := .EnvVars}}  - {{$k}}: {{$v}}
{{end}}{{end}}{{if .WaitStrategies}}Wait Strategies:
{{range .WaitStrategies}}  - {{.Type}}: {{.Value}}
{{end}}{{end}}

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/modules/{{.PackageName}}"
	)

	func TestWith{{.ModuleName | title}}(t *testing.T) {
		container := testctr.New(t, "{{if .DefaultImage}}{{.DefaultImage}}{{else}}{{.ModuleName}}:latest{{end}}", {{.PackageName}}.Default())
		// Use container...
	}

{{if gt (len .Options) 0}}# Configuration Options

{{range .Options}}## {{.Name}}

{{.Description}}

{{if .Effect}}Effect: {{.Effect}}
{{end}}{{if .EnvVar}}Sets environment variable: {{.EnvVar}}
{{end}}
{{end}}{{end}}*/
package {{.PackageName}}
`

const testTemplate = `// Code generated by parse-tc-module. DO NOT EDIT.

package {{.PackageName}}_test

import (
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/exp/modules/{{.PackageName}}"
)

func Test{{.ModuleName | title}}Container(t *testing.T) {
	t.Parallel()

	container := testctr.New(t, "{{if .DefaultImage}}{{.DefaultImage}}{{else}}{{.ModuleName}}:latest{{end}}", {{.PackageName}}.Default())
	
	if container.ID() == "" {
		t.Fatal("container ID should not be empty")
	}

	{{if .DefaultPort}}port := container.Port()
	if port == "" {
		t.Fatal("container port should not be empty")
	}

	addr := container.Addr("{{.DefaultPort}}")
	if addr == "" {
		t.Fatalf("failed to get address for port {{.DefaultPort}}")
	}
	{{end}}
}

{{if gt (len .Options) 0}}func Test{{.ModuleName | title}}WithOptions(t *testing.T) {
	t.Parallel()

	container := testctr.New(t, "{{if .DefaultImage}}{{.DefaultImage}}{{else}}{{.ModuleName}}:latest{{end}}",
		{{.PackageName}}.Default(),
		// Add custom options here
	)

	if container.ID() == "" {
		t.Fatal("container ID should not be empty")
	}
}
{{end}}`

func generateMainFile(data *ModuleData, outputPath string) error {
	return generateFromTemplate(moduleTemplate, data, filepath.Join(outputPath, data.PackageName+".go"))
}

func generateDocFile(data *ModuleData, outputPath string) error {
	return generateFromTemplate(docTemplate, data, filepath.Join(outputPath, "doc.go"))
}

func generateTestFile(data *ModuleData, outputPath string) error {
	return generateFromTemplate(testTemplate, data, filepath.Join(outputPath, data.PackageName+"_test.go"))
}

func generateFromTemplate(tmplStr string, data *ModuleData, outputFile string) error {
	tmpl, err := template.New("gen").Funcs(template.FuncMap{
		"title": strings.Title,
	}).Parse(tmplStr)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return err
	}

	// Try to format the code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// If formatting fails, use unformatted
		formatted = buf.Bytes()
	}

	return os.WriteFile(outputFile, formatted, 0644)
}