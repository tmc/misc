# testctr-testcontainers

This submodule provides a testcontainers-go backend implementation for testing purposes.

The main `testctr` package uses direct Docker commands and has zero dependencies. This submodule exists to verify that the same API concepts could work with testcontainers-go if desired.

## Usage

```go
package main

import (
    "testing"
    tc "github.com/tmc/misc/testctr/testctr-testcontainers"
)

func TestWithTestcontainers(t *testing.T) {
    c := tc.New(t, "redis:7-alpine")
    
    output, err := c.Exec("redis-cli", "PING")
    // ...
}
```

## Note

This is primarily for testing and demonstration purposes. The main package's zero-dependency approach using direct Docker commands is the recommended implementation.