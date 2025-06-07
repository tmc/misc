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
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/tmc/misc/macgo"
	// Import sandbox for initialization, even though we don't use it directly
	_ "github.com/tmc/misc/macgo/auto/sandbox"
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

	// Create pipes for IO redirection
	pipes := make([]string, 3)
	for i, name := range []string{"stdin", "stdout", "stderr"} {
		pipe, err := createPipe("macgo-" + name)
		if err != nil {
			macgo.Debug("error creating %s pipe: %v", name, err)
			return
		}
		pipes[i] = pipe
		defer os.Remove(pipe)
	}

	// We need to replace the args with our own, ignoring what was passed in
	// This ensures proper pipe IO redirection
	pipeArgs := []string{
		"-a", appPath,
		"--wait-apps",
		"--stdin", pipes[0],
		"--stdout", pipes[1],
		"--stderr", pipes[2],
	}

	// Pass original arguments
	if len(os.Args) > 1 {
		pipeArgs = append(pipeArgs, "--args")
		pipeArgs = append(pipeArgs, os.Args[1:]...)
	}

	// Launch app bundle with more robust approach
	toolPath, err := exec.LookPath("open")
	if err != nil {
		macgo.Debug("error finding open command: %v", err)
		return
	}

	toolCmd := &exec.Cmd{
		Path: toolPath,
		Args: append([]string{toolPath}, pipeArgs...),
		SysProcAttr: &syscall.SysProcAttr{
			Setpgid: true,
			Pgid:    0, // Use the parent's process group
		},
	}

	err = toolCmd.Start()
	if err != nil {
		macgo.Debug("error starting app bundle: %v", err)
		return
	}

	// Set up signal handling
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

	// Handle stdin
	go pipeIO(pipes[0], os.Stdin, nil)

	// Handle stdout
	go pipeIO(pipes[1], nil, os.Stdout)

	// Handle stderr
	go pipeIO(pipes[2], nil, os.Stderr)

	// Wait for process to finish
	err = toolCmd.Wait()

	// Clean up signal handling
	signal.Stop(c)
	close(c)

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

// createPipe creates a named pipe.
func createPipe(prefix string) (string, error) {
	tmp, err := os.CreateTemp("", prefix+"-*")
	if err != nil {
		return "", err
	}

	path := tmp.Name()
	tmp.Close()
	os.Remove(path)

	cmd := exec.Command("mkfifo", path)
	return path, cmd.Run()
}

// pipeIO copies data between a pipe and stdin/stdout/stderr.
func pipeIO(pipe string, in io.Reader, out io.Writer) {
	mode := os.O_RDONLY
	if in != nil {
		mode = os.O_WRONLY
	}

	f, err := os.OpenFile(pipe, mode, 0)
	if err != nil {
		macgo.Debug("error opening pipe: %v", err)
		return
	}
	defer f.Close()

	if in != nil {
		io.Copy(f, in)
	} else {
		io.Copy(out, f)
	}
}
