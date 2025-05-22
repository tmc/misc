# MCP Context Tools Compatibility Checklist

This document provides a checklist for ensuring compatibility with the Model Context Protocol (MCP) tools ecosystem.

## Command-Line Interface

### Basic Flags
- [ ] `-help` - Display usage information
- [ ] `-version` - Display tool version information
- [ ] `-escape` - Enable escaping of special characters
- [ ] `-json` - Output in JSON format instead of XML-like tags
- [ ] `-tag` - Override the output tag name

### Extended Flags
- [ ] `-color` - Enable/disable colored output
- [ ] `-debug` - Enable debug logging

### Environment Variables
- [ ] `CTX_TOOL_ESCAPE` - Enable XML escaping (true/false)
- [ ] `CTX_TOOL_JSON` - Enable JSON output (true/false)
- [ ] `CTX_TOOL_TAG` - Override default tag name
- [ ] `CTX_TOOL_COLOR` - Control colored output
- [ ] `CTX_TOOL_DEBUG` - Enable debug output
- [ ] `NO_COLOR` - Standard for disabling colored output

## Output Formatting

### XML Format
- [ ] Output wrapped in configurable XML-like tags
- [ ] Command included as attribute or nested element
- [ ] Standard `<stdout>` element for command output
- [ ] Standard `<stderr>` element for error output
- [ ] Standard `<e>` or `<error>` element for error messages
- [ ] Proper nesting and consistent structure
- [ ] Optional HTML escaping of special characters

### JSON Format
- [ ] Structured JSON object output
- [ ] Standard `cmd` or `command` field
- [ ] Standard `stdout` field (omitted if empty)
- [ ] Standard `stderr` field (omitted if empty)
- [ ] Standard `error` field (omitted if successful)
- [ ] Consistent field naming across tools
- [ ] Properly formatted JSON with consistent indentation

## Error Handling

- [ ] Non-zero exit code when command fails
- [ ] Error message included in structured output
- [ ] Appropriate validation of input parameters
- [ ] Clear error messages for common failures
- [ ] Graceful failure with helpful messages
- [ ] Error details accessible in both XML and JSON output

## Documentation

- [ ] `doc.go` file with package documentation
- [ ] Clear usage examples in README.md
- [ ] Complete documentation of flags and variables
- [ ] Examples of both success and error cases
- [ ] Installation instructions

## Testing

- [ ] Unit tests for core functionality
- [ ] Integration tests (where applicable)
- [ ] Test cases for error conditions
- [ ] Test examples in testdata/ directory
- [ ] Test coverage for different output formats

## Implementation Standards

- [ ] Cross-platform compatibility
- [ ] Minimal external dependencies
- [ ] Clean separation of concerns
- [ ] Consistent naming conventions
- [ ] Go idiomatic code style
- [ ] Proper logging approach
- [ ] Configuration via both flags and environment

## Tool Evaluation

### ctx-exec

| Feature | Supported | Notes |
|---------|-----------|-------|
| Basic Flags | ✅ | All implemented (`-escape`, `-json`, `-tag`) |
| Extended Flags | ✅ | Has `-color`, `-exit-code`, `-shell`, `-x` |
| Environment Variables | ✅ | Has specific vars for each flag |
| XML Format | ✅ | Full implementation with all standard elements |
| JSON Format | ✅ | Complete implementation with standard fields |
| Error Handling | ✅ | Thorough error capture and reporting |
| Documentation | ✅ | Well documented in doc.go and README |
| Testing | ✅ | Comprehensive test cases |
| Implementation | ✅ | Clean, idiomatic Go code |

### ctx-go-doc

| Feature | Supported | Notes |
|---------|-----------|-------|
| Basic Flags | ⚠️ | Missing `-escape`, `-json` |
| Extended Flags | ❌ | Limited flag support |
| Environment Variables | ⚠️ | Only supports `CTX_GO_DOC_DEBUG` |
| XML Format | ⚠️ | Basic formatting but inconsistent with ctx-exec |
| JSON Format | ❌ | Not supported |
| Error Handling | ✅ | Good error capture |
| Documentation | ✅ | Good documentation in doc.go |
| Testing | ⚠️ | Limited test coverage |
| Implementation | ✅ | Good implementation quality |

### ctx-src

| Feature | Supported | Notes |
|---------|-----------|-------|
| Basic Flags | ⚠️ | Different flag naming conventions |
| Extended Flags | ✅ | Has custom flags for file filtering |
| Environment Variables | ⚠️ | Limited environment variable support |
| XML Format | ✅ | Good XML formatting |
| JSON Format | ❌ | Not fully supported |
| Error Handling | ✅ | Good error reporting |
| Documentation | ✅ | Well documented |
| Testing | ✅ | Has test cases |
| Implementation | ⚠️ | Uses bash script for core functionality |

### ctx-src-server

| Feature | Supported | Notes |
|---------|-----------|-------|
| Basic Flags | ❌ | Different flag naming approach |
| Extended Flags | ✅ | Has server-specific flags |
| Environment Variables | ⚠️ | Limited environment variable support |
| XML Format | ⚠️ | Relies on ctx-src formatting |
| JSON Format | ✅ | Good JSON response format |
| Error Handling | ✅ | Good HTTP error handling |
| Documentation | ✅ | Well documented |
| Testing | ⚠️ | Limited test coverage |
| Implementation | ✅ | Good server implementation |

## Improvement Actions

Based on the evaluation, here are recommended actions to improve compatibility:

1. **For ctx-go-doc**:
   - Add `-escape`, `-json` flags
   - Implement JSON output format
   - Standardize environment variables
   - Add more test cases

2. **For ctx-src**:
   - Standardize flag names to match other tools
   - Expand environment variable support
   - Implement proper JSON output format
   - Consider pure Go implementation

3. **For ctx-src-server**:
   - Standardize flag names
   - Improve XML output formatting consistency
   - Expand test coverage

4. **General improvements**:
   - Create a shared library for common functionality
   - Standardize error format across all tools
   - Develop consistent documentation template
   - Implement version flag on all tools