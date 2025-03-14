package main

import (
	"fmt"
	"os"

	// Import macgo package directly
	_ "github.com/tmc/misc/macgo/auto"
)

func main() {
	fmt.Println("MacGo Simple Example")
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
