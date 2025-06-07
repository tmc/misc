// Signal Handling Test Example
// This example demonstrates the improved signal handling with IO redirection in macgo
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
	macgo.SetAppName("SignalTestApp")
	macgo.SetBundleID("com.example.macgo.signaltest")

	// Enable improved signal handling with IO redirection
	// This provides better Ctrl+C handling and preserves stdin/stdout/stderr
	macgo.EnableImprovedSignalHandling()

	// Enable debug output to see what's happening
	macgo.EnableDebug()
}

func main() {
	// Start macgo - this creates the app bundle and relaunches if needed
	// Using the improved signal handling with IO redirection
	macgo.Start()

	fmt.Println("Signal Handling Test")
	fmt.Println("===================")
	fmt.Println("Press Ctrl+C to test signal handling")
	fmt.Println("The application should exit gracefully")
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
