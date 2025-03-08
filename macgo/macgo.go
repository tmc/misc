// Package macgo automatically creates and launches macOS app bundles
// to gain TCC permissions for command-line Go programs.
//
// Basic blank import usage (auto-initializes the package):
//
//	import (
//	    _ "github.com/tmc/misc/macgo"
//	)
//
// Simple direct usage with permission functions:
//
//	import "github.com/tmc/misc/macgo"
//
//	func init() {
//	    // Set specific permissions
//	    macgo.SetCamera()
//	    macgo.SetMic()
//	    
//	    // Or set all permissions at once
//	    // macgo.SetAll()
//	}
//
// Configure with environment variables:
//
//	MACGO_APP_NAME="MyApp" MACGO_BUNDLE_ID="com.example.myapp" MACGO_CAMERA=1 MACGO_MIC=1 ./myapp
//
// Advanced usage with configuration API:
//
//	import "github.com/tmc/misc/macgo"
//
//	func init() {
//	    // Create a custom configuration
//	    cfg := macgo.NewConfig()
//	    cfg.Name = "CustomApp"
//	    cfg.BundleID = "com.example.customapp" 
//	    cfg.AddPermission(macgo.PermCamera)
//	    cfg.AddPermission(macgo.PermMic)
//
//	    // Add custom Info.plist entries
//	    cfg.AddPlistEntry("LSUIElement", true) // Make app run in background
//
//	    // Apply configuration (must be called)
//	    macgo.Configure(cfg)
//	}
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
	"sync"
	"time"
)

// Entitlement is a type for macOS entitlement identifiers
type Entitlement string

// Available app sandbox entitlements
const (
	// App Sandbox entitlements
	EntAppSandbox    Entitlement = "com.apple.security.app-sandbox"
	EntNetworkClient Entitlement = "com.apple.security.network.client"
	EntNetworkServer Entitlement = "com.apple.security.network.server"
	
	// Device entitlements
	EntCamera     Entitlement = "com.apple.security.device.camera"
	EntMicrophone Entitlement = "com.apple.security.device.microphone"
	EntBluetooth  Entitlement = "com.apple.security.device.bluetooth"
	EntUSB        Entitlement = "com.apple.security.device.usb"
	EntAudioInput Entitlement = "com.apple.security.device.audio-input"
	EntPrint      Entitlement = "com.apple.security.print"
	
	// Personal information entitlements
	EntAddressBook Entitlement = "com.apple.security.personal-information.addressbook"
	EntLocation    Entitlement = "com.apple.security.personal-information.location"
	EntCalendars   Entitlement = "com.apple.security.personal-information.calendars"
	EntPhotos      Entitlement = "com.apple.security.personal-information.photos-library"
	
	// File entitlements
	EntUserSelectedReadOnly  Entitlement = "com.apple.security.files.user-selected.read-only"
	EntUserSelectedReadWrite Entitlement = "com.apple.security.files.user-selected.read-write"
	EntDownloadsReadOnly     Entitlement = "com.apple.security.files.downloads.read-only"
	EntDownloadsReadWrite    Entitlement = "com.apple.security.files.downloads.read-write"
	EntPicturesReadOnly      Entitlement = "com.apple.security.assets.pictures.read-only"
	EntPicturesReadWrite     Entitlement = "com.apple.security.assets.pictures.read-write"
	EntMusicReadOnly         Entitlement = "com.apple.security.assets.music.read-only"
	EntMusicReadWrite        Entitlement = "com.apple.security.assets.music.read-write"
	EntMoviesReadOnly        Entitlement = "com.apple.security.assets.movies.read-only"
	EntMoviesReadWrite       Entitlement = "com.apple.security.assets.movies.read-write"
	
	// Hardened Runtime entitlements
	EntAllowJIT                      Entitlement = "com.apple.security.cs.allow-jit"
	EntAllowUnsignedExecutableMemory Entitlement = "com.apple.security.cs.allow-unsigned-executable-memory"
	EntAllowDyldEnvVars              Entitlement = "com.apple.security.cs.allow-dyld-environment-variables"
	EntDisableLibraryValidation      Entitlement = "com.apple.security.cs.disable-library-validation"
	EntDisableExecutablePageProtection Entitlement = "com.apple.security.cs.disable-executable-page-protection"
	EntDebugger                      Entitlement = "com.apple.security.cs.debugger"
	
	// For backward compatibility
	PermCamera    Entitlement = EntCamera
	PermMic       Entitlement = EntMicrophone
	PermLocation  Entitlement = EntLocation
	PermContacts  Entitlement = EntAddressBook
	PermPhotos    Entitlement = EntPhotos
	PermCalendar  Entitlement = EntCalendars
	PermReminders Entitlement = "com.apple.security.personal-information.reminders"
)

// Config provides a way to customize the app bundle behavior
type Config struct {
	// Name overrides the default app name (executable name)
	Name string
	
	// BundleID overrides the default bundle identifier
	BundleID string
	
	// Entitlements contains entitlements to request
	Entitlements map[Entitlement]bool
	
	// Permissions contains entitlements to request (legacy name)
	// For backward compatibility
	Permissions map[Entitlement]bool
	
	// PlistEntries contains additional Info.plist entries
	PlistEntries map[string]any
	
	// Relaunch controls whether to auto-relaunch (default: true)
	Relaunch bool
	
	// AppPath specifies a custom path for the app bundle
	AppPath string
	
	// KeepTemp prevents temporary bundles from being cleaned up
	KeepTemp bool
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		Entitlements: make(map[Entitlement]bool),
		Permissions:  make(map[Entitlement]bool),
		PlistEntries: make(map[string]any),
		Relaunch:     true,
	}
}

// AddEntitlement adds an entitlement to the configuration
func (c *Config) AddEntitlement(e Entitlement) {
	c.Entitlements[e] = true
	c.Permissions[e] = true // For backward compatibility
}

// AddPermission adds a TCC permission to the configuration (legacy method)
func (c *Config) AddPermission(p Entitlement) {
	c.AddEntitlement(p)
}

// AddPlistEntry adds a custom entry to the Info.plist file
func (c *Config) AddPlistEntry(key string, value any) {
	c.PlistEntries[key] = value
}

// global configuration state
var (
	// config contains internal app bundle settings
	config struct {
		// Name overrides the default app name (executable name)
		Name string
		
		// ID overrides the default bundle identifier
		ID string
		
		// Plist contains additional Info.plist entries
		Plist map[string]any
		
		// Entitlements contains entitlement key-value pairs
		Entitlements map[string]bool
		
		// Options
		DoRelaunch bool
		CustomPath string
		KeepTemp   bool
		
		// initialized tracks if init has been called
		initialized bool
		
		// initOnce ensures init happens only once
		initOnce sync.Once
	}
	
	// registerLock protects entitlement registration
	registerLock sync.Mutex
)

// Configure applies a custom configuration for macgo
func Configure(cfg *Config) {
	if cfg == nil {
		return
	}
	
	registerLock.Lock()
	defer registerLock.Unlock()
	
	config.Name = cfg.Name
	config.ID = cfg.BundleID
	config.CustomPath = cfg.AppPath
	config.DoRelaunch = cfg.Relaunch
	config.KeepTemp = cfg.KeepTemp
	
	// Copy entitlements
	for e, enabled := range cfg.Entitlements {
		if enabled {
			config.Entitlements[string(e)] = true
		}
	}
	
	// Copy permissions to entitlements (for backward compatibility)
	for p, enabled := range cfg.Permissions {
		if enabled {
			config.Entitlements[string(p)] = true
		}
	}
	
	// Copy plist entries
	for k, v := range cfg.PlistEntries {
		config.Plist[k] = v
	}
	
	// Ensure we initialize on the first Configure call
	initializeMacGo()
}

// LegacyConfig contains app bundle settings for backward compatibility.
// New code should use the new Config type instead.
var LegacyConfig struct {
	// Name overrides the default app name (executable name).
	Name string
	
	// ID overrides the default bundle identifier.
	ID string
	
	// Plist contains additional Info.plist entries.
	Plist map[string]any
	
	// Entitlements contains entitlement key-value pairs.
	Entitlements map[string]bool
}

func init() {
	LegacyConfig.Plist = make(map[string]any)
	LegacyConfig.Entitlements = make(map[string]bool)
	config.Plist = make(map[string]any)
	config.Entitlements = make(map[string]bool)
	config.DoRelaunch = true
	
	// Initialize config from environment
	if name := os.Getenv("MACGO_APP_NAME"); name != "" {
		config.Name = name
	}
	
	if id := os.Getenv("MACGO_BUNDLE_ID"); id != "" {
		config.ID = id
	}
	
	if path := os.Getenv("MACGO_APP_PATH"); path != "" {
		config.CustomPath = path
	}
	
	if os.Getenv("MACGO_NO_RELAUNCH") == "1" {
		config.DoRelaunch = false
	}
	
	if os.Getenv("MACGO_KEEP_TEMP") == "1" {
		config.KeepTemp = true
	}
	
	// Initialize once on import
	initializeMacGo()
}

// initializeMacGo is called once to set up the app bundle
func initializeMacGo() {
	config.initOnce.Do(func() {
		// Skip if already initialized
		if config.initialized {
			return
		}
		
		// Run only on macOS
		if runtime.GOOS != "darwin" {
			return
		}
		
		// Copy settings from legacy Config for backward compatibility
		if LegacyConfig.Name != "" {
			config.Name = LegacyConfig.Name
		}
		
		if LegacyConfig.ID != "" {
			config.ID = LegacyConfig.ID
		}
		
		for k, v := range LegacyConfig.Plist {
			config.Plist[k] = v
		}
		
		for k, v := range LegacyConfig.Entitlements {
			config.Entitlements[k] = v
		}
		
		// Get executable path
		exec, err := os.Executable()
		if err != nil {
			debugf("error getting executable path: %v", err)
			return
		}
		
		// Check if already in an app bundle
		if inBundle(exec) {
			debugf("already in app bundle: %s", exec)
			return
		}
		
		// Skip relaunch if disabled
		if !config.DoRelaunch {
			debugf("relaunch disabled, skipping")
			return
		}
		
		// Create app bundle and relaunch
		debugf("creating app bundle for: %s", exec)
		app, err := createBundle(exec)
		if err != nil {
			debugf("error creating app bundle: %v", err)
			return
		}
		
		debugf("relaunching via app bundle: %s", app)
		relaunch(app, exec)
		
		config.initialized = true
	})
}

// debugf prints debug messages when MACGO_DEBUG=1.
func debugf(format string, args ...any) {
	if os.Getenv("MACGO_DEBUG") == "1" {
		log.Printf("[macgo] "+format, args...)
	}
}

// inBundle checks if a path is within an app bundle.
func inBundle(path string) bool {
	name := filepath.Base(path)
	return strings.Contains(path, ".app/Contents/MacOS/"+name)
}

// createBundle creates an app bundle for an executable.
func createBundle(execPath string) (string, error) {
	// Get executable name and determine app name
	name := filepath.Base(execPath)
	appName := name
	if config.Name != "" {
		appName = config.Name
	}
	
	// Check if using go run (temporary binary)
	isTemp := strings.Contains(execPath, "go-build")
	
	// Determine bundle location
	var dir, appPath string
	var fileHash string
	
	// Use custom path if specified
	if config.CustomPath != "" {
		// Ensure .app extension
		if !strings.HasSuffix(config.CustomPath, ".app") {
			appPath = config.CustomPath + ".app"
		} else {
			appPath = config.CustomPath
		}
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
	bundleID := config.ID
	if bundleID == "" {
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
	}
	
	// Add user-defined entries
	for k, v := range config.Plist {
		plist[k] = v
	}
	
	// Write Info.plist
	infoPlistPath := filepath.Join(contentsPath, "Info.plist")
	if err := writePlist(infoPlistPath, plist); err != nil {
		return "", fmt.Errorf("write Info.plist: %w", err)
	}
	
	// Write entitlements if any
	if len(config.Entitlements) > 0 {
		entitlements := make(map[string]any)
		for k, v := range config.Entitlements {
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
	if isTemp && !config.KeepTemp {
		fmt.Fprintf(os.Stderr, "[macgo] Created temporary app bundle\n")
		go func() {
			time.Sleep(10 * time.Second)
			os.RemoveAll(dir)
		}()
	} else {
		fmt.Fprintf(os.Stderr, "[macgo] Created app bundle at: %s\n", appPath)
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
func writePlist(path string, data map[string]any) error {
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

// RegisterEntitlement registers a TCC entitlement key-value pair
func RegisterEntitlement(key string, value bool) {
	registerLock.Lock()
	defer registerLock.Unlock()
	
	// Add to both config structures for compatibility
	LegacyConfig.Entitlements[key] = value
	config.Entitlements[key] = value
}

// Environment variable detection for entitlements
func init() {
	// Check environment variables for permissions and entitlements
	envVars := map[string]string{
		// Basic TCC permissions (legacy)
		"MACGO_CAMERA":    string(EntCamera),
		"MACGO_MIC":       string(EntMicrophone),
		"MACGO_LOCATION":  string(EntLocation),
		"MACGO_CONTACTS":  string(EntAddressBook),
		"MACGO_PHOTOS":    string(EntPhotos),
		"MACGO_CALENDAR":  string(EntCalendars),
		"MACGO_REMINDERS": string(PermReminders),
		
		// App Sandbox entitlements
		"MACGO_APP_SANDBOX":     string(EntAppSandbox),
		"MACGO_NETWORK_CLIENT":  string(EntNetworkClient),
		"MACGO_NETWORK_SERVER":  string(EntNetworkServer),
		
		// Device entitlements
		"MACGO_BLUETOOTH":       string(EntBluetooth),
		"MACGO_USB":             string(EntUSB),
		"MACGO_AUDIO_INPUT":     string(EntAudioInput),
		"MACGO_PRINT":           string(EntPrint),
		
		// File entitlements
		"MACGO_USER_FILES_READ":      string(EntUserSelectedReadOnly),
		"MACGO_USER_FILES_WRITE":     string(EntUserSelectedReadWrite),
		"MACGO_DOWNLOADS_READ":       string(EntDownloadsReadOnly),
		"MACGO_DOWNLOADS_WRITE":      string(EntDownloadsReadWrite),
		"MACGO_PICTURES_READ":        string(EntPicturesReadOnly),
		"MACGO_PICTURES_WRITE":       string(EntPicturesReadWrite),
		"MACGO_MUSIC_READ":           string(EntMusicReadOnly),
		"MACGO_MUSIC_WRITE":          string(EntMusicReadWrite),
		"MACGO_MOVIES_READ":          string(EntMoviesReadOnly),
		"MACGO_MOVIES_WRITE":         string(EntMoviesReadWrite),
		
		// Hardened Runtime entitlements
		"MACGO_ALLOW_JIT":               string(EntAllowJIT),
		"MACGO_ALLOW_UNSIGNED_MEMORY":   string(EntAllowUnsignedExecutableMemory),
		"MACGO_ALLOW_DYLD_ENV":          string(EntAllowDyldEnvVars),
		"MACGO_DISABLE_LIBRARY_VALIDATION": string(EntDisableLibraryValidation),
		"MACGO_DISABLE_EXEC_PAGE_PROTECTION": string(EntDisableExecutablePageProtection),
		"MACGO_DEBUGGER":                string(EntDebugger),
	}
	
	for env, entitlement := range envVars {
		if os.Getenv(env) == "1" {
			RegisterEntitlement(entitlement, true)
		}
	}
}

// Entitlement setting functions - can be used directly or with blank import
// These can be used in both ways:
// 1. Direct call: macgo.SetCamera()
// 2. Blank import with init: import _ "github.com/tmc/misc/macgo/camera"

// TCC Permission entitlements

// SetCamera enables camera access.
func SetCamera() {
	RegisterEntitlement(string(EntCamera), true)
}

// SetMic enables microphone access.
func SetMic() {
	RegisterEntitlement(string(EntMicrophone), true)
}

// SetLocation enables location access.
func SetLocation() {
	RegisterEntitlement(string(EntLocation), true)
}

// SetContacts enables contacts access.
func SetContacts() {
	RegisterEntitlement(string(EntAddressBook), true)
}

// SetPhotos enables photos library access.
func SetPhotos() {
	RegisterEntitlement(string(EntPhotos), true)
}

// SetCalendar enables calendar access.
func SetCalendar() {
	RegisterEntitlement(string(EntCalendars), true)
}

// SetReminders enables reminders access.
func SetReminders() {
	RegisterEntitlement(string(PermReminders), true)
}

// App Sandbox entitlements

// SetAppSandbox enables App Sandbox.
func SetAppSandbox() {
	RegisterEntitlement(string(EntAppSandbox), true)
}

// SetNetworkClient enables outgoing network connections.
func SetNetworkClient() {
	RegisterEntitlement(string(EntNetworkClient), true)
}

// SetNetworkServer enables incoming network connections.
func SetNetworkServer() {
	RegisterEntitlement(string(EntNetworkServer), true)
}

// Device entitlements

// SetBluetooth enables Bluetooth access.
func SetBluetooth() {
	RegisterEntitlement(string(EntBluetooth), true)
}

// SetUSB enables USB device access.
func SetUSB() {
	RegisterEntitlement(string(EntUSB), true)
}

// SetAudioInput enables audio input access.
func SetAudioInput() {
	RegisterEntitlement(string(EntAudioInput), true)
}

// SetPrinting enables printing capabilities.
func SetPrinting() {
	RegisterEntitlement(string(EntPrint), true)
}

// Hardened Runtime entitlements

// SetAllowJIT enables JIT compilation.
func SetAllowJIT() {
	RegisterEntitlement(string(EntAllowJIT), true)
}

// SetAllowUnsignedMemory allows unsigned executable memory.
func SetAllowUnsignedMemory() {
	RegisterEntitlement(string(EntAllowUnsignedExecutableMemory), true)
}

// SetAllowDyldEnvVars allows DYLD environment variables.
func SetAllowDyldEnvVars() {
	RegisterEntitlement(string(EntAllowDyldEnvVars), true)
}

// SetDisableLibraryValidation disables library validation.
func SetDisableLibraryValidation() {
	RegisterEntitlement(string(EntDisableLibraryValidation), true)
}

// SetDisableExecutablePageProtection disables executable memory page protection.
func SetDisableExecutablePageProtection() {
	RegisterEntitlement(string(EntDisableExecutablePageProtection), true)
}

// SetDebugger enables attaching to other processes as a debugger.
func SetDebugger() {
	RegisterEntitlement(string(EntDebugger), true)
}

// Convenience functions

// SetAllTCCPermissions enables all TCC permissions.
func SetAllTCCPermissions() {
	SetCamera()
	SetMic()
	SetLocation()
	SetContacts()
	SetPhotos()
	SetCalendar()
	SetReminders()
}

// SetAllDeviceAccess enables all device access permissions.
func SetAllDeviceAccess() {
	SetCamera()
	SetMic()
	SetBluetooth()
	SetUSB()
	SetAudioInput()
	SetPrinting()
}

// SetAllNetworking enables all networking permissions.
func SetAllNetworking() {
	SetNetworkClient()
	SetNetworkServer()
}

// SetAll enables all basic TCC permissions (legacy function).
func SetAll() {
	SetAllTCCPermissions()
}