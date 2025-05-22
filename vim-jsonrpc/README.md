# vim-jsonrpc

A robust JSON-RPC 2.0 implementation for Go with Vim/Neovim integration support.

## Features

- **Complete JSON-RPC 2.0 implementation** - Full support for requests, responses, notifications, and error handling
- **Multiple transport layers** - stdio, TCP, and Unix socket support
- **Vim/Neovim integration** - Easy integration with Vim and Neovim plugins
- **Concurrent operation** - Thread-safe client and server implementations
- **Comprehensive testing** - Well-tested with extensive unit tests
- **Simple API** - Easy-to-use client and server APIs

## Installation

```bash
go get github.com/tmc/misc/vim-jsonrpc
```

## Quick Start

### Server Example

```go
package main

import (
    "context"
    "log"
    
    "github.com/tmc/misc/vim-jsonrpc/pkg/server"
)

func main() {
    s, err := server.New("stdio", "", "")
    if err != nil {
        log.Fatal(err)
    }
    
    // Register a simple echo handler
    s.RegisterHandler("echo", func(ctx context.Context, params interface{}) (interface{}, error) {
        return params, nil
    })
    
    // Start the server
    if err := s.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### Client Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/tmc/misc/vim-jsonrpc/pkg/client"
)

func main() {
    c, err := client.New("tcp", "localhost:8080", "")
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    if err := c.Connect(); err != nil {
        log.Fatal(err)
    }
    
    // Make a request
    result, err := c.Call(context.Background(), "echo", "Hello, World!")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Result: %v\n", result)
}
```

## Transport Types

### stdio
Perfect for Vim/Neovim plugins that communicate via standard input/output:

```go
server, err := server.New("stdio", "", "")
client, err := client.New("stdio", "", "")
```

### TCP
For network-based communication:

```go
server, err := server.New("tcp", "localhost:8080", "")
client, err := client.New("tcp", "localhost:8080", "")
```

### Unix Socket
For local inter-process communication:

```go
server, err := server.New("unix", "", "/tmp/vim-jsonrpc.sock")
client, err := client.New("unix", "", "/tmp/vim-jsonrpc.sock")
```

## Vim Integration

This library is designed to work seamlessly with Vim and Neovim. See the `examples/vim_plugin_example.vim` for a complete Vim plugin implementation.

### Basic Vim Plugin Setup

1. Start your Go JSON-RPC server
2. Use Vim's `job_start()` to communicate with the server
3. Send JSON-RPC messages using `ch_sendexpr()`

Example Vim function:
```vim
function! SendRequest(method, params)
    let request = {
        \ 'jsonrpc': '2.0',
        \ 'id': localtime(),
        \ 'method': a:method,
        \ 'params': a:params
        \ }
    call ch_sendexpr(g:job_channel, request)
endfunction
```

## API Reference

### Server

#### Creating a Server
```go
server, err := server.New(transport, addr, socket)
```

#### Registering Handlers
```go
server.RegisterHandler("method_name", func(ctx context.Context, params interface{}) (interface{}, error) {
    // Handle the request
    return result, nil
})
```

#### Starting the Server
```go
err := server.Start() // Blocks until server stops
```

### Client

#### Creating a Client
```go
client, err := client.New(transport, addr, socket)
```

#### Connecting
```go
err := client.Connect()
```

#### Making Requests
```go
result, err := client.Call(ctx, "method_name", params)
```

#### Sending Notifications
```go
err := client.Notify("method_name", params)
```

#### Handling Notifications
```go
client.OnNotification("method_name", func(params interface{}) {
    // Handle notification
})
```

## Examples

The `examples/` directory contains:

- `simple_server.go` - Basic JSON-RPC server with example handlers
- `simple_client.go` - Client example showing various call types
- `vim_plugin_example.vim` - Complete Vim plugin implementation

Run the examples:

```bash
# Terminal 1: Start server
go run examples/simple_server.go

# Terminal 2: Test with client
go run examples/simple_client.go
```

## Testing

Run the test suite:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

## JSON-RPC 2.0 Compliance

This implementation fully supports the JSON-RPC 2.0 specification:

- ✅ Request/Response pattern
- ✅ Notification support
- ✅ Batch requests
- ✅ Error handling with standard error codes
- ✅ Parameter passing (positional and named)

### Standard Error Codes

- `-32700` Parse error
- `-32600` Invalid Request
- `-32601` Method not found
- `-32602` Invalid params
- `-32603` Internal error

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Ensure all tests pass
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Related Projects

- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [Neovim JSON-RPC](https://neovim.io/doc/user/api.html)
- [Vim Jobs](https://vimhelp.org/eval.txt.html#job-control)