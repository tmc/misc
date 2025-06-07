// Specific Folder Access Example
// Demonstrates how to access specific folders with entitlements
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tmc/misc/macgo"
)

func init() {
	// Create a custom configuration for specific folder access
	cfg := macgo.NewConfig()

	// Set application details
	cfg.ApplicationName = "FolderAccessExample"
	cfg.BundleID = "com.example.macgo.folder-access"

	// Request sandbox (default)
	cfg.AddEntitlement(macgo.EntAppSandbox)

	// Add specific folder access entitlements
	// Unlike user-selected files which prompt at runtime,
	// these provide access to standard folders without prompting
	// but require explicit entitlements
	cfg.RequestEntitlements(
		// Standard folder access
		macgo.EntDownloadsReadOnly, // Read access to Downloads folder
		macgo.EntPicturesReadOnly,  // Read access to Pictures folder
		macgo.EntMusicReadOnly,     // Read access to Music folder
		macgo.EntMoviesReadOnly,    // Read access to Movies folder

		// For Applications (not usually needed)
		// "com.apple.security.files.bookmarks.app-scope",  // For security-scoped bookmarks

		// Uncomment for write access (use carefully)
		// macgo.EntDownloadsReadWrite,    // Read/write access to Downloads folder
		// macgo.EntPicturesReadWrite,     // Read/write access to Pictures folder
	)

	// Enable debug logging
	macgo.EnableDebug()

	// Apply configuration
	macgo.Configure(cfg)

	// Initialize
	macgo.Start()
}

// Helper to read and display directory contents
func listDirectory(path string) {
	fmt.Printf("Reading %s: ", path)

	files, err := os.ReadDir(path)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	fmt.Printf("SUCCESS! Found %d files\n", len(files))
	for i, file := range files {
		if i >= 5 {
			fmt.Println("...")
			break
		}
		fmt.Printf("  - %s\n", file.Name())
	}
	fmt.Println()
}

func main() {
	fmt.Println("macOS Specific Folder Access Example")
	fmt.Println("====================================")
	fmt.Println("This example demonstrates accessing specific folders with entitlements.")
	fmt.Println()

	// Get standard directories
	homeDir, _ := os.UserHomeDir()

	// These should succeed with the specific entitlements
	fmt.Println("1. Standard Folders with Entitlements")
	fmt.Println("------------------------------------")
	listDirectory(filepath.Join(homeDir, "Downloads"))
	listDirectory(filepath.Join(homeDir, "Pictures"))
	listDirectory(filepath.Join(homeDir, "Music"))
	listDirectory(filepath.Join(homeDir, "Movies"))

	// These should fail (no entitlements)
	fmt.Println("2. Folders without Specific Entitlements")
	fmt.Println("---------------------------------------")
	listDirectory(filepath.Join(homeDir, "Documents"))
	listDirectory(homeDir) // Home directory itself

	// These should succeed (access always allowed)
	fmt.Println("3. Folders Always Accessible")
	fmt.Println("---------------------------")
	listDirectory(os.TempDir())
	listDirectory("/tmp")

	// Creating files in Downloads (requires read-write entitlement)
	fmt.Println("4. Write Access Test (requires EntDownloadsReadWrite)")
	fmt.Println("-------------------------------------------------")
	testFilePath := filepath.Join(homeDir, "Downloads", "macgo-test.txt")
	err := os.WriteFile(testFilePath, []byte("Test file for macgo specific folder access"), 0644)
	if err != nil {
		fmt.Printf("Writing to %s: ERROR: %v\n", testFilePath, err)
	} else {
		fmt.Printf("Writing to %s: SUCCESS!\n", testFilePath)
		// Clean up
		os.Remove(testFilePath)
	}

	fmt.Println("\nPress Enter to exit...")
	fmt.Scanln()
}
