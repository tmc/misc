// Blank import example that uses environment variables for configuration
package main

import (
	"fmt"
	"os"

	// Import with blank identifier - still initializes the package
	_ "github.com/tmc/misc/macgo"
)

// This example doesn't use init() - all configuration is via environment variables
// Run with:
// MACGO_APP_NAME="MacGoBlank" MACGO_PHOTOS=1 MACGO_LOCATION=1 MACGO_DEBUG=1 go run main.go

func main() {
	fmt.Println("MacGo Blank Import Example")
	fmt.Println("This app uses environment variables for configuration")
	fmt.Println("Run with: MACGO_APP_NAME=\"MacGoBlank\" MACGO_PHOTOS=1 MACGO_LOCATION=1 MACGO_DEBUG=1 go run main.go")
	fmt.Println()
	
	// Print active environment variables
	printEnvVar("MACGO_APP_NAME")
	printEnvVar("MACGO_BUNDLE_ID")
	printEnvVar("MACGO_CAMERA")
	printEnvVar("MACGO_MIC")
	printEnvVar("MACGO_PHOTOS")
	printEnvVar("MACGO_LOCATION")
	printEnvVar("MACGO_CONTACTS")
	printEnvVar("MACGO_CALENDAR")
	printEnvVar("MACGO_REMINDERS")
	printEnvVar("MACGO_DEBUG")
	fmt.Println()
	
	// Show some protected directories
	homeDir, _ := os.UserHomeDir()
	dirs := []string{
		homeDir + "/Desktop",
		homeDir + "/Pictures",
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

func printEnvVar(name string) {
	value := os.Getenv(name)
	if value != "" {
		fmt.Printf("%s=%s\n", name, value)
	}
}