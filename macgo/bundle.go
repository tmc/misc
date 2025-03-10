package macgo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// createBundle creates an app bundle for an executable.
func createBundle(execPath string) (string, error) {
	// Get executable name and determine app name
	name := filepath.Base(execPath)
	appName := name
	if DefaultConfig.ApplicationName != "" {
		appName = DefaultConfig.ApplicationName
	}

	// Check if using go run (temporary binary)
	isTemp := strings.Contains(execPath, "go-build")

	// Determine bundle location
	var dir, appPath string
	var fileHash string

	// Use custom path if specified
	if DefaultConfig.CustomDestinationAppPath != "" {
		appPath = DefaultConfig.CustomDestinationAppPath
		dir = filepath.Dir(appPath)
	} else if isTemp {
		// For temporary binaries, use a system temp directory
		tmp, err := os.MkdirTemp("", "macgo-*")
		if err != nil {
			return "", fmt.Errorf("create temp dir: %w", err)
		}

		// Create unique name with hash
		fileHash, err = checksum(execPath)
		if err != nil {
			fileHash = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		shortHash := fileHash[:8]

		// Unique app name for temporary bundles
		appName = fmt.Sprintf("%s-%s", appName, shortHash)
		appPath = filepath.Join(tmp, appName+".app")
		dir = tmp
	} else {
		// For regular binaries, use GOPATH/bin
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("get home dir: %w", err)
			}
			gopath = filepath.Join(home, "go")
		}

		dir = filepath.Join(gopath, "bin")
		appPath = filepath.Join(dir, appName+".app")

		// Check for existing bundle that's up to date
		if existing := checkExisting(appPath, execPath); existing {
			return appPath, nil
		}
	}

	// Create app bundle structure
	contentsPath := filepath.Join(appPath, "Contents")
	macosPath := filepath.Join(contentsPath, "MacOS")

	if err := os.MkdirAll(macosPath, 0755); err != nil {
		return "", fmt.Errorf("create bundle dirs: %w", err)
	}

	// Generate bundle ID
	bundleID := DefaultConfig.BundleID
	if bundleID == "" {
		// TODO: infer from go binary runtime package
		bundleID = fmt.Sprintf("com.macgo.%s", appName)
		if isTemp && len(fileHash) >= 8 {
			bundleID = fmt.Sprintf("com.macgo.%s.%s", appName, fileHash[:8])
		}
	}

	// Create Info.plist entries
	plist := map[string]any{
		"CFBundleExecutable":      name,
		"CFBundleIdentifier":      bundleID,
		"CFBundleName":            appName,
		"CFBundlePackageType":     "APPL",
		"CFBundleVersion":         "1.0",
		"NSHighResolutionCapable": true,
		// Set LSUIElement based on whether app should be visible in dock
		// If LSUIElement=true, app runs in background (no dock icon or menu)
		// If false, app appears in dock
		"LSUIElement": !DefaultConfig.Relaunch, // If relaunch is true, we want to be visible
	}

	// Add user-defined entries
	for k, v := range DefaultConfig.PlistEntries {
		plist[k] = v
	}

	// Write Info.plist
	infoPlistPath := filepath.Join(contentsPath, "Info.plist")
	if err := writePlist(infoPlistPath, plist); err != nil {
		return "", fmt.Errorf("write Info.plist: %w", err)
	}

	// Write entitlements if any
	if len(DefaultConfig.Entitlements) > 0 {
		entitlements := make(map[Entitlement]any)
		for k, v := range DefaultConfig.Entitlements {
			entitlements[k] = v
		}
		entPath := filepath.Join(contentsPath, "entitlements.plist")
		if err := writePlist(entPath, entitlements); err != nil {
			return "", fmt.Errorf("write entitlements: %w", err)
		}
	}

	// Copy the executable
	bundleExecPath := filepath.Join(macosPath, name)
	if err := copyFile(execPath, bundleExecPath); err != nil {
		return "", fmt.Errorf("copy executable: %w", err)
	}

	// Make executable
	if err := os.Chmod(bundleExecPath, 0755); err != nil {
		return "", fmt.Errorf("chmod: %w", err)
	}

	// Set cleanup for temporary bundles
	if isTemp && !DefaultConfig.KeepTemp {
		debugf("Created temporary app bundle at: %s", appPath)
		go func() {
			time.Sleep(30 * time.Second) // Increased to allow for app launch
			os.RemoveAll(dir)
		}()
	} else {
		debugf("Created app bundle at: %s", appPath)
	}

	// Auto-sign the bundle if requested
	if DefaultConfig.AutoSign {
		if err := signBundle(appPath); err != nil {
			debugf("Error signing bundle: %v", err)
			// Continue even if signing fails
		}
	}

	return appPath, nil
}

// checkExisting checks if an existing app bundle is up to date.
func checkExisting(appPath, execPath string) bool {
	name := filepath.Base(execPath)
	bundleExecPath := filepath.Join(appPath, "Contents", "MacOS", name)

	// Check if the app bundle exists
	if _, err := os.Stat(appPath); err != nil {
		return false
	}

	// Check if the executable exists in the bundle
	if _, err := os.Stat(bundleExecPath); err != nil {
		return false
	}

	// Compare checksums
	srcHash, err := checksum(execPath)
	if err != nil {
		debugf("error calculating source checksum: %v", err)
		return false
	}

	bundleHash, err := checksum(bundleExecPath)
	if err != nil {
		debugf("error calculating bundle checksum: %v", err)
		return false
	}

	if srcHash == bundleHash {
		debugf("app bundle is up to date")
		return true
	}

	// Update the executable
	debugf("updating app bundle executable")
	if err := copyFile(execPath, bundleExecPath); err != nil {
		debugf("error updating executable: %v", err)
		return false
	}

	os.Chmod(bundleExecPath, 0755)
	debugf("app bundle updated")
	return true
}

// relaunch restarts the application through the app bundle.
func relaunch(appPath, execPath string) {
	// Create pipes for IO redirection
	pipes := make([]string, 3)
	for i, name := range []string{"stdin", "stdout", "stderr"} {
		pipe, err := createPipe("macgo-" + name)
		if err != nil {
			debugf("error creating %s pipe: %v", name, err)
			return
		}
		pipes[i] = pipe
		defer os.Remove(pipe)
	}

	// Prepare open command arguments
	args := []string{
		"-a", appPath,
		"--wait-apps",
		"--stdin", pipes[0],
		"--stdout", pipes[1],
		"--stderr", pipes[2],
	}

	// Set environment to prevent relaunching again
	os.Setenv("MACGO_NO_RELAUNCH", "1")

	// Pass original arguments
	if len(os.Args) > 1 {
		args = append(args, "--args")
		args = append(args, os.Args[1:]...)
	}

	// Launch app bundle
	cmd := exec.Command("open", args...)
	if err := cmd.Start(); err != nil {
		debugf("error starting app bundle: %v", err)
		return
	}

	// Handle stdin
	go pipeIO(pipes[0], os.Stdin, nil)

	// Handle stdout
	go pipeIO(pipes[1], nil, os.Stdout)

	// Handle stderr
	go pipeIO(pipes[2], nil, os.Stderr)

	// Wait for process to finish
	if err := cmd.Wait(); err != nil {
		debugf("error waiting for app bundle: %v", err)
		return
	}

	os.Exit(0)
}

// pipeIO copies data between a pipe and stdin/stdout/stderr.
func pipeIO(pipe string, in io.Reader, out io.Writer) {
	mode := os.O_RDONLY
	if in != nil {
		mode = os.O_WRONLY
	}

	f, err := os.OpenFile(pipe, mode, 0)
	if err != nil {
		debugf("error opening pipe: %v", err)
		return
	}
	defer f.Close()

	if in != nil {
		io.Copy(f, in)
	} else {
		io.Copy(out, f)
	}
}

// createPipe creates a named pipe.
func createPipe(prefix string) (string, error) {
	tmp, err := os.CreateTemp("", prefix+"-*")
	if err != nil {
		return "", err
	}

	path := tmp.Name()
	tmp.Close()
	os.Remove(path)

	cmd := exec.Command("mkfifo", path)
	return path, cmd.Run()
}

// checksum calculates the SHA-256 hash of a file.
func checksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

// writePlist writes a map to a plist file.
func writePlist[K ~string](path string, data map[K]any) error {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)

	for k, v := range data {
		sb.WriteString(fmt.Sprintf("\t<key>%s</key>\n", k))

		switch val := v.(type) {
		case bool:
			if val {
				sb.WriteString("\t<true/>\n")
			} else {
				sb.WriteString("\t<false/>\n")
			}
		case string:
			sb.WriteString(fmt.Sprintf("\t<string>%s</string>\n", val))
		case int, int32, int64:
			sb.WriteString(fmt.Sprintf("\t<integer>%v</integer>\n", val))
		case float32, float64:
			sb.WriteString(fmt.Sprintf("\t<real>%v</real>\n", val))
		default:
			sb.WriteString(fmt.Sprintf("\t<string>%v</string>\n", val))
		}
	}

	sb.WriteString("</dict>\n</plist>")
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// Environment variable detection for entitlements
func init() {
	// Check environment variables for permissions and entitlements
	envVars := map[string]string{
		// Basic TCC permissions (legacy)
		"MACGO_CAMERA":   string(EntCamera),
		"MACGO_MIC":      string(EntMicrophone),
		"MACGO_LOCATION": string(EntLocation),
		"MACGO_CONTACTS": string(EntAddressBook),
		"MACGO_PHOTOS":   string(EntPhotos),
		"MACGO_CALENDAR": string(EntCalendars),

		// App Sandbox entitlements
		"MACGO_APP_SANDBOX":    string(EntAppSandbox),
		"MACGO_NETWORK_CLIENT": string(EntNetworkClient),
		"MACGO_NETWORK_SERVER": string(EntNetworkServer),

		// Device entitlements
		"MACGO_BLUETOOTH":   string(EntBluetooth),
		"MACGO_USB":         string(EntUSB),
		"MACGO_AUDIO_INPUT": string(EntAudioInput),
		"MACGO_PRINT":       string(EntPrint),

		// File entitlements
		"MACGO_USER_FILES_READ":  string(EntUserSelectedReadOnly),
		"MACGO_USER_FILES_WRITE": string(EntUserSelectedReadWrite),
		"MACGO_DOWNLOADS_READ":   string(EntDownloadsReadOnly),
		"MACGO_DOWNLOADS_WRITE":  string(EntDownloadsReadWrite),
		"MACGO_PICTURES_READ":    string(EntPicturesReadOnly),
		"MACGO_PICTURES_WRITE":   string(EntPicturesReadWrite),
		"MACGO_MUSIC_READ":       string(EntMusicReadOnly),
		"MACGO_MUSIC_WRITE":      string(EntMusicReadWrite),
		"MACGO_MOVIES_READ":      string(EntMoviesReadOnly),
		"MACGO_MOVIES_WRITE":     string(EntMoviesReadWrite),

		// Hardened Runtime entitlements
		"MACGO_ALLOW_JIT":                    string(EntAllowJIT),
		"MACGO_ALLOW_UNSIGNED_MEMORY":        string(EntAllowUnsignedExecutableMemory),
		"MACGO_ALLOW_DYLD_ENV":               string(EntAllowDyldEnvVars),
		"MACGO_DISABLE_LIBRARY_VALIDATION":   string(EntDisableLibraryValidation),
		"MACGO_DISABLE_EXEC_PAGE_PROTECTION": string(EntDisableExecutablePageProtection),
		"MACGO_DEBUGGER":                     string(EntDebugger),
	}

	for env, entitlement := range envVars {
		if os.Getenv(env) == "1" {
			DefaultConfig.AddEntitlement(Entitlement(entitlement))
		}
	}
}

// createFromTemplate creates an app bundle from an embedded template
func createFromTemplate(template fs.FS, appPath, execPath, appName string) (string, error) {
	// Create the app bundle directory
	if err := os.MkdirAll(appPath, 0755); err != nil {
		return "", fmt.Errorf("create app bundle directory: %w", err)
	}

	// Walk the template and copy all files to the app bundle
	err := fs.WalkDir(template, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if path == "." {
			return nil
		}

		// Full path in the target app bundle
		targetPath := filepath.Join(appPath, path)

		// Create directories
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Special handling for executables - replace with the actual executable
		if strings.Contains(path, "Contents/MacOS/") && strings.HasSuffix(path, ".placeholder") {
			// Extract the executable name without the .placeholder suffix
			dirPath := filepath.Dir(targetPath)
			execName := filepath.Base(execPath)

			// Ensure the directory exists
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("create executable directory: %w", err)
			}

			// Copy the executable to the bundle
			bundleExecPath := filepath.Join(dirPath, execName)
			if err := copyFile(execPath, bundleExecPath); err != nil {
				return fmt.Errorf("copy executable: %w", err)
			}

			// Make it executable
			return os.Chmod(bundleExecPath, 0755)
		}

		// Special handling for Info.plist - process templated values
		if strings.HasSuffix(path, "Info.plist") {
			// Read the template plist
			data, err := fs.ReadFile(template, path)
			if err != nil {
				return fmt.Errorf("read template Info.plist: %w", err)
			}

			// Replace placeholder values
			content := string(data)
			content = strings.ReplaceAll(content, "{{BundleName}}", appName)
			content = strings.ReplaceAll(content, "{{BundleExecutable}}", filepath.Base(execPath))

			bundleID := DefaultConfig.BundleID
			if bundleID == "" {
				bundleID = fmt.Sprintf("com.macgo.%s", appName)
			}
			content = strings.ReplaceAll(content, "{{BundleIdentifier}}", bundleID)

			// Add user-defined plist entries
			// This is a simple approach - for more complex needs, use a proper plist library
			for k, v := range DefaultConfig.PlistEntries {
				key := fmt.Sprintf("<key>%s</key>", k)
				var valueTag string
				switch val := v.(type) {
				case bool:
					if val {
						valueTag = "<true/>"
					} else {
						valueTag = "<false/>"
					}
				case string:
					valueTag = fmt.Sprintf("<string>%s</string>", val)
				case int, int32, int64:
					valueTag = fmt.Sprintf("<integer>%v</integer>", val)
				case float32, float64:
					valueTag = fmt.Sprintf("<real>%v</real>", val)
				default:
					valueTag = fmt.Sprintf("<string>%v</string>", val)
				}

				// Insert before closing dict
				closingDict := "</dict>"
				insertPos := strings.LastIndex(content, closingDict)
				if insertPos != -1 {
					content = content[:insertPos] + "\t" + key + "\n\t" + valueTag + "\n" + content[insertPos:]
				}
			}

			// Write the processed plist
			return os.WriteFile(targetPath, []byte(content), 0644)
		}

		// Special handling for entitlements.plist
		if strings.HasSuffix(path, "entitlements.plist") && len(DefaultConfig.Entitlements) > 0 {
			// Create a map for entitlements
			entitlements := make(map[string]any)
			for k, v := range DefaultConfig.Entitlements {
				entitlements[string(k)] = v
			}

			// Write the entitlements plist
			return writePlist(targetPath, entitlements)
		}

		// For normal files, just copy them
		data, err := fs.ReadFile(template, path)
		if err != nil {
			return fmt.Errorf("read template file %s: %w", path, err)
		}

		return os.WriteFile(targetPath, data, 0644)
	})

	if err != nil {
		return "", fmt.Errorf("process template: %w", err)
	}

	// Auto-sign the bundle if requested
	if DefaultConfig.AutoSign {
		if err := signBundle(appPath); err != nil {
			debugf("Error signing bundle: %v", err)
			// Continue even if signing fails
		}
	}

	return appPath, nil
}

// signBundle codesigns the app bundle using the system's codesign tool
func signBundle(appPath string) error {
	identity := DefaultConfig.SigningIdentity

	// Build the codesign command
	args := []string{"--force", "--deep"}

	// Add entitlements if available
	entitlementsPath := filepath.Join(appPath, "Contents", "entitlements.plist")
	if _, err := os.Stat(entitlementsPath); err == nil {
		args = append(args, "--entitlements", entitlementsPath)
	}

	// Add signing identity if specified
	if identity != "" {
		args = append(args, "--sign", identity)
	}

	// Add the app path
	args = append(args, appPath)

	// Execute codesign
	cmd := exec.Command("codesign", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign failed: %w, output: %s", err, output)
	}

	debugf("Codesigned app bundle: %s", appPath)
	return nil
}

func debugf(format string, args ...any) {
	if os.Getenv("MACGO_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
