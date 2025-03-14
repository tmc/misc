// Package signalhandler provides automatic initialization for macgo with app sandboxing
// and improved signal handling using the Go tool's signal handling pattern.
//
// Import this package to automatically set up app sandboxing on startup with improved signal handling:
//
//	import (
//	    _ "github.com/tmc/misc/macgo/auto/sandbox/signalhandler"
//	)
//
// This will automatically enable app sandboxing and create the app bundle with improved signal handling.
package signalhandler

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/tmc/misc/macgo"
)

func init() {
	// Replace the default relaunch function with our improved one
	macgo.SetReLaunchFunction(improvedRelaunch)
}

// Improved relaunch that uses the Go tool's signal handling pattern
// which is more robust than the previous approach
func improvedRelaunch(appPath, execPath string, args []string) {
	// Set environment to prevent relaunching again
	os.Setenv("MACGO_NO_RELAUNCH", "1")

	// Launch app bundle with more robust approach
	toolPath, err := exec.LookPath("open")
	if err != nil {
		macgo.Debug("error finding open command: %v", err)
		return
	}

	toolCmd := &exec.Cmd{
		Path:   toolPath,
		Args:   append([]string{toolPath}, args...),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		SysProcAttr: &syscall.SysProcAttr{
			Setpgid: true,
			Pgid:    0, // Use the parent's process group
		},
	}

	err = toolCmd.Start()
	if err == nil {
		c := make(chan os.Signal, 100)
		signal.Notify(c)
		go func() {
			for sig := range c {
				macgo.Debug("Forwarding signal %v to app bundle process group", sig)
				// Forward to entire process group using negative PID
				sigNum := sig.(syscall.Signal)

				// Skip SIGCHLD as we don't need to forward it
				if sigNum == syscall.SIGCHLD {
					continue
				}

				// Using negative PID sends to the entire process group
				if err := syscall.Kill(-toolCmd.Process.Pid, sigNum); err != nil {
					macgo.Debug("Error forwarding signal %v: %v", sigNum, err)
				}

				// Special handling for terminal signals
				if sigNum == syscall.SIGTSTP || sigNum == syscall.SIGTTIN || sigNum == syscall.SIGTTOU {
					// Use SIGSTOP for terminal signals
					syscall.Kill(syscall.Getpid(), syscall.SIGSTOP)
				}
			}
		}()
		err = toolCmd.Wait()
		signal.Stop(c)
		close(c)
	}

	if err != nil {
		// Only print about the exit status if the command
		// didn't even run or it didn't exit cleanly
		if e, ok := err.(*exec.ExitError); !ok || !e.Exited() {
			macgo.Debug("error waiting for app bundle: %v", err)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	os.Exit(0)
}
