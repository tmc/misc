package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var (
		output = flag.String("o", "", "Output file path (default: stdout)")
		help   = flag.Bool("h", false, "Show usage information")
	)
	flag.StringVar(output, "output", "", "Output file path (default: stdout)")
	flag.BoolVar(help, "help", false, "Show usage information")
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

	md, err := getMarkdown(pkgPath)
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

func getMarkdown(pkgPath string) (string, error) {
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

	docPkg := doc.New(pkg, pkgPath, doc.AllDecls)
	return formatAsMarkdown(docPkg, fset, pkgPath), nil
}

func formatAsMarkdown(pkg *doc.Package, fset *token.FileSet, pkgPath string) string {
	var md strings.Builder

	// Use command name for main packages, otherwise package name
	title := pkg.Name
	if pkg.Name == "main" {
		// Use directory name as command name
		absPath, err := filepath.Abs(pkgPath)
		if err == nil {
			title = filepath.Base(absPath)
		}
	}
	md.WriteString(fmt.Sprintf("# `%s`\n\n", title))

	if pkg.Doc != "" {
		md.Write(pkg.Markdown(pkg.Doc))
		md.WriteString("\n")
	}

	return md.String()
}
