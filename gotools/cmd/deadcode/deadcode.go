// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"slices"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var (
	// Existing flags
	testFlag      = flag.Bool("test", false, "include test code")
	formatFlag    = flag.String("format", "", "output format (template)")
	jsonFlag      = flag.Bool("json", false, "output JSON")
	filterFlag    = flag.String("filter", "<module>", "filter packages")
	generatedFlag = flag.Bool("generated", false, "include generated code")
	whyLiveFlag   = flag.String("whylive", "", "explain why function is live")

	// New flags for additional analysis
	typesFlag  = flag.Bool("types", false, "report unreferenced types")
	ifacesFlag = flag.Bool("ifaces", false, "report unused interfaces")
	fieldsFlag = flag.Bool("fields", false, "report unused struct fields")
	allFlag    = flag.Bool("all", false, "enable all dead code checks")
)

func main() {
	flag.Parse()
	
	// Process each package and its files for analysis and 
	// detect generated files to exclude them from results
	for _, arg := range flag.Args() {
		// Check if the directory exists
		if _, err := os.Stat(arg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", arg, err)
			continue
		}
	}
	
	if err := doCallgraph("", "", *testFlag, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "deadcode: %s\n", err)
		os.Exit(1)
	}
}

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
	if gopath != "" {
		cfg.Env = append(os.Environ(), "GOPATH="+gopath)
	}

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

	// Print analysis stats
	fmt.Fprintf(os.Stderr, "Analysis found:\n")
	fmt.Fprintf(os.Stderr, "  Dead functions: %d\n", len(res.deadFuncs))
	fmt.Fprintf(os.Stderr, "  Dead types: %d\n", len(res.deadTypes))
	fmt.Fprintf(os.Stderr, "  Dead interfaces: %d\n", len(res.deadIfaces))
	fmt.Fprintf(os.Stderr, "  Dead fields: %d\n", len(res.deadFields))

	// Handle -whylive flag
	if *whyLiveFlag != "" {
		return explainLiveness(prog, res, *whyLiveFlag)
	}

	// Format results
	return formatResults(prog.Fset, res, *formatFlag, *jsonFlag)
}
