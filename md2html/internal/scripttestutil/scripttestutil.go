// Package scripttestutil helps with script-based testing.
package scripttestutil

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"rsc.io/script"
)

// BackgroundCmd returns a command that runs prog in the background
// with graceful shutdown support via SIGTERM instead of SIGKILL.
//
// The signature matches script.Program exactly for drop-in replacement.
//
// Parameters:
//   - prog: The program to run. Can be a program name (looked up in PATH),
//     an absolute path, or a relative path containing separators.
//   - cancel: Optional function called when the script's context is cancelled.
//     If nil, sends SIGTERM for graceful shutdown.
//     If provided, called with the *exec.Cmd to allow custom shutdown logic.
//   - waitDelay: Maximum time to wait for the program to exit after cancellation
//     before forcibly killing it. Passed to exec.Cmd.WaitDelay.
//
// Differences from script.Program:
//   - Default cancellation sends SIGTERM instead of SIGKILL, allowing:
//     * Graceful shutdown with cleanup
//     * Coverage data to be written
//     * Exit code 0 on clean shutdown
//   - Context cancellation with exit code 0 is treated as success, not error
//   - Ensures GOCOVERDIR environment variable is preserved for coverage
//
// Example:
//
//	// Drop-in replacement with graceful shutdown
//	engine.Cmds["myserver"] = scripttestutil.BackgroundCmd(exe, nil, 0)
//
//	// Custom shutdown signal
//	engine.Cmds["myapp"] = scripttestutil.BackgroundCmd(exe, func(cmd *exec.Cmd) error {
//	    return cmd.Process.Signal(os.Interrupt)
//	}, 2*time.Second)
func BackgroundCmd(prog string, cancel func(*exec.Cmd) error, waitDelay time.Duration) script.Cmd {
	return script.Command(
		script.CmdUsage{
			Summary: "run " + filepath.Base(prog),
			Async:   true,
		},
		func(s *script.State, args ...string) (script.WaitFunc, error) {
			// If prog is an absolute path or contains separators, use it directly.
			// Otherwise look it up in PATH.
			name := prog
			path := prog
			if !filepath.IsAbs(prog) && !strings.Contains(prog, string(filepath.Separator)) {
				var err error
				path, err = exec.LookPath(prog)
				if err != nil {
					return nil, err
				}
			}
			return startBackgroundCommand(s, name, path, args, cancel, waitDelay)
		},
	)
}

// startBackgroundCommand starts a command with graceful shutdown support.
// It follows the same pattern as rsc.io/script's startCommand but with key differences:
//   - Default cancel sends SIGTERM instead of SIGKILL for graceful shutdown
//   - Exit code 0 with context.Canceled is treated as success
//   - Ensures GOCOVERDIR is set for test coverage collection
//   - Handles ETXTBSY errors by retrying (executable still being written)
//
// This allows background servers to shut down cleanly, write coverage data,
// and exit successfully when the test ends.
func startBackgroundCommand(s *script.State, name, path string, args []string, cancel func(*exec.Cmd) error, waitDelay time.Duration) (script.WaitFunc, error) {
	var (
		cmd                  *exec.Cmd
		stdout, stderr strings.Builder
	)

	// Retry loop to handle ETXTBSY errors
	for {
		cmd = exec.CommandContext(s.Context(), path, args...)
		cmd.Args[0] = name
		cmd.Dir = s.Getwd()
		cmd.Env = s.Environ()
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.WaitDelay = waitDelay

		// Set cancel function - default to SIGTERM for graceful shutdown
		if cancel == nil {
			cmd.Cancel = func() error {
				if cmd.Process != nil {
					return cmd.Process.Signal(syscall.SIGTERM)
				}
				return nil
			}
		} else {
			cmd.Cancel = func() error { return cancel(cmd) }
		}

		// Ensure GOCOVERDIR is set for coverage collection
		if gcd := os.Getenv("GOCOVERDIR"); gcd != "" {
			found := false
			for i, e := range cmd.Env {
				if strings.HasPrefix(e, "GOCOVERDIR=") {
					cmd.Env[i] = "GOCOVERDIR=" + gcd
					found = true
					break
				}
			}
			if !found {
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+gcd)
			}
		}

		err := cmd.Start()
		if err == nil {
			break // Successfully started
		}
		if isETXTBSY(err) {
			// If the script just wrote the executable we're trying to run,
			// a fork+exec in another thread may be holding open the FD
			// that we used to write the executable (see https://go.dev/issue/22315).
			// Since the descriptor should have CLOEXEC set, the problem should
			// resolve as soon as the forked child reaches its exec call.
			// Keep retrying until that happens.
			continue
		}
		return nil, err
	}

	wait := func(s *script.State) (string, string, error) {
		err := cmd.Wait()
		// Treat exit code 0 as success even if context was cancelled
		if err == context.Canceled && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0 {
			err = nil
		}
		return stdout.String(), stderr.String(), err
	}
	return wait, nil
}

// TestMain runs m.Run() normally, or runs mainFunc if invoked without -test flags.
func TestMain(m interface{ Run() int }, mainFunc func()) {
	if !strings.HasSuffix(os.Args[0], ".test") {
		os.Exit(m.Run())
	}

	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.") {
			os.Exit(m.Run())
		}
	}

	mainFunc()
	os.Exit(0)
}

// isETXTBSY reports whether err is a "text file busy" error (ETXTBSY).
// This can occur on Unix systems when trying to execute a file that
// is still being written by another process.
func isETXTBSY(err error) bool {
	if runtime.GOOS == "windows" {
		return false // Windows doesn't have ETXTBSY
	}
	return errors.Is(err, syscall.ETXTBSY)
}
