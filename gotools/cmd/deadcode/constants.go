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

// findDeadConstants identifies constants that are declared but never used
func findDeadConstants(pkgs []*packages.Package, res *analysisResult) {
	// Track used constants
	usedConstants := make(map[*types.Const]bool)

	// First pass: collect all constants
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.CONST {
					for _, spec := range decl.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range vs.Names {
								if obj, ok := pkg.TypesInfo.Defs[name].(*types.Const); ok {
									res.deadConstants[obj] = true
								}
							}
						}
					}
				}
				return true
			})
		}
	}

	// Second pass: find constant usage
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					if obj, ok := pkg.TypesInfo.Uses[id]; ok {
						if constant, ok := obj.(*types.Const); ok {
							usedConstants[constant] = true
							delete(res.deadConstants, constant)
						}
					}
				}
				return true
			})
		}
	}
}