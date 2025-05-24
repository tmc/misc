/*
gocmddoc is a simple command-line tool that generates Markdown documentation
from Go package documentation.

The tool extracts package documentation, including the package comment,
exported types, functions, methods, and constants, and formats them as
a Markdown file suitable for README files or other documentation purposes.

Usage:

	gocmddoc [flags] [package]

The package argument can be:
  - A relative path (e.g., ./mypackage)
  - An import path (e.g., github.com/user/repo/pkg)
  - Empty (defaults to current directory)

Flags:

	-o, -output string
		Output file path (default: stdout)
		If not specified, the Markdown is written to standard output.

	-h, -help
		Show usage information

Examples:

Generate documentation for the current package:

	gocmddoc

Generate documentation for a specific package and save to file:

	gocmddoc -o README.md github.com/user/repo/pkg

Generate documentation for a local package:

	gocmddoc -o docs/api.md ./internal/mypackage

Output Format:

The generated Markdown follows this structure:

	# Package packagename

	Package description from the package comment.

	## Constants

	List of exported constants with their documentation.

	## Variables

	List of exported variables with their documentation.

	## Functions

	List of exported functions with their signatures and documentation.

	## Types

	List of exported types, their methods, and documentation.

The tool focuses on simplicity and generates clean, readable Markdown
that can be directly used in GitHub repositories or other documentation
systems that support Markdown.
*/
package main

//go:generate go run . -output README.md
