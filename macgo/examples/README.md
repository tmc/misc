# MacGo Examples

This directory contains examples demonstrating different ways to use the macgo package.

## Core Examples

### Minimal Usage
[minimal/main.go](minimal/main.go) - The simplest way to use macgo with a blank import of the auto package.

```go
import (
    _ "github.com/tmc/misc/macgo/auto" // --
)
```

### Simple Usage
[simple/main.go](simple/main.go) - A basic example showing file access with the auto package.

### Customization With Environment Variables
[customization/main.go](customization/main.go) - Configure macgo through environment variables.

```
MACGO_APP_NAME="CustomApp" MACGO_BUNDLE_ID="com.example.custom" MACGO_CAMERA=1 MACGO_MIC=1 MACGO_DEBUG=1 go run main.go
```

### Signal Handling
[signal-handling/main.go](signal-handling/main.go) - Legacy signal handling example (now redundant as robust signal handling is enabled by default).

```go
import (
    _ "github.com/tmc/misc/macgo/auto/sandbox/signalhandler" // --
)
```

### Default Signal Handling Test
[signal-test/main.go](signal-test/main.go) - Tests the default robust signal handling (no special imports needed).

### Legacy Signal Handling
[legacy-signals/main.go](legacy-signals/main.go) - Demonstrates how to opt out of robust signal handling.

```go
func init() {
    // Opt out of robust signal handling
    macgo.DisableRobustSignals()
}
```

## Advanced Examples

### Advanced Configuration
[advanced/main.go](advanced/main.go) - Full control using the Config API.

```go
cfg := macgo.NewConfig()
cfg.ApplicationName = "AdvancedExampleApp"
cfg.AddEntitlement(macgo.EntCamera)
// ...and more
```

### Custom Template
[custom-template/main.go](custom-template/main.go) - Using a custom app template with embedded files.

### Entitlements From JSON
[entitlements/main.go](entitlements/main.go) - Loading entitlements from embedded JSON.

## Sandbox Examples

### Sandbox File and Execution Testing
[sandboxed-file-exec/main.go](sandboxed-file-exec/main.go) - Tests file access and process execution in sandbox.

```go
import (
    _ "github.com/tmc/misc/macgo/auto/sandbox" // --
)
```

### Sandbox Best Practices
[sandbox-best-practices/main.go](sandbox-best-practices/main.go) - Demonstrates recommended patterns for working with App Sandbox.

### Specific Folder Access
[specific-folder-access/main.go](specific-folder-access/main.go) - Shows how to access standard folders with specific entitlements.

```go
cfg.RequestEntitlements(
    macgo.EntDownloadsReadOnly,
    macgo.EntPicturesReadOnly,
    // ...and more
)
```

### Security-Scoped Bookmarks
[security-bookmarks/main.go](security-bookmarks/main.go) - How to use security-scoped bookmarks for persistent file access.

## Auto-Initialization Packages

Macgo provides several auto-initialization packages for different use cases:

- `github.com/tmc/misc/macgo/auto` - Basic auto-initialization (default settings, includes robust signal handling)
- `github.com/tmc/misc/macgo/auto/sandbox` - Auto-initialization with app sandbox
- `github.com/tmc/misc/macgo/auto/sandbox/readonly` - App sandbox with user read access
- `github.com/tmc/misc/macgo/auto/sandbox/signalhandler` - (Legacy) Kept for backward compatibility, but redundant as robust signal handling is now enabled by default

## Running the Examples

To run any example with debug output enabled:

```
MACGO_DEBUG=1 go run ./examples/minimal/main.go
```

### Testing Sandbox Permissions

When testing sandbox permissions, keep these points in mind:

1. The App Sandbox restricts access to most filesystem locations
2. User-selected files can be accessed with EntUserSelectedReadOnly
3. Standard folders need specific entitlements (Downloads, Pictures, etc.)
4. Temporary directories are always accessible
5. Network access works by default in Go (bypasses sandbox restrictions)
6. Process execution may be restricted based on the process