// Command autofix is a tool to automatically fix code issues.
//
// Autofix analyzes and automatically fixes common code problems. It provides
// subcommands for different operations:
//
// Usage:
//
//	autofix [command] [flags]
//
// Available Commands:
//
//	analyze    Analyze code for potential issues
//	fix        Apply automatic fixes to code
//	help       Help about any command
//
// Flags:
//
//	-h, --help      help for autofix
//	-v, --version   version for autofix
//
// Examples:
//
//	# Analyze code in the current directory
//	autofix analyze .
//
//	# Apply fixes to specific files
//	autofix fix main.go utils.go
//
// Use "autofix [command] --help" for more information about a command.
package main

//go:generate go run github.com/tmc/misc/gocmddoc@latest -o README.md