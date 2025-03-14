// Advanced configuration example using the Config API and improved signal handling
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/tmc/misc/macgo"
)

func init() {
	// No need to disable auto-initialization as it's disabled by default

	// Create a custom configuration
	cfg := macgo.NewConfig()

	// Set application details
	cfg.ApplicationName = "AdvancedExampleApp"
	cfg.BundleID = "com.example.macgo.advanced"

	// Add entitlements individually
	cfg.AddEntitlement(macgo.EntCamera)
	cfg.AddEntitlement(macgo.EntMicrophone)

	// Request multiple entitlements at once
	cfg.RequestEntitlements(
		macgo.EntLocation,
	)

	// Access to user-selected files (already enabled by default)
	cfg.AddEntitlement(macgo.EntUserSelectedReadOnly)

	// Make this a proper GUI application to control dock behavior
	cfg.AddPlistEntry("LSUIElement", false) // Show in dock

	// Add NSHighResolutionCapable for proper Retina support
	cfg.AddPlistEntry("NSHighResolutionCapable", true)

	// Override app name in dock
	cfg.AddPlistEntry("CFBundleDisplayName", "Advanced macgo Example")

	// Control app bundle behavior
	cfg.Relaunch = true  // Auto-relaunch (default)
	cfg.AutoSign = true  // Auto-sign the bundle (default)
	cfg.KeepTemp = false // Don't keep temporary bundles

	// Apply configuration (must be called)
	macgo.Configure(cfg)

	// Enable debug logging (optional)
	macgo.EnableDebug()
	// Start the application
	macgo.Start()
}

func main() {
	fmt.Println("macgo Advanced Configuration Example with Signal Handling")
	fmt.Println("-------------------------------------------------------")
	fmt.Println("This example showcases the Config API and improved signal handling.")
	fmt.Println("Try pressing Ctrl+C to see proper signal handling in action.")
	fmt.Println()

	// Set up signal handling for demonstration
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	// Print which entitlements are likely enabled
	fmt.Println("Enabled entitlements:")
	fmt.Println("- Camera access")
	fmt.Println("- Microphone access")
	fmt.Println("- User-selected files (read-only)")
	fmt.Println()

	// Show that we can access the Desktop (through user-selected files)
	homeDir, _ := os.UserHomeDir()
	fmt.Printf("Reading Desktop directory: %s/Desktop\n", homeDir)

	files, err := os.ReadDir(homeDir + "/Desktop")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d files\n", len(files))
	for i, file := range files {
		if i >= 5 {
			fmt.Println("...")
			break
		}
		fmt.Printf("- %s\n", file.Name())
	}

	// Runtime information
	fmt.Println("\nRuntime information:")
	fmt.Println("- This process is either:")
	fmt.Println("  1. The first run: Setting up the app bundle")
	fmt.Println("  2. Running inside the bundle: UI settings are active")

	fmt.Printf("- Currently running in app bundle: %v\n", macgo.IsInAppBundle())
	fmt.Println("- If in bundle: App will show in dock with NO bouncing")
	fmt.Println("  (Using Objective-C to control dock behavior)")

	// Wait for user input
	fmt.Println("\nPress Enter to exit")
	var input string
	fmt.Scanln(&input)
}
