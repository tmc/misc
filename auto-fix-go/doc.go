// Command auto-fix-go automatically fixes failing Go tests using AI assistance.
//
// This tool runs tests in a specified directory, and when tests fail, it uses
// an AI model (Anthropic's Claude) to analyze the failures and suggest fixes.
// It then applies the fixes and re-runs the tests, repeating this process
// until all tests pass.
//
// Usage:
//
//	auto-fix-go <directory>
//
// The tool requires the ANTHROPIC_API_KEY environment variable to be set with
// a valid Anthropic API key.
//
// Example:
//
//	export ANTHROPIC_API_KEY="your-api-key"
//	auto-fix-go ./myproject
//
// The tool will:
//  1. Run tests in the specified directory
//  2. If tests fail, analyze the failure output
//  3. Generate fixes using AI
//  4. Apply the fixes to the source code
//  5. Re-run tests and repeat until all pass
//
// Note: This tool modifies source files in place. It's recommended to use
// version control and review all changes made by the tool.
package main

//go:generate go run github.com/tmc/misc/gocmddoc@latest -o README.md