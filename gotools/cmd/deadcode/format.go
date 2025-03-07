// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/types"
)

// Helper function for package tracking
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