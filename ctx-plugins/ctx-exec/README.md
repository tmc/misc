# ctx-exec

ctx-exec is a command-line tool that executes shell commands and wraps their output in XML-like tags or JSON format.

## Installation

To install ctx-exec, make sure you have Go installed on your system, then run:

```
go install github.com/yourusername/ctx-exec@latest
```

Replace `yourusername` with your actual GitHub username or the appropriate path where you've hosted the project.

## Usage

```
ctx-exec [options] 'your shell command here'
```

Example:
```
ctx-exec 'echo Hello, World!'
```

Output (XML format):
```
<exec-output cmd="echo Hello, World!">Hello, World!
</exec-output>
```

JSON format:
```
ctx-exec -json 'echo Hello, World!'
```

Output:
```json
{
  "cmd": "echo Hello, World!",
  "stdout": "Hello, World!\n"
}
```

## Features

- Executes shell commands in the current environment
- Captures both stdout and stderr
- Wraps the output in XML-like tags or JSON format
- Supports escaping of special characters

## Error Handling

The program handles various error cases, including:
- Empty command input
- Commands that produce no output
- Commands that produce very large outputs
- Commands that return non-zero exit codes
- Special characters in commands and outputs

If an error occurs, it will be printed to stderr, and the program will exit with a non-zero status code.

## Environment Variables

- `CTX_EXEC_ESCAPE`: Set to "true" to enable XML escaping
- `CTX_EXEC_TAG`: Override the default output tag name (default: "exec-output")
- `CTX_EXEC_JSON`: Set to "true" to enable JSON output format

Example:
```bash
CTX_EXEC_TAG=custom-output ctx-exec 'echo hello'
# Output will use <custom-output> tags instead of <exec-output>

CTX_EXEC_JSON=true ctx-exec 'echo hello'
# Output will be in JSON format
```

