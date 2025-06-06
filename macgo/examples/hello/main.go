package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tmc/misc/macgo"
	"github.com/tmc/misc/macgo/debug" // Using the dedicated debug package
)

func init() {
	// Initialize macgo's debug package early.
	// This reads env vars like MACGO_SIGNAL_DEBUG, MACGO_PPROF.
	debug.Init()

	// Enable macgo's internal debug logging (MACGO_DEBUG=1)
	macgo.EnableDebug()

	// Configure macgo (optional, defaults will be used if not set)
	macgo.SetAppName("HelloMacgoApp")
	macgo.SetBundleID("com.example.hellomacgo")
	// macgo.RequestEntitlements(macgo.EntCamera) // Example: if camera needed

	// Start macgo - creates bundle and relaunches if necessary
	macgo.Start()
}

func main() {
	fmt.Printf("Hello from macgo! PID: %d\n", os.Getpid())
	fmt.Printf("Running in app bundle: %t\n", macgo.IsInAppBundle())

	if os.Getenv("MACGO_DEBUG") == "1" {
		fmt.Println("macgo internal debug logging is enabled.")
	}
	if debug.IsTraceEnabled() {
		fmt.Println("macgo.debug signal tracing is enabled (check logs).")
	}
	if debug.IsPprofEnabled() {
		fmt.Println("macgo.debug pprof server for this app is enabled (check logs for port).")
	}

	fmt.Println("Application will run for 3 seconds then exit.")
	time.Sleep(3 * time.Second)

	fmt.Println("Exiting HelloMacgoApp.")
	// Clean up debug package resources if it was used for logging to file
	debug.Close()
}
