# xq - XML and HTML Query Tool

xq is a command-line tool for formatting, querying, and converting XML and HTML documents, with a CLI similar to jq.

## Features

- XML and HTML formatting
- XPath querying
- Streaming support for large XML files
- JSON input and output
- Colorized output
- Multiple input files support

## Usage

```
xq [options] [file...]
```

If no file is specified, xq reads from standard input.

Options:
  -c    compact instead of pretty-printed output
  -r    output raw strings, not JSON texts
  -C    colorize JSON
  -n    use `null` as the single input value
  -s    read (slurp) all inputs into an array
  -f    input is JSON, not XML
  -j    output as JSON
  -h    treat input as HTML
  -S    stream large XML files
  -v    output version information and exit
  -x string
        XPath query to select nodes

## Examples

Format an XML file:
```
xq input.xml
```

Format an HTML file:
```
xq -h input.html
```

Extract nodes using XPath:
```
xq -x "//book/title" input.xml
```

Stream a large XML file:
```
xq -S large_input.xml
```

Output as JSON:
```
xq -j input.xml
```

Use colorized output:
```
xq -C input.xml
```

Process JSON input:
```
xq -f input.json
```

## Building and Testing

To build the project:
```
go build
```

To run tests:
```
go test
```

