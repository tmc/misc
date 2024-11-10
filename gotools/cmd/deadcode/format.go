// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
    "go/token"
)

func formatResults(fset *token.FileSet, res *analysisResult, format string, json bool) error {
    // Convert analysis results to output format
    var packages []jsonPackage

    // Group by package
    pkgMap := make(map[string]*jsonPackage)

    // Add dead functions
    for fn := range res.deadFuncs {
        pkg := addToPackage(pkgMap, fn.Pkg.Pkg.Path(), fn.Pkg.Pkg.Name())
        pkg.Funcs = append(pkg.Funcs, functionToJSON(fn, fset))
    }

    // Add dead types
    for typ := range res.deadTypes {
        pkg := addToPackage(pkgMap, typ.Obj().Pkg().Path(), typ.Obj().Pkg().Name())
        pkg.Types = append(pkg.Types, typeToJSON(typ, fset))
    }

    // Add dead interfaces
    for iface := range res.deadIfaces {
        // For interfaces, we need to handle them differently since they don't have Obj()
        // This is a simplified version - you might want to enhance this
        pkg := addToPackage(pkgMap, "<unknown>", "<unknown>")
        pkg.Ifaces = append(pkg.Ifaces, interfaceToJSON(iface, fset))
    }

    // Add dead fields
    for field := range res.deadFields {
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
