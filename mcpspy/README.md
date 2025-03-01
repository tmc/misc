# mcpspy

`mcpspy` is a tool for recording Model Context Protocol (MCP) traffic. It acts as a proxy that reads JSON-RPC 2.0 messages, logs them, and forwards them.

## Usage

```
mcpspy [flags] [command]
```

### Flags

- `--input-file`: File to read MCP messages from (default: stdin)
- `--output-file`: File to write MCP messages to (default: stdout)
- `--record-file`: File to record MCP traffic (default: recording.mcp)
- `--verbose`: Enable verbose logging

## Example

```
mcpspy --record-file=session.mcp
```

This will act as a proxy for MCP messages, recording them to session.mcp.
