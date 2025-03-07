# Deadcode Tool Development Guidelines

## Build Commands
- Build: `go build`
- Run: `go build && ./deadcode [args] ./...`
- Test all: `go test`
- Test specific: `go test -run TestName`
- Test verbose: `go test -v`
- Format code: `gofmt -s -w .`
- Lint: `go vet`

## Code Style
- Format with `gofmt -s -w .` before committing
- Imports: standard library first, then external packages, then local packages
- Naming: camelCase for private symbols, PascalCase for exported ones
- Error handling: check errors immediately, return early, provide context
- Comments: document exported functions, types, and non-obvious logic
- Structure: small functions with single responsibilities
- Types: explicit type declarations for public APIs
- Tests: use table-driven tests with clear test cases

## Tool Features
- Detect unused code elements including functions, types, interfaces, fields
- Configurable with CLI flags for different analysis types
- Debugging with `-debug` flag
- Custom output formats with templates
- Why-live analysis with `-whylive`