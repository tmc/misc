// Example demonstrating custom app template with go:embed and auto-signing
package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/tmc/misc/macgo"
)

// Embed the entire app template directory
//
//go:embed app-template
var appTemplate embed.FS

func init() {
	// Method 1: Using the simplified API for the custom template
	macgo.WithCustomAppBundle(appTemplate)

	// Set application details
	macgo.WithAppName("CustomTemplateApp")
	macgo.WithBundleID("com.example.macgo.custom-template")

	// Enable auto-signing (commented out for example purposes)
	// macgo.WithSigning("") // Use default identity
	// macgo.WithSigning("Developer ID Application: Your Name (XXXXXXXXXX)") // Specific identity

	// Add entitlements
	macgo.WithEntitlements(
		macgo.EntCamera,
		macgo.EntMicrophone,
		"com.apple.security.automation.apple-events",
	)

	// Method 2: Using the Config API for more complex scenarios
	// Uncomment to use this approach instead
	/*
		cfg := macgo.NewConfig()
		cfg.ApplicationName = "CustomTemplateApp"
		cfg.BundleID = "com.example.macgo.custom-template"
		cfg.AppTemplate = appTemplate
		cfg.AutoSign = true
		cfg.Entitlements = macgo.Entitlements{
			macgo.EntCamera:     true,
			macgo.EntMicrophone: true,
		}
		macgo.Configure(cfg)
	*/
}

func main() {
	fmt.Println("MacGo Custom Template Example")
	fmt.Println("This app demonstrates:")
	fmt.Println("1. Using a custom app template with go:embed")
	fmt.Println("2. Auto-signing capability (commented out by default)")
	fmt.Println("3. Combined with entitlements configuration")
	fmt.Println()
	fmt.Println("The app template includes:")
	fmt.Println("- Templated Info.plist with placeholder variables")
	fmt.Println("- Pre-configured entitlements for network and virtualization")
	fmt.Println("- Placeholder for executable injection")

	// Try to read desktop files
	homeDir, _ := os.UserHomeDir()
	fmt.Printf("\nReading %s/Desktop: ", homeDir)

	files, err := os.ReadDir(homeDir + "/Desktop")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Found %d files\n", len(files))
		for i, f := range files {
			if i >= 3 {
				fmt.Println("...")
				break
			}
			fmt.Printf("- %s\n", f.Name())
		}
	}
}
