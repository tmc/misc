# txtarpkg - Generic Directory Packager with Escaping Support

A simple, general-purpose CLI tool to package directories as txtar files with support for escaping/unescaping special characters.

## Installation

```bash
go install github.com/tmc/misc/txtarpkg@latest
```

## Enhanced Features

- **Escaping**: Automatically escape special characters that could break txtar format
- **Unescaping**: Reverse escaping when extracting files
- **Extraction**: Extract txtar files back to directories

## Usage

### Basic Usage (unchanged)

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

### Escaping Content

Use the `-escape` flag to escape special characters that could break the txtar format:

```bash
# Escape special characters when packaging
txtarpkg -escape myproject -o myproject.txtar

# This will escape lines that look like file markers (-- filename --)
# and leading backslashes that could interfere with escape sequences
```

### Extracting and Unescaping

Extract a txtar file back to directories:

```bash
# Basic extraction
txtarpkg -extract myproject.txtar

# Extract with unescaping
txtarpkg -extract -unescape myproject.txtar
```

### Advanced Options

```bash
# Specify a custom source directory
txtarpkg -path=/path/to/source mydir

# Add a custom comment
txtarpkg -comment="# My Project Files" myproject

# Combine options
txtarpkg -escape -comment="# Escaped content" -o safe.txtar myproject
```

## Escaping Details

The escaping feature handles two main cases:

1. **File Markers**: Lines that start with `-- ` and end with ` --` are escaped to prevent them from being interpreted as txtar file boundaries
2. **Leading Backslashes**: Lines starting with `\` are escaped to prevent escape sequence issues

When using `-unescape` during extraction, these escapes are automatically reversed.

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

### Example with Escaped Content

If a file contains lines that could be confused with txtar markers:

```
# myproject (escaped)

-- tricky.txt --
This is a normal line
\-- fake-file.txt --
This line would normally break txtar format, but it's escaped
Another normal line
```

## Extract a txtar file

Using the built-in extract feature:

```bash
# Extract with this tool
txtarpkg -extract myproject.txtar

# Or use the standard Go tool
go tool txtar extract myproject.txtar
```

## Features

- **Simple**: Easy to use with minimal dependencies
- **Generic**: Works with any directory structure
- **Selective**: Can package a single directory or all directories
- **Customizable**: Add your own header comments
- **Filters**: Automatically skips hidden files and directories (starting with .)
- **Portable**: Output to stdout or a file
- **Safe**: Escape/unescape special characters to prevent format corruption
- **Bidirectional**: Create archives and extract them back
- **Embeddable**: Just a single Go file