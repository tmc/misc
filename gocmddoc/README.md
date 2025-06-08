# gocmddoc

[![Go Reference](https://pkg.go.dev/badge/github.com/tmc/misc/gocmddoc.svg)](https://pkg.go.dev/github.com/tmc/misc/gocmddoc)

gocmddoc generates Markdown documentation from Go package documentation comments.

The tool extracts package documentation and formats it as clean, readable Markdown suitable for README files or other documentation purposes. For library packages, it includes exported types, functions, methods, constants, and variables. For main packages, it shows only the package documentation by default.
## Installation

<details>
<summary><b>Prerequisites: Go Installation</b></summary>

You'll need Go 1.24 or later. [Install Go](https://go.dev/doc/install) if you haven't already.

<details>
<summary><b>Setting up your PATH</b></summary>

After installing Go, ensure that `$HOME/go/bin` is in your PATH:

<details>
<summary><b>For bash users</b></summary>

Add to `~/.bashrc` or `~/.bash_profile`:
```bash
export PATH="$PATH:$HOME/go/bin"
```

Then reload your configuration:
```bash
source ~/.bashrc
```

</details>

<details>
<summary><b>For zsh users</b></summary>

Add to `~/.zshrc`:
```bash
export PATH="$PATH:$HOME/go/bin"
```

Then reload your configuration:
```bash
source ~/.zshrc
```

</details>

</details>

</details>

### Install

```console
go install github.com/tmc/misc/gocmddoc@latest
```

### Run directly

```console
go run github.com/tmc/misc/gocmddoc@latest [arguments]
```

## Usage

	gocmddoc [flags] [package]

	  -a	Include all declarations for main packages
	  -add-install-section
			Add installation instructions section (default true)
	  -all
			Include all declarations for main packages
	  -badge
			Add pkg.go.dev badge for library packages (default true)
	  -o string
			Output file path (default: stdout)
	  -output string
			Output file path (default: stdout)
	  -shields string
			Add shields: all, version, license, build, report (comma-separated)
	  -toc
			Generate table of contents

The package argument can be:

  - A relative path (e.g., ./mypackage)
  - An import path (e.g., github.com/user/repo/pkg)
  - Empty (defaults to current directory)

## Examples

Generate documentation for the current package:

	gocmddoc

Generate documentation for a specific package and save to file:

	gocmddoc -o README.md github.com/user/repo/pkg

Generate documentation for a local package:

	gocmddoc -o docs/api.md ./internal/mypackage

Show all declarations for a command-line tool:

	gocmddoc -all -o README.md ./cmd/mytool

Generate documentation with installation instructions:

	gocmddoc -add-install-section -o README.md

## Output Format

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

For main packages (commands), only the package comment is shown by default, with the binary name as the title. Use -all to include declarations.

## Features

The tool provides intelligent formatting:

  - Recognizes and formats code blocks with proper indentation
  - Converts documentation sections (like FLAGS, USAGE) to proper headings
  - Preserves code examples and formatting from source comments
  - Uses the directory name as the title for main packages
  - Generates table of contents with clickable links
  - Adds pkg.go.dev badge for Go packages
  - Includes installation instructions with Go PATH setup
  - Supports additional shields/badges for GitHub projects

## Go Generate

Add this directive to your Go files to automatically update documentation:

	//go:generate gocmddoc -o README.md

This will regenerate the README.md whenever go generate is run.
