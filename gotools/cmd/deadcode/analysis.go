// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// analysisResult tracks all forms of dead code
type analysisResult struct {
	deadFuncs     map[*ssa.Function]bool
	deadTypes     map[*types.Named]bool
	deadIfaces    map[*types.Interface]bool
	deadFields    map[*types.Var]bool
	reachablePosn map[token.Position]bool
}

func newAnalysisResult() *analysisResult {
	return &analysisResult{
		deadFuncs:     make(map[*ssa.Function]bool),
		deadTypes:     make(map[*types.Named]bool),
		deadIfaces:    make(map[*types.Interface]bool),
		deadFields:    make(map[*types.Var]bool),
		reachablePosn: make(map[token.Position]bool),
	}
}

// mainPackages returns the main packages to analyze.
// Each resulting package is named "main" and has a main function.
func mainPackages(pkgs []*ssa.Package) ([]*ssa.Package, error) {
	var mains []*ssa.Package
	for _, p := range pkgs {
		if p != nil && p.Pkg.Name() == "main" && p.Func("main") != nil {
			mains = append(mains, p)
		}
	}
	if len(mains) == 0 {
		return nil, fmt.Errorf("no main packages")
	}
	return mains, nil
}

// analyzeProgram performs comprehensive dead code analysis
func analyzeProgram(prog *ssa.Program, ssaPkgs []*ssa.Package, initial []*packages.Package, tests bool) (*analysisResult, error) {
	res := newAnalysisResult()

	// Find main packages
	mains, err := mainPackages(ssaPkgs)
	if err != nil {
		return nil, err
	}

	// Gather roots (main + init functions)
	var roots []*ssa.Function
	for _, main := range mains {
		roots = append(roots, main.Func("init"), main.Func("main"))
	}

	// Run RTA analysis
	rtaRes := rta.Analyze(roots, true)

	// First mark all functions as potentially dead
	for _, pkg := range ssaPkgs {
		for _, mem := range pkg.Members {
			if fn, ok := mem.(*ssa.Function); ok {
				res.deadFuncs[fn] = true
			}
		}
	}

	// Remove reachable functions from dead set
	for fn := range rtaRes.Reachable {
		delete(res.deadFuncs, fn)
		if fn.Pos().IsValid() || fn.Name() == "init" {
			res.reachablePosn[prog.Fset.Position(fn.Pos())] = true
		}
	}

	// Find all types and mark them as potentially dead
	for _, pkg := range initial {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.TypeSpec:
					if obj, ok := pkg.TypesInfo.Defs[n.Name].(*types.TypeName); ok {
						if named, ok := obj.Type().(*types.Named); ok {
							res.deadTypes[named] = true
						}
						if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
							res.deadIfaces[iface] = true
						}
					}
				case *ast.StructType:
					for _, field := range n.Fields.List {
						for _, name := range field.Names {
							if obj, ok := pkg.TypesInfo.Defs[name].(*types.Var); ok {
								res.deadFields[obj] = true
							}
						}
					}
				}
				return true
			})
		}
	}

	// Remove types/interfaces/fields that are used
	for fn := range rtaRes.Reachable {
		analyzeTypeUsage(fn, res)
	}

	return res, nil
}

// Add helper function to convert Position to jsonPosition
func toJSONPosition(pos token.Position) jsonPosition {
	return jsonPosition{
		File: pos.Filename,
		Line: pos.Line,
		Col:  pos.Column,
	}
}

func analyzeTypeUsage(fn *ssa.Function, res *analysisResult) {
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch v := instr.(type) {
			case *ssa.Alloc:
				if named, ok := v.Type().(*types.Named); ok {
					delete(res.deadTypes, named)
				}
			case *ssa.TypeAssert:
				if iface, ok := v.Type().(*types.Interface); ok {
					delete(res.deadIfaces, iface)
				}
			case *ssa.FieldAddr:
				if field := v.X.Type().(*types.Pointer).Elem().Underlying().(*types.Struct).Field(v.Field); field != nil {
					delete(res.deadFields, field)
				}
			}
		}
	}
}

func findDeadTypes(pkgs []*packages.Package, res *analysisResult) {
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.TypeSpec); ok {
					if obj, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName); ok {
						if named, ok := obj.Type().(*types.Named); ok {
							res.deadTypes[named] = true
						}
					}
				}
				return true
			})
		}
	}
}

func findDeadInterfaces(pkgs []*packages.Package, res *analysisResult) {
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.TypeSpec); ok {
					if obj, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName); ok {
						if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
							res.deadIfaces[iface] = true
						}
					}
				}
				return true
			})
		}
	}
}

func findDeadFields(pkgs []*packages.Package, res *analysisResult) {
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.TypeSpec); ok {
					if st, ok := spec.Type.(*ast.StructType); ok {
						for _, field := range st.Fields.List {
							if obj, ok := pkg.TypesInfo.Defs[field.Names[0]].(*types.Var); ok {
								res.deadFields[obj] = true
							}
						}
					}
				}
				return true
			})
		}
	}
}

func explainLiveness(prog *ssa.Program, res *analysisResult, target string) error {
	// Implementation of path finding to target function
	return fmt.Errorf("not implemented")
}
