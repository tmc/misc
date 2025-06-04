# testctr-dockerclient

A Docker client backend for testctr that uses the Docker Go SDK directly instead of CLI commands.

## Usage

Import this package to register the "dockerclient" backend:

```go
import (
    "testing"
    "github.com/tmc/misc/testctr"
    _ "github.com/tmc/misc/testctr/testctr-dockerclient" // Register backend
)

func TestWithDockerClient(t *testing.T) {
    container := testctr.New(t, "redis:7-alpine",
        testctr.WithBackend("dockerclient"),
        testctr.WithPort("6379"),
    )
    
    // Use container...
}
```

## Features

- Uses Docker Go SDK for all operations
- No shell command execution
- Full support for all testctr features:
  - Port mapping
  - Environment variables
  - File copying
  - Container inspection
  - Command execution
  - Log streaming

## Advantages over CLI backend

- More type-safe and structured error handling
- Better performance (no process spawning)
- Direct access to Docker API features
- Cleaner integration for Go applications

## Requirements

- Docker daemon must be accessible (via DOCKER_HOST or default socket)
- Same permissions as Docker CLI