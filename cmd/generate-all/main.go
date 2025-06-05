package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // Limit concurrent operations

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.Name() == "go.mod" {
			dir := filepath.Dir(path)
			wg.Add(1)
			go func(d string) {
				defer wg.Done()
				sem <- struct{}{}        // Acquire semaphore
				defer func() { <-sem }() // Release semaphore

				fmt.Printf("Running go generate in %s...\n", d)
				cmd := exec.Command("go", "generate", "./...")
				cmd.Dir = d
				if output, err := cmd.CombinedOutput(); err != nil {
					fmt.Printf("Error in %s: %v\n%s\n", d, err, output)
				}
			}(dir)
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directories: %v\n", err)
		os.Exit(1)
	}

	wg.Wait()
	fmt.Println("Done!")
}