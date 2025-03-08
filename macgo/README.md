# macgo

Enables macOS App Sandbox and Hardened Runtime entitlements for Go command-line tools by auto-creating and launching app bundles.

## Overview

macOS requires applications to be packaged as `.app` bundles to access protected resources like Camera, Microphone, Files, etc. This package automatically:

1. Detects if your command is already running in an app bundle
2. If not, creates an app bundle with the requested entitlements and relaunches itself through it
3. Handles all I/O redirection using named pipes

## Usage

### Simplest Usage (Blank Import)

The simplest way to use macgo is with a blank import:

```go
package main

import (
    "fmt"
    "os"
    
    // Just import with blank identifier
    _ "github.com/tmc/misc/macgo"
)

// Configure with environment variables:
// MACGO_CAMERA=1 MACGO_MIC=1 go run main.go

func main() {
    // Your code here will have access to protected resources
    files, err := os.ReadDir("~/Desktop")
    // ...
}
```

### Direct Function Calls

For more control, import directly and use entitlement functions:

```go
package main

import (
    "fmt"
    "os"
    "github.com/tmc/misc/macgo"
)

func init() {
    // Request specific TCC permissions
    macgo.SetCamera()
    macgo.SetMic()
    
    // Set up App Sandbox with network access
    macgo.SetAppSandbox()
    macgo.SetNetworkClient()
    
    // Configure Hardened Runtime settings
    macgo.SetAllowJIT()
    macgo.SetDisableLibraryValidation()
    
    // Or request all TCC permissions at once
    // macgo.SetAllTCCPermissions()
    
    // Or set up all device access
    // macgo.SetAllDeviceAccess()
}

func main() {
    // Now your app has all the requested entitlements
}
```

### Advanced Configuration API

For complete control, use the configuration API:

```go
package main

import (
    "github.com/tmc/misc/macgo"
)

func init() {
    // Create a custom configuration
    cfg := macgo.NewConfig()
    
    // Set application details
    cfg.Name = "CustomApp"
    cfg.BundleID = "com.example.customapp" 
    
    // Request entitlements using the new API
    cfg.AddEntitlement(macgo.EntCamera)
    cfg.AddEntitlement(macgo.EntMicrophone)
    cfg.AddEntitlement(macgo.EntAppSandbox)
    cfg.AddEntitlement(macgo.EntNetworkClient)
    cfg.AddEntitlement(macgo.EntAllowJIT)
    
    // For backward compatibility, this also works:
    // cfg.AddPermission(macgo.PermCamera)
    
    // Add custom Info.plist entries
    cfg.AddPlistEntry("LSUIElement", true) // Make app run in background
    
    // Control app bundle behavior
    cfg.Relaunch = true    // Auto-relaunch (default)
    cfg.AppPath = "/custom/path/MyApp" // Custom bundle path
    
    // Apply configuration (must be called)
    macgo.Configure(cfg)
}
```

### Environment Variables

Configure macgo with environment variables:

```bash
# Configure the app name and bundle ID
MACGO_APP_NAME="MyApp" MACGO_BUNDLE_ID="com.example.myapp" ./myapp

# Enable specific permissions
MACGO_CAMERA=1 MACGO_MIC=1 MACGO_PHOTOS=1 ./myapp

# Customize app bundle location and behavior
MACGO_APP_PATH="/Applications/MyApp" MACGO_KEEP_TEMP=1 ./myapp

# Enable debugging
MACGO_DEBUG=1 ./myapp
```

## Available Entitlements

Entitlements can be requested in three ways:

1. **Direct Function Calls**:
   ```go
   // TCC Permissions
   macgo.SetCamera()
   macgo.SetMic()
   
   // App Sandbox
   macgo.SetAppSandbox()
   macgo.SetNetworkClient()
   
   // Hardened Runtime
   macgo.SetAllowJIT()
   macgo.SetDisableLibraryValidation()
   
   // Convenience functions
   macgo.SetAllTCCPermissions()  // Set all TCC permissions
   macgo.SetAllDeviceAccess()    // Set all device permissions
   macgo.SetAllNetworking()      // Set all networking permissions
   ```

2. **Config API**:
   ```go
   cfg := macgo.NewConfig()
   
   // Using new API
   cfg.AddEntitlement(macgo.EntCamera)
   cfg.AddEntitlement(macgo.EntAppSandbox)
   
   // Legacy API (still works)
   cfg.AddPermission(macgo.PermCamera)
   ```

3. **Environment Variables**:
   ```bash
   # TCC permissions
   MACGO_CAMERA=1 MACGO_MIC=1 ./myapp
   
   # App Sandbox
   MACGO_APP_SANDBOX=1 MACGO_NETWORK_CLIENT=1 ./myapp
   
   # Hardened Runtime
   MACGO_ALLOW_JIT=1 MACGO_DISABLE_LIBRARY_VALIDATION=1 ./myapp
   ```

### TCC Permissions

| Entitlement | Function | Constant | Environment Var |
|------------|----------|----------|----------------|
| Camera     | `SetCamera()`   | `EntCamera`   | `MACGO_CAMERA=1` |
| Microphone | `SetMic()`      | `EntMicrophone` | `MACGO_MIC=1` |
| Location   | `SetLocation()` | `EntLocation` | `MACGO_LOCATION=1` |
| Contacts   | `SetContacts()` | `EntAddressBook` | `MACGO_CONTACTS=1` |
| Photos     | `SetPhotos()`   | `EntPhotos`   | `MACGO_PHOTOS=1` |
| Calendar   | `SetCalendar()` | `EntCalendars` | `MACGO_CALENDAR=1` |
| Reminders  | `SetReminders()`| `PermReminders`| `MACGO_REMINDERS=1` |

### App Sandbox Entitlements

| Entitlement | Function | Constant | Environment Var |
|------------|----------|----------|----------------|
| App Sandbox | `SetAppSandbox()` | `EntAppSandbox` | `MACGO_APP_SANDBOX=1` |
| Outgoing Network | `SetNetworkClient()` | `EntNetworkClient` | `MACGO_NETWORK_CLIENT=1` |
| Incoming Network | `SetNetworkServer()` | `EntNetworkServer` | `MACGO_NETWORK_SERVER=1` |
| Bluetooth | `SetBluetooth()` | `EntBluetooth` | `MACGO_BLUETOOTH=1` |
| USB | `SetUSB()` | `EntUSB` | `MACGO_USB=1` |
| Audio Input | `SetAudioInput()` | `EntAudioInput` | `MACGO_AUDIO_INPUT=1` |
| Printing | `SetPrinting()` | `EntPrint` | `MACGO_PRINT=1` |

### File Access Entitlements

| Entitlement | Constant | Environment Var |
|------------|----------|----------------|
| User-Selected Files (Read) | `EntUserSelectedReadOnly` | `MACGO_USER_FILES_READ=1` |
| User-Selected Files (Write) | `EntUserSelectedReadWrite` | `MACGO_USER_FILES_WRITE=1` |
| Downloads Folder (Read) | `EntDownloadsReadOnly` | `MACGO_DOWNLOADS_READ=1` |
| Downloads Folder (Write) | `EntDownloadsReadWrite` | `MACGO_DOWNLOADS_WRITE=1` |
| Pictures Folder (Read) | `EntPicturesReadOnly` | `MACGO_PICTURES_READ=1` |
| Pictures Folder (Write) | `EntPicturesReadWrite` | `MACGO_PICTURES_WRITE=1` |
| Music Folder (Read) | `EntMusicReadOnly` | `MACGO_MUSIC_READ=1` |
| Music Folder (Write) | `EntMusicReadWrite` | `MACGO_MUSIC_WRITE=1` |
| Movies Folder (Read) | `EntMoviesReadOnly` | `MACGO_MOVIES_READ=1` |
| Movies Folder (Write) | `EntMoviesReadWrite` | `MACGO_MOVIES_WRITE=1` |

### Hardened Runtime Entitlements

| Entitlement | Function | Constant | Environment Var |
|------------|----------|----------|----------------|
| Allow JIT | `SetAllowJIT()` | `EntAllowJIT` | `MACGO_ALLOW_JIT=1` |
| Allow Unsigned Memory | `SetAllowUnsignedMemory()` | `EntAllowUnsignedExecutableMemory` | `MACGO_ALLOW_UNSIGNED_MEMORY=1` |
| Allow DYLD Env Vars | `SetAllowDyldEnvVars()` | `EntAllowDyldEnvVars` | `MACGO_ALLOW_DYLD_ENV=1` |
| Disable Library Validation | `SetDisableLibraryValidation()` | `EntDisableLibraryValidation` | `MACGO_DISABLE_LIBRARY_VALIDATION=1` |
| Disable Exec Page Protection | `SetDisableExecutablePageProtection()` | `EntDisableExecutablePageProtection` | `MACGO_DISABLE_EXEC_PAGE_PROTECTION=1` |
| Debugger | `SetDebugger()` | `EntDebugger` | `MACGO_DEBUGGER=1` |

## Features

- Works with both compiled binaries and `go run`
- Intelligent handling of temporary vs permanent app bundles
- SHA256 verification to update only when needed
- I/O redirection with named pipes
- Simple entitlement controls
- Uses `GOPATH/bin` for persistent storage
- Proper cleanup of temporary bundles
- Zero shell script dependency

## Environment Variables

### Configuration Variables
- `MACGO_APP_NAME`: Set the app bundle name
- `MACGO_BUNDLE_ID`: Set the bundle identifier
- `MACGO_APP_PATH`: Custom path for the app bundle
- `MACGO_NO_RELAUNCH=1`: Disable automatic relaunching
- `MACGO_KEEP_TEMP=1`: Keep temporary bundles (don't clean up)
- `MACGO_DEBUG=1`: Enable debug logging

### TCC Permission Variables
- `MACGO_CAMERA=1`: Enable camera access
- `MACGO_MIC=1`: Enable microphone access
- `MACGO_LOCATION=1`: Enable location services
- `MACGO_CONTACTS=1`: Enable contacts access
- `MACGO_PHOTOS=1`: Enable photos library access
- `MACGO_CALENDAR=1`: Enable calendar access
- `MACGO_REMINDERS=1`: Enable reminders access

### App Sandbox Variables
- `MACGO_APP_SANDBOX=1`: Enable App Sandbox
- `MACGO_NETWORK_CLIENT=1`: Enable outgoing network connections
- `MACGO_NETWORK_SERVER=1`: Enable incoming network connections
- `MACGO_BLUETOOTH=1`: Enable Bluetooth access
- `MACGO_USB=1`: Enable USB device access
- `MACGO_AUDIO_INPUT=1`: Enable audio input access
- `MACGO_PRINT=1`: Enable printing capabilities

### File Access Variables
- `MACGO_USER_FILES_READ=1`: Enable read access to user-selected files
- `MACGO_USER_FILES_WRITE=1`: Enable write access to user-selected files
- `MACGO_DOWNLOADS_READ=1`: Enable read access to Downloads folder
- `MACGO_DOWNLOADS_WRITE=1`: Enable write access to Downloads folder
- `MACGO_PICTURES_READ=1`: Enable read access to Pictures folder
- `MACGO_PICTURES_WRITE=1`: Enable write access to Pictures folder
- `MACGO_MUSIC_READ=1`: Enable read access to Music folder
- `MACGO_MUSIC_WRITE=1`: Enable write access to Music folder
- `MACGO_MOVIES_READ=1`: Enable read access to Movies folder
- `MACGO_MOVIES_WRITE=1`: Enable write access to Movies folder

### Hardened Runtime Variables
- `MACGO_ALLOW_JIT=1`: Enable JIT compilation
- `MACGO_ALLOW_UNSIGNED_MEMORY=1`: Allow unsigned executable memory
- `MACGO_ALLOW_DYLD_ENV=1`: Allow DYLD environment variables
- `MACGO_DISABLE_LIBRARY_VALIDATION=1`: Disable library validation
- `MACGO_DISABLE_EXEC_PAGE_PROTECTION=1`: Disable executable page protection
- `MACGO_DEBUGGER=1`: Enable debugger capabilities

## Examples

See the examples directory for complete examples:

- [Simple Example](examples/simple/main.go): Using direct function calls
- [Blank Import Example](examples/blank/main.go): Using environment variables  
- [Advanced Example](examples/advanced/main.go): Using the configuration API

## Design

- Auto-detects app bundles by looking for `.app/Contents/MacOS/[executable name]`
- Temporary binaries (from `go run`) get temporary bundles with cleanup
- Permanent binaries get persistent bundles in `GOPATH/bin`
- SHA256 checksums verify when bundle updates are needed
- Named pipes connect standard input/output/error

## Limitations

- macOS only (silently does nothing on other platforms)
- macOS permission prompts will appear for each protected resource
- For permanent binaries, `GOPATH/bin` must be writable