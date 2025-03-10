// Simple example using direct entitlements API (recommended approach)
package main

import (
	"fmt"
	"os"

	// Import macgo package directly
	"github.com/tmc/misc/macgo"
)

func init() {
	// Request permissions using the With* style API functions
	macgo.WithEntitlements(
		macgo.EntCamera,
		macgo.EntMicrophone
	)
	
	// Optional: customize app name and bundle ID
	macgo.WithAppName("MacGoSimple")
	macgo.WithBundleID("com.example.macgo.simple")
}

func main() {
	fmt.Println("MacGo Simple Example")
	fmt.Println("This app demonstrates the recommended API with WithEntitlements function")
	fmt.Println("Camera and microphone access is enabled")
	fmt.Println()
	
	// Show some protected directories
	homeDir, _ := os.UserHomeDir()
	dirs := []string{
		homeDir + "/Desktop",
		homeDir + "/Downloads",
	}
	
	for _, dir := range dirs {
		fmt.Printf("Reading %s: ", dir)
		files, err := os.ReadDir(dir)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		fmt.Printf("%d files found\n", len(files))
		for i, f := range files {
			if i >= 3 {
				fmt.Println("...")
				break
			}
			fmt.Printf("- %s\n", f.Name())
		}
		fmt.Println()
	}
}