# mcptools

A collection of tools for working with the Model Context Protocol (MCP).

## Installation

```bash
go install github.com/tmc/misc/mcptools/cmd/...@latest
```

## Tools

### mcpspy

Records MCP traffic between client and server:

```bash
mcpspy -f recording.mcp < input > output
```

### mcpreplay

Replays recorded MCP traffic to a server:

```bash
mcpreplay [-n] [-d duration] recording.mcp
```

Options:
- `-n`: Dry run mode (print messages instead of sending)
- `-d`: Delay between messages (e.g. `-d 100ms`)

### mcpverify

Verifies server behavior against a recording:

```bash
mcpverify -s "server-command" -f recording.mcp
```

### mcpstart

Starts a server and optionally records its traffic:

```bash
mcpstart [-f recording.mcp] server-command [args...]
```

### mcptest

Runs MCP test specifications:

```bash
mcptest [flags] [files...]
```

Options:
- `-update`: Update golden files

## Recording Format

The recording format is line-oriented JSON with direction prefixes:

```
mcp-in {"method":"ListTools"}
mcp-out {"tools":[{"name":"list_contacts"}]}
```

Each line starts with either `mcp-in` or `mcp-out` followed by a space and a JSON message. 