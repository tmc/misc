# macgo Development Guidelines

## Project Overview
macgo provides tools to prepare Go binaries as macOS .app bundles with sandboxing and TCC request handling. It solves the problem of accessing protected resources in macOS by enabling Go applications to run within properly configured .app bundles.

## Build Commands
- Build all: `go build ./...`
- Run example: `go run ./example/main.go`
- Test all: `go test ./...`
- Test single package: `go test github.com/tmc/misc/macgo/[package]`
- Test single test: `go test -run TestName github.com/tmc/misc/macgo/[package]`
- Verbose test: `go test -v -run TestName github.com/tmc/misc/macgo/[package]`
- Format code: `gofmt -s -w .`
- Lint code: `go vet ./...`
- Build with debug: `MACGO_DEBUG=1 go run ./example/main.go`

## Code Style
- Format with `gofmt -s -w .` before committing
- Imports order: standard library, external packages, local packages
- Type names: PascalCase for exported, camelCase for unexported
- Variable names: camelCase, descriptive, short for local scope
- Errors: always check, use early returns, wrap with `fmt.Errorf("package: %w", err)`
- Documentation: clear comments for all exported symbols
- Tests: table-driven with descriptive names
- Context: pass as first parameter for I/O operations
- Concurrency: protect shared state with sync.Mutex

## Key Features
- Bundle Go binaries as macOS .app bundles
- Handle TCC permissions (camera, mic, location, contacts, etc.)
- Embed app bundles in Go binaries
- Self-extraction and relaunching
- Permission status checking

## Usage Example
```go
import "github.com/tmc/misc/macgo"

func init() {
    // Request permissions
    macgo.SetCamera()
    macgo.SetMic()
    
    // Or all at once:
    // macgo.SetAll()
}
```