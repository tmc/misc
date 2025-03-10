// This example shows the minimal usage of macgo with blank imports.
package main

import (
	"fmt"
	"os"

	// Core functionality with blank import (uses environment variables for configuration)
	_ "github.com/tmc/misc/macgo"
)

// Run with:
// MACGO_APP_NAME="MinimalApp" MACGO_CAMERA=1 go run main.go

func main() {
	fmt.Println("macgo minimal example")
	fmt.Println("This app has permissions via environment variables and blank import")
	fmt.Println("Run with: MACGO_APP_NAME=\"MinimalApp\" MACGO_CAMERA=1 go run main.go")

	homeDir, _ := os.UserHomeDir()
	fmt.Printf("Reading Desktop directory: %s/Desktop\n", homeDir)

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
