// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/token"
	"go/types"
)

func formatResults(fset *token.FileSet, res *analysisResult, format string, json bool) error {
	var packages []jsonPackage
	pkgMap := make(map[string]*jsonPackage)
	seen := make(map[string]bool)

	// Format functions
	for fn := range res.deadFuncs {
		name := fn.Name()
		if recv := fn.Signature.Recv(); recv != nil {
			// This is a method
			recvType := recv.Type().String()
			name = fmt.Sprintf("%s() method", name)
		}
		if !seen[name] {
			seen[name] = true
			fmt.Println(name)
		}
	}

	// Format types
	for typ := range res.deadTypes {
		name := typ.Obj().Name()
		if !seen[name] {
			seen[name] = true
			fmt.Println(name)
		}
	}

	// Format interfaces
	for iface := range res.deadIfaces {
		if typeName := res.typeInfo[iface]; typeName != nil {
			name := typeName.Name()
			if !seen[name] {
				seen[name] = true
				fmt.Println(name)
			}
		}
	}

	// Format fields
	for field := range res.deadFields {
		name := field.Name()
		if !seen[name] {
			seen[name] = true
			fmt.Println(name)
		}
	}

	return nil
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
