// Simple example with direct function calls
package main

import (
	"fmt"
	"os"

	// Import the package directly
	"github.com/tmc/misc/macgo"
)

func init() {
	// Request permissions with direct function calls
	macgo.SetCamera()
	macgo.SetMic()
	
	// You can also use SetAll() to request all permissions
	// macgo.SetAll()
	
	// Optional: customize app bundle
	macgo.LegacyConfig.Name = "MacGoSimple"
}

func main() {
	fmt.Println("MacGo Simple Example")
	fmt.Println("This app has camera and microphone permissions via direct function calls")
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