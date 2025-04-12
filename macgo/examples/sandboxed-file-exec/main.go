package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	// Import sandboxed macgo package 
	_ "github.com/tmc/misc/macgo/auto/sandbox" // --
)

func main() {
	fmt.Println("MacGo Sandboxed Example: File Access and Package Execution")
	fmt.Println("-----------------------------------------------------------")
	fmt.Println()

	// Attempt to read files from home directory (will be blocked in sandbox)
	homeDir, _ := os.UserHomeDir()
	fmt.Printf("Home directory: %s\n", homeDir)
	
	// Try to read various home dir locations
	dirsToTry := []string{
		homeDir,
		filepath.Join(homeDir, "Documents"),
		filepath.Join(homeDir, "Desktop"),
		filepath.Join(homeDir, "Downloads"),
		"/tmp", // This might be accessible
	}

	fmt.Println("\n1. Attempting to access directories without permission:")
	fmt.Println("-----------------------------------------------------")
	for _, dir := range dirsToTry {
		fmt.Printf("Reading %s: ", dir)
		files, err := os.ReadDir(dir)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}

		fmt.Printf("SUCCESS! Found %d files\n", len(files))
		for i, f := range files {
			if i >= 3 {
				fmt.Println("...")
				break
			}
			fmt.Printf("  - %s\n", f.Name())
		}
		fmt.Println()
	}

	// Try to execute some commands
	fmt.Println("\n2. Attempting to execute commands:")
	fmt.Println("--------------------------------")
	cmdsToTry := []string{
		"ls",
		"echo Hello from subprocess",
		"whoami",
		"date",
	}

	for _, cmdStr := range cmdsToTry {
		fmt.Printf("Executing: %s - ", cmdStr)
		
		cmd := exec.Command("bash", "-c", cmdStr)
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		
		fmt.Printf("SUCCESS!\n")
		if len(output) > 100 {
			fmt.Printf("  Output: %s...\n", output[:100])
		} else {
			fmt.Printf("  Output: %s\n", output)
		}
	}

	fmt.Println("\nPress Enter to exit...")
	fmt.Scanln()
}