# parse-tc-module

A source-to-source transformation tool that parses testcontainers-go modules and generates equivalent testctr modules.

## Features

- **AST Parsing**: Analyzes Go source code to extract container configuration
- **Smart Extraction**: Identifies images, ports, environment variables, wait strategies
- **Option Analysis**: Parses With* functions and their effects
- **Complete Generation**: Creates fully functional testctr modules

## Usage

```bash
# Parse a module from the module cache
go run main.go -module mysql

# Parse from a specific path
go run main.go -module postgres -path /path/to/postgres/module

# Generate to a specific directory
go run main.go -module redis -out ./generated/redis

# Verbose output
go run main.go -module mongodb -v
```

## What it extracts

From testcontainers module source code:
- Default container images
- Exposed ports
- Environment variables
- Wait strategies (log, HTTP, exec)
- Configuration options (With* functions)
- Command and entrypoint settings
- Volume mounts and networks
- DSN support detection

## Output

Generates a complete testctr module with:
- Main module file with Default() and With* functions
- Documentation (doc.go)
- Test file
- Proper testctr API compatibility

## Example

```bash
go run main.go -module mysql -v
```

This will:
1. Find the mysql module in your Go module cache
2. Parse all Go files to extract configuration
3. Generate a testctr-compatible mysql module
4. Output to ./mysql/ directory