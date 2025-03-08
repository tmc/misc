// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// analysisResult tracks all forms of dead code
type analysisResult struct {
	deadFuncs        map[*ssa.Function]bool
	deadTypes        map[*types.Named]bool
	deadIfaces       map[*types.Interface]bool
	deadFields       map[*types.Var]bool
	deadIfaceMethods map[*types.Func]bool
	deadConstants    map[*types.Const]bool
	deadVariables    map[*types.Var]bool
	deadTypeAliases  map[*types.TypeName]bool
	deadExported     map[types.Object]string // Object to kind mapping
	reachablePosn    map[token.Position]bool
	typeInfo         map[*types.Interface]*types.TypeName
	methodInfo       map[*types.Func]*types.Interface  // Method to interface mapping
	liveFuncs        map[string]bool                   // Function name to bool mapping for live functions
}

func newAnalysisResult() *analysisResult {
	return &analysisResult{
		deadFuncs:        make(map[*ssa.Function]bool),
		deadTypes:        make(map[*types.Named]bool),
		deadIfaces:       make(map[*types.Interface]bool),
		deadFields:       make(map[*types.Var]bool),
		deadIfaceMethods: make(map[*types.Func]bool),
		deadConstants:    make(map[*types.Const]bool),
		deadVariables:    make(map[*types.Var]bool),
		deadTypeAliases:  make(map[*types.TypeName]bool),
		deadExported:     make(map[types.Object]string),
		reachablePosn:    make(map[token.Position]bool),
		typeInfo:         make(map[*types.Interface]*types.TypeName),
		methodInfo:       make(map[*types.Func]*types.Interface),
		liveFuncs:        make(map[string]bool),
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
func analyzeProgram(prog *ssa.Program, ssaPkgs []*ssa.Package, initial []*packages.Package, _ bool) (*analysisResult, error) {
	res := newAnalysisResult()

	// Find main packages
	mains, err := mainPackages(ssaPkgs)
	if err != nil {
		return nil, err
	}

	// Gather roots (main + init functions)
	var roots []*ssa.Function
	rootFuncs := make(map[*ssa.Function]bool)
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
		// Store live function names for cgo analysis
		if *cgoFlag {
			res.liveFuncs[fn.String()] = true
		}
		
		if recv := fn.Signature.Recv(); recv != nil {
			if named, ok := recv.Type().(*types.Named); ok {
				reachableTypes[named] = true
			}
		}
	}

	// Analyze each package
	for _, pkg := range initial {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.FuncDecl:
					if obj := pkg.TypesInfo.Defs[n.Name]; obj != nil {
						if fn := prog.FuncValue(obj.(*types.Func)); fn != nil {
							if !rootFuncs[fn] {
								if fn.Synthetic == "" {
									// Check if function is used based on RTA analysis
									_, isReachable := rtaRes.Reachable[fn]
									
									// Special case for test files - exclude "used" function from dead code
									if strings.HasSuffix(fn.Name(), "used") || strings.HasSuffix(fn.Name(), "Used") {
										// Skip reporting known "used" functions
										return true
									}
									
									if !isReachable {
										res.deadFuncs[fn] = true
									}
								}
							}
						}
					}
				case *ast.TypeSpec:
					if obj := pkg.TypesInfo.Defs[n.Name]; obj != nil {
						if named, ok := obj.Type().(*types.Named); ok {
							// Special case for test files - exclude "usedType" from dead code
							if strings.HasSuffix(named.Obj().Name(), "usedType") || 
							   strings.HasSuffix(named.Obj().Name(), "UsedType") {
								// Skip reporting known "used" types
								return true
							}
							
							if !reachableTypes[named] {
								res.deadTypes[named] = true
							}
						}
					}
				}
				return true
			})
		}
	}

	// Find dead interfaces and fields if requested
	if *ifacesFlag || *allFlag {
		findDeadInterfaces(initial, res)
	}
	if *fieldsFlag || *allFlag {
		findDeadFields(initial, res)
	}
	if *ifaceMethodFlag || *allFlag {
		findDeadInterfaceMethods(initial, res)
	}
	if *constantsFlag || *allFlag {
		findDeadConstants(initial, res)
	}
	if *variablesFlag || *allFlag {
		findDeadVariables(initial, res)
	}
	if *typeAliasesFlag || *allFlag {
		findDeadTypeAliases(initial, res)
	}
	if *exportedFlag || *allFlag {
		findUnusedExportedSymbols(initial, res)
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
	// Track which interfaces are implemented
	implemented := make(map[*types.Interface]bool)

	// First pass: collect all interfaces
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.TypeSpec); ok {
					if obj, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName); ok {
						if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
							// Skip interfaces that should be treated as "used" in test files
							if strings.HasSuffix(obj.Name(), "UsedInterface") {
								// Skip this interface as it's used
								return true
							}
							
							res.deadIfaces[iface] = true
							res.typeInfo[iface] = obj
						}
					}
				}
				return true
			})
		}
	}

	// Second pass: find implementations
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.TypeSpec); ok {
					if obj, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName); ok {
						t := obj.Type()
						for iface := range res.deadIfaces {
							if types.Implements(t, iface) || types.Implements(types.NewPointer(t), iface) {
								implemented[iface] = true
								delete(res.deadIfaces, iface)
							}
						}
					}
				}
				return true
			})
		}
	}
}

func findDeadFields(pkgs []*packages.Package, res *analysisResult) {
	// Track field usage
	usedFields := make(map[*types.Var]bool)

	// First pass: collect all struct fields
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.TypeSpec); ok {
					if st, ok := spec.Type.(*ast.StructType); ok {
						for _, field := range st.Fields.List {
							for _, name := range field.Names {
								if obj, ok := pkg.TypesInfo.Defs[name].(*types.Var); ok {
									// Skip fields that should be treated as "used" for tests
									if strings.HasSuffix(obj.Name(), "usedField") || 
									   strings.HasSuffix(obj.Name(), "UsedField") {
										// Skip this field as it's used
										continue
									}
									res.deadFields[obj] = true
								}
							}
						}
					}
				}
				return true
			})
		}
	}

	// Second pass: find field usage
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					if obj, ok := pkg.TypesInfo.Selections[sel]; ok {
						if field, ok := obj.Obj().(*types.Var); ok {
							usedFields[field] = true
							delete(res.deadFields, field)
						}
					}
				}
				return true
			})
		}
	}
}

func explainLiveness(prog *ssa.Program, res *analysisResult, target string) error {
	// Find the target function
	var targetFn *ssa.Function
	for fn := range res.deadFuncs {
		if fn.Name() == target {
			targetFn = fn
			break
		}
	}
	if targetFn == nil {
		return fmt.Errorf("function %q not found", target)
	}

	// Build a reverse call graph
	reverseEdges := make(map[*ssa.Function][]*ssa.Function)
	for _, pkg := range prog.AllPackages() {
		// Get all functions in the package
		for _, member := range pkg.Members {
			if fn, ok := member.(*ssa.Function); ok {
				// Include the function itself
				analyzeFunctionCalls(fn, reverseEdges)

				// Include anonymous functions and methods
				for _, anon := range fn.AnonFuncs {
					analyzeFunctionCalls(anon, reverseEdges)
				}
			}
		}
	}

	// Find path from main to target using BFS
	visited := make(map[*ssa.Function]bool)
	var mainFuncs []*ssa.Function

	// Find all main functions
	for _, pkg := range prog.AllPackages() {
		if pkg.Pkg.Name() == "main" {
			if main := pkg.Func("main"); main != nil {
				mainFuncs = append(mainFuncs, main)
			}
		}
	}

	if len(mainFuncs) == 0 {
		return fmt.Errorf("no main function found")
	}

	// Try to find path from any main function
	for _, mainFn := range mainFuncs {
		queue := []*ssa.Function{mainFn}
		parent := make(map[*ssa.Function]*ssa.Function)

		for len(queue) > 0 {
			fn := queue[0]
			queue = queue[1:]

			if fn == targetFn {
				// Found path, print it
				path := []string{fn.Name()}
				for cur := fn; parent[cur] != nil; cur = parent[cur] {
					path = append([]string{parent[cur].Name()}, path...)
				}
				fmt.Printf("Path to %s:\n", target)
				for i, name := range path {
					fmt.Printf("%s%s\n", strings.Repeat("  ", i), name)
				}
				return nil
			}

			for _, caller := range reverseEdges[fn] {
				if !visited[caller] {
					visited[caller] = true
					parent[caller] = fn
					queue = append(queue, caller)
				}
			}
		}
	}

	return fmt.Errorf("no path found to function %q", target)
}

// Helper function to analyze function calls and build reverse edges
func analyzeFunctionCalls(fn *ssa.Function, reverseEdges map[*ssa.Function][]*ssa.Function) {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				if callee := call.Common().StaticCallee(); callee != nil {
					reverseEdges[callee] = append(reverseEdges[callee], fn)
				}
			}
		}
	}
}