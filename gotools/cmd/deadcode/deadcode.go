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

var (
	// Existing flags
	testFlag      = flag.Bool("test", false, "include test code")
	formatFlag    = flag.String("format", "", "output format (template)")
	jsonFlag      = flag.Bool("json", false, "output JSON")
	filterFlag    = flag.String("filter", "<module>", "filter packages")
	generatedFlag = flag.Bool("generated", false, "include generated code")
	whyLiveFlag   = flag.String("whylive", "", "explain why function is live")
	debugFlag     = flag.Bool("debug", false, "enable debug output")
	cgoFlag       = flag.Bool("cgo", false, "enable cgo support for analyzing C bindings")

	// New flags for additional analysis
	typesFlag       = flag.Bool("types", false, "report unreferenced types")
	ifacesFlag      = flag.Bool("ifaces", false, "report unused interfaces")
	fieldsFlag      = flag.Bool("fields", false, "report unused struct fields")
	ifaceMethodFlag = flag.Bool("ifacemethods", false, "report unused interface methods")
	constantsFlag   = flag.Bool("constants", false, "report unused constants")
	variablesFlag   = flag.Bool("variables", false, "report unused variables")
	typeAliasesFlag = flag.Bool("typealiases", false, "report unused type aliases")
	exportedFlag    = flag.Bool("exported", false, "report exported symbols not used outside their package")
	allFlag         = flag.Bool("all", false, "enable all dead code checks")
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
