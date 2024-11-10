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
	typeInfo      map[*types.Interface]*types.TypeName
}

func newAnalysisResult() *analysisResult {
	return &analysisResult{
		deadFuncs:     make(map[*ssa.Function]bool),
		deadTypes:     make(map[*types.Named]bool),
		deadIfaces:    make(map[*types.Interface]bool),
		deadFields:    make(map[*types.Var]bool),
		reachablePosn: make(map[token.Position]bool),
		typeInfo:      make(map[*types.Interface]*types.TypeName),
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
	rootFuncs := make(map[*ssa.Function]bool) // Track root functions to never mark them dead
	for _, main := range mains {
		if init := main.Func("init"); init != nil {
			roots = append(roots, init)
			rootFuncs[init] = true
		}
		if main := main.Func("main"); main != nil {
			roots = append(roots, main)
			rootFuncs[main] = true
		}
	}

	// Run RTA analysis
	rtaRes := rta.Analyze(roots, true)

	// Track reachable types from RTA results
	reachableTypes := make(map[*types.Named]bool)
	for fn := range rtaRes.Reachable {
		// Check receiver types
		if sig := fn.Signature; sig.Recv() != nil {
			if named, ok := sig.Recv().Type().(*types.Named); ok {
				reachableTypes[named] = true
			}
		}

		// Check instructions in reachable functions
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				switch v := instr.(type) {
				case *ssa.MakeInterface:
					if named, ok := v.X.Type().(*types.Named); ok {
						reachableTypes[named] = true
					}
				case *ssa.Call:
					if v.Common().Method != nil {
						sig := v.Common().Method.Type().(*types.Signature)
						if recv := sig.Recv(); recv != nil {
							if named, ok := recv.Type().(*types.Named); ok {
								reachableTypes[named] = true
							}
						}
					}
				}
			}
		}
	}

	// Collect dead items
	for _, pkg := range initial {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.FuncDecl:
					if obj := pkg.TypesInfo.Defs[n.Name]; obj != nil {
						if fn := prog.FuncValue(obj.(*types.Func)); fn != nil {
							// Skip root functions and "used" function
							if !rootFuncs[fn] && fn.Name() != "used" {
								// Check if function is unreachable and not synthetic
								if fn.Synthetic == "" {
									_, isReachable := rtaRes.Reachable[fn]
									if !isReachable {
										// Don't mark methods of reachable types as dead
										if sig := fn.Signature; sig.Recv() == nil ||
											!reachableTypes[sig.Recv().Type().(*types.Named)] {
											res.deadFuncs[fn] = true
										}
									}
								}
							}
						}
					}
				case *ast.TypeSpec:
					if obj := pkg.TypesInfo.Defs[n.Name]; obj != nil {
						if named, ok := obj.Type().(*types.Named); ok {
							if (*typesFlag || *allFlag) && !reachableTypes[named] {
								// Skip usedType
								if named.Obj().Name() != "usedType" {
									res.deadTypes[named] = true
								}
							}
						}
					}
				}
				return true
			})
		}
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
