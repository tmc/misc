# MCP Combined Tools

This repository contains a comprehensive set of tools for working with the Model Context Protocol (MCP), a JSON-RPC 2.0 based protocol for communication between models and context tools.

## Key Components

### 1. mcptools

A collection of command-line tools and utilities for working with MCP:

- `mcp-spy`: Monitor and record MCP traffic
- `mcp-replay`: Replay recorded MCP sessions
- `mcp-start`: Start MCP servers
- `mcp-test`: Test MCP server implementations
- `mcp-verify`: Verify MCP protocol compliance
- Script testing framework for ensuring MCP tool reliability

### 2. mcpspy

A standalone tool for recording MCP traffic. It acts as a proxy that reads JSON-RPC 2.0 messages, logs them, and forwards them.

### 3. mcp-servers

Reference implementations of MCP servers:

- `mcp-macos-contacts`: A TypeScript-based MCP server that provides access to macOS contacts

## Getting Started

1. Build the tools:

```
cd mcptools
go build ./...
```

2. Run a test:

```
cd mcptools
go test ./...
```

3. Start a server:

```
cd mcp-servers/mcp-macos-contacts
npm install
npm start
```

4. Monitor MCP traffic:

```
mcpspy --record-file=session.mcp
```

## Documentation

See the README files in each directory for more detailed documentation:

- [mcptools/README.md](mcptools/README.md)
- [mcpspy/README.md](mcpspy/README.md)

## License

MIT
