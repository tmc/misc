# macgo Development Guidelines

## Project Overview
macgo provides tools to prepare Go binaries as macOS .app bundles with sandboxing and TCC request handling. It solves the problem of accessing protected resources in macOS by enabling Go applications to run within properly configured .app bundles.

### Key Features
- Bundle Go binaries as macOS .app bundles
- Handle TCC (Transparency, Consent, and Control) permissions
- Embed app bundles in Go binaries for single-binary distribution
- Self-extraction and relaunching from embedded app bundles
- Permission checking and status reporting

### Package Structure
- `/cmd/macgo`: The main CLI tool entry point
- `/cmd/testapp`: Test application for TCC permissions
- `/pkg/app`: Core .app bundle creation functionality
- `/pkg/tcc`: TCC permission handling and checking
- `/pkg/log`: macOS system log parsing for permissions
- `/pkg/embed`: App bundle embedding functionality
- `/examples`: Reference implementations

## Build Commands
- Build: `go build ./...`
- Run: `go run main.go`
- Test all: `go test ./...`
- Test single package: `go test github.com/tmc/misc/macgo/[package]`
- Test single test: `go test -run TestName github.com/tmc/misc/macgo/[package]`
- Format and lint: `gofmt -s -w . && go vet ./...`

## Common Usage Patterns

### Creating and Running App Bundles
```bash
# Build your Go binary
go build -o myapp ./cmd/myapp

# Create an app bundle (with camera and microphone permissions)
./macgo build --name=MyApp --bundle-id=com.example.myapp --entitlements=camera,microphone ./myapp

# Run the app bundle
open MyApp.app
```

### Creating and Embedding App Bundles
```bash
# Build your Go binary
go build -o myapp ./cmd/myapp

# Create an app bundle with embedding code generation
./macgo build --name=MyApp --bundle-id=com.example.myapp --embed --entitlements=camera,microphone ./myapp

# This generates myapp_embed.go with the embedded app bundle
```

### Self-Extracting and Relaunching Pattern
The pattern for a binary that can relaunch as an app bundle:

1. Check if running inside app bundle with `strings.Contains(execPath, ".app/Contents/MacOS")`
2. If not in app bundle, extract embedded app bundle or find existing one
3. Relaunch using `open -a /path/to/MyApp.app --args [original args]`
4. Fall back to plain binary if relaunching fails

### Testing TCC Permissions
```bash
# Check camera permission status for a specific bundle ID
./macgo test-tcc com.example.myapp camera
```

## Supported TCC Permissions
- `camera`: Camera access
- `microphone`: Microphone access
- `location`: Location services
- `contacts`: Contacts access
- `calendar`: Calendar access
- `photos`: Photo library access

## Code Style
- Format code with `gofmt -s -w .` before committing
- Imports order: standard library, external packages, local packages
- Use descriptive variable names (camelCase for variables, PascalCase for exported)
- Document all exported functions, types, and constants
- Error handling: always check errors, use early returns
- Consistent error messages: `fmt.Errorf("package: %w", err)`
- Follow Go standard practices with clean, minimal code
- Tests should use table-driven pattern with descriptive names
- Pass context.Context as first parameter for I/O operations
- Use sync.Mutex for protecting concurrent access