package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Find the script in the same directory as the executable
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error determining executable path: %v\n", err)
		os.Exit(1)
	}

	scriptPath := filepath.Join(filepath.Dir(execPath), "ctx-src.sh")
	
	// Check if the script exists at the expected path
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// If not, try next to the current file (for development)
		scriptPath = filepath.Join(filepath.Dir(os.Args[0]), "ctx-src.sh")
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			// Last resort: use the GOPATH-derived location
			gopath := os.Getenv("GOPATH")
			if gopath == "" {
				gopath = filepath.Join(os.Getenv("HOME"), "go")
			}
			scriptPath = filepath.Join(gopath, "src", "github.com", "tmc", "misc", "ctx-plugins", "ctx-src", "ctx-src.sh")
			if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: ctx-src.sh script not found\n")
				os.Exit(1)
			}
		}
	}

	// Execute the script with any command-line arguments
	cmd := exec.Command("bash", scriptPath)
	cmd.Args = append(cmd.Args, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error executing script: %v\n", err)
		os.Exit(1)
	}
}