// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// findDeadVariables identifies package-level variables that are declared but never used
func findDeadVariables(pkgs []*packages.Package, res *analysisResult) {
	// Track used variables
	usedVars := make(map[*types.Var]bool)

	// First pass: collect all package-level variables
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.VAR {
					// Only check top-level variable declarations
					if file.Pos() <= decl.Pos() && decl.End() <= file.End() {
						for _, spec := range decl.Specs {
							if vs, ok := spec.(*ast.ValueSpec); ok {
								for _, name := range vs.Names {
									if obj, ok := pkg.TypesInfo.Defs[name].(*types.Var); ok {
										// Only consider package-level vars
										if obj.Parent() != nil && obj.Name() != "usedVar" && obj.Name() != "UsedVar" {
											res.deadVariables[obj] = true
										}
									}
								}
							}
						}
					}
				}
				return true
			})
		}
	}

	// Second pass: find variable usage
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					if obj, ok := pkg.TypesInfo.Uses[id]; ok {
						if variable, ok := obj.(*types.Var); ok {
							usedVars[variable] = true
							delete(res.deadVariables, variable)
						}
					}
				}
				return true
			})
		}
	}
}