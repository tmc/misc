// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// findUnusedExportedSymbols identifies exported symbols not used outside of their package
func findUnusedExportedSymbols(pkgs []*packages.Package, res *analysisResult) {
	// Track exports by package
	type exportInfo struct {
		obj  types.Object
		kind string // "function", "type", "const", "var"
	}
	
	exportsByPkg := make(map[*types.Package]map[string]exportInfo)
	usedExports := make(map[types.Object]bool)

	// First pass: collect all exported symbols
	for _, pkg := range pkgs {
		pkgExports := make(map[string]exportInfo)
		exportsByPkg[pkg.Types] = pkgExports
		
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj.Exported() {
				var kind string
				switch obj := obj.(type) {
				case *types.Func:
					// Skip methods - they're covered by deadFuncs
					if obj.Type().(*types.Signature).Recv() != nil {
						continue
					}
					kind = "function"
				case *types.TypeName:
					kind = "type"
				case *types.Const:
					kind = "const"
				case *types.Var:
					// Only package level vars
					if obj.Parent() != nil {
						kind = "var"
					} else {
						continue
					}
				default:
					continue // Skip if not one of these types
				}
				pkgExports[name] = exportInfo{obj, kind}
				res.deadExported[obj] = kind
			}
		}
	}

	// Second pass: find usage of exported symbols across packages
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					if obj, ok := pkg.TypesInfo.Uses[id]; ok {
						objPkg := obj.Pkg()
						if objPkg != nil && objPkg != pkg.Types && obj.Exported() {
							// This is a cross-package reference to an exported symbol
							usedExports[obj] = true
							delete(res.deadExported, obj)
						}
					}
				}
				return true
			})
		}
	}
}