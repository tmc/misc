// Package scripttestutil helps with script-based testing.
package scripttestutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"rsc.io/script"
)

// BackgroundCmd returns a command that runs prog in the background
// and sends SIGTERM on context cancellation for graceful shutdown.
// The signature matches script.Program for drop-in replacement.
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
// It follows the same pattern as script's startCommand but defaults to SIGTERM.
func startBackgroundCommand(s *script.State, name, path string, args []string, cancel func(*exec.Cmd) error, waitDelay time.Duration) (script.WaitFunc, error) {
	var stdout, stderr strings.Builder

	cmd := exec.CommandContext(s.Context(), path, args...)
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

	if err := cmd.Start(); err != nil {
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
