# macgo Signal & IO Handling Analysis

## Current Implementation Analysis (Pre-Refactor)

### Signal Handling Approach (Old `bundle.go relaunch`)

1.  **Basic Signal Forwarding**:
    *   Launches child with `open` command.
    *   Sets up a simple signal handler (`forwardSignals` in `signalforwarder.go`) to catch common signals.
    *   Forwards caught signals directly to the child process PID (not process group).
    *   Some special handling for terminal stop signals (`SIGTSTP`, etc.) by sending `SIGSTOP` to the parent.

### IO Redirection Approach (Old `bundle.go relaunch`)

1.  **Named Pipes**:
    *   Creates three named pipes (FIFOs) for stdin, stdout, and stderr.
    *   Uses system temp directory with unique names.
2.  **Process Launch**:
    *   Launches child with `open` command, passing pipe paths via `--stdin`, `--stdout`, `--stderr` arguments.
3.  **IO Goroutines**:
    *   Uses `pipeIO` in separate goroutines to copy data between application's stdio and the named pipes connected to the child.

## Improved Implementation (Post-Refactor - Default Behavior)

The refactoring aims to make the "improved" signal and IO handling (inspired by Go tools and previously in `improvedsignals.go` / `auto/sandbox/signalhandler`) the default.

### Signal Handling Approach (New Default - `relaunchWithIORedirectionContext` -> `relaunchWithRobustSignalHandlingContext`)

1.  **Process Group-Based Forwarding**:
    *   `open` command launched via `exec.Cmd`.
    *   `SysProcAttr.Setpgid = true` ensures the `open` command (and subsequently the app bundle it launches) is in a new process group.
    *   Signals are caught by the Go parent process.
    *   Signals are forwarded to the entire process group of the `open` command using `syscall.Kill(-toolCmd.Process.Pid, sigNum)`. This is more robust as it targets all processes spawned by `open` for the app.
2.  **Comprehensive Signal Capture**:
    *   `signal.Notify(c)` with a buffer (e.g., 100).
    *   Captures a wide range of signals.
3.  **Terminal Signal Handling**:
    *   Maintains special handling for terminal-related signals (`SIGTSTP`, `SIGTTIN`, `SIGTTOU`) by sending `SIGSTOP` to the Go parent process itself, allowing the shell to manage job control correctly.
4.  **Context-Aware Cleanup**:
    *   Uses `context.Context` for cancellation, allowing signal forwarding goroutines to terminate.
    *   `signal.Stop(c)` and `close(c)` are called after the child process exits.

### IO Redirection Approach (New Default - `relaunchWithIORedirectionContext`)

1.  **Named Pipes**:
    *   Same as before: creates three named pipes (FIFOs).
2.  **Process Launch**:
    *   `open` command is prepared with arguments `--stdin <pipe0>`, `--stdout <pipe1>`, `--stderr <pipe2>`.
3.  **IO Goroutines with Context**:
    *   `pipeIOContext` is used, which takes a `context.Context` for cancellation of IO operations. This prevents goroutines from leaking if the main process needs to shut down prematurely.
4.  **Debug Teeing**:
    *   If `MACGO_DEBUG=1`, stdout/stderr from the child process (read from pipes) are tee'd to both the parent's stdout/stderr and separate debug log files.

## Potential Issues & Considerations (with New Default)

1.  **`open` Command Hang / Xcode Configuration:**
    *   The `open` command can hang if the Xcode Command Line Tools or Xcode itself is not properly configured (specifically, missing `Platforms` directory).
    *   **Mitigation (in code):** `relaunchWithRobustSignalHandlingContext` includes a timeout for the `open` command and a fallback to direct execution if it hangs. `checkDeveloperEnvironment()` in `bundle.go` (called during `createBundle`) logs warnings if `MACGO_DEBUG=1`.
2.  **Named Pipe Cleanup:**
    *   Pipes are created in `/tmp` and `defer os.Remove(pipe)` is used in `relaunchWithIORedirectionContext`. This defer will execute when `relaunchWithIORedirectionContext` returns. Since this function calls `os.Exit()`, these defers might not run reliably.
    *   **Suggestion:** While temporary files in `/tmp` are often managed by the OS, explicitly schedule cleanup. The `createBundle` function already has a timed cleanup for temporary *bundles*. A similar approach or more robust tracking for pipes could be considered, or ensure the `defer` statements have a chance to run before `os.Exit`. *However, for named pipes, they only exist as filesystem entries; the data flow stops when processes close them. The `os.Remove` is for the filesystem entry.*
3.  **Signal Buffer Overflow**:
    *   `make(chan os.Signal, 100)` is generally sufficient but theoretically could overflow under extreme signal storms. This is a common pattern and usually not an issue.
4.  **Error Handling in IO Goroutines:**
    *   `pipeIOContext` logs errors via `debugf` but doesn't propagate them back in a way that would, for example, terminate the parent if a critical pipe breaks. For `stdout`/`stderr`, this is often acceptable. For `stdin`, a broken pipe might mean the child hangs.
    *   **Consideration:** For `stdin`, if `io.Copy` fails, it might be worth signaling the child or parent.

## Key Improvements from Refactor

*   **Default Robustness:** The more robust signal handling (process group forwarding) and IO redirection (named pipes with context-aware goroutines) are now the default.
*   **Context Propagation:** Better lifecycle management using `context.Context`.
*   **Fallback Mechanism:** `open` command hang detection and fallback to direct execution.
*   **Debuggability:** Enhanced debug logging for the relaunch process.
