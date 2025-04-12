// Legacy Signal Handling Example
// This example demonstrates how to opt out of the default robust signal handling
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tmc/misc/macgo"
)

func init() {
	// Set up basic configuration
	macgo.SetAppName("LegacySignalsApp")
	macgo.SetBundleID("com.example.macgo.legacysignals")
	
	// Disable the robust signal handling
	// This is rarely needed, but demonstrates how to opt out
	macgo.DisableRobustSignals()
	
	// Enable debug output to see what's happening
	macgo.EnableDebug()
}

func main() {
	// Start macgo - this creates the app bundle and relaunches if needed
	// Legacy signal handling will be used due to DisableRobustSignals()
	macgo.Start()
	
	fmt.Println("Legacy Signal Handling Test")
	fmt.Println("==========================")
	fmt.Println("Press Ctrl+C to test signal handling")
	fmt.Println("Using legacy signal handling by request")
	fmt.Println()
	
	// Set up a channel to listen for signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	
	// Display a countdown
	go func() {
		for i := 30; i > 0; i-- {
			fmt.Printf("\rWaiting... %d seconds remaining", i)
			time.Sleep(1 * time.Second)
		}
		fmt.Println("\rTimeout reached, exiting normally        ")
	}()
	
	// Wait for a signal or timeout
	sig := <-c
	fmt.Printf("\n\nReceived signal: %v\n", sig)
	fmt.Println("Exiting gracefully")
}