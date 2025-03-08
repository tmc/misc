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
	cfg.Name = "MacGoAdvanced"
	cfg.BundleID = "com.example.macgo.advanced"
	
	// Request specific permissions
	cfg.AddPermission(macgo.PermCamera)
	cfg.AddPermission(macgo.PermMic)
	cfg.AddPermission(macgo.PermPhotos)
	
	// Add custom Info.plist entries
	cfg.AddPlistEntry("LSUIElement", false)          // Show in Dock
	cfg.AddPlistEntry("CFBundleDisplayName", "MacGo Advanced Example")
	
	// Control app bundle behavior
	cfg.Relaunch = true     // Auto-relaunch (default)
	cfg.KeepTemp = false    // Cleanup temp bundles (default)
	// cfg.AppPath = "/custom/path/MyApp"  // Uncomment to specify custom bundle path
	
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