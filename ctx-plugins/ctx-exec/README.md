# ctx-exec

ctx-exec is a command-line tool that executes shell commands and wraps their output in XML-like tags.

## Installation

To install ctx-exec, make sure you have Go installed on your system, then run:

```
go install github.com/yourusername/ctx-exec@latest
```

Replace `yourusername` with your actual GitHub username or the appropriate path where you've hosted the project.

## Usage

```
ctx-exec 'your shell command here'
```

Example:
```
ctx-exec 'echo Hello, World!'
```

Output:
```
<exec-output cmd="echo Hello, World!">Hello, World!
</exec-output>
```

## Features

- Executes shell commands in the current environment
- Captures both stdout and stderr
- Wraps the output in XML-like tags

## Error Handling

The program handles various error cases, including:
- Empty command input
- Commands that produce no output
- Commands that produce very large outputs
- Commands that return non-zero exit codes
- Special characters in commands and outputs

If an error occurs, it will be printed to stderr, and the program will exit with a non-zero status code.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.


## Environment Variables

- `CTX_EXEC_ESCAPE`: Set to "true" to enable XML escaping
- `CTX_EXEC_TAG`: Override the default output tag name (default: "exec-output")

Example:
```bash
CTX_EXEC_TAG=custom-output ctx-exec 'echo hello'
# Output will use <custom-output> tags instead of <exec-output>
```
