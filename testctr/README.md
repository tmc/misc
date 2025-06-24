# testctr - Zero-Dependency Test Containers for Go

A minimal, zero-dependency library for container-based testing in Go. Uses Docker CLI directly for simplicity and speed.

[![Go Reference](https://pkg.go.dev/badge/github.com/tmc/misc/testctr.svg)](https://pkg.go.dev/github.com/tmc/misc/testctr)
[![Test Coverage](https://img.shields.io/badge/coverage-59.9%25-yellow.svg)](https://github.com/tmc/misc/testctr)
[![Go Report Card](https://goreportcard.com/badge/github.com/tmc/misc/testctr)](https://goreportcard.com/report/github.com/tmc/misc/testctr)

## Features

- **Zero Dependencies** - Uses Docker CLI, no external packages for the core functionality.
- **Simple API** - One line to create a container with automatic cleanup.
- **Database Support** - Built-in DSN generation with per-test isolation.
- **Parallel Testing** - Full support for `t.Parallel()` with coordination (118% adoption rate in tests!).
- **Multiple Runtimes** - Docker, Podman, nerdctl, Finch, Colima, Lima (via CLI).
- **Fast Startup** - No API overhead, direct CLI execution for default mode.
- **Debugging Support** - Keep failed containers with `-testctr.keep-failed`.
- **File Copying** - Easy file/content injection into containers (supports files, readers, and byte content).
- **Pluggable Backends** - Optional support for other container libraries like Testcontainers-Go.
- **Comprehensive Testing** - 59.9% test coverage with dedicated test suites for all major features.

## Installation

```bash
go get github.com/tmc/misc/testctr
```

## Quick Start

```go
import "github.com/tmc/misc/testctr"

func TestMyApp(t *testing.T) {
    // Create a Redis container
    redis := testctr.New(t, "redis:7-alpine")
    
    // Get connection details
    addr := redis.Endpoint("6379") // "127.0.0.1:32768" (example port)
    
    // Execute commands
    output := redis.ExecSimple("redis-cli", "PING")
    // output: "PONG"
}
```

## Database Testing with Modules

testctr provides automatic per-test database isolation. Service-specific defaults and options are available in `ctropts/<service>` packages.

```go
import (
    "database/sql"
    "testing"
    "github.com/tmc/misc/testctr"
    "github.com/tmc/misc/testctr/ctropts/mysql"    // For MySQL defaults
    "github.com/tmc/misc/testctr/ctropts/postgres" // For PostgreSQL defaults
)

func TestMySQL(t *testing.T) {
    // Uses MySQL defaults (port, wait strategy, DSN provider)
    mysqlC := testctr.New(t, "mysql:8", mysql.Default()) 
    
    // Each test gets its own database, cleaned up automatically
    dsn := mysqlC.DSN(t) // e.g., "root:test@tcp(127.0.0.1:32768)/testmysql_testmysql?..."
    
    db, _ := sql.Open("mysql", dsn)
    // Test with isolated database
}

func TestPostgreSQL(t *testing.T) {
    pgC := testctr.New(t, "postgres:15", postgres.Default())
    dsn := pgC.DSN(t) // e.g., "postgresql://postgres@127.0.0.1:32769/testpostgresql_testpostgresql?sslmode=disable"
}
```

## Parallel Testing

All containers are safe for parallel tests:

```go
func TestParallel(t *testing.T) {
    t.Parallel() // Safe!
    
    mysqlC := testctr.New(t, "mysql:8", mysql.Default())
    
    // Run parallel subtests with isolated databases
    t.Run("Test1", func(t *testing.T) {
        t.Parallel()
        dsn := mysqlC.DSN(t) // Gets unique database for TestParallel/Test1
    })
    
    t.Run("Test2", func(t *testing.T) {
        t.Parallel()
        dsn := mysqlC.DSN(t) // Gets unique database for TestParallel/Test2
    })
}
```

## Options

### Basic Options (from `testctr` package)

```go
// Environment variables
c := testctr.New(t, "myapp:latest",
    testctr.WithEnv("DEBUG", "true"),
    testctr.WithEnv("PORT", "8080"),
)

// Ports (maps to a random host port)
c := testctr.New(t, "nginx:alpine",
    testctr.WithPort("80"), // Exposes container port 80
    testctr.WithPort("443/udp"), // Can specify protocol
)

// Command
c := testctr.New(t, "alpine:latest",
    testctr.WithCommand("sh", "-c", "echo hello && sleep 10"),
)
```

### File Copying (from `testctr` package)

```go
import "strings"

// Copy a single file
c := testctr.New(t, "alpine:latest",
    testctr.WithFile("./config.json", "/app/config.json"),
)

// Copy with specific permissions
c := testctr.New(t, "alpine:latest",
    testctr.WithFileMode("./script.sh", "/app/script.sh", 0755),
)

// Copy from io.Reader
reader := strings.NewReader("Hello, World!")
c := testctr.New(t, "alpine:latest",
    testctr.WithFileReader(reader, "/app/message.txt"),
)

// Copy multiple files using FileEntry struct
c := testctr.New(t, "alpine:latest",
    testctr.WithFiles(
        testctr.FileEntry{Source: "./config.json", Target: "/etc/config.json"},
        testctr.FileEntry{Source: "./data.csv", Target: "/data/input.csv"},
    ),
)
```

### Advanced Options (from `ctropts` package)

```go
import (
    "time"
    "github.com/tmc/misc/testctr/ctropts"
)

c := testctr.New(t, "myapp:latest",
    // Mounts
    ctropts.WithBindMount("./config", "/app/config"),
    
    // Network
    ctropts.WithNetwork("host"),
    
    // Resources
    ctropts.WithMemoryLimit("512m"),
    
    // Wait strategies
    ctropts.WithWaitForLog("Server started", 30*time.Second),
    ctropts.WithWaitForExec([]string{"healthcheck"}, 10*time.Second),
    // ctropts.WithWaitForHTTP("/health", "8080", 200, 15*time.Second), // Example
    
    // Runtime
    ctropts.WithPodman(), // Or WithNerdctl(), WithFinch(), etc.
)
```

### Service-Specific Module Options (from `ctropts/<service>` packages)

Modules provide a `Default()` option and other specific configurations.

```go
// MySQL example
import "github.com/tmc/misc/testctr/ctropts/mysql"

c := testctr.New(t, "mysql:8",
    mysql.Default(), // Includes DSN support, wait strategies, common env vars
    mysql.WithPassword("secret"), // Override default password
    mysql.WithDatabase("myapp"),  // Override default database
    mysql.WithCharacterSet("utf8mb4"),
)

// PostgreSQL example
import "github.com/tmc/misc/testctr/ctropts/postgres"

c := testctr.New(t, "postgres:15",
    postgres.Default(),
    postgres.WithPassword("secret"),
    postgres.WithDatabase("myapp"),
    postgres.WithExtensions("uuid-ossp", "postgis"),
)
```

## Backend System

testctr uses a pluggable backend system. The default CLI backend uses docker/podman commands directly, providing zero dependencies and maximum compatibility.

```go
// Use a specific backend
c := testctr.New(t, "redis:7",
    testctr.WithBackend("local"),        // Default CLI backend
    // testctr.WithBackend("dockerclient"), // Docker client API backend
    // testctr.WithBackend("testcontainers"), // Testcontainers-go backend
)
```

## Command-Line Flags & Environment Variables

(Same as in your original GoDoc - this section is good)

## Container Runtimes (Default CLI Backend)

(Same as in your original GoDoc - this section is good)

## API Reference

### Container Methods (`*testctr.Container`)

```go
// Create container
c := testctr.New(t, "image:tag", opts...)

// Connection info
c.Host()               // Container host (usually "127.0.0.1")
c.Port("3306")         // Get mapped host port for a container port string (e.g., "3306" or "3306/tcp")
c.Endpoint("3306")     // Get "host:port" string for a container port
c.ID()                 // Container ID
c.Runtime()            // Runtime being used (e.g., "docker", "podman", or backend name like "testcontainers")

// Execute commands
exitCode, output, err := c.Exec(ctx, []string{"cmd", "arg"})
output := c.ExecSimple("cmd", "arg") // Panics on error, returns trimmed output

// Database support (if DSNProvider is configured, typically by module.Default())
dsn := c.DSN(t)        // Get database connection string for a test-specific database
```

### File Operations

```go
// Copy file from host
c := testctr.New(t, "alpine:latest",
    testctr.WithFile("./config.json", "/app/config.json"),
)

// Copy file with specific permissions
c := testctr.New(t, "alpine:latest",
    testctr.WithFileMode("./script.sh", "/app/script.sh", 0755),
)

// Copy from io.Reader
config := strings.NewReader(`{"key": "value"}`)
c := testctr.New(t, "alpine:latest",
    testctr.WithFileReader(config, "/app/config.json"),
)

// Copy multiple files
c := testctr.New(t, "alpine:latest",
    testctr.WithFiles(map[string]testctr.FileContent{
        "/app/config.json": {Content: []byte(`{"key": "value"}`)},
        "/app/script.sh":   {Content: []byte("#!/bin/sh\necho hello"), Mode: 0755},
    }),
)
```

### Option Helpers (`testctr` package)

(Same as in your original GoDoc - this section is good)

## Architecture

(Same as in your original GoDoc, perhaps with a note about the pluggable backend system)

## Comparison with testcontainers-go

(Same as in your original GoDoc - this section is good)

## Debugging

(Same as in your original GoDoc - this section is good)

## Examples

See the `testctr-tests/` directory for comprehensive examples.

## Contributing

(Same as in your original GoDoc - this section is good)

## License

MIT
