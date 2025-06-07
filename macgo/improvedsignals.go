package macgo

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
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
	relaunchWithRobustSignalHandlingContext(context.Background(), appPath, execPath, args)
}

// relaunchWithRobustSignalHandlingContext relaunches the app with robust signal handling and context support
func relaunchWithRobustSignalHandlingContext(ctx context.Context, appPath, execPath string, args []string) {
	debugf("=== relaunchWithRobustSignalHandling START ===")
	debugf("appPath: %s", appPath)
	debugf("execPath: %s", execPath)
	debugf("args: %v", args)

	// Set environment to prevent relaunching again
	os.Setenv("MACGO_NO_RELAUNCH", "1")
	debugf("Set MACGO_NO_RELAUNCH=1")

	// Launch app bundle with more robust approach
	debugf("Looking for 'open' command")
	toolPath, err := exec.LookPath("open")
	if err != nil {
		debugf("error finding open command: %v", err)
		return
	}
	debugf("Found open command at: %s", toolPath)

	debugf("Creating command with args: %v", append([]string{toolPath}, args...))
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

	debugf("Starting command...")
	err = toolCmd.Start()
	if err != nil {
		debugf("error starting app bundle: %v", err)
		return
	}
	debugf("Command started successfully, PID: %d", toolCmd.Process.Pid)

	// Set up robust signal handling with context awareness
	debugf("Setting up signal handling...")
	c := make(chan os.Signal, 100)
	signal.Notify(c)
	go func() {
		for {
			select {
			case <-ctx.Done():
				debugf("Context cancelled, stopping signal forwarding")
				return
			case sig, ok := <-c:
				if !ok {
					return
				}
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
		}
	}()
	debugf("Signal handling setup complete")

	// Wait for process to finish and exit with its status code
	debugf("Waiting for command to finish...")

	// Add a timeout to detect hangs with context support
	done := make(chan error, 1)
	go func() {
		done <- toolCmd.Wait()
	}()

	select {
	case <-ctx.Done():
		debugf("Context cancelled, killing process...")
		toolCmd.Process.Kill()
		err = <-done
	case err = <-done:
		debugf("Command finished with error: %v", err)
	case <-time.After(5 * time.Second):
		debugf("TIMEOUT: Command hung for 5+ seconds, likely due to 'open' command issues")
		debugf("This is probably caused by the missing Xcode Platforms directory")
		debugf("Killing the hung process...")
		toolCmd.Process.Kill()
		err = <-done // wait for the kill to complete
		debugf("Process killed, error: %v", err)

		// Fall back to direct execution
		debugf("FALLBACK: Attempting direct execution of binary...")
		fallbackDirectExecutionContext(ctx, appPath, execPath)
	}

	// Clean up signal handling
	debugf("Cleaning up signal handling...")
	signal.Stop(c)
	close(c)

	if err != nil {
		debugf("Command exited with error: %v", err)
		// Only print about the exit status if the command
		// didn't even run or it didn't exit cleanly
		if e, ok := err.(*exec.ExitError); !ok || !e.Exited() {
			debugf("error waiting for app bundle: %v", err)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			debugf("Exiting with code: %d", exitErr.ExitCode())
			os.Exit(exitErr.ExitCode())
		}
		debugf("Exiting with code: 1")
		os.Exit(1)
	}

	debugf("Command completed successfully, exiting with code: 0")
	os.Exit(0)
}

// relaunchWithIORedirection relaunches the app with IO redirection through named pipes
// and uses robust signal handling by default
func relaunchWithIORedirection(appPath, execPath string) {
	relaunchWithIORedirectionContext(context.Background(), appPath, execPath)
}

// relaunchWithIORedirectionContext relaunches the app with IO redirection and context support
func relaunchWithIORedirectionContext(ctx context.Context, appPath, execPath string) {
	debugf("=== Starting relaunchWithIORedirection ===")
	debugf("appPath: %s", appPath)
	debugf("execPath: %s", execPath)
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

	// Launch app bundle with robust signal handling and context
	debugf("About to call relaunchWithRobustSignalHandlingContext")
	relaunchWithRobustSignalHandlingContext(ctx, appPath, execPath, args)
	debugf("Returned from relaunchWithRobustSignalHandlingContext")

	// Create debug log files for stdout/stderr if debug is enabled
	var stdoutTee, stderrTee io.Writer = os.Stdout, os.Stderr
	debugf("Setting up IO redirection (debug enabled: %t)", isDebugEnabled())
	if isDebugEnabled() {
		if stdoutFile, err := createDebugLogFile("stdout"); err == nil {
			stdoutTee = io.MultiWriter(os.Stdout, stdoutFile)
			defer stdoutFile.Close()
		} else {
			debugf("Failed to create stdout debug log: %v", err)
		}
		if stderrFile, err := createDebugLogFile("stderr"); err == nil {
			stderrTee = io.MultiWriter(os.Stderr, stderrFile)
			defer stderrFile.Close()
		} else {
			debugf("Failed to create stderr debug log: %v", err)
		}
	}

	// Handle stdin with context
	go pipeIOContext(ctx, pipes[0], os.Stdin, nil)

	// Handle stdout with context
	go pipeIOContext(ctx, pipes[1], nil, stdoutTee)

	// Handle stderr with context
	go pipeIOContext(ctx, pipes[2], nil, stderrTee)
}

// fallbackDirectExecution directly executes the binary when 'open' command fails
func fallbackDirectExecution(appPath, execPath string) {
	fallbackDirectExecutionContext(context.Background(), appPath, execPath)
}

// fallbackDirectExecutionContext directly executes the binary with context support
func fallbackDirectExecutionContext(ctx context.Context, appPath, execPath string) {
	debugf("=== FALLBACK DIRECT EXECUTION ===")

	// Find the actual executable in the app bundle
	bundleExecName := filepath.Base(execPath)
	bundleExecPath := filepath.Join(appPath, "Contents", "MacOS", bundleExecName)

	debugf("Looking for bundle executable at: %s", bundleExecPath)
	if _, err := os.Stat(bundleExecPath); err != nil {
		debugf("Bundle executable not found: %v", err)
		debugf("Falling back to original executable: %s", execPath)
		bundleExecPath = execPath
	}

	debugf("Executing directly: %s", bundleExecPath)

	// Set up the command with the same environment and context
	cmd := exec.CommandContext(ctx, bundleExecPath)
	cmd.Args = os.Args // Pass through original arguments
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "MACGO_NO_RELAUNCH=1") // Prevent recursive relaunching

	// Execute and replace current process
	debugf("Starting direct execution...")
	err := cmd.Run()
	if err != nil {
		debugf("Direct execution failed: %v", err)
		os.Exit(1)
	}

	debugf("Direct execution completed successfully")
	os.Exit(0)
}
