// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !cgo

package main

import (
	"fmt"
	"os"

	"golang.org/x/tools/go/packages"
)

// setupCgoOptions is a stub for the non-cgo build
func setupCgoOptions(cfg *packages.Config) *packages.Config {
	if *cgoFlag {
		fmt.Fprintf(os.Stderr, "Warning: cgo support not available in this build\n")
	}
	return cfg
}

// analyzeCgoRefs is a stub for the non-cgo build
func analyzeCgoRefs(res *analysisResult) {
	// Do nothing in non-cgo builds
}