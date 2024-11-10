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

	// New flags for additional analysis
	typesFlag  = flag.Bool("types", false, "report unreferenced types")
	ifacesFlag = flag.Bool("ifaces", false, "report unused interfaces")
	fieldsFlag = flag.Bool("fields", false, "report unused struct fields")
	allFlag    = flag.Bool("all", false, "enable all dead code checks")
)

func main() {
	flag.Parse()
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

	initial, err := packages.Load(cfg, args...)
	if err != nil {
		return err
	}
	if packages.PrintErrors(initial) > 0 {
		return fmt.Errorf("packages contain errors")
	}

	// Create and build SSA
	prog, pkgs := ssautil.AllPackages(initial, ssa.InstantiateGenerics)
	prog.Build()

	// Perform analysis
	res, err := analyzeProgram(prog, pkgs, initial, tests)
	if err != nil {
		return err
	}

	// Handle -whylive flag
	if *whyLiveFlag != "" {
		return explainLiveness(prog, res, *whyLiveFlag)
	}

	// Format results
	return formatResults(prog.Fset, res, *formatFlag, *jsonFlag)
}
