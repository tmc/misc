# jsfmt

`jsfmt` is a tool for formatting JavaScript code, inspired by `gofmt`. It provides consistent code styling for JavaScript projects.

## Installation

```
go install github.com/tmc/misc/jsfmt/cmd/jsfmt@latest
```

## Usage

Format a file and print to stdout:
```
jsfmt file.js
```

Format a file in place:
```
jsfmt -w file.js
```

Check if a file is formatted correctly:
```
jsfmt -l file.js
```

Show diff of formatting changes:
```
jsfmt -d file.js
```

Process stdin:
```
cat file.js | jsfmt
```

Format all JavaScript files in a directory:
```
jsfmt -w directory/
```

## Formatting Style

jsfmt follows Chrome's JavaScript formatting style with the following rules:

- 2 space indentation (configurable with `-tab-size` flag)
- Single quotes for string literals
- Semicolons at end of statements
- Space after keywords and before braces
- No trailing commas
- Space around operators
- No trailing whitespace

## Requirements

Default implementation uses Node.js and Prettier for maximum compatibility with modern JavaScript. If Node.js is not available, a fallback Go-native formatter is used.

## License

This project is licensed under the BSD-3-Clause License - see the LICENSE file for details.