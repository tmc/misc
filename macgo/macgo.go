// Package macgo provides Go utilities for macOS app bundle creation and TCC permission handling.
package macgo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// debugf prints debug messages when MACGO_DEBUG=1 is set
func debugf(format string, args ...interface{}) {
	if os.Getenv("MACGO_DEBUG") == "1" {
		log.Printf("[macgo] "+format, args...)
	}
}

// When this package is imported with _, it will automatically check if it's running
// in an app bundle, and if not, attempt to find or create one and relaunch through it.
func init() {
	// Only run on macOS (Darwin)
	if runtime.GOOS != "darwin" {
		return
	}

	// No need for MACGO_NO_RELAUNCH environment variable anymore
	// We now detect app bundles precisely based on the full path pattern

	// Check if we're already running inside an app bundle
	execPath, err := os.Executable()
	if err != nil {
		debugf("Error getting executable path: %v", err)
		return
	}
	debugf("Executable path: %s", execPath)

	// If already in an app bundle, do nothing
	// Look for the pattern "/[something].app/Contents/MacOS/[executable name]"
	execName := filepath.Base(execPath)
	appBundlePath := fmt.Sprintf(".app/Contents/MacOS/%s", execName)
	if strings.Contains(execPath, appBundlePath) {
		debugf("Already running inside an app bundle, continuing normally")
		return
	}

	// Generate a unique binary copy even for temporary binaries like go-build
	appPath, err := createAppBundle(execPath)
	if err != nil {
		debugf("Error creating app bundle: %v", err)
		return
	}

	// Relaunch through the app bundle
	if appPath != "" {
		debugf("Relaunching through app bundle: %s", appPath)
		relaunchThroughBundle(appPath, execPath)
	}
}

// createAppBundle creates an app bundle for the executable
func createAppBundle(execPath string) (string, error) {
	execName := filepath.Base(execPath)
	debugf("Creating app bundle for: %s", execPath)

	// Determine if we're running from a temporary binary (go run)
	isTemporary := strings.Contains(execPath, "go-build")

	var appPath string
	var bundleDir string

	if isTemporary {
		// For temporary binaries, use a temporary directory
		tempDir, err := os.MkdirTemp("", "macgo-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temp directory: %w", err)
		}

		// Create a unique name based on the hash of the binary
		fileHash, err := calculateSHA256(execPath)
		if err != nil {
			// If we can't get a hash, use timestamp
			fileHash = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		shortHash := fileHash[:8] // Use first 8 chars of the hash

		// Use a unique name for the app
		uniqueName := fmt.Sprintf("%s-%s", execName, shortHash)
		appPath = filepath.Join(tempDir, uniqueName+".app")
		bundleDir = tempDir

		debugf("Using temporary app bundle for go run: %s", appPath)
	} else {
		// For permanent binaries, use GOPATH/bin or similar location
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			// Try to use default GOPATH if not set
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get user home directory: %w", err)
			}
			gopath = filepath.Join(home, "go")
		}

		// Use GOPATH/bin directory
		bundleDir = filepath.Join(gopath, "bin")
		appPath = filepath.Join(bundleDir, execName+".app")

		// Check if the app bundle already exists and is up to date
		if _, err := os.Stat(appPath); err == nil {
			debugf("App bundle exists at: %s", appPath)

			// Check if the executable exists in the app bundle
			bundleExecPath := filepath.Join(appPath, "Contents", "MacOS", execName)
			if _, err := os.Stat(bundleExecPath); err == nil {
				// Verify the executable checksums match
				srcChecksum, err := calculateSHA256(execPath)
				if err != nil {
					debugf("Error calculating source checksum: %v", err)
					// If we can't verify, assume we need to update
				} else {
					bundleChecksum, err := calculateSHA256(bundleExecPath)
					if err != nil {
						debugf("Error calculating bundle checksum: %v", err)
						// If we can't verify, assume we need to update
					} else if srcChecksum == bundleChecksum {
						// Checksums match, we can use the existing app bundle
						debugf("App bundle is up to date (checksums match)")
						return appPath, nil
					} else {
						debugf("App bundle needs updating (checksums don't match)")
						debugf("  Source SHA256: %s", srcChecksum)
						debugf("  Bundle SHA256: %s", bundleChecksum)
					}
				}

				// Update the executable
				debugf("Updating app bundle executable")
				err = copyFile(execPath, bundleExecPath)
				if err != nil {
					return "", fmt.Errorf("failed to update app bundle executable: %w", err)
				}
				// Make it executable
				os.Chmod(bundleExecPath, 0755)
				debugf("App bundle updated successfully")
				return appPath, nil
			}
		}
	}

	contentsPath := filepath.Join(appPath, "Contents")
	macosPath := filepath.Join(contentsPath, "MacOS")

	// Create directories
	err := os.MkdirAll(macosPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create app bundle directories: %w", err)
	}

	// Create a unique bundle ID using a format that combines the name and a hash
	bundleID := fmt.Sprintf("com.macgo.%s", execName)
	if isTemporary {
		// For temporary binaries, add hash to bundle ID to make it unique
		fileHash, _ := calculateSHA256(execPath)
		if fileHash != "" {
			bundleID = fmt.Sprintf("com.macgo.%s.%s", execName, fileHash[:8])
		}
	}

	// Create Info.plist
	infoPlistPath := filepath.Join(contentsPath, "Info.plist")
	infoPlist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>%s</string>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleVersion</key>
	<string>1.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>`, execName, bundleID, execName)

	err = os.WriteFile(infoPlistPath, []byte(infoPlist), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write Info.plist: %w", err)
	}

	// Copy the executable to the app bundle
	bundleExecPath := filepath.Join(macosPath, execName)
	debugf("Copying %s to %s", execPath, bundleExecPath)

	err = copyFile(execPath, bundleExecPath)
	if err != nil {
		return "", fmt.Errorf("failed to copy executable to app bundle: %w", err)
	}

	// Make it executable
	err = os.Chmod(bundleExecPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to make executable: %w", err)
	}

	// For temporary bundles, we don't create a launcher script
	if !isTemporary {
		fmt.Fprintf(os.Stderr, "[macgo] Created permanent app bundle at: %s\n", appPath)
	} else {
		fmt.Fprintf(os.Stderr, "[macgo] Created temporary app bundle for this session\n")

		// Schedule cleanup of temporary bundle when the process exits
		go func() {
			// Wait for the parent process to exit
			time.Sleep(10 * time.Second)
			debugf("Cleaning up temporary app bundle: %s", appPath)
			os.RemoveAll(appPath)
		}()
	}

	return appPath, nil
}

// relaunchThroughBundle relaunches the executable through the app bundle
func relaunchThroughBundle(appPath, execPath string) {
	// Create pipes for stdin, stdout, stderr
	stdinPipe, err := createNamedPipe("macgo-stdin")
	if err != nil {
		debugf("Error creating stdin pipe: %v", err)
		return
	}
	defer os.Remove(stdinPipe)

	stdoutPipe, err := createNamedPipe("macgo-stdout")
	if err != nil {
		debugf("Error creating stdout pipe: %v", err)
		return
	}
	defer os.Remove(stdoutPipe)

	stderrPipe, err := createNamedPipe("macgo-stderr")
	if err != nil {
		debugf("Error creating stderr pipe: %v", err)
		return
	}
	defer os.Remove(stderrPipe)

	// Prepare arguments for open command with file paths for IO redirection
	// Include --wait-apps to make 'open' wait for the app to exit
	args := []string{"-a", appPath, "--wait-apps",
		"--stdin", stdinPipe,
		"--stdout", stdoutPipe,
		"--stderr", stderrPipe}

	// No need to set MACGO_NO_RELAUNCH anymore since we precisely detect app bundles

	if len(os.Args) > 1 {
		args = append(args, "--args")
		args = append(args, os.Args[1:]...)
	}
	debugf("Launching with args: %v", args)

	// Launch the app bundle
	cmd := exec.Command("open", args...)

	// Start the command without attaching stdin/stdout/stderr directly
	err = cmd.Start()
	if err != nil {
		debugf("Error starting app bundle: %v", err)
		return
	}

	// Set up goroutines to handle IO redirection through the pipes
	go func() {
		// Connect stdin to the pipe
		stdinFile, err := os.OpenFile(stdinPipe, os.O_WRONLY, 0)
		if err != nil {
			debugf("Error opening stdin pipe: %v", err)
			return
		}
		defer stdinFile.Close()

		// Copy from os.Stdin to the pipe
		io.Copy(stdinFile, os.Stdin)
	}()

	go func() {
		// Connect stdout from the pipe
		stdoutFile, err := os.OpenFile(stdoutPipe, os.O_RDONLY, 0)
		if err != nil {
			debugf("Error opening stdout pipe: %v", err)
			return
		}
		defer stdoutFile.Close()

		// Copy from the pipe to os.Stdout
		io.Copy(os.Stdout, stdoutFile)
	}()

	go func() {
		// Connect stderr from the pipe
		stderrFile, err := os.OpenFile(stderrPipe, os.O_RDONLY, 0)
		if err != nil {
			debugf("Error opening stderr pipe: %v", err)
			return
		}
		defer stderrFile.Close()

		// Copy from the pipe to os.Stderr
		io.Copy(os.Stderr, stderrFile)
	}()

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		debugf("Error relaunching through app bundle: %v", err)
		return
	}

	debugf("Successfully relaunched through app bundle, exiting this process")
	// Exit this process since we've relaunched
	os.Exit(0)
}

// createNamedPipe creates a named pipe (FIFO) and returns its path
func createNamedPipe(prefix string) (string, error) {
	// Create temporary filename
	tmpFile, err := os.CreateTemp("", prefix+"-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	pipePath := tmpFile.Name()
	tmpFile.Close()
	os.Remove(pipePath) // Remove the regular file

	// Create the named pipe (FIFO)
	cmd := exec.Command("mkfifo", pipePath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create named pipe: %w", err)
	}

	return pipePath, nil
}

// calculateSHA256 computes the SHA-256 hash of a file
func calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Helper functions
func copyFile(src, dst string) error {
	debugf("Copying file from %s to %s", src, dst)
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}