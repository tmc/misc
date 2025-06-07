// Signal Debugging Utility
// This example helps diagnose signal handling and forwarding issues
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tmc/misc/macgo"
)

func init() {
	// Set up basic configuration
	macgo.SetAppName("SignalDebugApp")
	macgo.SetBundleID("com.example.macgo.signaldebug")

	// Enable improved signal handling for testing
	macgo.EnableImprovedSignalHandling()

	// Enable debug output to see what's happening
	macgo.EnableDebug()
}

func main() {
	// Check if we're in child mode
	if len(os.Args) > 1 && os.Args[1] == "child" {
		runChild()
		return
	}

	// Start macgo - this creates the app bundle and relaunches if needed
	macgo.Start()

	fmt.Println("Signal Debugging Utility")
	fmt.Println("=======================")
	fmt.Println("This utility helps diagnose signal handling issues")
	fmt.Println("Parent PID:", os.Getpid())
	fmt.Println()

	// Set up a signal monitor to log all signals received
	allSignals := make(chan os.Signal, 100)
	signal.Notify(allSignals)
	go func() {
		for sig := range allSignals {
			fmt.Printf("[Parent] Received signal: %v\n", sig)
		}
	}()

	// Launch a child process
	cmd := exec.Command(os.Args[0], "child")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting child: %v\n", err)
		return
	}

	fmt.Printf("Started child process with PID: %d\n", cmd.Process.Pid)
	fmt.Println("Process hierarchy:")
	fmt.Printf("  - Parent (this process): PID %d\n", os.Getpid())
	fmt.Printf("  - Child process: PID %d\n", cmd.Process.Pid)
	fmt.Println()
	fmt.Println("Testing signal propagation...")
	fmt.Println("1. Press Ctrl+C to test SIGINT handling")
	fmt.Println("2. Run 'kill -TERM <parent-pid>' to test SIGTERM")
	fmt.Println("3. Run 'kill -STOP <parent-pid>' to test SIGTSTP")
	fmt.Println()

	// Wait for signals or timeout
	done := make(chan struct{})
	go func() {
		cmd.Wait()
		fmt.Println("Child process exited")
		close(done)
	}()

	// Run for 60 seconds or until done
	select {
	case <-done:
		fmt.Println("Child process terminated")
	case <-time.After(60 * time.Second):
		fmt.Println("Timeout reached, exiting")
	}
}

// Child process - just waits for signals
func runChild() {
	fmt.Printf("[Child] Started with PID: %d\n", os.Getpid())

	// Set up signal handling
	allSignals := make(chan os.Signal, 100)
	signal.Notify(allSignals)

	// Process signal info
	go func() {
		for sig := range allSignals {
			fmt.Printf("[Child] Received signal: %v\n", sig)

			// Exit on termination signals
			if sig == syscall.SIGINT || sig == syscall.SIGTERM {
				fmt.Println("[Child] Exiting due to termination signal")
				os.Exit(1)
			}
		}
	}()

	// Print process group info
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		fmt.Printf("[Child] Error getting PGID: %v\n", err)
	} else {
		fmt.Printf("[Child] Process Group ID: %d\n", pgid)
	}

	// Print parent process info
	ppid := os.Getppid()
	fmt.Printf("[Child] Parent PID: %d\n", ppid)

	// Run ps to show process hierarchy
	cmd := exec.Command("ps", "-o", "pid,ppid,pgid,command", "-p", fmt.Sprintf("%d,%d", os.Getpid(), ppid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[Child] Error running ps: %v\n", err)
	} else {
		fmt.Println("[Child] Process hierarchy:")
		fmt.Println(strings.TrimSpace(string(output)))
	}

	// Wait for signals until terminated
	fmt.Println("[Child] Waiting for signals...")
	select {}
}
