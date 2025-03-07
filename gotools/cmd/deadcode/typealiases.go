// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// findDeadTypeAliases identifies type aliases that are declared but never used
func findDeadTypeAliases(pkgs []*packages.Package, res *analysisResult) {
	// Track used type aliases
	usedAliases := make(map[*types.TypeName]bool)

	// First pass: collect all type aliases
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.TypeSpec); ok && spec.Assign.IsValid() { // Assign token position is valid for type aliases
					if obj, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName); ok {
						if obj.IsAlias() {
							res.deadTypeAliases[obj] = true
						}
					}
				}
				return true
			})
		}
	}

	// Second pass: find type alias usage
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.Ident:
					if obj, ok := pkg.TypesInfo.Uses[n]; ok {
						if typeName, ok := obj.(*types.TypeName); ok {
							if typeName.IsAlias() {
								usedAliases[typeName] = true
								delete(res.deadTypeAliases, typeName)
							}
						}
					}
				}
				return true
			})
		}
	}
}