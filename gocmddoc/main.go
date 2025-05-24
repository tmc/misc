package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var (
		output  = flag.String("o", "", "Output file path (default: stdout)")
		help    = flag.Bool("h", false, "Show usage information")
		allDecl = flag.Bool("all", false, "Include all declarations for main packages")
	)
	flag.StringVar(output, "output", "", "Output file path (default: stdout)")
	flag.BoolVar(help, "help", false, "Show usage information")
	flag.BoolVar(allDecl, "a", false, "Include all declarations for main packages")
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Get package path from args or default to current directory
	pkgPath := "."
	if flag.NArg() > 0 {
		pkgPath = flag.Arg(0)
	}

	md, err := getMarkdown(pkgPath, *allDecl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		err := os.WriteFile(*output, []byte(md), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", *output, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Documentation written to %s\n", *output)
	} else {
		fmt.Print(md)
	}
}

func getMarkdown(pkgPath string, showAll bool) (string, error) {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, pkgPath, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse package: %w", err)
	}

	if len(pkgs) == 0 {
		return "", fmt.Errorf("no Go packages found in %s", pkgPath)
	}

	var pkg *ast.Package
	for name, p := range pkgs {
		if !strings.HasSuffix(name, "_test") {
			pkg = p
			break
		}
	}

	if pkg == nil {
		return "", fmt.Errorf("no non-test packages found in %s", pkgPath)
	}

	docPkg := doc.New(pkg, pkgPath, doc.Mode(0))
	return formatAsMarkdown(docPkg, fset, pkgPath, showAll), nil
}

func formatAsMarkdown(pkg *doc.Package, fset *token.FileSet, pkgPath string, showAll bool) string {
	var md strings.Builder

	// Use command name for main packages, otherwise package name
	title := pkg.Name
	isMainPackage := pkg.Name == "main"
	if isMainPackage {
		// Use directory name as command name
		absPath, err := filepath.Abs(pkgPath)
		if err == nil {
			title = filepath.Base(absPath)
		}
	}
	md.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Package documentation
	if pkg.Doc != "" {
		md.WriteString(formatDocComment(pkg.Doc))
		md.WriteString("\n")
	}

	// For main packages, typically only show the package documentation
	// Skip other declarations unless showAll is set
	if isMainPackage && !showAll {
		return md.String()
	}

	// For library packages, show all exported declarations
	// Constants
	if len(pkg.Consts) > 0 {
		md.WriteString("## Constants\n\n")
		for _, c := range pkg.Consts {
			formatValue(&md, c, fset)
		}
	}

	// Variables
	if len(pkg.Vars) > 0 {
		md.WriteString("## Variables\n\n")
		for _, v := range pkg.Vars {
			formatValue(&md, v, fset)
		}
	}

	// Functions
	if len(pkg.Funcs) > 0 {
		md.WriteString("## Functions\n\n")
		for _, f := range pkg.Funcs {
			formatFunc(&md, f, fset)
		}
	}

	// Types
	if len(pkg.Types) > 0 {
		md.WriteString("## Types\n\n")
		for _, t := range pkg.Types {
			formatType(&md, t, fset)
		}
	}

	return md.String()
}

// formatDocComment converts a doc comment to proper markdown
func formatDocComment(doc string) string {
	var result strings.Builder
	parser := &docParser{
		input: doc,
		sections: map[string]sectionType{
			"USAGE":         sectionCode,
			"EXAMPLE":       sectionCode,
			"EXAMPLES":      sectionCode,
			"FLAGS":         sectionList,
			"REQUIREMENTS":  sectionList,
			"DEPENDENCIES":  sectionList,
			"PREREQUISITES": sectionList,
			"ARGUMENTS":     sectionList,
			"OPTIONS":       sectionList,
		},
	}
	
	result.WriteString(parser.parse())
	return result.String()
}

// sectionType defines how a section should be formatted
type sectionType int

const (
	sectionNormal sectionType = iota
	sectionCode
	sectionList
)

// docParser handles parsing and formatting of documentation comments
type docParser struct {
	input    string
	sections map[string]sectionType
}

func (p *docParser) parse() string {
	var result strings.Builder
	lines := strings.Split(p.input, "\n")
	
	var currentSection string
	var sectionType sectionType
	
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		
		// Check for headers
		if header := p.extractHeader(trimmed); header != "" {
			currentSection = strings.ToUpper(header)
			if st, ok := p.sections[currentSection]; ok {
				sectionType = st
			} else {
				sectionType = sectionNormal
			}
			result.WriteString("## " + header + "\n\n")
			continue
		}
		
		// Process based on section type
		switch sectionType {
		case sectionCode:
			if trimmed != "" || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
				result.WriteString("    " + strings.TrimPrefix(line, "\t") + "\n")
			} else {
				result.WriteString("\n")
			}
		case sectionList:
			if trimmed != "" && !strings.HasPrefix(trimmed, "-") {
				result.WriteString("- " + trimmed + "\n")
			} else if trimmed != "" {
				result.WriteString(trimmed + "\n")
			} else {
				result.WriteString("\n")
			}
		default:
			// Handle code blocks (indented lines)
			if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
				result.WriteString("    " + strings.TrimPrefix(line, "\t") + "\n")
			} else {
				result.WriteString(line + "\n")
			}
		}
	}
	
	return strings.TrimRight(result.String(), "\n")
}

func (p *docParser) extractHeader(line string) string {
	// Check for markdown-style headers
	if strings.HasPrefix(line, "# ") {
		return strings.TrimSpace(line[2:])
	}
	
	// Check for all-caps headers
	if isHeaderLine(line) {
		return line
	}
	
	return ""
}

func isHeaderLine(line string) bool {
	if len(line) < 2 || strings.Contains(line, "\t") {
		return false
	}
	
	// Must be mostly uppercase letters
	upperCount := 0
	letterCount := 0
	for _, r := range line {
		if r >= 'A' && r <= 'Z' {
			upperCount++
			letterCount++
		} else if r >= 'a' && r <= 'z' {
			letterCount++
		}
	}
	
	return letterCount > 0 && upperCount == letterCount
}

// formatValue formats a constant or variable declaration
func formatValue(md *strings.Builder, v *doc.Value, fset *token.FileSet) {
	md.WriteString("```go\n")
	// Format the declaration
	var buf strings.Builder
	format.Node(&buf, fset, v.Decl)
	md.WriteString(strings.TrimSpace(buf.String()))
	md.WriteString("\n```\n\n")
	
	// Add documentation
	if v.Doc != "" {
		md.WriteString(formatDocComment(v.Doc))
		md.WriteString("\n\n")
	}
}

// formatFunc formats a function declaration
func formatFunc(md *strings.Builder, f *doc.Func, fset *token.FileSet) {
	md.WriteString(fmt.Sprintf("### %s\n\n", f.Name))
	
	md.WriteString("```go\n")
	md.WriteString(synopsis(f.Decl, fset))
	md.WriteString("\n```\n\n")
	
	// Documentation
	if f.Doc != "" {
		md.WriteString(formatDocComment(f.Doc))
		md.WriteString("\n\n")
	}
}

// formatType formats a type declaration
func formatType(md *strings.Builder, t *doc.Type, fset *token.FileSet) {
	md.WriteString(fmt.Sprintf("### %s\n\n", t.Name))
	
	md.WriteString("```go\n")
	// Format the type declaration
	var buf strings.Builder
	format.Node(&buf, fset, t.Decl)
	md.WriteString(strings.TrimSpace(buf.String()))
	md.WriteString("\n```\n\n")
	
	// Documentation
	if t.Doc != "" {
		md.WriteString(formatDocComment(t.Doc))
		md.WriteString("\n\n")
	}
	
	// Methods
	if len(t.Methods) > 0 {
		md.WriteString("#### Methods\n\n")
		for _, m := range t.Methods {
			md.WriteString(fmt.Sprintf("##### %s\n\n", m.Name))
			
			md.WriteString("```go\n")
			md.WriteString(synopsis(m.Decl, fset))
			md.WriteString("\n```\n\n")
			
			if m.Doc != "" {
				md.WriteString(formatDocComment(m.Doc))
				md.WriteString("\n\n")
			}
		}
	}
}

// synopsis extracts a one-line summary of a function declaration
func synopsis(decl *ast.FuncDecl, fset *token.FileSet) string {
	var buf strings.Builder
	buf.WriteString("func ")
	
	// Receiver
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		buf.WriteString("(")
		if len(decl.Recv.List[0].Names) > 0 {
			buf.WriteString(decl.Recv.List[0].Names[0].Name)
			buf.WriteString(" ")
		}
		buf.WriteString(formatNode(fset, decl.Recv.List[0].Type))
		buf.WriteString(") ")
	}
	
	// Name
	buf.WriteString(decl.Name.Name)
	
	// Parameters
	buf.WriteString("(")
	for i, field := range decl.Type.Params.List {
		if i > 0 {
			buf.WriteString(", ")
		}
		if len(field.Names) > 0 {
			for j, name := range field.Names {
				if j > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(name.Name)
			}
			buf.WriteString(" ")
		}
		buf.WriteString(formatNode(fset, field.Type))
	}
	buf.WriteString(")")
	
	// Results
	if decl.Type.Results != nil && len(decl.Type.Results.List) > 0 {
		buf.WriteString(" ")
		if len(decl.Type.Results.List) > 1 || len(decl.Type.Results.List[0].Names) > 0 {
			buf.WriteString("(")
		}
		for i, field := range decl.Type.Results.List {
			if i > 0 {
				buf.WriteString(", ")
			}
			if len(field.Names) > 0 {
				for j, name := range field.Names {
					if j > 0 {
						buf.WriteString(", ")
					}
					buf.WriteString(name.Name)
					buf.WriteString(" ")
				}
			}
			buf.WriteString(formatNode(fset, field.Type))
		}
		if len(decl.Type.Results.List) > 1 || len(decl.Type.Results.List[0].Names) > 0 {
			buf.WriteString(")")
		}
	}
	
	return buf.String()
}

// formatNode formats an AST node as a string
func formatNode(fset *token.FileSet, node ast.Node) string {
	var buf strings.Builder
	format.Node(&buf, fset, node)
	return buf.String()
}
