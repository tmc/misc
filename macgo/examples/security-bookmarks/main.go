// Security-Scoped Bookmarks Example
// Demonstrates how to use security-scoped bookmarks for persistent file access
// Note: This example requires CGO due to the need for Objective-C APIs
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tmc/misc/macgo"
)

/*
Note: This example demonstrates conceptually how security-scoped bookmarks work,
but doesn't include actual implementation since it requires CGO and Objective-C.

In a real implementation, you would use:
1. The NSURL startAccessingSecurityScopedResource API
2. The NSURL bookmarkDataWithOptions API
3. Store bookmark data persistently between app launches

The macgo package doesn't currently provide a native Go API for security-scoped
bookmarks, so this example is for educational purposes.
*/

func init() {
	// Create a custom configuration for security-scoped bookmarks
	cfg := macgo.NewConfig()

	// Set application details
	cfg.ApplicationName = "SecurityBookmarksExample"
	cfg.BundleID = "com.example.macgo.security-bookmarks"

	// Request sandbox
	cfg.AddEntitlement(macgo.EntAppSandbox)

	// Add user-selected file access (read/write)
	cfg.AddEntitlement(macgo.EntUserSelectedReadWrite)

	// Add security-scoped bookmark entitlement
	// This allows storing and using bookmarks to access files across launches
	cfg.AddEntitlement("com.apple.security.files.bookmarks.app-scope")

	// Enable debug logging
	macgo.EnableDebug()

	// Apply configuration
	macgo.Configure(cfg)

	// Initialize
	macgo.Start()
}

func main() {
	fmt.Println("macOS Security-Scoped Bookmarks Example")
	fmt.Println("=======================================")
	fmt.Println("This example explains how security-scoped bookmarks work.")
	fmt.Println()

	// Get user's home directory
	homeDir, _ := os.UserHomeDir()

	// Security-scoped bookmarks conceptual explanation
	fmt.Println("About Security-Scoped Bookmarks")
	fmt.Println("------------------------------")
	fmt.Println("Security-scoped bookmarks allow your app to maintain access to")
	fmt.Println("user-selected files and folders across app launches.")
	fmt.Println()
	fmt.Println("The typical workflow is:")
	fmt.Println("1. User selects a file/folder (usually through NSOpenPanel)")
	fmt.Println("2. App creates a persistent bookmark using NSURL bookmarkDataWithOptions")
	fmt.Println("3. App stores the bookmark data (e.g., in UserDefaults)")
	fmt.Println("4. On next launch, app resolves the bookmark to regain access")
	fmt.Println("5. App calls startAccessingSecurityScopedResource before access")
	fmt.Println("6. App calls stopAccessingSecurityScopedResource when done")
	fmt.Println()

	// Example folder we want to access
	targetFolder := filepath.Join(homeDir, "Documents")
	fmt.Printf("Target folder we want persistent access to: %s\n", targetFolder)
	fmt.Println()

	// Simulate bookmark creation (actual implementation needs CGO/Objective-C)
	fmt.Println("Creating a Security-Scoped Bookmark")
	fmt.Println("---------------------------------")
	fmt.Println("// In real code, you would do something like this:")
	fmt.Println("import \"github.com/progrium/macdriver/cocoa\"")
	fmt.Println("import \"github.com/progrium/macdriver/objc\"")
	fmt.Println()
	fmt.Println("func createBookmark(path string) ([]byte, error) {")
	fmt.Println("    url := cocoa.NSURL.fileURLWithPath(path)")
	fmt.Println("    options := cocoa.NSURLBookmarkCreationWithSecurityScope")
	fmt.Println("    bookmarkData, err := url.bookmarkDataWithOptions_includingResourceValuesForKeys_relativeToURL_error(")
	fmt.Println("        options, nil, nil)")
	fmt.Println("    if err != nil {")
	fmt.Println("        return nil, err")
	fmt.Println("    }")
	fmt.Println("    return bookmarkData.toBytes(), nil")
	fmt.Println("}")
	fmt.Println()

	// Simulate accessing files via bookmark
	fmt.Println("Accessing Files via Bookmark")
	fmt.Println("---------------------------")
	fmt.Println("// In real code, you would do something like this:")
	fmt.Println("func accessBookmarkedPath(bookmarkData []byte) (string, func(), error) {")
	fmt.Println("    data := cocoa.NSData.dataWithBytes(bookmarkData)")
	fmt.Println("    url, isStale, err := cocoa.NSURL.URLByResolvingBookmarkData_options_relativeToURL_bookmarkDataIsStale_error(")
	fmt.Println("        data, cocoa.NSURLBookmarkResolutionWithSecurityScope, nil)")
	fmt.Println("    if err != nil {")
	fmt.Println("        return \"\", nil, err")
	fmt.Println("    }")
	fmt.Println("    if isStale {")
	fmt.Println("        // Consider refreshing the bookmark")
	fmt.Println("    }")
	fmt.Println("    success := url.startAccessingSecurityScopedResource()")
	fmt.Println("    if !success {")
	fmt.Println("        return \"\", nil, errors.New(\"failed to access security scoped resource\")")
	fmt.Println("    }")
	fmt.Println("    cleanup := func() {")
	fmt.Println("        url.stopAccessingSecurityScopedResource()")
	fmt.Println("    }")
	fmt.Println("    return url.path(), cleanup, nil")
	fmt.Println("}")
	fmt.Println()

	// Security considerations
	fmt.Println("Security Considerations")
	fmt.Println("----------------------")
	fmt.Println("• Don't keep security-scoped resources open longer than needed")
	fmt.Println("• Always call stopAccessingSecurityScopedResource when done")
	fmt.Println("• Handle stale bookmarks appropriately")
	fmt.Println("• Use app-scoped bookmarks for most applications")
	fmt.Println("• Document-scoped bookmarks are for document-based apps")

	fmt.Println("\nPress Enter to exit...")
	fmt.Scanln()
}
