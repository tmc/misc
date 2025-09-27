// Package scripttestutil helps with script-based testing.
package scripttestutil

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"rsc.io/script"
)

// BackgroundCmd returns a command that runs prog in the background
// and sends SIGTERM on context cancellation for graceful shutdown.
func BackgroundCmd(prog string) script.Cmd {
	return script.Command(
		script.CmdUsage{Summary: "run " + prog, Async: true},
		bgExec(prog),
	)
}

func bgExec(prog string) func(*script.State, ...string) (script.WaitFunc, error) {
	return func(s *script.State, args ...string) (script.WaitFunc, error) {
		cmd := exec.Command(prog, args...)
		cmd.Dir = s.Getwd()
		cmd.Env = s.Environ()

		// Ensure GOCOVERDIR is set
		if gcd := os.Getenv("GOCOVERDIR"); gcd != "" && cmd.Env != nil {
			found := false
			for _, e := range cmd.Env {
				if strings.HasPrefix(e, "GOCOVERDIR=") {
					found = true
					break
				}
			}
			if !found {
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+gcd)
			}
		}

		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			return nil, err
		}

		return func(s *script.State) (string, string, error) {
			select {
			case <-s.Context().Done():
				cmd.Process.Signal(syscall.SIGTERM)
				time.AfterFunc(2*time.Second, func() { cmd.Process.Kill() })
			default:
			}

			err := cmd.Wait()
			// Exit 0 is success even if context cancelled
			if err == context.Canceled && cmd.ProcessState.ExitCode() == 0 {
				err = nil
			}
			return stdout.String(), stderr.String(), err
		}, nil
	}
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