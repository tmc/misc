# Context Tools Specification v2

This document outlines a comprehensive specification for context tools, combining insights from both the individual ctx-plugins and the github.com/tmc/ctx framework.

## Overview

Context tools are utilities that gather, process, and format contextual information for use with LLMs (Large Language Models). They follow a standardized plugin architecture that enables discovery, orchestration, and consistent output formatting.

## Architectural Components

### Core Framework

A central command (`ctx`) provides:
1. **Discovery** - Finding available plugins in PATH
2. **Planning** - Determining which plugins to execute
3. **Execution** - Running plugins and formatting output

### Plugins

Individual tools that:
1. Are executable from the command line
2. Follow naming convention `ctx-*`
3. Report capabilities in a standard format
4. Produce standardized output

## Plugin Interface

### Capabilities Reporting

When invoked with `--capabilities`, plugins should output a JSON object:

```json
{
  "name": "ctx-example",
  "description": "Example context tool",
  "version": "1.0.0",
  "author": "Author Name",
  "flags": [
    {"name": "escape", "type": "bool", "description": "Enable escaping of special characters"},
    {"name": "json", "type": "bool", "description": "Output in JSON format"},
    {"name": "tag", "type": "string", "description": "Override the output tag name"}
  ],
  "environment_variables": [
    {"name": "CTX_EXAMPLE_ESCAPE", "type": "bool", "description": "Enable escaping of special characters"},
    {"name": "CTX_EXAMPLE_JSON", "type": "bool", "description": "Output in JSON format"},
    {"name": "CTX_EXAMPLE_TAG", "type": "string", "description": "Override the output tag name"}
  ],
  "relevance": {
    "repo": 0.8,         // Score for Git repository context
    "filesystem": 0.5,   // Score for general filesystem context
    "language": {        // Language-specific relevance scores
      "go": 0.9,
      "python": 0.2,
      "javascript": 0.1
    }
  }
}
```

### Plan Relevance

When invoked with `--plan-relevance`, plugins should output a relevance score (0.0-1.0) for the current context:

```json
{
  "score": 0.85,
  "reason": "Found Go code with package imports in current directory"
}
```

### Standard Flags

All plugins should support:

- `--help` - Display usage information
- `--version` - Display tool version
- `--capabilities` - Output capabilities JSON
- `--plan-relevance` - Output contextual relevance
- `--escape` - Enable escaping of special characters
- `--json` - Output in JSON format
- `--tag <name>` - Override the output tag name

### Environment Variables

For each flag, an environment variable should be available:

- `CTX_TOOL_ESCAPE` - Enable escaping
- `CTX_TOOL_JSON` - Enable JSON output
- `CTX_TOOL_TAG` - Set output tag name
- `CTX_TOOL_DEBUG` - Enable debug mode

Plus tool-specific variables following `CTX_TOOLNAME_OPTION` pattern.

## Output Format

### XML-like Format (Default)

```
<ctx-toolname cmd="command that was run">
  <stdout>standard output text</stdout>
  <stderr>standard error text</stderr>
  <e>error message (if any)</e>
</ctx-toolname>
```

### JSON Format

```json
{
  "tool": "ctx-toolname",
  "cmd": "command that was run",
  "stdout": "standard output text",
  "stderr": "standard error text", 
  "error": "error message (if any)"
}
```

## Implementation Guidelines

### Plugin Development

1. **Entry Point:**
   - Single binary named `ctx-toolname`
   - Executable from command line

2. **Capability Reporting:**
   - Implement `--capabilities` flag
   - Return standardized JSON

3. **Relevance Scoring:**
   - Implement `--plan-relevance` flag
   - Return contextual relevance score

4. **Output Formatting:**
   - Support both XML-like and JSON formats
   - Use standardized structure

5. **Error Handling:**
   - Include errors in standard output structure
   - Use appropriate exit codes

### Framework Integration

1. **Discovery:**
   - Plugins found by `ctx-*` prefix in PATH
   - Capabilities queried and cached

2. **Planning:**
   - Relevance scores determine execution order
   - Language/context detection aids selection

3. **Execution:**
   - Plugins run with appropriate flags
   - Output formatted per user preferences

4. **Composition:**
   - Combined output from multiple plugins
   - Consistent formatting across all tools

## Common Implementation Patterns

### Command Execution

```go
func executeCommand(command string) (stdout, stderr string, err error) {
    cmd := exec.Command("bash", "-o", "pipefail", "-c", command)
    cmd.Env = os.Environ()
    
    var stdoutBuf, stderrBuf bytes.Buffer
    cmd.Stdout = &stdoutBuf
    cmd.Stderr = &stderrBuf
    
    err = cmd.Run()
    return stdoutBuf.String(), stderrBuf.String(), err
}
```

### Output Formatting

```go
func formatOutput(toolName, command, stdout, stderr string, err error, jsonOutput bool) string {
    if jsonOutput {
        return formatJSON(toolName, command, stdout, stderr, err)
    }
    return formatXML(toolName, command, stdout, stderr, err)
}
```

### Flag Parsing

```go
func parseFlags() (options Options) {
    flag.BoolVar(&options.Escape, "escape", false, "Enable escaping of special characters")
    flag.BoolVar(&options.JSON, "json", false, "Output in JSON format")
    flag.StringVar(&options.Tag, "tag", "", "Override the output tag name")
    flag.Parse()
    
    // Override with environment variables if set
    if envEscape := os.Getenv("CTX_TOOL_ESCAPE"); envEscape == "true" {
        options.Escape = true
    }
    // ...similarly for other options
    
    return options
}
```

## Example Tools

1. **ctx-exec** - Execute commands with structured output
2. **ctx-go-doc** - Fetch Go documentation
3. **ctx-src** - Extract source code with formatting
4. **ctx-src-server** - HTTP server for remote source access

## Future Directions

1. **Shared Libraries:**
   - Common utility functions
   - Standardized flag parsing
   - Output formatting helpers

2. **Composability:**
   - Pipeline support
   - Stream processing
   - Filtering capabilities

3. **Extended Outputs:**
   - Markdown formatting
   - Rich media support
   - Structured data formats

4. **Ecosystem:**
   - Plugin repository
   - Version management
   - Dependency handling