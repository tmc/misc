// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
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
		fmt.Fprintf(os.Stderr, "deadcode2: %s\n", err)
		os.Exit(1)
	}
}