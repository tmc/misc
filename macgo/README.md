# macgo

A simple Go package that automatically creates a macOS app bundle for your Go applications to gain permission access.

## Purpose

macOS uses the TCC (Transparency, Consent, and Control) framework to manage permissions for applications. Command-line applications typically have limited access to protected resources like camera, microphone, disk access, etc.

macgo provides a simple way to:
1. Detect if your app is running inside an app bundle
2. If not, create and use an app bundle automatically
3. Relaunch through the app bundle to gain proper permissions

## Usage

Just import the package with an underscore import:

```go
package main

import (
    "fmt"
    "os"
    
    // Import macgo for automatic app bundle handling
    _ "github.com/tmc/misc/macgo"
)

func main() {
    // Try to access a protected directory (will auto-relaunch if needed)
    files, err := os.ReadDir("~/Desktop")
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    
    fmt.Println("Files on Desktop:")
    for _, file := range files {
        fmt.Println("-", file.Name())
    }
}
```

That's it! The package will:
1. Check if it's running inside an app bundle
2. If not, create an app bundle and relaunch through it
3. The relaunched instance will have proper permissions

The app automatically detects when it's running within an app bundle by analyzing the executable path, so no manual configuration is needed to prevent infinite relaunching.

For debugging, you can enable verbose output by setting the `MACGO_DEBUG` environment variable:

```bash
MACGO_DEBUG=1 yourapp
```

## How It Works

1. When your program starts, macgo checks if its executable path matches the pattern `.app/Contents/MacOS/[executable name]`
2. If not, it creates an app bundle:
   - For compiled binaries: Creates a persistent app bundle in GOPATH/bin
   - For temporary binaries (go run): Creates a temporary app bundle that cleans up after use
3. It copies your binary into the app bundle
4. It creates named pipes (FIFOs) for stdin, stdout, and stderr
5. It relaunches your application through the app bundle using `open -a /path/to/YourApp.app --wait-apps --stdin pipe1 --stdout pipe2 --stderr pipe3 --args ...`
6. It sets up bidirectional communication through the named pipes
7. The original process exits, and the app bundle instance continues with proper permissions

## Features

- **Works with Go Run**: Now supports temporary binaries created by `go run`
- **SHA256 Verification**: Only updates the app bundle when the binary changes
- **Advanced I/O Redirection**: Uses named pipes for reliable stdin, stdout, and stderr handling
- **Persistent Storage**: Stores app bundles in GOPATH/bin for permanent binaries
- **Temporary Bundles**: Uses temporary bundles with cleanup for `go run`
- **Auto-Update**: Automatically updates the app bundle when the binary changes

## Behavior Details

### Permanent Binaries
When running a compiled binary (like from `go build`):
- Creates a persistent app bundle in GOPATH/bin
- Only recreates the bundle when the binary changes (using SHA256 verification)
- The app bundle remains for future use

### Temporary Binaries
When running with `go run` (which uses temporary binaries):
- Creates a temporary app bundle in the system temp directory
- Uses a unique name based on binary hash
- Automatically cleans up the temporary bundle after execution
- No persistent files are left behind

## Limitations

- Only works on macOS (Darwin) systems - silently does nothing on other platforms
- Will prompt the user for permissions as needed (this is controlled by macOS)
- For permanent binaries, GOPATH/bin directory must be writable

## License

MIT