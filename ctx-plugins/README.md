# Context Tools

This repository contains a collection of tools and specifications for creating context-gathering utilities for use with Large Language Models (LLMs).

## Overview

Context tools are command-line utilities that assist in preparing structured content for Language Model context windows. They follow a consistent pattern of fetching, formatting, and outputting information in a standardized way.

## Specifications

- [ctx-tool-spec.md](ctx-tool-spec.md) - Base specification for context tools
- [mcp-tools-compatibility-checklist.md](mcp-tools-compatibility-checklist.md) - Compatibility checklist for evaluating tools
- [ctx-tool-spec-v2.md](ctx-tool-spec-v2.md) - Enhanced specification that incorporates plugin architecture
- [mcp-tools-library-proposal.md](mcp-tools-library-proposal.md) - Proposal for a shared tools library

## Library Interface

[ctx-tools-lib-interface.go](ctx-tools-lib-interface.go) provides a Go interface for implementing context tools that follow the specification.

## Example Implementation

The [ctx-example](ctx-example/) directory contains a sample implementation of a context tool that demonstrates the concepts in the specification:

- Capabilities reporting
- Plan relevance scoring
- Standardized output formats (XML and JSON)
- Common flag patterns
- Environment variable support

## Tools

The following context tools are included in this repository:

- [ctx-exec](ctx-exec/) - Execute shell commands and format their output
- [ctx-go-doc](ctx-go-doc/) - Fetch Go package documentation
- [ctx-src](ctx-src/) - Process source code files for LLM input
- [ctx-src-server](ctx-src-server/) - HTTP server for remote source code access

## Usage

Each tool follows a similar pattern for command-line usage:

```
$ ctx-toolname [options] [arguments]
```

Common options include:

- `--escape` - Enable escaping of special characters
- `--json` - Output in JSON format
- `--tag <name>` - Override the output tag name
- `--capabilities` - Display tool capabilities
- `--plan-relevance` - Display relevance score for current context

## Related Projects

- [github.com/tmc/ctx](https://github.com/tmc/ctx) - Framework for context gathering with discovery, planning, and execution phases

## Future Work

- Shared library implementation
- Migration of existing tools to the new specification
- Test framework for validating compatibility
- Plugin repository and discovery mechanisms