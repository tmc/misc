// This example shows how to customize app behavior with environment variables.
package main

import (
	"fmt"
	"os"
	
	// Import all permissions with one import
	_ "github.com/tmc/misc/macgo/all"
)

// This example doesn't need init() as everything is configured
// through environment variables:
//
// Run with:
// MACGO_APP_NAME="CustomApp" MACGO_BUNDLE_ID="com.example.custom" MACGO_DEBUG=1 go run main.go

func main() {
	fmt.Println("macgo customization example")
	fmt.Println("This app is configured with environment variables")
	fmt.Println("Run with: MACGO_APP_NAME=\"CustomApp\" MACGO_BUNDLE_ID=\"com.example.custom\" MACGO_DEBUG=1 go run main.go")
	
	// Show environment variable configuration
	printEnvConfig("MACGO_APP_NAME")
	printEnvConfig("MACGO_BUNDLE_ID") 
	printEnvConfig("MACGO_APP_PATH")
	printEnvConfig("MACGO_DEBUG")
	printEnvConfig("MACGO_CAMERA")
	printEnvConfig("MACGO_MIC")
	printEnvConfig("MACGO_PHOTOS")
	printEnvConfig("MACGO_LOCATION")
	printEnvConfig("MACGO_CONTACTS")
	printEnvConfig("MACGO_CALENDAR")
	printEnvConfig("MACGO_REMINDERS")
	
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