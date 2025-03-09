// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package deadcode2 implements improved dead code detection for Go programs.

The deadcode2 tool analyzes Go packages to find unused code elements
including functions, methods, variables, constants, types, and type aliases.
It is an enhancement of the original deadcode tool with several improvements.

# Improvements over deadcode

  - Enhanced Interface Method Detection
    Improved algorithm for detecting unused interface methods by properly
    tracking all interface methods defined in the codebase and checking
    their usage through implementations.

  - Better Code Organization
    More modular code structure with clear separation of concerns through
    dedicated files for different aspects of analysis:
    - callgraph.go: Call graph analysis functionality
    - ifacemethods.go: Interface method detection
    - output_helpers.go: Output formatting helpers
    - main.go: Command-line handling and main logic

  - Advanced Test File Handling
    Special handling for test files to avoid false positives with improved
    detection of deliberately "used" items in tests using naming conventions
    such as "UsedInterface" or "usedField".

  - Comprehensive Detection
    Support for detecting multiple types of unused code elements:
    - Functions and methods
    - Interface methods
    - Types and interfaces
    - Struct fields
    - Constants and variables
    - Type aliases
    - Exported symbols

  - Improved Output Formatting
    Enhanced output formatting with more context and detail, including
    better organization and clearer messaging.

# Usage

Deadcode2 can be run with various flags to control which types of
unused code elements to detect:

    deadcode2 -ifacemethods .    # Find unused interface methods
    deadcode2 -constants .       # Find unused constants
    deadcode2 -all .             # Find all types of dead code

For complete flag options, run:

    deadcode2 -help

*/
package main