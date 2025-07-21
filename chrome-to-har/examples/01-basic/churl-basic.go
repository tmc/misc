// Basic churl usage example
// This example shows how to use churl for simple HTTP requests through Chrome
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run churl-basic.go <URL>")
		os.Exit(1)
	}

	url := os.Args[1]

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run churl command
	cmd := exec.CommandContext(ctx, "churl", url)
	output, err := cmd.Output()

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("HTML content from %s:\n", url)
	fmt.Print(string(output))
}
