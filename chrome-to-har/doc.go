// Command chrome-to-har records browser activity and generates HAR (HTTP Archive) files.
//
// This tool launches Chrome/Chromium, navigates to specified URLs, and captures all
// network traffic in the standard HAR format. It supports both interactive browsing
// and automated capture modes.
//
// Usage:
//
//	chrome-to-har [flags] [URL...]
//
// Common flags:
//
//	-o file        Output HAR file (default: output.har)
//	-profile name  Use specific Chrome profile
//	-timeout sec   Global timeout in seconds (default: 30)
//	-headless      Run in headless mode
//	-filter regex  Filter requests by URL pattern
//	-block regex   Block requests matching pattern
//	-interactive   Enable interactive JavaScript mode
//
// Examples:
//
//	# Record a single page
//	chrome-to-har https://example.com
//
//	# Use a specific Chrome profile
//	chrome-to-har -profile "Default" https://github.com
//
//	# Filter only API requests
//	chrome-to-har -filter "api\." https://example.com
//
//	# Block tracking scripts
//	chrome-to-har -block "analytics|tracking" https://news.site.com
//
//	# Interactive mode with JavaScript console
//	chrome-to-har -interactive https://example.com
//
// The tool captures detailed timing information, request/response headers,
// cookies, and response content. The output HAR file can be analyzed using
// various HAR viewers and development tools.
//
// Interactive mode provides a JavaScript console for executing commands in
// the browser context, useful for debugging and automation tasks.
package main

//go:generate go run github.com/tmc/misc/gocmddoc@latest -o README.md

import _ "embed"

//go:embed docs/usage.md
var usageDoc string

// Version is the current version of chrome-to-har
const Version = "1.0.0"

// GetUsageDoc returns the full usage documentation
func GetUsageDoc() string {
	return usageDoc
}
