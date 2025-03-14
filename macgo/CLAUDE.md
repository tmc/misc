# macgo Development Guidelines

## Project Overview
macgo provides tools to prepare Go binaries as macOS .app bundles with sandboxing and TCC request handling. It solves the problem of accessing protected resources in macOS by enabling Go applications to run within properly configured .app bundles.

## Build Commands
- Build all: `go build ./...`
- Run example: `go run ./examples/simple/main.go`
- Test all: `go test ./...`
- Test single package: `go test github.com/tmc/misc/macgo/[package]`
- Test single test: `go test -run TestName github.com/tmc/misc/macgo/[package]`
- Verbose test: `go test -v -run TestName github.com/tmc/misc/macgo/[package]`
- Format code: `gofmt -s -w .`
- Lint code: `go vet ./...`
- Build with debug: `MACGO_DEBUG=1 go run ./examples/simple/main.go`

## Architecture
macgo uses several strategies to achieve its goals:
1. **App Bundle Creation**: Creates a macOS .app bundle for a Go binary
2. **Entitlements Configuration**: Sets up sandbox and TCC entitlements
3. **Relaunch Mechanism**: Relaunches the Go binary inside the app bundle when needed
4. **Signal Handling**: Properly forwards signals (SIGINT, etc.) between processes
5. **IO Redirection**: Redirects stdin/stdout/stderr between parent and child processes

### Core Components
- **Config**: Central configuration object that controls bundle behavior
- **Entitlements**: Declarations for macOS security permissions
- **Bundle Creation**: Functions to create and manage app bundles
- **Relaunch Logic**: Code for relaunching in the bundle context
- **Signal Handling**: Code for propagating signals between processes

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
- Sandboxed by default with user-selected file read access
- No dock icon or menu bar entry by default (LSUIElement)
- Handle TCC permissions (camera, mic, location, contacts, etc.)
- Embed app bundles in Go binaries
- Self-extraction and relaunching
- Permission status checking
- Signal forwarding (including Ctrl+C handling)
- Stdin/stdout/stderr redirection

## Common Patterns

### Basic Usage (Minimal)
```go
import "github.com/tmc/misc/macgo"

func init() {
    // Request specific permissions
    macgo.RequestEntitlements(
        macgo.EntCamera,
        macgo.EntMicrophone
    )
    
    // Initialize macgo
    macgo.Start()
}
```

### Advanced Configuration
```go
import "github.com/tmc/misc/macgo"

func init() {
    // Create a custom configuration
    cfg := macgo.NewConfig()
    
    // Set application details
    cfg.ApplicationName = "MyCustomApp"
    cfg.BundleID = "com.example.mycustomapp"
    
    // Add specific entitlements
    cfg.RequestEntitlements(
        macgo.EntAppSandbox,
        macgo.EntCamera,
        macgo.EntUserSelectedReadOnly,
    )
    
    // Show in dock
    cfg.AddPlistEntry("LSUIElement", false)
    
    // Apply configuration
    macgo.Configure(cfg)
    
    // Enable debug logging
    macgo.EnableDebug()
    
    // Start macgo
    macgo.Start()
}
```

### Improved Signal Handling
For better signal propagation (especially Ctrl+C handling):

```go
import "github.com/tmc/misc/macgo"

func init() {
    // Enable improved signal handling (for better Ctrl+C handling)
    macgo.EnableImprovedSignalHandling()
    
    // Add your permissions
    macgo.RequestEntitlements(macgo.EntCamera)
    
    // Start macgo
    macgo.Start()
}
```

### Auto-Initialization Packages
For simplified imports, macgo offers auto-initialization packages:

```go
// Basic auto-initialization (no sandbox)
import _ "github.com/tmc/misc/macgo/auto"

// With app sandbox
import _ "github.com/tmc/misc/macgo/auto/sandbox"

// With app sandbox and user read access
import _ "github.com/tmc/misc/macgo/auto/sandbox/readonly"

// With improved signal handling (better Ctrl+C handling)
import _ "github.com/tmc/misc/macgo/auto/sandbox/signalhandler"
```

## Available Entitlements
macgo supports many macOS entitlements, including:

### TCC Permissions
- `macgo.EntCamera` - Camera access
- `macgo.EntMicrophone` - Microphone access
- `macgo.EntLocation` - Location services
- `macgo.EntAddressBook` - Contacts access
- `macgo.EntPhotos` - Photos library access
- `macgo.EntCalendars` - Calendar access
- `macgo.EntReminders` - Reminders access

### App Sandbox
- `macgo.EntAppSandbox` - Enable app sandbox
- `macgo.EntNetworkClient` - Outgoing network connections
- `macgo.EntNetworkServer` - Incoming network connections
- `macgo.EntUserSelectedReadOnly` - Read access to user-selected files
- `macgo.EntUserSelectedReadWrite` - Read/write access to user-selected files

### Hardware Access
- `macgo.EntBluetooth` - Bluetooth device access
- `macgo.EntUSB` - USB device access
- `macgo.EntAudioInput` - Audio input devices
- `macgo.EntPrint` - Printing services

## Debugging Tips
- Enable debug logging with `macgo.EnableDebug()` or set `MACGO_DEBUG=1`
- Look for log messages with the `[macgo]` prefix
- Check inside the app bundle at `~/go/bin/YourApp.app/` or in `/tmp` for temporary bundles
- Verify entitlements with `codesign -d --entitlements :- /path/to/YourApp.app`
- For stdin/stdout issues, check pipes creation and IO redirection
- For signal handling issues, use `macgo.EnableImprovedSignalHandling()`