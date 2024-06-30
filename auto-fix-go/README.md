# auto-fix-go

auto-fix-go is a tool that automatically fixes Go code by running tests, submitting failures and source code to a language model, and making source edits in a loop to drive a program to completion.

## Installation

1. Ensure you have Go 1.22 or later installed on your system.
2. Install the tool using `go install`:
   ```
   go install github.com/tmc/misc/auto-fix-go@latest
   ```
   This will download, compile, and install the `auto-fix-go` binary in your `$GOPATH/bin` directory.

3. Make sure your `$GOPATH/bin` is in your system's PATH.

## Usage

Run auto-fix-go by providing the directory containing the Go project you want to fix:

```
auto-fix-go /path/to/your/go/project
```

The tool will:
1. Run tests in the specified directory
2. If tests fail, it will submit the source code and test output to a language model
3. Apply the suggested fixes
4. Repeat the process until all tests pass

## Requirements

- Go 1.22 or later
- An Anthropic API key set in the `ANTHROPIC_API_KEY` environment variable

## Development

If you want to contribute to the project or run it from source:

1. Clone the repository:
   ```
   git clone https://github.com/tmc/misc/auto-fix-go.git
   ```
2. Change to the project directory:
   ```
   cd auto-fix-go
   ```
3. Build the project:
   ```
   go build
   ```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details