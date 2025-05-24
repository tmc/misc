package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	output := flag.String("o", "", "Output file path (default: stdout)")
	allDecl := flag.Bool("all", false, "Include all declarations for main packages")
	flag.StringVar(output, "output", "", "Output file path (default: stdout)")
	flag.BoolVar(allDecl, "a", false, "Include all declarations for main packages")
	flag.Parse()

	pkgPath := "."
	if flag.NArg() > 0 {
		pkgPath = flag.Arg(0)
	}

	md, err := generateMarkdown(pkgPath, *allDecl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		if err := os.WriteFile(*output, []byte(md), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to %s: %v\n", *output, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Documentation written to %s\n", *output)
	} else {
		fmt.Print(md)
	}
}

func generateMarkdown(pkgPath string, showAll bool) (string, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgPath, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse package: %w", err)
	}

	// Find first non-test package
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
	
	var out strings.Builder
	isMain := docPkg.Name == "main"
	
	// Title
	title := docPkg.Name
	if isMain {
		if abs, err := filepath.Abs(pkgPath); err == nil {
			title = filepath.Base(abs)
		}
	}
	fmt.Fprintf(&out, "# %s\n\n", title)

	// Package doc
	if docPkg.Doc != "" {
		out.WriteString(formatDoc(docPkg.Doc))
		out.WriteString("\n")
	}

	// Skip declarations for main packages unless -all
	if isMain && !showAll {
		return out.String(), nil
	}

	// Declarations
	writeSection(&out, "Constants", docPkg.Consts, fset)
	writeSection(&out, "Variables", docPkg.Vars, fset)
	writeFuncs(&out, "Functions", docPkg.Funcs, fset)
	writeTypes(&out, "Types", docPkg.Types, fset)

	return out.String(), nil
}

// formatDoc converts doc comments to markdown
func formatDoc(doc string) string {
	var out strings.Builder
	lines := strings.Split(doc, "\n")
	
	// Section types for special formatting
	listSections := regexp.MustCompile(`^(FLAGS?|ARGUMENTS?|OPTIONS?|REQUIREMENTS?|DEPENDENCIES?|PREREQUISITES?)$`)
	codeSections := regexp.MustCompile(`^(USAGE|EXAMPLES?)$`)
	
	inSection := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check for headers (# Header or ALL-CAPS)
		if strings.HasPrefix(trimmed, "# ") {
			inSection = strings.ToUpper(trimmed[2:])
			fmt.Fprintf(&out, "## %s\n\n", trimmed[2:])
			continue
		}
		if isAllCaps(trimmed) && len(trimmed) > 1 {
			inSection = trimmed
			fmt.Fprintf(&out, "## %s\n\n", trimmed)
			continue
		}
		
		// Format based on section type
		switch {
		case codeSections.MatchString(inSection):
			// Code sections - preserve indentation
			if strings.HasPrefix(line, "\t") {
				out.WriteString("    " + line[1:] + "\n")
			} else {
				out.WriteString(line + "\n")
			}
		case listSections.MatchString(inSection) && trimmed != "" && !strings.HasPrefix(trimmed, "-"):
			// List sections - add bullets
			out.WriteString("- " + trimmed + "\n")
		default:
			// Normal text or already formatted
			if strings.HasPrefix(line, "\t") {
				out.WriteString("    " + line[1:] + "\n")
			} else {
				out.WriteString(line + "\n")
			}
		}
	}
	
	return strings.TrimRight(out.String(), "\n")
}

// isAllCaps checks if a string is all uppercase letters
func isAllCaps(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'A' || r > 'Z') && r != ' ' && r != '-' && r != '_' {
			return false
		}
	}
	return strings.ContainsAny(s, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
}

// writeSection writes a section for constants/variables
func writeSection(out *strings.Builder, title string, values []*doc.Value, fset *token.FileSet) {
	if len(values) == 0 {
		return
	}
	
	fmt.Fprintf(out, "## %s\n\n", title)
	for _, v := range values {
		out.WriteString("```go\n")
		out.WriteString(nodeString(fset, v.Decl))
		out.WriteString("\n```\n\n")
		
		if v.Doc != "" {
			out.WriteString(formatDoc(v.Doc))
			out.WriteString("\n\n")
		}
	}
}

// writeFuncs writes function documentation
func writeFuncs(out *strings.Builder, title string, funcs []*doc.Func, fset *token.FileSet) {
	if len(funcs) == 0 {
		return
	}
	
	fmt.Fprintf(out, "## %s\n\n", title)
	for _, f := range funcs {
		fmt.Fprintf(out, "### %s\n\n```go\n%s\n```\n\n", f.Name, funcSig(f.Decl, fset))
		if f.Doc != "" {
			out.WriteString(formatDoc(f.Doc))
			out.WriteString("\n\n")
		}
	}
}

// writeTypes writes type documentation
func writeTypes(out *strings.Builder, title string, types []*doc.Type, fset *token.FileSet) {
	if len(types) == 0 {
		return
	}
	
	fmt.Fprintf(out, "## %s\n\n", title)
	for _, t := range types {
		fmt.Fprintf(out, "### %s\n\n```go\n%s\n```\n\n", t.Name, nodeString(fset, t.Decl))
		if t.Doc != "" {
			out.WriteString(formatDoc(t.Doc))
			out.WriteString("\n\n")
		}
		
		// Methods
		if len(t.Methods) > 0 {
			out.WriteString("#### Methods\n\n")
			for _, m := range t.Methods {
				fmt.Fprintf(out, "##### %s\n\n```go\n%s\n```\n\n", m.Name, funcSig(m.Decl, fset))
				if m.Doc != "" {
					out.WriteString(formatDoc(m.Doc))
					out.WriteString("\n\n")
				}
			}
		}
	}
}

// funcSig returns a formatted function signature
func funcSig(fn *ast.FuncDecl, fset *token.FileSet) string {
	var buf bytes.Buffer
	buf.WriteString("func ")
	
	// Receiver
	if fn.Recv != nil {
		buf.WriteString("(")
		if r := fn.Recv.List[0]; len(r.Names) > 0 {
			buf.WriteString(r.Names[0].Name + " ")
		}
		buf.WriteString(nodeString(fset, fn.Recv.List[0].Type))
		buf.WriteString(") ")
	}
	
	// Name and params
	buf.WriteString(fn.Name.Name)
	buf.WriteString(nodeString(fset, fn.Type.Params))
	
	// Results
	if fn.Type.Results != nil {
		buf.WriteString(" ")
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			// Single unnamed result
			buf.WriteString(nodeString(fset, fn.Type.Results.List[0].Type))
		} else {
			// Multiple or named results
			buf.WriteString(nodeString(fset, fn.Type.Results))
		}
	}
	
	return buf.String()
}

// nodeString formats an AST node as a string
func nodeString(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	format.Node(&buf, fset, node)
	return strings.TrimSpace(buf.String())
}