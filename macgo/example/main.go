package main

import (
	"fmt"
	"os"

	// Import macgo with underscore to enable auto-relaunching as an app bundle on macOS.
	_ "github.com/tmc/misc/macgo"
)

func main() {
	fmt.Println("Hello from macgo example!")
	fmt.Println("  -> ", os.Executable())

	// Try to access a protected directory
	homeDir, _ := os.UserHomeDir()
	desktopDir := homeDir + "/Desktop"

	fmt.Println("Attempting to list files in", desktopDir)
	files, err := os.ReadDir(desktopDir)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Files on Desktop:")
	for _, file := range files {
		fmt.Println("-", file.Name())
	}

	// Getting other TCC-protected locations
	docDir := homeDir + "/Documents"
	fmt.Println("\nAttempting to list files in", docDir)
	docFiles, err := os.ReadDir(docDir)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Files in Documents (first 5):")
		count := 0
		for _, file := range docFiles {
			if count >= 5 {
				fmt.Println("- [and more...]")
				break
			}
			fmt.Println("-", file.Name())
			count++
		}
	}
}
