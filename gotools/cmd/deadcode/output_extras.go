// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/token"
	"go/types"
	"os"
	"regexp"
	"sort"
)

// printDeadConstants outputs information about dead constants
func printDeadConstants(fset *token.FileSet, res *analysisResult, filter *regexp.Regexp) {
	fmt.Fprintf(os.Stderr, "Dead constants:\n")
	for constant := range res.deadConstants {
		if pkg := constant.Pkg(); pkg != nil {
			pkgPath := pkg.Path()
			if !filter.MatchString(pkgPath) {
				continue
			}
			pos := fset.Position(constant.Pos())
			fmt.Fprintf(os.Stderr, "  %s %s.%s\n", pos, packageName(pkgPath), constant.Name())
			
			// Output for testing
			fmt.Printf("%s\n", constant.Name())
		}
	}
}

// printDeadVariables outputs information about dead package-level variables
func printDeadVariables(fset *token.FileSet, res *analysisResult, filter *regexp.Regexp) {
	fmt.Fprintf(os.Stderr, "Dead variables:\n")
	for variable := range res.deadVariables {
		if pkg := variable.Pkg(); pkg != nil {
			pkgPath := pkg.Path()
			if !filter.MatchString(pkgPath) {
				continue
			}
			pos := fset.Position(variable.Pos())
			fmt.Fprintf(os.Stderr, "  %s %s.%s\n", pos, packageName(pkgPath), variable.Name())
			
			// Output for testing
			fmt.Printf("%s\n", variable.Name())
		}
	}
}

// printDeadTypeAliases outputs information about dead type aliases
func printDeadTypeAliases(fset *token.FileSet, res *analysisResult, filter *regexp.Regexp) {
	fmt.Fprintf(os.Stderr, "Dead type aliases:\n")
	for typeAlias := range res.deadTypeAliases {
		if pkg := typeAlias.Pkg(); pkg != nil {
			pkgPath := pkg.Path()
			if !filter.MatchString(pkgPath) {
				continue
			}
			pos := fset.Position(typeAlias.Pos())
			originalType := types.TypeString(typeAlias.Type(), types.RelativeTo(pkg))
			fmt.Fprintf(os.Stderr, "  %s %s.%s = %s\n", pos, packageName(pkgPath), typeAlias.Name(), originalType)
			
			// Output for testing
			fmt.Printf("%s\n", typeAlias.Name())
		}
	}
}

// printUnusedExportedSymbols outputs information about exported symbols not used outside their package
func printUnusedExportedSymbols(fset *token.FileSet, res *analysisResult, filter *regexp.Regexp) {
	fmt.Fprintf(os.Stderr, "Unused exported symbols:\n")
	
	// Sort the exported symbols by kind, then by name for consistent output
	type exportedEntry struct {
		obj  types.Object
		kind string
	}
	var entries []exportedEntry
	for obj, kind := range res.deadExported {
		entries = append(entries, exportedEntry{obj, kind})
	}
	
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].kind != entries[j].kind {
			return entries[i].kind < entries[j].kind
		}
		return entries[i].obj.Name() < entries[j].obj.Name()
	})
	
	for _, entry := range entries {
		obj := entry.obj
		kind := entry.kind
		
		if pkg := obj.Pkg(); pkg != nil {
			pkgPath := pkg.Path()
			if !filter.MatchString(pkgPath) {
				continue
			}
			pos := fset.Position(obj.Pos())
			fmt.Fprintf(os.Stderr, "  %s %s %s.%s\n", pos, kind, packageName(pkgPath), obj.Name())
			
			// Output for testing
			fmt.Printf("%s %s\n", kind, obj.Name())
		}
	}
}