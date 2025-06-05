// Command haranalyzer analyzes HAR (HTTP Archive) files to provide insights into web performance.
//
// This tool reads HAR files and provides various analysis features including
// performance metrics, resource breakdowns, and AI-powered insights using
// Anthropic's Claude API.
//
// Usage:
//
//	haranalyzer [command] [flags] <har-file>
//
// Available Commands:
//
//	analyze    Perform AI-powered analysis of the HAR file
//	summary    Display a summary of requests and performance metrics
//	export     Export HAR data to CSV format
//	help       Help about any command
//
// Global Flags:
//
//	-h, --help    help for haranalyzer
//
// Examples:
//
//	# Show summary statistics for a HAR file
//	haranalyzer summary recording.har
//
//	# Perform AI analysis (requires ANTHROPIC_API_KEY)
//	haranalyzer analyze recording.har
//
//	# Export to CSV for spreadsheet analysis
//	haranalyzer export recording.har > analysis.csv
//
// The tool provides insights into:
//   - Request count and types
//   - Total data transferred
//   - Response time statistics
//   - Resource type breakdowns
//   - Performance bottlenecks (with AI analysis)
//
// For AI-powered analysis, set the ANTHROPIC_API_KEY environment variable
// with your Anthropic API key.
package main

//go:generate go run github.com/tmc/misc/gocmddoc@latest -o README.md