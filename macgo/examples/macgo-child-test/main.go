// This is a test utility meant to be run as a child process by macgo's relaunch mechanism.
// It logs its state to a file for inspection during tests or debugging.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tmc/misc/macgo"
	"github.com/tmc/misc/macgo/debug"
)

func init() {
	// Initialize debug package early (reads env vars)
	debug.Init()

	// Enable macgo's internal debug logs if MACGO_DEBUG is set
	if os.Getenv("MACGO_DEBUG") == "1" {
		macgo.EnableDebug()
	}
	// No macgo.Start() here, as this is intended to BE the child process.
	// If this were a standalone app that *uses* macgo, Start() would be here.
}

func main() {
	logFileName := fmt.Sprintf("macgo-child-process-test-pid%d.log", os.Getpid())
	logFilePath := filepath.Join(os.TempDir(), logFileName)

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fallback to stderr if log file fails
		fmt.Fprintf(os.Stderr, "CHILD_ERROR: Failed to create log file %s: %v\n", logFilePath, err)
		logFile = os.Stderr // Log to stderr as a fallback
	} else {
		defer logFile.Close()
		fmt.Fprintf(os.Stdout, "CHILD_INFO: Logging to %s\n", logFilePath) // Also to stdout for visibility
	}

	logMsg := func(format string, args ...interface{}) {
		fmt.Fprintf(logFile, "[%s] CHILD_LOG: ", time.Now().Format("15:04:05.000"))
		fmt.Fprintf(logFile, format, args...)
		fmt.Fprint(logFile, "\n")
		logFile.Sync() // Ensure logs are written immediately
	}

	logMsg("=== MACGO CHILD PROCESS STARTED ===")
	logMsg("PID: %d, PPID: %d", os.Getpid(), os.Getppid())
	logMsg("In app bundle: %t (determined by macgo.IsInAppBundle)", macgo.IsInAppBundle())
	logMsg("Args: %v", os.Args)
	logMsg("--- Environment Variables ---")
	for _, envVar := range []string{"MACGO_DEBUG", "MACGO_NO_RELAUNCH", "MACGO_APP_NAME", "MACGO_BUNDLE_ID", "MACGO_PPROF_PORT"} {
		logMsg("  %s: %s", envVar, os.Getenv(envVar))
	}
	logMsg("---------------------------")


	if wd, err := os.Getwd(); err == nil {
		logMsg("Working dir: %s", wd)
	} else {
		logMsg("Error getting working dir: %v", err)
	}

	// Also try stdout/stderr for direct capture by parent if pipes are working
	fmt.Fprintf(os.Stdout, "CHILD_STDOUT: Child process is running! PID: %d\n", os.Getpid())
	fmt.Fprintf(os.Stderr, "CHILD_STDERR: Child process is running! PID: %d\n", os.Getpid())

	logMsg("Simulating work for 2 seconds...")
	time.Sleep(2 * time.Second)

	logMsg("=== MACGO CHILD PROCESS ENDING ===")
	fmt.Fprintf(os.Stdout, "CHILD_STDOUT: Child process ending. PID: %d\n", os.Getpid())
	fmt.Fprintf(os.Stderr, "CHILD_STDERR: Child process ending. PID: %d\n", os.Getpid())

	// Clean up debug package resources if it was used for logging to file
	debug.Close()
}
