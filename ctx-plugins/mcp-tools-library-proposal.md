# MCP Tools Library Proposal

This document proposes a shared library to standardize functionality across MCP context tools.

## Overview

To improve consistency and reduce code duplication across MCP context tools, we propose creating a shared Go library. This library would provide common functionality for command-line parsing, output formatting, error handling, and other cross-cutting concerns.

## Goals

- Standardize interfaces and behaviors across all MCP context tools
- Reduce code duplication and maintenance burden
- Simplify development of new tools
- Ensure consistent user experience
- Provide a base for future extensions

## Proposed Library Structure

```
github.com/tmc/ctx-tools
├── cmd/            # Example tool commands
├── doc.go          # Package documentation
├── flag/           # Common flag handling
├── format/         # Output formatting utilities
│   ├── json/       # JSON output formatting
│   └── xml/        # XML output formatting
├── output/         # Output helpers
├── shell/          # Shell command utilities
├── go.mod
└── README.md
```

## Key Components

### Common Flag Handling

```go
// Package flag provides standardized flag handling for MCP context tools
package flag

// SetupStandardFlags sets up the common flags used across all MCP tools
func SetupStandardFlags() (*Options, error) {
    opts := &Options{}
    flag.BoolVar(&opts.Escape, "escape", false, "Enable escaping of special characters")
    flag.BoolVar(&opts.JSON, "json", false, "Output in JSON format")
    flag.StringVar(&opts.Tag, "tag", "", "Override the output tag name")
    flag.BoolVar(&opts.Color, "color", isTerminal(), "Enable colored output")
    flag.BoolVar(&opts.Debug, "debug", false, "Enable debug output")
    flag.Parse()
    
    // Process environment variables
    if envEscape := os.Getenv("CTX_TOOL_ESCAPE"); envEscape == "true" && !opts.Escape {
        opts.Escape = true
    }
    // ...similarly for other options
    
    return opts, nil
}
```

### Output Formatting

```go
// Package format provides standardized output formatting for MCP context tools
package format

// FormatOutput formats the given output according to the options
func FormatOutput(options *flag.Options, command, stdout, stderr string, err error) string {
    if options.JSON {
        return formatJSON(options, command, stdout, stderr, err)
    }
    return formatXML(options, command, stdout, stderr, err)
}

// FormatJSON formats the output as JSON
func formatJSON(options *flag.Options, command, stdout, stderr string, err error) string {
    output := map[string]interface{}{
        "cmd": command,
    }
    if stdout != "" {
        output["stdout"] = stdout
    }
    if stderr != "" {
        output["stderr"] = stderr
    }
    if err != nil {
        output["error"] = err.Error()
    }
    
    jsonBytes, _ := json.MarshalIndent(output, "", "  ")
    return string(jsonBytes)
}

// FormatXML formats the output using XML-like tags
func formatXML(options *flag.Options, command, stdout, stderr string, err error) string {
    tagName := options.Tag
    if tagName == "" {
        tagName = "ctx-output"
    }
    
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("<%s cmd=%q>\n", tagName, command))
    if stdout != "" {
        sb.WriteString(fmt.Sprintf("<stdout>%s</stdout>\n", escapeIfNeeded(stdout, options.Escape)))
    }
    if stderr != "" {
        sb.WriteString(fmt.Sprintf("<stderr>%s</stderr>\n", escapeIfNeeded(stderr, options.Escape)))
    }
    if err != nil {
        sb.WriteString(fmt.Sprintf("<e>%s</e>\n", escapeIfNeeded(err.Error(), options.Escape)))
    }
    sb.WriteString(fmt.Sprintf("</%s>\n", tagName))
    
    return sb.String()
}
```

### Shell Command Utilities

```go
// Package shell provides utilities for executing shell commands
package shell

// ExecuteCommand executes a command with proper error handling
func ExecuteCommand(command string, options *flag.Options) (stdout, stderr string, err error) {
    shell := os.Getenv("SHELL")
    if shell == "" {
        shell = "bash"
    }
    
    cmd := exec.Command(shell, "-o", "pipefail", "-c", command)
    cmd.Env = os.Environ()
    
    var stdoutBuf, stderrBuf bytes.Buffer
    cmd.Stdout = &stdoutBuf
    cmd.Stderr = &stderrBuf
    
    err = cmd.Run()
    stdout = stdoutBuf.String()
    stderr = stderrBuf.String()
    
    if options.Debug {
        fmt.Fprintf(os.Stderr, "DEBUG: Command: %s\n", command)
        fmt.Fprintf(os.Stderr, "DEBUG: Exit code: %d\n", cmd.ProcessState.ExitCode())
    }
    
    return stdout, stderr, err
}
```

## Example Tool Implementation

Using the library, a new tool could be implemented with minimal code:

```go
package main

import (
    "fmt"
    "os"
    
    "github.com/tmc/ctx-tools/flag"
    "github.com/tmc/ctx-tools/format"
    "github.com/tmc/ctx-tools/shell"
)

func main() {
    options, err := flag.SetupStandardFlags()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    
    if len(flag.Args()) < 1 {
        fmt.Fprintf(os.Stderr, "Error: No command specified\n")
        fmt.Fprintf(os.Stderr, "Usage: %s [options] command [args...]\n", os.Args[0])
        os.Exit(1)
    }
    
    command := fmt.Sprintf("%s", flag.Args()[0])
    if len(flag.Args()) > 1 {
        command = fmt.Sprintf("%s %s", command, flag.Args()[1:])
    }
    
    stdout, stderr, err := shell.ExecuteCommand(command, options)
    
    output := format.FormatOutput(options, command, stdout, stderr, err)
    fmt.Print(output)
    
    if err != nil && options.ExitCode {
        os.Exit(1)
    }
}
```

## Implementation Plan

1. **Phase 1: Library Foundation**
   - Implement core package structure
   - Create flag, format, and shell packages
   - Add comprehensive documentation
   - Write unit tests

2. **Phase 2: Migrate Existing Tools**
   - Update ctx-exec to use the shared library
   - Update ctx-go-doc to use the shared library
   - Update ctx-src to use the shared library
   - Update ctx-src-server to use the shared library

3. **Phase 3: Additional Features**
   - Add support for additional output formats
   - Implement caching utilities
   - Add debugging and profiling support
   - Create scaffolding tools for new MCP tools

## Benefits

- **Consistency**: All tools will have the same flags, environment variables, and output formats
- **Maintainability**: Bug fixes and improvements to the shared library benefit all tools
- **Extensibility**: New tools can be created faster with less boilerplate
- **Testing**: Shared test utilities ensure consistent behavior
- **Documentation**: Centralized documentation for common features

## Next Steps

1. Create the shared library repository
2. Implement the core components
3. Update existing tools to use the library
4. Create documentation and examples
5. Establish a release and versioning strategy