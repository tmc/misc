// Sandbox Best Practices Example
// This example demonstrates recommended patterns for working with the macOS App Sandbox
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	_ "github.com/tmc/misc/macgo/auto/sandbox"
)

// Define structured logging for better understanding of sandbox behavior
func logOperation(operation, target string, err error) {
	if err != nil {
		fmt.Printf("❌ %s %s: %v\n", operation, target, err)
	} else {
		fmt.Printf("✅ %s %s: Success\n", operation, target)
	}
}

// Safely get temporary directory (always accessible in sandbox)
func getSafeTempDir() string {
	// Temporary directory is always accessible in sandbox
	tempDir := os.TempDir()
	// Create a unique subdirectory for our app
	appTempDir := filepath.Join(tempDir, "sandbox-example-"+time.Now().Format("20060102-150405"))
	os.MkdirAll(appTempDir, 0755)
	return appTempDir
}

func main() {
	fmt.Println("macOS App Sandbox Best Practices Example")
	fmt.Println("========================================")
	fmt.Println()

	// 1. Demonstrate safe access to system locations
	fmt.Println("1. System Location Access Patterns")
	fmt.Println("----------------------------------")

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	logOperation("Get Home Directory", homeDir, err)

	// Get temp directory (always accessible)
	tempDir := getSafeTempDir()
	logOperation("Create Temp Directory", tempDir, nil)

	// Try accessing locations with different sandbox permissions
	locationsToCheck := []string{
		homeDir,                             // Should fail (no permission)
		filepath.Join(homeDir, "Desktop"),   // Should fail (no permission)
		filepath.Join(homeDir, "Downloads"), // May fail without specific entitlement
		tempDir,                             // Should succeed (temp is always accessible)
		"/tmp",                              // Should succeed
		"/var/tmp",                          // Should succeed
		"/private/tmp",                      // Should succeed
	}

	for _, location := range locationsToCheck {
		_, err := os.ReadDir(location)
		logOperation("Read Directory", location, err)
	}

	// 2. Demonstrate proper file operations with security bookmarks
	fmt.Println("\n2. File Operations Best Practices")
	fmt.Println("--------------------------------")

	// Create a test file in the temp directory (always accessible)
	testFilePath := filepath.Join(tempDir, "test-file.txt")
	err = os.WriteFile(testFilePath, []byte("Test data for sandbox example"), 0644)
	logOperation("Create File", testFilePath, err)

	// Read the file back
	data, err := os.ReadFile(testFilePath)
	if err == nil {
		fmt.Printf("✅ Read File %s: Content: %s\n", testFilePath, string(data))
	} else {
		logOperation("Read File", testFilePath, err)
	}

	// 3. Network operations - generally allowed in sandbox
	fmt.Println("\n3. Network Access Patterns")
	fmt.Println("-------------------------")

	// Outgoing network request
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	// Make a safe request to a well-known endpoint
	resp, err := client.Get("https://httpbin.org/ip")
	if err != nil {
		logOperation("HTTP GET", "https://httpbin.org/ip", err)
	} else {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logOperation("Read Response", "httpbin.org", err)
		} else {
			var ipData map[string]interface{}
			json.Unmarshal(body, &ipData)
			fmt.Printf("✅ Network Request: Success - Your IP: %v\n", ipData["origin"])
		}
	}

	// 4. Process execution
	fmt.Println("\n4. Process Execution Patterns")
	fmt.Println("----------------------------")

	// Execute a simple process (date command)
	cmd := exec.Command("date")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logOperation("Execute", "date", err)
	} else {
		fmt.Printf("✅ Execute date: Output: %s", output)
	}

	// Try executing a process with arguments
	cmd = exec.Command("bash", "-c", "echo 'Hello from sandbox!'")
	output, err = cmd.CombinedOutput()
	logOperation("Execute bash -c", "echo command", err)
	if err == nil {
		fmt.Printf("   Output: %s", output)
	}

	// 5. Security recommendations
	fmt.Println("\n5. Sandbox Security Best Practices")
	fmt.Println("--------------------------------")
	fmt.Println("✓ Always validate user input")
	fmt.Println("✓ Use secure default permissions (0600 for sensitive files)")
	fmt.Println("✓ Don't request more permissions than needed")
	fmt.Println("✓ Use Security-Scoped Bookmarks for persistent file access")
	fmt.Println("✓ Provide clear explanations for permission requests")
	fmt.Println("✓ Release permissions when no longer needed")
	fmt.Println("✓ Use temporary directory for transient files")

	// 6. Debugging sandbox issues
	fmt.Println("\n6. Debugging Sandbox Issues")
	fmt.Println("-------------------------")
	fmt.Println("✓ Check Console.app for sandbox violation messages")
	fmt.Println("✓ Use MACGO_DEBUG=1 environment variable to see debugging information")
	fmt.Println("✓ Inspect entitlements with: codesign -d --entitlements :- /path/to/YourApp.app")
	fmt.Println("✓ Verify TCC permissions in System Settings > Privacy & Security")
	fmt.Println("✓ Use log stream --predicate 'subsystem == \"com.apple.sandbox\"' for real-time logs")

	// Wait for user input before exiting
	fmt.Println("\nPress Enter to exit...")
	fmt.Scanln()

	// Clean up
	os.RemoveAll(tempDir)
}
