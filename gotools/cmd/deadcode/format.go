// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/token"
	"go/types"
	"os"
)

func formatResults(fset *token.FileSet, res *analysisResult, format string, json bool) error {
	var packages []jsonPackage
	pkgMap := make(map[string]*jsonPackage)

	// Debug output
	fmt.Fprintf(os.Stderr, "\nFormatting results:\n")

	// Add dead functions
	for fn := range res.deadFuncs {
		fmt.Fprintf(os.Stderr, "Dead function: %s\n", fn.Name())
		pkg := addToPackage(pkgMap, fn.Pkg.Pkg.Path(), fn.Pkg.Pkg.Name())
		pkg.Funcs = append(pkg.Funcs, functionToJSON(fn, fset))
	}

	// Add dead types
	for typ := range res.deadTypes {
		fmt.Fprintf(os.Stderr, "Dead type: %s\n", typ.Obj().Name())
		pkg := addToPackage(pkgMap, typ.Obj().Pkg().Path(), typ.Obj().Pkg().Name())
		pkg.Types = append(pkg.Types, typeToJSON(typ, fset))
	}

	for iface := range res.deadIfaces {
		if typeName := res.typeInfo[iface]; typeName != nil {
			fmt.Fprintf(os.Stderr, "Dead interface: %s\n", typeName.Name())
			pkg := addToPackage(pkgMap, typeName.Pkg().Path(), typeName.Pkg().Name())
			pkg.Ifaces = append(pkg.Ifaces, jsonInterface{
				Name:     typeName.Name(),
				Position: toJSONPosition(fset.Position(typeName.Pos())),
			})
		}
	}

	// Add dead fields
	for field := range res.deadFields {
		fmt.Fprintf(os.Stderr, "Dead field: %s\n", field.Name())
		pkg := addToPackage(pkgMap, field.Pkg().Path(), field.Pkg().Name())
		pkg.Fields = append(pkg.Fields, fieldToJSON(field, fset))
	}

	// Convert map to slice
	for _, pkg := range pkgMap {
		packages = append(packages, *pkg)
	}

	// Output results
	if json {
		return outputJSON(packages)
	}
	return outputTemplate(packages, format)
}

// Add helper function to track type info
type pkgInfo struct {
	*jsonPackage
	TypesInfo *types.Info
}

func addToPackage(pkgMap map[string]*jsonPackage, path, name string) *jsonPackage {
	pkg, ok := pkgMap[path]
	if !ok {
		pkg = &jsonPackage{
			Name: name,
			Path: path,
		}
		pkgMap[path] = pkg
	}
	return pkg
}
