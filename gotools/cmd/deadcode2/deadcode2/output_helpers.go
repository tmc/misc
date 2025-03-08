// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/token"
	"os"
	"regexp"
)

// checkTestCase is a stub for the deadcode2 version
func checkTestCase() bool {
	return false
}

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
		}
	}
}

// printDeadVariables outputs information about dead variables
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
			fmt.Fprintf(os.Stderr, "  %s %s.%s = %s\n", pos, packageName(pkgPath), typeAlias.Name(), typeAlias.Type().String())
		}
	}
}

// printUnusedExportedSymbols outputs information about unused exported symbols
func printUnusedExportedSymbols(fset *token.FileSet, res *analysisResult, filter *regexp.Regexp) {
	fmt.Fprintf(os.Stderr, "Unused exported symbols:\n")
	for obj, kind := range res.deadExported {
		if pkg := obj.Pkg(); pkg != nil {
			pkgPath := pkg.Path()
			if !filter.MatchString(pkgPath) {
				continue
			}
			pos := fset.Position(obj.Pos())
			fmt.Fprintf(os.Stderr, "  %s %s.%s (%s)\n", pos, packageName(pkgPath), obj.Name(), kind)
		}
	}
}