# Go MCP Servers

A collection of Model Context Protocol (MCP) servers implemented in Go, providing various tools and resources for AI assistants.

## Servers

### 1. Filesystem MCP Server
**Binary:** `filesystem-mcp-server`

Provides filesystem operations including:
- Read/write files
- List directories (recursive option)
- Search files with patterns
- Create/delete directories
- File system resources

**Tools:**
- `read_file` - Read file contents
- `write_file` - Write content to a file
- `list_directory` - List directory contents
- `search_files` - Search for files matching patterns
- `create_directory` - Create directories
- `delete` - Delete files/directories

### 2. Git MCP Server
**Binary:** `git-mcp-server`

Provides Git repository operations including:
- Repository status and history
- Branch management
- Commit operations
- Diff viewing

**Tools:**
- `git_status` - Get repository status
- `git_log` - View commit history
- `git_diff` - Show differences
- `git_branches` - List branches
- `git_commit` - Create commits
- `git_add` - Stage files
- `git_checkout` - Switch branches
- `git_show` - Show commit details

### 3. HTTP MCP Server
**Binary:** `http-mcp-server`

Provides HTTP client operations including:
- REST API calls (GET, POST, PUT, DELETE, HEAD, OPTIONS)
- URL parsing and analysis
- Header management
- Response handling

**Tools:**
- `http_get` - Perform GET requests
- `http_post` - Perform POST requests
- `http_put` - Perform PUT requests
- `http_delete` - Perform DELETE requests
- `http_head` - Get headers only
- `http_options` - Check available methods
- `parse_url` - Parse and analyze URLs

### 4. System MCP Server
**Binary:** `system-mcp-server`

Provides system utilities including:
- Process management
- System information
- Command execution
- Service status checking

**Tools:**
- `exec_command` - Execute shell commands
- `system_info` - Get system information
- `list_processes` - List running processes
- `kill_process` - Kill processes by PID
- `get_env` - Get environment variables
- `disk_usage` - Check disk usage
- `network_info` - Get network interface info
- `service_status` - Check service status

### 5. Time MCP Server
**Binary:** `time-mcp-server`

Provides time and date utilities including:
- Current time in various formats
- Timezone conversions
- Date parsing and formatting
- Duration calculations

**Tools:**
- `current_time` - Get current time
- `parse_time` - Parse time strings
- `convert_timezone` - Convert between timezones
- `add_duration` - Add duration to time
- `time_diff` - Calculate time differences
- `format_time` - Format time in various ways
- `list_timezones` - List available timezones
- `sleep` - Sleep for specified duration

### 6. Database MCP Server
**Binary:** `database-mcp-server`

Provides SQLite database operations including:
- SQL query execution
- Schema inspection
- Data export/import
- Database management

**Tools:**
- `sql_query` - Execute SQL queries
- `list_tables` - List database tables
- `describe_table` - Get table schema
- `create_database` - Create new database
- `execute_script` - Run SQL script files
- `export_table` - Export table to CSV
- `database_info` - Get database statistics

## Installation

### Prerequisites
- Go 1.21 or later
- Git (for git server functionality)

### Build All Servers
```bash
make build
```

### Build Individual Servers
```bash
make build-filesystem
make build-git
make build-http
make build-system
make build-time
make build-database
```

### Install to System PATH
```bash
make install
```

## Usage

### Running Servers Directly
Each server reads JSON-RPC messages from stdin and writes responses to stdout:

```bash
# Run filesystem server
./bin/filesystem-mcp-server

# Run git server
./bin/git-mcp-server

# Run HTTP server
./bin/http-mcp-server

# Run system server
./bin/system-mcp-server

# Run time server
./bin/time-mcp-server

# Run database server
./bin/database-mcp-server
```

### Configuration with MCP Clients

Add servers to your MCP client configuration. Example for Claude Desktop:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/path/to/bin/filesystem-mcp-server"
    },
    "git": {
      "command": "/path/to/bin/git-mcp-server"
    },
    "http": {
      "command": "/path/to/bin/http-mcp-server"
    },
    "system": {
      "command": "/path/to/bin/system-mcp-server"
    },
    "time": {
      "command": "/path/to/bin/time-mcp-server"
    },
    "database": {
      "command": "/path/to/bin/database-mcp-server"
    }
  }
}
```

## Examples

### Filesystem Operations
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"README.md"}}}' | ./bin/filesystem-mcp-server
```

### Git Operations
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"git_status","arguments":{}}}' | ./bin/git-mcp-server
```

### HTTP Requests
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"http_get","arguments":{"url":"https://httpbin.org/get"}}}' | ./bin/http-mcp-server
```

### System Information
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"system_info","arguments":{"detailed":true}}}' | ./bin/system-mcp-server
```

### Time Operations
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"current_time","arguments":{"timezone":"UTC","format":"RFC3339"}}}' | ./bin/time-mcp-server
```

### Database Operations
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sql_query","arguments":{"database_path":"test.db","query":"SELECT name FROM sqlite_master WHERE type='table'"}}}' | ./bin/database-mcp-server
```

## Development

### Project Structure
```
go-mcp-servers/
├── lib/mcpframework/          # Shared MCP framework
│   ├── types.go              # MCP protocol types
│   └── server.go             # Base server implementation
├── servers/                   # Individual server implementations
│   ├── filesystem/
│   ├── git/
│   ├── http/
│   ├── system/
│   └── time/
├── examples/                  # Usage examples
├── docs/                     # Documentation
└── bin/                      # Built binaries
```

### Adding New Servers
1. Create a new directory under `servers/`
2. Implement `main.go` using the `mcpframework` package
3. Register tools and resource handlers
4. Add build target to `Makefile`
5. Update this README

### Testing
```bash
make test
```

### Code Formatting
```bash
make fmt
```

### Linting (requires golangci-lint)
```bash
make lint
```

## Framework Features

The `mcpframework` package provides:
- JSON-RPC 2.0 message handling
- Tool registration and execution
- Resource management
- Error handling
- Logging support

### Basic Server Implementation
```go
package main

import (
    "context"
    "os"
    "github.com/tmc/misc/go-mcp-servers/lib/mcpframework"
)

func main() {
    server := mcpframework.NewServer("my-server", "1.0.0")
    server.SetInstructions("Description of what this server does")
    
    // Register tools
    server.RegisterTool("my_tool", "Tool description", schema, handler)
    
    // Run server
    ctx := context.Background()
    server.Run(ctx, os.Stdin, os.Stdout)
}
```

## License

MIT License - see individual server files for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Troubleshooting

### Common Issues

1. **Permission denied**: Ensure the binary has execute permissions
2. **Git operations fail**: Ensure git is installed and the directory is a git repository
3. **HTTP requests fail**: Check network connectivity and URL validity
4. **System commands fail**: Verify the command exists and permissions are correct

### Debug Mode
Most servers support verbose logging by setting the log level:

```go
server.SetLogger(log.New(os.Stderr, "[DEBUG] ", log.LstdFlags))
```

### Testing Individual Tools
You can test individual tools by sending JSON-RPC messages directly:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/filesystem-mcp-server
```