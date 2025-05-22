# mcpscripttest Summary

## Overview

`mcpscripttest` is a Go testing framework built on top of `rsc.io/script/scripttest` that provides script-based testing for Model Context Protocol (MCP) tools and servers. It extends the standard scripttest functionality with MCP-specific commands and utilities.

## Main Implementation

The core implementation is located at:
- `/Volumes/tmc/go/src/github.com/tmc/mcp/exp/mcpscripttest/scripttest.go`

Key components include:
- `MCPScripttestOptions` - Configuration options for tests
- `Test()` - Main function to run script tests
- `NewEngine()` - Creates a script engine with MCP commands
- Built-in MCP commands like `mcp-replay`, `mcp-spy`, `mcp-start`, etc.

## Script Format

Tests are written in a simple script format (`.txt` files) that combines:
- Shell-like commands
- JSON-RPC messages
- Output assertions
- File operations

Example test script structure:
```
# Comment describing the test
mkdir -p cmd/test-server
cat > cmd/test-server/main.go << 'EOF'
// Go code here
EOF

# Run the server with stdin input
setstdin {"jsonrpc":"2.0","method":"initialize",...}
exec go run cmd/test-server/main.go --stdio
stdout '"jsonrpc":"2.0"'
```

## Available Commands

### MCP-specific commands:
- `mcp-replay` - Replay MCP recordings
- `mcp-spy` - Spy on MCP traffic
- `mcp-start` - Start MCP components (async)
- `mcp-test` - Run MCP tests
- `mcp-verify` - Verify MCP recordings
- `mcp-send` - Send MCP messages
- `mcp-recv` - Receive MCP messages
- `mcp-serve` - Start MCP server
- `mcp-scripttest-server` - Run scriptable MCP server
- `mcpspy` - Alias for mcp-spy
- `mcpdiff` - Compare MCP files
- `mcpcat` - Colorize MCP trace files
- `mcp-sort` - Sort MCP traces by timestamp
- `mcp-shadow` - Shadow MCP traffic to test servers
- `mcp-probe` - Probe MCP server capabilities

### Utility commands:
- `setstdin` - Set stdin content for next command
- `stdout` - Verify stdout contains text
- `cat` - Output text

### Standard scripttest commands:
All standard commands from `rsc.io/script/scripttest` are available, including:
- `exec` - Execute commands
- `mkdir` - Create directories
- `echo` - Print text
- `grep` - Search text
- And more...

## Usage Examples

### Running tests:
```go
// In a test file
func TestMCPScripts(t *testing.T) {
    mcpscripttest.Test(t, "testdata/*.txt")
}

// With custom options
opts := mcpscripttest.DefaultOptions()
opts.DebugMode = true
mcpscripttest.Test(t, "testdata/*.txt", opts)
```

### Test script example:
```
# Test basic server with stdio transport
echo "Testing basic server..."

# Create a simple server
cat > server.go << 'EOF'
package main
// ... server code
EOF

# Send initialize request
setstdin {"jsonrpc":"2.0","method":"initialize","id":1}
exec go run server.go --stdio
stdout '"result"'
```

## Test Organization

Tests are typically organized as:
```
testdata/
├── server_coverage/
│   ├── basic_server.txt
│   ├── error_handling.txt
│   └── sse_transport.txt
├── tools/
│   └── integration.txt
└── mcp_conformance/
    ├── 01_base_messaging.txt
    ├── 02_lifecycle.txt
    └── ...
```

## Features

- **Script-based testing**: Write tests as simple scripts
- **MCP-aware**: Built-in support for MCP commands and protocols
- **Coverage support**: Can run with Go test coverage
- **Flexible configuration**: Customizable options and commands
- **Environment control**: Manage test environment variables
- **Async support**: Handle background processes
- **Debug mode**: Interactive debugging on test failure

## Best Practices

1. Use descriptive comments in test scripts
2. Organize tests by category in subdirectories
3. Use `setstdin` for providing JSON-RPC input
4. Verify outputs with `stdout` and `stderr` commands
5. Clean up resources in tests
6. Use the `--` separator for complex commands

## Related Files

- Test implementations: `/Volumes/tmc/go/src/github.com/tmc/mcp/exp/mcpscripttest/`
- Test examples: `/Volumes/tmc/go/src/github.com/tmc/mcp/testdata/server_coverage/`
- Documentation: `/Volumes/tmc/go/src/github.com/tmc/mcp/docs/mcpscripttest-conformance.md`
- Command-line tool: `/Volumes/tmc/go/src/github.com/tmc/mcp/exp/cmd/mcpscripttest/`

## Coverage Analysis

Tests can be run with coverage:
```bash
go test -coverprofile=coverage.out -v ./exp/mcpscripttest
go tool cover -html=coverage.out
```

Or using the standalone binary:
```bash
./mcpscripttest -all -coverage
```