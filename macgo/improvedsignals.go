package macgo

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// DisableSignalHandling allows opting out of the default signal handling
// when necessary for compatibility with specific environments or requirements.
// By default, signal handling is enabled for better Ctrl+C and signal behavior.
var DisableSignalHandling = false

// DisableSignals disables the signal handling
// This can be used in specific environments where it might cause issues
func DisableSignals() {
	DisableSignalHandling = true
}

// For backward compatibility
func DisableRobustSignals() {
	DisableSignalHandling = true
}

// For backward compatibility
func EnableLegacySignalHandling() {
	DisableSignalHandling = true
}

// relaunchWithRobustSignalHandling relaunches the app with robust signal handling
// This approach is inspired by the Go tools implementation and works better
// in many scenarios, especially with Ctrl+C handling
func relaunchWithRobustSignalHandling(appPath, execPath string, args []string) {
	// Set environment to prevent relaunching again
	os.Setenv("MACGO_NO_RELAUNCH", "1")

	// Launch app bundle with more robust approach
	toolPath, err := exec.LookPath("open")
	if err != nil {
		debugf("error finding open command: %v", err)
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
	if err != nil {
		debugf("error starting app bundle: %v", err)
		return
	}

	// Set up robust signal handling
	c := make(chan os.Signal, 100)
	signal.Notify(c)
	go func() {
		for sig := range c {
			debugf("Forwarding signal %v to app bundle process group", sig)

			// Forward to entire process group using negative PID
			sigNum := sig.(syscall.Signal)

			// Skip SIGCHLD as we don't need to forward it
			if sigNum == syscall.SIGCHLD {
				continue
			}

			// Using negative PID sends to the entire process group
			if err := syscall.Kill(-toolCmd.Process.Pid, sigNum); err != nil {
				debugf("Error forwarding signal %v: %v", sigNum, err)
			}

			// Special handling for terminal signals
			if sigNum == syscall.SIGTSTP || sigNum == syscall.SIGTTIN || sigNum == syscall.SIGTTOU {
				// Use SIGSTOP for these terminal signals
				syscall.Kill(syscall.Getpid(), syscall.SIGSTOP)
			}
		}
	}()

	// Wait for process to finish and exit with its status code
	err = toolCmd.Wait()

	// Clean up signal handling
	signal.Stop(c)
	close(c)

	if err != nil {
		// Only print about the exit status if the command
		// didn't even run or it didn't exit cleanly
		if e, ok := err.(*exec.ExitError); !ok || !e.Exited() {
			debugf("error waiting for app bundle: %v", err)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	os.Exit(0)
}

// relaunchWithIORedirection relaunches the app with IO redirection through named pipes
// and uses robust signal handling by default
func relaunchWithIORedirection(appPath, execPath string) {
	// Create pipes for IO redirection
	pipes := make([]string, 3)
	for i, name := range []string{"stdin", "stdout", "stderr"} {
		pipe, err := createPipe("macgo-" + name)
		if err != nil {
			debugf("error creating %s pipe: %v", name, err)
			return
		}
		pipes[i] = pipe
		defer os.Remove(pipe)
	}

	// Prepare open command arguments
	args := []string{
		"-a", appPath,
		"--wait-apps",
		"--stdin", pipes[0],
		"--stdout", pipes[1],
		"--stderr", pipes[2],
	}

	// Set environment to prevent relaunching again
	os.Setenv("MACGO_NO_RELAUNCH", "1")

	// Pass original arguments
	if len(os.Args) > 1 {
		args = append(args, "--args")
		args = append(args, os.Args[1:]...)
	}

	// Launch app bundle with robust signal handling
	relaunchWithRobustSignalHandling(appPath, execPath, args)

	// Handle stdin
	go pipeIO(pipes[0], os.Stdin, nil)

	// Handle stdout
	go pipeIO(pipes[1], nil, os.Stdout)

	// Handle stderr
	go pipeIO(pipes[2], nil, os.Stderr)
}
