// This example shows how to customize app behavior with environment variables.
package main

import (
	"fmt"
	"os"

	// Blank import of macgo for environment variable configuration
	_ "github.com/tmc/misc/macgo"
)

// This example doesn't need init() as everything is configured
// through environment variables:
//
// Run with:
// MACGO_APP_NAME="CustomApp" MACGO_BUNDLE_ID="com.example.custom" MACGO_CAMERA=1 MACGO_MIC=1 MACGO_DEBUG=1 go run main.go

func main() {
	fmt.Println("macgo Environment Variables Example")
	fmt.Println("This app is configured entirely with environment variables")
	fmt.Println("Run with: MACGO_APP_NAME=\"CustomApp\" MACGO_BUNDLE_ID=\"com.example.custom\" MACGO_CAMERA=1 MACGO_MIC=1 MACGO_DEBUG=1 go run main.go")
	fmt.Println()

	// Show environment variable configuration
	printEnvConfig("MACGO_APP_NAME")
	printEnvConfig("MACGO_BUNDLE_ID")
	printEnvConfig("MACGO_DEBUG")

	// TCC Permission environment variables
	fmt.Println("\nTCC Permission Variables:")
	printEnvConfig("MACGO_CAMERA")
	printEnvConfig("MACGO_MIC")
	printEnvConfig("MACGO_PHOTOS")
	printEnvConfig("MACGO_LOCATION")
	printEnvConfig("MACGO_CONTACTS")
	printEnvConfig("MACGO_CALENDAR")
	printEnvConfig("MACGO_REMINDERS")

	// Advanced configuration variables
	fmt.Println("\nAdvanced Configuration Variables:")
	printEnvConfig("MACGO_KEEP_TEMP")
	printEnvConfig("MACGO_NO_RELAUNCH")
	printEnvConfig("MACGO_APP_PATH")
	printEnvConfig("MACGO_SIGN")
	printEnvConfig("MACGO_SIGN_IDENTITY")

	homeDir, _ := os.UserHomeDir()
	fmt.Printf("\nReading Desktop directory: %s/Desktop\n", homeDir)

	files, err := os.ReadDir(homeDir + "/Desktop")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d files\n", len(files))
	for i, file := range files {
		if i >= 5 {
			fmt.Println("...")
			break
		}
		fmt.Printf("- %s\n", file.Name())
	}
}

func printEnvConfig(name string) {
	value := os.Getenv(name)
	if value != "" {
		fmt.Printf("%s = %s\n", name, value)
	} else {
		fmt.Printf("%s not set\n", name)
	}
}
