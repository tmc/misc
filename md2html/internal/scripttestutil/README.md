# scripttestutil

Minimal utilities for script-based testing with graceful shutdown.

## Problem

The script framework sends SIGKILL to background processes, preventing:
- Graceful shutdown
- Coverage data collection
- Clean exit codes

## Solution

```go
// In your test - matches script.Program signature exactly
engine.Cmds["myserver"] = scripttestutil.BackgroundCmd(exe, nil, 0)

// In your TestMain
func TestMain(m *testing.M) {
    scripttestutil.TestMain(m, func() {
        // Run your main program
    })
}
```

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

BackgroundCmd has the same signature as script.Program but sends SIGTERM instead of SIGKILL and treats exit code 0 as success.