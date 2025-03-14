// Using go:embed with the LoadEntitlementsFromJSON API
// Run with: MACGO_DEBUG=1 go run main.go
package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	// Import the macgo package directly
	"github.com/tmc/misc/macgo"
)

// Define custom entitlements in a JSON file and embed it
//
//go:embed entitlements.json
var entitlementsData []byte

// Embed using the embed.FS type for multiple files
//
//go:embed *.json
var entitlementsFS embed.FS

func init() {
	// This example focuses on using go:embed with entitlements configuration

	// Use the LoadEntitlementsFromJSON API to load the embedded data
	fmt.Println("Loading entitlements from embedded JSON data")
	if err := macgo.LoadEntitlementsFromJSON(entitlementsData); err != nil {
		log.Fatalf("Failed to load entitlements: %v", err)
	}

	// Additional configuration
	macgo.SetAppName("MacGoEmbed")
	macgo.SetBundleID("com.example.macgo.embed")
	macgo.EnableDebug() // Enable debug output
}

func main() {
	fmt.Println("\nMacGo go:embed Example")
	fmt.Println("This example demonstrates how to use go:embed with macgo")
	fmt.Println("The key benefits of this approach:")
	fmt.Println("- Keep entitlements configuration in a separate JSON file")
	fmt.Println("- Embed the configuration directly in the binary")
	fmt.Println("- No code changes needed to update entitlements")
	fmt.Println("- Configuration can be selected at build time")
	fmt.Println()

	// Show what permissions are likely configured
	fmt.Println("Entitlements from JSON typically include:")
	fmt.Println("- App Sandbox")
	fmt.Println("- Network client/server")
	fmt.Println("- Camera and microphone access")
	fmt.Println("- Location and photos access")
	fmt.Println()

	// Try to read some directories
	home, _ := os.UserHomeDir()
	dirs := []string{
		home + "/Desktop",
		home + "/Pictures",
		home + "/Documents",
	}

	for _, dir := range dirs {
		fmt.Printf("Reading %s: ", dir)

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
		fmt.Println()
	}
}
