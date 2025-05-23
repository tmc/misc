# Sandboxed osascript Executor

This example demonstrates how to create a sandboxed application that can execute AppleScript commands through `osascript` while maintaining the proper entitlements and permissions.

## Features

- Runs `osascript` commands from within a sandboxed macOS app bundle
- Automatically detects and uses the name it was invoked with (works with symlinks)
- Forwards all command-line arguments to `osascript`
- Includes necessary AppleEvents entitlements for proper AppleScript functionality
- No dock icon (runs invisibly in the background)
- Preserves stdin/stdout/stderr redirection

## Usage

```bash
# Build the app
go build -o sandboxed-osascript

# Run a simple AppleScript
./sandboxed-osascript -e "tell app \"System Events\" to display dialog \"Hello from AppleScript!\""

# Use a timeout for scripts that might hang
./sandboxed-osascript --timeout 10s -e "tell app \"System Events\" to display dialog \"Hello\""

# Create symlinks with custom names
ln -s sandboxed-osascript my-custom-script
./my-custom-script -e "display notification \"Hello from a custom named script\""

# Pass script files
./sandboxed-osascript /path/to/script.scpt

# Use with heredocs
./sandboxed-osascript <<EOF
tell application "System Events"
    display dialog "Hello from heredoc script"
end tell
EOF
```

## Options

- `--timeout DURATION`: Set a timeout for script execution (e.g., `30s`, `1m`, `5m`)
  - This is useful for preventing scripts from hanging indefinitely
  - If the script doesn't complete within the specified time, it will be terminated

## Permissions

This application requests the following entitlements:

- App Sandbox (`com.apple.security.app-sandbox`)
- AppleEvents (`com.apple.security.automation.apple-events`)
- Network Client (`com.apple.security.network.client`)

## How It Works

1. When first run, macgo creates an app bundle based on the executable name
2. The app bundle includes the necessary entitlements for AppleScript execution
3. The process relaunches itself inside the app bundle to gain the required permissions
4. Inside the bundle, it executes the osascript command with the provided arguments
5. All standard I/O is properly redirected between processes

This approach allows for running AppleScript in the macOS sandbox while maintaining proper permissions.