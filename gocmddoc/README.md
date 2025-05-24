# gocmddoc

[![Go Reference](https://pkg.go.dev/badge/github.com/tmc/misc/gocmddoc.svg)](https://pkg.go.dev/github.com/tmc/misc/gocmddoc)

Gocmddoc generates Markdown documentation from Go package documentation
comments.

The tool extracts package documentation and formats it as clean, readable
Markdown suitable for README files or other documentation purposes. For
library packages, it includes exported types, functions, methods, constants,
and variables. For main packages, it shows only the package documentation
by default.
## Installation

<details>
<summary><b>Prerequisites: Go Installation</b></summary>

You'll need Go 1.21 or later. [Install Go](https://go.dev/doc/install) if you haven't already.

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

```bash
go install github.com/tmc/misc/gocmddoc@latest
```

### Run directly

```bash
go run github.com/tmc/misc/gocmddoc@latest [arguments]
```

## Usage


    gocmddoc [flags] [package]

The package argument can be:
  - A relative path (e.g., ./mypackage)
  - An import path (e.g., github.com/user/repo/pkg)
  - Empty (defaults to current directory)

## Flags


The following flags control the tool's behavior:

    -o, -output string
    	Output file path. If not specified, writes to stdout.

    -toc
    	Generate table of contents (default: false).
    	Use -toc to enable.

    -badge
    	Add pkg.go.dev badge for packages (default: true).
    	Use -badge=false to disable.

    -add-install-section
    	Add installation instructions section (default: true).
    	Shows go install/run commands for tools and go get for libraries.
    	Use -add-install-section=false to disable.

    -shields string
    	Add GitHub shields/badges. Options: all, version, license, build, report.
    	Use comma-separated values (e.g., -shields=version,license).
    	Use -shields=all to include all available shields.
    	Only works for GitHub-hosted packages.

    -h, -help
    	Show usage information.

## Examples


Generate documentation for the current package:

    gocmddoc

Generate documentation for a specific package and save to file:

    gocmddoc -o README.md github.com/user/repo/pkg

Generate documentation for a local package:

    gocmddoc -o docs/api.md ./internal/mypackage

Generate documentation for a command-line tool:

    gocmddoc -o README.md ./cmd/mytool

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

For main packages (commands), the binary name is used as the title, and all
declarations are included along with the package comment.

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
