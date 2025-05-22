# txtarpkg - Generic Directory Packager

A simple, general-purpose CLI tool to package directories as txtar files.

## Installation

```bash
go install github.com/tmc/misc/txtarpkg@latest
```

## Usage

Package a single directory:

```bash
# Output to stdout
txtarpkg myproject

# Save to a file
txtarpkg config -o config.txtar
```

Package all directories:

```bash
# Output all directories to stdout
txtarpkg -a

# Save all directories to a file
txtarpkg -a -o alldir.txtar
```

Specify a custom source directory:

```bash
# Use a specific source path
txtarpkg -path=/path/to/source mydir
```

Add a custom comment:

```bash
txtarpkg -comment="# My Project Files" myproject
```

## What are txtar files?

Txtar is a simple text-based archive format used for bundling multiple files into a single text file. It's commonly used in Go for storing examples, test data, and other file collections.

The format is simple:
- A comment section at the top
- Each file starts with a header line with the format `-- filename --`
- The file content follows until the next file header or end of archive

## Example Output

Example of a txtar file containing a project:

```
# myproject

-- config.json --
{
  "name": "My Project",
  "version": "1.0.0",
  "description": "A sample project"
}

-- src/main.go --
package main

import "fmt"

func main() {
  fmt.Println("Hello World!")
}

-- README.md --
# My Project

A sample project that does amazing things.
```

You can extract a txtar file using the `go tool txtar extract` command:

```bash
go tool txtar extract myproject.txtar
```

This will create a directory structure with all the files from the txtar archive.

## Features

- **Simple**: Easy to use with minimal dependencies
- **Generic**: Works with any directory structure
- **Selective**: Can package a single directory or all directories
- **Customizable**: Add your own header comments
- **Filters**: Automatically skips hidden files and directories (starting with .)
- **Portable**: Output to stdout or a file
- **Embeddable**: Just a single Go file