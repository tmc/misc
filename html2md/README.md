# html2md

[![Go Reference](https://pkg.go.dev/badge/github.com/tmc/misc/html2md.svg)](https://pkg.go.dev/github.com/tmc/misc/html2md)

A command-line tool to convert HTML to Markdown with optional sanitization.

## Features

- Converts HTML from files or stdin to clean Markdown
- Uses GitHub Flavored Markdown by default
  - Tables, code fencing, task lists, strikethrough
  - No additional configuration needed
- Optional HTML sanitization (via bluemonday)
- Simple and composable, following Unix philosophy

## Installation

```bash
go install github.com/tmc/misc/html2md@latest
```

## Usage

```
html2md [-input=<filename>] [-sanitize]
```

**Options:**
- `-input string`: Input file (default: "-" for stdin)
- `-sanitize`: Sanitize HTML before conversion (removes potentially malicious content)

## Examples

Convert an HTML file to Markdown:
```bash
html2md -input=document.html > document.md
```

Convert and sanitize in one step:
```bash
html2md -input=untrusted.html -sanitize > safe.md
```

Use in a pipeline:
```bash
curl https://example.com | html2md > example.md
```

## Tool Comparison

For a detailed comparison with other HTML to Markdown tools, see [docs/comparison.md](docs/comparison.md).

## License

ISC License - See [LICENSE](LICENSE) file for details.
