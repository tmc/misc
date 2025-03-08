// macgo example showing simple TCC permission access
// Run with: MACGO_DEBUG=1 go run main.go
package main

import (
	"fmt"
	"os"
	
	// Import macgo directly
	"github.com/tmc/misc/macgo"
)

func init() {
	// Request photos permission directly
	macgo.SetPhotos()
	
	// Could also use:
	// macgo.SetAll() // Request all permissions
	
	// Or with environment variables:
	// MACGO_PHOTOS=1 MACGO_CAMERA=1 go run main.go
}

func main() {
	fmt.Println("MacGo Test")
	
	// Try to read desktop files
	home, _ := os.UserHomeDir()
	dirs := []string{
		home + "/Desktop",
		home + "/Pictures",
		home + "/Documents",
	}
	
	for _, dir := range dirs {
		fmt.Printf("\nReading %s: ", dir)
		
		files, err := os.ReadDir(dir)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		
		fmt.Printf("%d files\n", len(files))
		// Show first few files
		for i, f := range files {
			if i >= 3 {
				fmt.Println("...")
				break
			}
			fmt.Printf("- %s\n", f.Name())
		}
	}
}