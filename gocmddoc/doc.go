/*
Gocmddoc generates Markdown documentation from Go package documentation
comments.

The tool extracts package documentation and formats it as clean, readable
Markdown suitable for README files or other documentation purposes. For
library packages, it includes exported types, functions, methods, constants,
and variables. For main packages, it shows only the package documentation
by default.

# Usage

	gocmddoc [flags] [package]

The package argument can be:
  - A relative path (e.g., ./mypackage)
  - An import path (e.g., github.com/user/repo/pkg)
  - Empty (defaults to current directory)

# Flags

The following flags control the tool's behavior:

	-o, -output string
		Output file path. If not specified, writes to stdout.

	-a, -all
		Include all declarations for main packages.
		By default, main packages only show package documentation.

	-h, -help
		Show usage information.

# Examples

Generate documentation for the current package:

	gocmddoc

Generate documentation for a specific package and save to file:

	gocmddoc -o README.md github.com/user/repo/pkg

Generate documentation for a local package:

	gocmddoc -o docs/api.md ./internal/mypackage

Show all declarations for a command-line tool:

	gocmddoc -all -o README.md ./cmd/mytool

# Output Format

The generated Markdown follows this structure:

For library packages:

	# Package packagename
	
	Package description from the package comment.
	
	## Constants
	
	Exported constants with their documentation.
	
	## Variables
	
	Exported variables with their documentation.
	
	## Functions
	
	Exported functions with their signatures and documentation.
	
	## Types
	
	Exported types, their methods, and documentation.

For main packages (commands), only the package comment is shown by default,
with the binary name as the title. Use -all to include declarations.

# Features

The tool provides intelligent formatting:
  - Recognizes and formats code blocks with proper indentation
  - Converts documentation sections (like FLAGS, USAGE) to proper headings
  - Formats lists appropriately based on context
  - Preserves code examples and formatting from source comments
  - Uses the directory name as the title for main packages

# Go Generate

Add this directive to your Go files to automatically update documentation:

	//go:generate gocmddoc -o README.md

This will regenerate the README.md whenever go generate is run.
*/
package main

//go:generate go run . -output README.md
