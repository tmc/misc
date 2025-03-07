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

// setupCgoOptions is a stub when cgo is not enabled.
// It returns the original config unchanged.
func setupCgoOptions(cfg *packages.Config) *packages.Config {
	if *cgoFlag {
		fmt.Fprintf(os.Stderr, "Warning: cgo flag specified but cgo support not built in\n")
		fmt.Fprintf(os.Stderr, "Rebuild with CGO_ENABLED=1 go build\n")
	}
	return cfg
}

// analyzeCgoRefs is a no-op in the non-cgo build
func analyzeCgoRefs(res *analysisResult) {
	// Do nothing in non-cgo builds
}