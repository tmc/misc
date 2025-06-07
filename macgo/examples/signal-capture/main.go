// Signal Capturing Utility
// This example is a simple utility that captures and logs signals it receives
package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	// Create a signal log file
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("macgo-signal-log-%d.txt", os.Getpid()))
	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Log startup info
	fmt.Fprintf(logFile, "Signal capture started at %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(logFile, "PID: %d\n", os.Getpid())
	fmt.Fprintf(logFile, "PPID: %d\n", os.Getppid())
	fmt.Fprintf(logFile, "Args: %v\n\n", os.Args)
	logFile.Sync()

	// Also output to console
	fmt.Println("Signal Capture Utility")
	fmt.Println("====================")
	fmt.Printf("PID: %d\n", os.Getpid())
	fmt.Printf("Logging signals to: %s\n", logPath)
	fmt.Println("Press Ctrl+C to test or use kill command")

	// Set up signal handling
	sigCh := make(chan os.Signal, 10)
	signal.Notify(sigCh,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGTSTP,
		syscall.SIGTTIN,
		syscall.SIGTTOU,
	)

	// Wait for signals
	for sig := range sigCh {
		now := time.Now().Format(time.RFC3339Nano)
		sigMessage := fmt.Sprintf("[%s] Received signal: %v\n", now, sig)

		// Log to file
		fmt.Fprint(logFile, sigMessage)
		logFile.Sync()

		// Output to console
		fmt.Print(sigMessage)

		// Exit on termination signals
		if sig == syscall.SIGINT || sig == syscall.SIGTERM {
			fmt.Fprintf(logFile, "[%s] Exiting due to signal: %v\n", now, sig)
			fmt.Printf("Exiting due to signal: %v\n", sig)
			break
		}
	}
}
