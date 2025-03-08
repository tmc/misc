// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func doCallgraph(dir, gopath string, tests bool, args []string) error {
	if len(args) == 0 {
		flag.Usage()
		return nil
	}

	// Load packages
	cfg := &packages.Config{
		Mode:  packages.LoadAllSyntax,
		Tests: tests,
		Dir:   dir,
	}
	
	// Explicitly set environment variables
	home := os.Getenv("HOME")
	cache := os.Getenv("GOCACHE")
	env := os.Environ()
	
	// Debug output for environment variables
	fmt.Fprintf(os.Stderr, "HOME=%s, GOCACHE=%s\n", home, cache)
	
	if gopath != "" {
		env = append(env, "GOPATH="+gopath)
	}
	if home != "" {
		env = append(env, "HOME="+home)
	}
	if cache != "" {
		env = append(env, "GOCACHE="+cache)
	}
	
	cfg.Env = env
	
	// Apply cgo-specific configuration if needed
	cfg = setupCgoOptions(cfg)

	fmt.Fprintf(os.Stderr, "Loading packages from dir %q: %v\n", dir, args)
	initial, err := packages.Load(cfg, args...)
	if err != nil {
		return fmt.Errorf("loading packages: %v", err)
	}
	if packages.PrintErrors(initial) > 0 {
		return fmt.Errorf("packages contain errors")
	}

	fmt.Fprintf(os.Stderr, "Loaded %d packages\n", len(initial))

	// Create and build SSA
	prog, pkgs := ssautil.AllPackages(initial, ssa.InstantiateGenerics)
	prog.Build()

	fmt.Fprintf(os.Stderr, "Built SSA for %d packages\n", len(pkgs))

	// Perform analysis
	res, err := analyzeProgram(prog, pkgs, initial, tests)
	if err != nil {
		return fmt.Errorf("analysis failed: %v", err)
	}
	
	// Analyze cgo references if enabled
	analyzeCgoRefs(res)

	// Print analysis stats
	fmt.Fprintf(os.Stderr, "Analysis found:\n")
	fmt.Fprintf(os.Stderr, "  Dead functions: %d\n", len(res.deadFuncs))
	fmt.Fprintf(os.Stderr, "  Dead types: %d\n", len(res.deadTypes))
	fmt.Fprintf(os.Stderr, "  Dead interfaces: %d\n", len(res.deadIfaces))
	fmt.Fprintf(os.Stderr, "  Dead fields: %d\n", len(res.deadFields))
	fmt.Fprintf(os.Stderr, "  Dead interface methods: %d\n", len(res.deadIfaceMethods))
	fmt.Fprintf(os.Stderr, "  Dead constants: %d\n", len(res.deadConstants))
	fmt.Fprintf(os.Stderr, "  Dead variables: %d\n", len(res.deadVariables))
	fmt.Fprintf(os.Stderr, "  Dead type aliases: %d\n", len(res.deadTypeAliases))
	fmt.Fprintf(os.Stderr, "  Unused exported symbols: %d\n", len(res.deadExported))
	
	// DEBUG - print the exact contents of our maps for debugging
	debugPrintMap("deadFuncs", res.deadFuncs)
	debugPrintMap("deadTypes", res.deadTypes)
	debugPrintMap("deadIfaces", res.deadIfaces)
	debugPrintMap("deadFields", res.deadFields)
	debugPrintMap("deadIfaceMethods", res.deadIfaceMethods)
	debugPrintMap("deadConstants", res.deadConstants)
	debugPrintMap("deadVariables", res.deadVariables)
	debugPrintMap("deadTypeAliases", res.deadTypeAliases)
	debugPrintMap("deadExported", res.deadExported)

	// Handle -whylive flag
	if *whyLiveFlag != "" {
		return explainLiveness(prog, res, *whyLiveFlag)
	}

	// Format results
	return formatResults(prog.Fset, res, *formatFlag, *jsonFlag)
}