# ctx-src

`ctx-src` is a command-line tool designed to prepare source code content from text files in a specified directory for feeding into large language models. It can read and print the contents of text files or count tokens, while respecting Git ignore rules and excluding specified directories and files.

## Features

- Recursively processes text files in a directory
- Respects Git ignore rules when in a Git repository
- Supports custom ignore patterns via .ctx-src-ignore file
- Excludes binary files and large text files automatically
- Option to count tokens instead of outputting file contents
- Verbose mode for detailed output
- Customizable through command-line arguments

## Installation

To install ctx-src, make sure you have Bash installed on your system. You can copy the script to a directory in your PATH:

```bash
# Option 1: Install from source
cd /path/to/ctx-plugins/ctx-src
chmod +x ctx-src.sh
ln -s "$(pwd)/ctx-src.sh" ~/bin/ctx-src

# Option 2: Install from GitHub (requires curl)
curl -o ~/bin/ctx-src https://raw.githubusercontent.com/tmc/misc/master/ctx-plugins/ctx-src/ctx-src.sh
chmod +x ~/bin/ctx-src
```

## Usage

```bash
ctx-src [OPTIONS] [<directory>] [<pathspec>...] [-- <pathspec>...]
```

### Options

- `--count-tokens`: Count tokens instead of outputting file contents
- `--verbose`: Enable verbose output
- `--no-xml-tags`: Disable XML tags around content
- `--include-svg`: Explicitly include SVG files
- `--include-xml`: Explicitly include XML files
- `--tracked-only`: Only include tracked files in Git repositories

### Arguments

- `<directory>`: Specify the directory to process (default: current directory)
- `<pathspec>`: One or more pathspec patterns to filter files

### Examples

Process all files in the current directory:
```bash
ctx-src
```

Process specific file types in a directory:
```bash
ctx-src /path/to/project '*.js' '*.py'
```

Exclude test directories and count tokens:
```bash
ctx-src --count-tokens /path/to/project '**/*.txt' '!**/test/**'
```

Only include tracked files in Git repository:
```bash
ctx-src --tracked-only .
```

Use complex Git pathspecs:
```bash
ctx-src /path/to/project '*.go' ':(exclude)vendor/**'
```

## Output Format

By default, the tool outputs files with XML-like tags for structured parsing:

```xml
<src path="~/projects/example">
  <file path="main.go">
    package main
    
    import "fmt"
    
    func main() {
        fmt.Println("Hello, World!")
    }
  </file>
  <file path="utils/helper.go">
    package utils
    
    func Helper() string {
        return "I'm a helper function"
    }
  </file>
</src>
```

## Custom Ignore Rules

You can create a `.ctx-src-ignore` file in your project to specify additional patterns to ignore. The format is similar to `.gitignore`:

```
# Example .ctx-src-ignore file
*.generated.go
build/
dist/
```

## Dependencies

- Bash
- Git (optional, for better file listing in Git repositories)
- Go (required for token counting with the --count-tokens flag)
- `tokencount` tool (automatically suggested for installation if not present when using `--count-tokens`)

## License

This project is licensed under the MIT License.