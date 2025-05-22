# LSP Server Implementations Demo

This repository contains multiple Language Server Protocol (LSP) implementations demonstrating how to build custom language servers in different languages.

## Directory Structure

- `/server/` - LSP server implementations
  - `bash-lsp-server.sh` - The basic LSP server implementation that returns dictionary words
  - `minimal-lsp-server.sh` - Simplified version focused on maximum reliability
  - `go-lsp-server/` - Go-based LSP server implementation using GLSP
    - `main.go` - Server initialization and configuration
    - `handlers.go` - LSP protocol handler implementations
  - `go-lsp-server.sh` - Launch script for the Go LSP server

- `/configs/` - Vim configuration files
  - `simple-vimrc` - A complete Vim configuration using vim-plug
  - `minimal-vimrc` - Absolutely minimal configuration for reliability
  - `go-lsp-vim-config` - Vim configuration for the Go LSP server

- `/scripts/` - Utility scripts
  - `run-minimal.sh` - Launch Vim with the minimal configuration
  - `direct-test.sh` - Test the LSP server directly without Vim
  - `test-minimal.sh` - Simplified test script for reliable testing
  - `setup-go-lsp.sh` - Setup script for building the Go LSP server
  - `test-go-lsp.sh` - Test script for the Go LSP server

- `/examples/` - Example files for testing
  - `test.txt` - Sample text file for completion testing

## Available Servers

### 1. Bash LSP Servers

Simple implementations in Bash that provide word completions:
- `bash-lsp-server.sh` - Returns dictionary words from `/usr/share/dict/words`
- `minimal-lsp-server.sh` - Returns hardcoded completions for maximum reliability

### 2. Go LSP Server

A more sophisticated implementation using the GLSP (Go Language Server Protocol) SDK:
- Built with the GLSP library for robust LSP support
- Demonstrates custom completions, diagnostics, and hover functionality
- Provides code snippets for common Go constructs
- Features TODO/FIXME detection in diagnostics

## How It Works

All servers implement the Language Server Protocol:
- Handle initialization requests
- Respond to completion requests
- Follow the JSON-RPC protocol with Content-Length headers
- Support stdio communication

## Getting Started

### Setting Up Go LSP Server

1. Build and setup the Go LSP server:
   ```bash
   ./scripts/setup-go-lsp.sh
   ```

2. Add the configuration to your Vim:
   ```vim
   source /path/to/lsp-misc/configs/go-lsp-vim-config
   ```

3. Update the path in the config file to point to your installation.

### Testing The Servers

For Bash servers:
```bash
cd scripts
./test-minimal.sh
```

For Go server:
```bash
./scripts/test-go-lsp.sh
```

### Using With Vim

1. For Bash servers with `.txt` files:
   ```bash
   ./scripts/run-minimal.sh examples/test.txt
   ```

2. For Go server, ensure vim-lsp is configured properly, then edit any `.go` file.

In Vim:
1. Go to insert mode
2. Press `Ctrl+Space` to trigger completions
3. Press `\l` (backslash followed by L) to check server status

### Go LSP Server Features

When editing Go files with the Go LSP server:
- **Completions**: Get suggestions for keywords and snippets
- **Diagnostics**: See TODO/FIXME markers and missing package declarations
- **Hover**: Get documentation on hover (K key)
- **Formatting**: Format documents (leader+f)
- **Signatures**: Get function signature help

### Troubleshooting

If you encounter issues:

1. Make servers executable:
   ```bash
   chmod +x server/*.sh
   chmod +x scripts/*.sh
   ```

2. Check the logs:
   - Bash servers: `/tmp/lsp-logs/`
   - Go server: Check console output when running directly

3. Test servers directly:
   ```bash
   # Bash server
   cd scripts
   ./direct-test.sh

   # Go server
   ./scripts/test-go-lsp.sh
   ```

4. For Go server build issues:
   ```bash
   cd server/go-lsp-server
   go mod download
   go build
   ```

5. Look for JSON-RPC protocol errors in the logs

## Implementation Details

### Bash Server Versions

1. **Basic (`bash-lsp-server.sh`)**
   - Returns dictionary words from `/usr/share/dict/words`
   - Full-featured but may have more complexity

2. **Minimal (`minimal-lsp-server.sh`)**
   - Returns just a few hardcoded word completions
   - Designed for maximum reliability
   - Extensive logging for troubleshooting

### Go LSP Server

The Go implementation uses:
- **GLSP SDK**: A comprehensive Go library for building LSP servers
- **Protocol 3.16**: Latest stable LSP specification
- **Modular design**: Separate handlers for different LSP methods
- **Features implemented**:
  - Text document synchronization
  - Completion with snippets
  - Diagnostics with custom rules
  - Hover information
  - Signature help
  - Document formatting (stub)
  - Code actions (stub)
  - Definition/References (stubs)

### Vim Configuration

The Vim configurations rely on these plugins:
- vim-lsp
- asyncomplete.vim
- asyncomplete-lsp.vim

These are expected to be in the `vim-plugins` directory.

## Extending the Servers

To add new features to the Go LSP server:

1. Add handler methods in `handlers.go`
2. Register them in `main.go`
3. Update server capabilities in the `initialize` function
4. Rebuild with `go build`

Example: Adding a new diagnostic rule:
- Edit `generateDiagnostics()` in `handlers.go`
- Add pattern matching logic
- Create appropriate `protocol.Diagnostic` entries