# generate-all

Generates all testctr modules from testcontainers-go modules.

## Usage

```bash
cd exp/cmd/generate-all
go run . -out ../../gen/modules -v
```

## Options

- `-out <dir>` - Output directory for generated modules (default: `./modules`)
- `-v` - Verbose output

## Generated Modules

This tool generates testctr-compatible modules for:

- **mysql** - MySQL database containers
- **postgres** - PostgreSQL database containers  
- **redis** - Redis cache containers
- **mongodb** - MongoDB database containers
- **qdrant** - Qdrant vector database containers

## Output Structure

Each generated module contains:

```
exp/gen/modules/
├── mysql/
│   ├── mysql.go      # Main module with Default() and With* functions
│   ├── doc.go        # Package documentation
│   └── mysql_test.go # Test cases
├── postgres/
│   ├── postgres.go
│   ├── doc.go
│   └── postgres_test.go
└── ...
```

## Usage Example

```go
import (
    "testing"
    "github.com/tmc/misc/testctr"
    "github.com/tmc/misc/testctr/exp/gen/modules/mysql"
)

func TestWithMySQL(t *testing.T) {
    container := testctr.New(t, "mysql:8.0.36", mysql.Default())
    // Use container...
}
```

## Parser Package

The `parser/` sub-package contains the module generation logic and can be imported by other tools.