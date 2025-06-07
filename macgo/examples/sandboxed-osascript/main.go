package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tmc/misc/macgo"
)

func init() {
	// Create a custom configuration
	cfg := macgo.NewConfig()

	// Get the name of the executable
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		return
	}

	// Try to get the real path if it's a symlink
	realPath, err := filepath.EvalSymlinks(execPath)
	if err == nil {
		execPath = realPath
	}

	// Get the command name from os.Args[0], which preserves symlink names
	cmdName := filepath.Base(os.Args[0])
	cmdName = strings.TrimSuffix(cmdName, filepath.Ext(cmdName))

	// Use the command name for the app name
	appName := cmdName
	if appName == "" {
		// Fallback to executable name if args[0] doesn't work
		appName = filepath.Base(execPath)
		appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	}

	// Use the name for the app name
	cfg.ApplicationName = appName
	cfg.BundleID = "com.example." + strings.ToLower(appName)

	// Request necessary entitlements for osascript
	cfg.RequestEntitlements(
		macgo.EntAppSandbox,
		macgo.EntNetworkClient,
	)

	// Add AppleEvents entitlement manually - required for osascript
	cfg.AddEntitlement(macgo.Entitlement("com.apple.security.automation.apple-events"))

	// Don't show in dock
	cfg.AddPlistEntry("LSUIElement", true)

	// Apply the configuration
	macgo.Configure(cfg)

	// Enable debug logging if requested
	if os.Getenv("MACGO_DEBUG") == "1" {
		macgo.EnableDebug()
	}

	// Start macgo
	macgo.Start()
}

func main() {
	// Skip the program name (first argument)
	args := os.Args[1:]

	// Default timeout (0 means no timeout)
	var timeout time.Duration

	// Check for timeout flag
	for i := 0; i < len(args); i++ {
		if args[i] == "--timeout" && i+1 < len(args) {
			// Parse timeout value
			timeoutVal, err := time.ParseDuration(args[i+1])
			if err == nil {
				timeout = timeoutVal
				// Remove --timeout and its value from args
				args = append(args[:i], args[i+2:]...)
			}
			break
		}
	}

	// If no arguments provided, show usage
	if len(args) == 0 {
		fmt.Println("Usage: sandboxed-osascript [--timeout DURATION] [osascript arguments]")
		fmt.Println("\nOptions:")
		fmt.Println("  --timeout DURATION    Set a timeout for script execution (e.g. 30s, 1m)")
		fmt.Println("\nThis tool runs osascript commands from within a macOS app sandbox")
		fmt.Println("with the necessary permissions for AppleEvents.")
		return
	}

	// Set up a channel to handle signals
	sigChan := make(chan os.Signal, 1)
	fmt.Fprintf(os.Stderr, "[DEBUG] Setting up signal handlers for Interrupt and SIGTERM\n")
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGURG, syscall.SIGPIPE)

	// Prepare osascript command with all arguments
	fmt.Fprintf(os.Stderr, "[DEBUG] Creating command: osascript %v\n", args)
	cmd := exec.Command("osascript", args...)

	// Connect standard I/O
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the command
	fmt.Fprintf(os.Stderr, "[DEBUG] Starting osascript process\n")
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting osascript: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] osascript started with PID: %d\n", cmd.Process.Pid)

	// Create a channel for the command's exit
	done := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stderr, "[DEBUG] Waiting for osascript to complete\n")
		err := cmd.Wait()
		fmt.Fprintf(os.Stderr, "[DEBUG] osascript process completed with error: %v\n", err)
		done <- err
	}()

	// Create timeout channel if timeout is set
	var timeoutChan <-chan time.Time
	if timeout > 0 {
		fmt.Fprintf(os.Stderr, "[DEBUG] Setting timeout: %s\n", timeout)
		timeoutChan = time.After(timeout)
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] Entering select loop to wait for completion/signals/timeout\n")

	// Wait for command completion, signal, or timeout
	select {
	case err := <-done:
		// Command completed
		fmt.Fprintf(os.Stderr, "[DEBUG] Received completion notification\n")
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				// Exit with the same code as osascript
				exitCode := exitErr.ExitCode()
				fmt.Fprintf(os.Stderr, "[DEBUG] Command failed with exit code: %d\n", exitCode)
				os.Exit(exitCode)
			}

			// Otherwise exit with a generic error code
			fmt.Fprintf(os.Stderr, "Error running osascript: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[DEBUG] Command completed successfully\n")
	case sig := <-sigChan:
		// Received termination signal, pass it to the child process
		fmt.Fprintf(os.Stderr, "[DEBUG] Received signal: %v (type %T), forwarding to PID %d\n",
			sig, sig, cmd.Process.Pid)

		// Just log the PID and process status
		fmt.Fprintf(os.Stderr, "[DEBUG] Process PID: %d\n", cmd.Process.Pid)
		// We can't call Wait() here as it's already being called in the goroutine

		// Try to forward the signal
		if err := cmd.Process.Signal(sig); err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Failed to send signal to osascript: %v\n", err)
			fmt.Fprintf(os.Stderr, "[DEBUG] Attempting to kill process forcibly\n")
			if killErr := cmd.Process.Kill(); killErr != nil {
				fmt.Fprintf(os.Stderr, "[DEBUG] Kill failed: %v\n", killErr)
			} else {
				fmt.Fprintf(os.Stderr, "[DEBUG] Process killed successfully\n")
			}
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] Signal forwarded successfully\n")

			// Give the process a moment to handle the signal
			fmt.Fprintf(os.Stderr, "[DEBUG] Waiting briefly for process to handle signal\n")
			time.Sleep(500 * time.Millisecond)

			// We can't check process state here since we're already waiting in the goroutine
			// Just add a log message
			fmt.Fprintf(os.Stderr, "[DEBUG] Waiting for process to exit after signal\n")
		}

		fmt.Fprintf(os.Stderr, "[DEBUG] Exiting with code 1 due to signal\n")
		os.Exit(1)
	case <-timeoutChan:
		// Timeout reached, kill the process
		fmt.Fprintf(os.Stderr, "[DEBUG] Timeout of %s reached, terminating osascript (PID: %d)\n",
			timeout, cmd.Process.Pid)

		if killErr := cmd.Process.Kill(); killErr != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Kill failed: %v\n", killErr)
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] Process killed successfully\n")
		}

		fmt.Fprintf(os.Stderr, "[DEBUG] Exiting with code 1 due to timeout\n")
		os.Exit(1)
	}
}
