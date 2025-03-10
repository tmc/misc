# MacGo Examples

This directory contains examples demonstrating different ways to use the macgo package.

## Simplified API Examples

### Minimal Usage

The simplest way to use macgo is with a blank import:

```go
import (
    _ "github.com/tmc/misc/macgo" // Enables the macOS app bundle wrapper
)
```

### Basic Entitlements

To specify entitlements in a single line:

```go
import "github.com/tmc/misc/macgo"

func init() {
    macgo.WithEntitlements(
        macgo.EntCamera,
        macgo.EntMicrophone,
        macgo.EntAppSandbox,
        "com.apple.security.virtualization", // Custom entitlements work too
    )
}
```

### App Configuration

Set app name and other properties:

```go
func init() {
    macgo.WithAppName("MyAwesomeApp")
    macgo.WithBundleID("com.example.myapp")
    macgo.WithOptions(macgo.OptKeepTemp, macgo.OptDebug)
}
```

### Embedded Entitlements

Use go:embed for configuration:

```go
//go:embed entitlements.json
var entitlementsData []byte

func init() {
    macgo.WithEntitlementsJSON(entitlementsData)
}
```

### Custom App Template

Embed and use a custom app template:

```go
//go:embed template/*
var appTemplate embed.FS

func init() {
    macgo.WithAppTemplate(appTemplate)
}
```

### Auto-Signing

Enable code signing for release builds:

```go
func init() {
    macgo.WithSigning("Developer ID Application: Your Name (XXXXXXXXXX)")
}
```

## Running the Examples

To run any example with debug output enabled:

```
MACGO_DEBUG=1 go run ./examples/minimal-api
```

See individual examples for more specific instructions.