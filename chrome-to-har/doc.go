/*
Package chrome-to-har provides a tool for capturing network activity from Chrome to HAR format.

# Installation

	go install github.com/tmc/misc/chrome-to-har@latest

# Basic Usage

Launch Chrome and capture network activity:

	chrome-to-har [-profile=/path/to/chrome/profile] [-output=output.har]

If no profile is specified, the first available Chrome profile will be automatically selected.
Use -verbose to see which profile was selected.

For more detailed usage information, see the embedded documentation below.
*/
package main

import _ "embed"

//go:embed docs/usage.md
var usageDoc string

// Version is the current version of chrome-to-har
const Version = "1.0.0"

// GetUsageDoc returns the full usage documentation
func GetUsageDoc() string {
	return usageDoc
}
