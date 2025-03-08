// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// findDeadInterfaceMethods identifies interface methods that are declared but never called
func findDeadInterfaceMethods(pkgs []*packages.Package, res *analysisResult) {
	// First pass: collect all interfaces and their methods
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.TypeSpec); ok {
					if obj, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName); ok {
						if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
							// Store interface in typeInfo map
							res.typeInfo[iface] = obj
							
							// Add all methods of this interface to the deadIfaceMethods map
							for i := 0; i < iface.NumMethods(); i++ {
								method := iface.Method(i)
								res.deadIfaceMethods[method] = true
								res.methodInfo[method] = iface
							}
						}
					}
				}
				return true
			})
		}
	}

	// Second pass: find method usage in call expressions
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						// Check if this is a method call
						if selection, ok := pkg.TypesInfo.Selections[sel]; ok {
							if selection.Kind() == types.MethodVal {
								// This is a method call, mark the method as used
								methodObj := selection.Obj()
								if method, ok := methodObj.(*types.Func); ok {
									delete(res.deadIfaceMethods, method)
								}
							}
						}
					}
				}
				return true
			})
		}
	}
}