// Advanced configuration example using the Config API
package main

import (
	"fmt"
	"os"

	// Import with direct reference for advanced configuration
	"github.com/tmc/misc/macgo"
)

func init() {
	// Create a custom configuration
	cfg := macgo.NewConfig()

	// Set application details
	cfg.ApplicationName = "MacGoAdvanced"
	cfg.BundleID = "com.example.macgo.advanced"

	// Add entitlements directly to the config
	cfg.Entitlements = macgo.Entitlements{
		macgo.EntCamera:     true,
		macgo.EntMicrophone: true,
		macgo.EntPhotos:     true,
		macgo.EntAppSandbox: true, // Enable app sandbox
	}

	// Add custom Info.plist entries
	cfg.AddPlistEntry("LSUIElement", false) // Show in Dock
	cfg.AddPlistEntry("CFBundleDisplayName", "MacGo Advanced Example")

	// Control app bundle behavior
	cfg.Relaunch = true  // Auto-relaunch (default)
	cfg.KeepTemp = false // Cleanup temp bundles (default)
	// cfg.CustomDestinationAppPath = "/custom/path/MyApp"  // Custom bundle path

	// Enable code signing
	cfg.AutoSign = true
	// cfg.SigningIdentity = "Developer ID Application: Your Name (XXXXXXXXXX)"

	// Apply the configuration (required)
	macgo.Configure(cfg)
}

func main() {
	fmt.Println("MacGo Advanced Configuration Example")
	fmt.Println("This app has camera, microphone, and photos permissions via explicit configuration")
	fmt.Println()

	// Try accessing protected resources
	homeDir, _ := os.UserHomeDir()
	dirs := []string{
		homeDir + "/Pictures",
		homeDir + "/Documents",
	}

	for _, dir := range dirs {
		fmt.Printf("Reading %s: ", dir)
		files, err := os.ReadDir(dir)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("%d files/directories\n", len(files))
		for i, file := range files {
			if i >= 3 {
				fmt.Println("...")
				break
			}
			fmt.Printf("- %s\n", file.Name())
		}
		fmt.Println()
	}
}
