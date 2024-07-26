package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/tools/cover"
)

type FunctionCoverage struct {
	Name     string
	Coverage float64
	Length   int
	Impact   float64
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	coverProfile := flag.String("cover", "coverage.out", "Path to the coverage profile")
	showCount := flag.Int("n", 10, "Number of functions to show")
	showImpact := flag.Bool("impact", false, "Sort by impact instead of coverage percentage")
	flag.Parse()

	profiles, err := cover.ParseProfiles(*coverProfile)
	if err != nil {
		return fmt.Errorf("error parsing coverage profile: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current working directory: %w", err)
	}

	functionCoverages, err := analyzeCoverage(cwd, profiles)
	if err != nil {
		return fmt.Errorf("error analyzing coverage: %w", err)
	}

	if len(functionCoverages) == 0 {
		fmt.Println("No functions found or no coverage data available.")
		return nil
	}

	n := *showCount
	if n > len(functionCoverages) {
		n = len(functionCoverages)
	}

	if *showImpact {
		sort.Slice(functionCoverages, func(i, j int) bool {
			return functionCoverages[i].Impact > functionCoverages[j].Impact
		})
		fmt.Printf("Top %d functions by impact (higher impact = larger uncovered area):\n", n)
		for i := 0; i < n; i++ {
			fc := functionCoverages[i]
			fmt.Printf("%d. %s (%.2f%% coverage, %d lines, impact score: %.2f)\n",
				i+1, fc.Name, fc.Coverage*100, fc.Length, fc.Impact)
		}
	} else {
		sort.Slice(functionCoverages, func(i, j int) bool {
			return functionCoverages[i].Coverage < functionCoverages[j].Coverage
		})
		fmt.Printf("Top %d functions with least coverage:\n", n)
		for i := 0; i < n; i++ {
			fc := functionCoverages[i]
			fmt.Printf("%d. %s (%.2f%% coverage, %d lines)\n",
				i+1, fc.Name, fc.Coverage*100, fc.Length)
		}
	}

	return nil
}

func analyzeCoverage(dir string, profiles []*cover.Profile) ([]FunctionCoverage, error) {
	var functionCoverages []FunctionCoverage

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("error parsing directory: %w", err)
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			fileName := fset.File(file.Pos()).Name()
			profile := findProfile(profiles, fileName)
			if profile == nil {
				continue
			}

			ast.Inspect(file, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncDecl:
					name := x.Name.Name
					if x.Recv != nil {
						if t, ok := x.Recv.List[0].Type.(*ast.StarExpr); ok {
							name = t.X.(*ast.Ident).Name + "." + name
						} else {
							name = x.Recv.List[0].Type.(*ast.Ident).Name + "." + name
						}
					}
					coverage := calculateFunctionCoverage(profile, fset.Position(x.Pos()).Line, fset.Position(x.End()).Line)
					length := fset.Position(x.End()).Line - fset.Position(x.Pos()).Line + 1
					impact := calculateImpact(coverage, length)
					functionCoverages = append(functionCoverages, FunctionCoverage{
						Name:     name,
						Coverage: coverage,
						Length:   length,
						Impact:   impact,
					})
				}
				return true
			})
		}
	}

	return functionCoverages, nil
}

func findProfile(profiles []*cover.Profile, fileName string) *cover.Profile {
	for _, profile := range profiles {
		if filepath.Base(profile.FileName) == filepath.Base(fileName) {
			return profile
		}
	}
	return nil
}

func calculateFunctionCoverage(profile *cover.Profile, startLine, endLine int) float64 {
	var covered, total int64
	for _, block := range profile.Blocks {
		if block.StartLine >= startLine && block.EndLine <= endLine {
			total += int64(block.NumStmt)
			if block.Count > 0 {
				covered += int64(block.NumStmt)
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total)
}

func calculateImpact(coverage float64, length int) float64 {
	return (1 - coverage) * float64(length)
}