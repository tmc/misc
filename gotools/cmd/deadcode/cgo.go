// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build cgo

package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

// setupCgoOptions configures the packages.Config to properly handle cgo.
// It returns a modified config with cgo-specific settings.
func setupCgoOptions(cfg *packages.Config) *packages.Config {
	// Enable cgo processing
	cfg.Env = append(cfg.Env, "CGO_ENABLED=1")
	
	// Add any additional cgo-specific settings here
	if *cgoFlag {
		fmt.Fprintf(os.Stderr, "Cgo support enabled\n")
		cfg.Mode |= packages.LoadSyntax | packages.NeedModule
	}
	
	return cfg
}

// analyzeCgoRefs looks for references to external C functions
// This helps track which C function bindings are actually used
func analyzeCgoRefs(res *analysisResult) {
	if !*cgoFlag {
		return
	}

	fmt.Fprintf(os.Stderr, "Analyzing cgo references...\n")
	
	// Count cgo functions by looking for "C." prefixes in function names
	cgoCount := 0
	for funcName := range res.liveFuncs {
		if strings.HasPrefix(funcName, "_cgo_") || strings.Contains(funcName, "C.") {
			cgoCount++
			if *debugFlag {
				fmt.Fprintf(os.Stderr, "Found cgo function: %s\n", funcName)
			}
		}
	}
	
	fmt.Fprintf(os.Stderr, "Found %d cgo functions\n", cgoCount)
}