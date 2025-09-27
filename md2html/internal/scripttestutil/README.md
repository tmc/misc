# scripttestutil

Minimal utilities for script-based testing with graceful shutdown.

## Problem

The script framework sends SIGKILL to background processes, preventing:
- Graceful shutdown
- Coverage data collection
- Clean exit codes

## Solution

`BackgroundCmd` is a drop-in replacement for `script.Program` that sends SIGTERM instead of SIGKILL:

```go
// Basic usage - SIGTERM on cancel, no wait delay
engine.Cmds["myserver"] = scripttestutil.BackgroundCmd(exe, nil, 0)

// Custom shutdown signal with 2 second wait
engine.Cmds["myapp"] = scripttestutil.BackgroundCmd(exe, func(cmd *exec.Cmd) error {
    return cmd.Process.Signal(os.Interrupt)
}, 2*time.Second)

// TestMain helper for dual-mode execution
func TestMain(m *testing.M) {
    scripttestutil.TestMain(m, func() {
        // Run your main program
    })
}
```

### Parameters

- **prog**: Program to run (name, absolute path, or relative path)
- **cancel**: Optional cancellation function. If nil, sends SIGTERM. If provided, receives *exec.Cmd for custom logic.
- **waitDelay**: Time to wait after cancellation before forcing kill (passed to exec.Cmd.WaitDelay)

## Example

```go
package main

import (
    "testing"
    "github.com/tmc/misc/md2html/internal/scripttestutil"
    "rsc.io/script"
    "rsc.io/script/scripttest"
)

func TestMain(m *testing.M) {
    scripttestutil.TestMain(m, main)
}

func TestScripts(t *testing.T) {
    exe, _ := os.Executable()

    engine := script.NewEngine()
    engine.Cmds["server"] = scripttestutil.BackgroundCmd(exe, nil, 0)
    engine.Cmds["curl"] = script.Program("curl", nil, 0)

    env := []string{"PATH=" + os.Getenv("PATH")}

    scripttest.Test(t, context.Background(), engine, env, "testdata/*.txt")
}
```

### Key Differences from script.Program

| Aspect | script.Program | BackgroundCmd |
|--------|---------------|---------------|
| Default cancel behavior | Sends SIGKILL immediately | Sends SIGTERM for graceful shutdown |
| Exit code 0 handling | Error if context cancelled | Success even if context cancelled |
| Coverage collection | Lost (process killed) | Preserved (clean exit) |
| Resource cleanup | Not possible | Allowed via signal handlers |
| Use case | Short-lived commands | Long-running servers |