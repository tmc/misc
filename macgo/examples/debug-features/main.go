// Debug Features Example
// This example demonstrates how to use the advanced debugging features in macgo
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
	macgo.SetAppName("DebugFeaturesApp")
	macgo.SetBundleID("com.example.macgo.debugfeatures")
	
	// Enable improved signal handling with IO redirection
	macgo.EnableImprovedSignalHandling()
	
	// Enable debug output to see what's happening
	macgo.EnableDebug()
	
	// Demonstrate how to enable advanced debug features through environment variables
	// In a real application, these would be set before running the program
	// MACGO_SIGNAL_DEBUG=1 MACGO_PPROF=1 MACGO_DEBUG_LEVEL=2 go run ./examples/debug-features/main.go
	
	// For this example, we'll check and explain the environment variables
	fmt.Println("Debug Environment Variables:")
	fmt.Printf("  MACGO_DEBUG=%s (Basic debug logging)\n", getEnvOrDefault("MACGO_DEBUG", "0"))
	fmt.Printf("  MACGO_SIGNAL_DEBUG=%s (Detailed signal tracing)\n", getEnvOrDefault("MACGO_SIGNAL_DEBUG", "0"))
	fmt.Printf("  MACGO_DEBUG_LEVEL=%s (Debug verbosity level)\n", getEnvOrDefault("MACGO_DEBUG_LEVEL", "0"))
	fmt.Printf("  MACGO_PPROF=%s (Enable pprof HTTP server)\n", getEnvOrDefault("MACGO_PPROF", "0"))
	fmt.Printf("  MACGO_PPROF_PORT=%s (Base port for pprof server)\n", getEnvOrDefault("MACGO_PPROF_PORT", "6060"))
	fmt.Println()
	
	// Print debugging instructions
	fmt.Println("To run with debugging enabled:")
	fmt.Println("  MACGO_DEBUG=1 MACGO_SIGNAL_DEBUG=1 MACGO_PPROF=1 go run ./examples/debug-features/main.go")
	fmt.Println()
	
	// Print how to use the macgo-debug utility
	fmt.Println("To debug with macgo-debug utility:")
	fmt.Println("  1. Run this example with MACGO_DEBUG=1")
	fmt.Println("  2. In another terminal, run: go run ./cmd/macgo-debug/main.go --help")
	fmt.Println("  3. To monitor: go run ./cmd/macgo-debug/main.go --pid=<pid> --monitor")
	fmt.Println("  4. To send signals: go run ./cmd/macgo-debug/main.go --pid=<pid> --signal=INT")
	fmt.Println()
}

func main() {
	// Start macgo - this creates the app bundle and relaunches if needed
	macgo.Start()
	
	fmt.Println("Debug Features Example")
	fmt.Println("=====================")
	fmt.Printf("Process ID: %d\n", os.Getpid())
	
	if os.Getenv("MACGO_PPROF") == "1" {
		port := os.Getenv("MACGO_PPROF_PORT")
		if port == "" {
			port = "6060"
		}
		fmt.Printf("pprof server running at: http://localhost:%s/debug/pprof/\n", port)
		fmt.Printf("Process info: http://localhost:%s/macgo/info\n", port)
	}
	
	fmt.Println("Press Ctrl+C to test signal handling")
	fmt.Println()
	
	// Set up a channel to listen for signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	
	// Display a countdown
	go func() {
		for i := 60; i > 0; i-- {
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

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}