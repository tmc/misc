# mcpspec

A tool for extracting and verifying Model Context Protocol (MCP) specifications.

## Usage

Extract a spec from a running server:
```bash
mcpspec -extract "./server :8080" spec.json
```

Verify a server against a spec:
```bash
mcpspec -verify "./server :8080" spec.json
```

## Spec Format

The spec file is a JSON document containing tool specifications:

```json
[
  {
    "name": "list_contacts",
    "description": "Lists all contacts",
    "examples": [
      {
        "name": "Basic usage",
        "input": {"name": "list_contacts"},
        "output": {"content": [{"type": "text", "text": "Alice <alice@example.com>"}]}
      }
    ],
    "tests": [
      "testdata/scripts/list_contacts.txt"
    ]
  }
]
```

## Test Files

Test files use the same format as MCP recordings, with comments and multiple test cases:

```
# Test basic functionality
mcp-in {"jsonrpc":"2.0","id":1,"method":"initialize",...}
mcp-out {"jsonrpc":"2.0","id":1,"result":{...}}

# Test error case
mcp-in {"jsonrpc":"2.0","id":2,"method":"callTool",...}
mcp-out {"jsonrpc":"2.0","id":2,"error":{...}}
```

Each test file can contain multiple test cases, separated by comments. The tool will run all test cases in sequence. 