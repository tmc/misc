# Model Context Protocol Tools Specification

This document outlines common patterns, interfaces, and expectations for creating MCP (Model Context Protocol) context tools.

## Overview

Context tools are command-line utilities that assist in preparing structured content for Language Model context windows. They follow a consistent pattern of fetching, formatting, and outputting information in a standardized way.

## Core Principles

1. **Single Responsibility**: Each tool should do one thing well.
2. **Composability**: Tools should be easily composable with other command-line utilities.
3. **Standardized Output**: Tools should produce consistently formatted output.
4. **Useful Defaults**: Tools should work well with minimal configuration.
5. **Extensibility**: Tools should be configurable for specific use cases.

## Input/Output Conventions

### Output Format

Tools should support two primary output formats:

1. **XML-like Tags** (default):
   ```
   <ctx-tool>
     <command>command that was run</command>
     <stdout>standard output</stdout>
     <stderr>standard error output (if any)</stderr>
     <error>error message (if any)</error>
   </ctx-tool>
   ```

2. **JSON Format**:
   ```json
   {
     "tool": "ctx-tool",
     "command": "command that was run",
     "stdout": "standard output",
     "stderr": "standard error output (if any)",
     "error": "error message (if any)"
   }
   ```

### Tag Customization

- Tools should allow for tag name customization via flags and environment variables.
- The standard flag for overriding the tag name is `-tag`.
- The standard environment variable is `CTX_TOOL_TAG`.

### Character Escaping

- Tools should provide an option to escape special characters.
- The standard flag is `-escape`.
- The standard environment variable is `CTX_TOOL_ESCAPE`.

## Command-line Interfaces

### Standard Flags

Each tool should support these common flags:

- `-tag`: Override the output tag name (default is based on the tool name)
- `-escape`: Enable escaping of special characters
- `-json`: Output in JSON format instead of XML-like tags
- `-help`: Display usage information
- `-version`: Display the tool version

### Environment Variables

For each flag, there should be a corresponding environment variable:

- `CTX_TOOL_TAG`: Override the output tag name
- `CTX_TOOL_ESCAPE`: Enable escaping of special characters
- `CTX_TOOL_JSON`: Output in JSON format
- `CTX_TOOL_DEBUG`: Enable debug output (where applicable)

## Error Handling

- Tools should handle errors gracefully and include error information in the output structure.
- Exit codes should follow standard conventions (0 for success, non-zero for errors).
- Error messages should be clear and descriptive.

## Implementation Guidelines

### Language

- Tools should be implemented in Go for consistency and cross-platform compatibility.
- Minimize external dependencies.

### Packaging

- Tools should be distributable as single binaries.
- Provide installation methods via Go modules (`go install`).

### Documentation

- Each tool should have a `doc.go` file with comprehensive documentation.
- Include usage examples in the README.md file.
- Document all flags, environment variables, and behaviors.

### Testing

- Include comprehensive test cases.
- Provide example inputs and expected outputs.
- Consider test data that demonstrates edge cases.

## Tool-Specific Conventions

### Command Execution Tools (ctx-exec)

- Should capture and structure command output.
- Should preserve exit codes.
- Should handle command arguments properly.

### Documentation Tools (ctx-go-doc)

- Should provide context-ready documentation for code.
- Should handle package resolution automatically.
- Should support all underlying documentation tool flags.

### Source Code Tools (ctx-src)

- Should respect Git exclusions (.gitignore files).
- Should provide file filtering options.
- Should maintain context about file paths and languages.

### Server Tools (ctx-src-server)

- Should implement a RESTful API for remote access.
- Should include appropriate caching mechanisms.
- Should provide health and metrics endpoints.
- Should handle concurrent requests efficiently.

## Security Considerations

- Tools should not execute arbitrary code without proper validation.
- Server implementations should include appropriate authentication.
- Be careful with handling sensitive information in outputs.
- Validate and sanitize inputs to prevent command injection.

## MCP Integration

While these tools are designed to work with MCP, they don't necessarily need to import MCP libraries directly. They should:

1. Produce output that's easily consumable by MCP implementations.
2. Follow MCP conventions for structuring context information.
3. Be composable with other MCP components.

## Example Implementations

See the following projects for reference implementations:

- `ctx-exec`: Executes commands and structures their output
- `ctx-go-doc`: Fetches Go documentation in a structured format
- `ctx-src`: Extracts source code with proper formatting
- `ctx-src-server`: Provides HTTP access to source code extraction

## Future Considerations

- Standard libraries for common functionality
- Shared configuration formats
- Plugin architecture for extensibility
- Structured metadata for context information