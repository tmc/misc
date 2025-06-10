package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var pipeFlag = flag.Bool("p", false, "output as pipe-separated list")

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	var tests []string
	for _, arg := range args {
		if strings.HasSuffix(arg, "/...") {
			if err := findPackages(arg, &tests); err != nil {
				log.Fatal(err)
			}
			continue
		}
		if err := findLongTests(arg, &tests); err != nil {
			log.Fatal(err)
		}
	}

	if *pipeFlag {
		fmt.Println(strings.Join(tests, "|"))
	} else {
		for _, test := range tests {
			fmt.Println(test)
		}
	}
}

func findPackages(pattern string, tests *[]string) error {
	base := strings.TrimSuffix(pattern, "/...")
	if base == "" {
		base = "."
	}
	pkg, err := build.Import(base, ".", build.FindOnly)
	if err != nil {
		return err
	}

	return filepath.Walk(pkg.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if info.Name() == "testdata" || strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}
		if _, err := build.ImportDir(path, 0); err != nil {
			if _, ok := err.(*build.NoGoError); ok {
				return nil
			}
			return nil
		}
		return findLongTests(path, tests)
	})
}

func findLongTests(dir string, tests *[]string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && isGoTestFile(path) {
			if err := inspectFile(path, tests); err != nil {
				return fmt.Errorf("inspecting %s: %v", path, err)
			}
		}
		return nil
	})
}

func isGoTestFile(path string) bool {
	return filepath.Ext(path) == ".go" && len(path) > 8 && path[len(path)-8:] == "_test.go"
}

func inspectFile(path string, tests *[]string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}

	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !isTest(fd) {
			continue
		}
		if hasShortSkip(fd) {
			*tests = append(*tests, fd.Name.Name)
		}
	}
	return nil
}

func isTest(fd *ast.FuncDecl) bool {
	if !fd.Name.IsExported() || len(fd.Type.Params.List) != 1 {
		return false
	}
	param := fd.Type.Params.List[0]
	star, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "T" && fd.Name.Name[:4] == "Test"
}

func hasShortSkip(fd *ast.FuncDecl) bool {
	var found bool
	ast.Inspect(fd, func(n ast.Node) bool {
		if found {
			return false
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		call, ok := ifStmt.Cond.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "Short" {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		if !ok || x.Name != "testing" {
			return true
		}
		found = true
		return false
	})
	return found
}
