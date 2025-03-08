// This example shows the minimal usage of macgo with blank imports.
package main

import (
	"fmt"
	"os"
	
	// Core functionality with blank import
	_ "github.com/tmc/misc/macgo"
	// Camera permission
	_ "github.com/tmc/misc/macgo/camera"
)

func main() {
	fmt.Println("macgo minimal example")
	fmt.Println("This app has camera permission via blank imports")
	
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