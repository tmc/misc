// Package macgo automatically creates and launches macOS app bundles
// to gain TCC permissions for command-line Go programs.
//
// Basic blank import usage (auto-initializes the package):
//
//	import (
//	    _ "github.com/tmc/misc/macgo/auto"
//	)
//
// Simple direct usage with permission functions:
//
//	import "github.com/tmc/misc/macgo"
//
//	func init() {
//	    // Set specific permissions
//	    macgo.RequestEntitlements(macgo.EntAppSandbox, macgo.EntCamera)
//	}
//
// Configure with environment variables:
//
//	MACGO_APP_NAME="MyApp" MACGO_BUNDLE_ID="com.example.myapp" MACGO_CAMERA=1 MACGO_MIC=1 ./myapp
package macgo

import (
	"io/fs"
	"os"
	"strings"
	"sync"
)

// Entitlement is a type for macOS entitlement identifiers
type Entitlement string

// Entitlements is a map of entitlement identifiers to boolean values
type Entitlements map[Entitlement]bool

// Available app sandbox entitlements
const (
	// App Sandbox entitlements
	EntAppSandbox Entitlement = "com.apple.security.app-sandbox"

	// Network entitlements
	// NOTE: These network entitlements only affect Objective-C/Swift network APIs.
	// Go's standard networking (net/http, etc.) bypasses these restrictions and will
	// work regardless of these entitlements being present or not. To properly restrict
	// network access in Go applications, additional measures are required.
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
	EntReminders   Entitlement = "com.apple.security.personal-information.reminders"

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
	EntAllowJIT                        Entitlement = "com.apple.security.cs.allow-jit"
	EntAllowUnsignedExecutableMemory   Entitlement = "com.apple.security.cs.allow-unsigned-executable-memory"
	EntAllowDyldEnvVars                Entitlement = "com.apple.security.cs.allow-dyld-environment-variables"
	EntDisableLibraryValidation        Entitlement = "com.apple.security.cs.disable-library-validation"
	EntDisableExecutablePageProtection Entitlement = "com.apple.security.cs.disable-executable-page-protection"
	EntDebugger                        Entitlement = "com.apple.security.cs.debugger"

	// Virtualization entitlements
	EntVirtualization Entitlement = "com.apple.security.virtualization"

	// IF you need to add more entitlements, you can simplty cast string to Entitlement via
	// Entitlement("com.apple.security.some.new.entitlement")
)

// DefaultConfig is the default configuration for macgo
var DefaultConfig = &Config{
	AutoSign:     true,
	Relaunch:     true, // Enable auto-relaunching by default
	Entitlements: map[Entitlement]bool{
		// EntUserSelectedReadWrite: true, // Enable user-selected file access by default
	},
	PlistEntries: map[string]any{
		"LSUIElement": false, // Hide dock icon and app menu by default
	},
}

// Config provides a way to customize the app bundle behavior
type Config struct {
	// ApplicationName overrides the default app name (executable name)
	ApplicationName string

	// BundleID overrides the default bundle identifier
	BundleID string

	// Entitlements contains entitlements to request
	Entitlements Entitlements

	// PlistEntries contains additional Info.plist entries
	PlistEntries map[string]any

	// Relaunch controls whether to auto-relaunch (default: true)
	// When true, the process will relaunch inside the app bundle to gain
	// TCC permissions. Set to false to disable this behavior.
	Relaunch bool

	// CustomDestinationAppPath specifies a custom path for the app bundle
	CustomDestinationAppPath string

	// KeepTemp prevents temporary bundles from being cleaned up
	KeepTemp bool

	// AppTemplate provides a custom app bundle template
	// This should be a directory structure with placeholder files
	// that will be filled in during app bundle creation.
	// Use with go:embed to embed an entire app structure.
	AppTemplate fs.FS

	// AutoSign enables automatic codesigning of the app bundle
	// When true, the app bundle will be code signed to enable proper functionality
	// of entitlements
	AutoSign bool

	// SigningIdentity specifies the identity to use for codesigning
	// If empty, ad-hoc signing ("-") will be used when AutoSign is true
	SigningIdentity string
}

// AddEntitlement adds an entitlement to the configuration
func (c *Config) AddEntitlement(e Entitlement) {
	if c.Entitlements == nil {
		c.Entitlements = make(map[Entitlement]bool)
	}
	c.Entitlements[e] = true
}

// AddPermission adds a TCC permission to the configuration (legacy method)
func (c *Config) AddPermission(p Entitlement) {
	c.AddEntitlement(p)
}

// AddPlistEntry adds a custom entry to the Info.plist file
func (c *Config) AddPlistEntry(key string, value any) {
	if c.PlistEntries == nil {
		c.PlistEntries = make(map[string]any)
	}
	c.PlistEntries[key] = value
}

var initOnce sync.Once

// init sets up the default configuration only
// It does not create the app bundle or relaunch the application
// For automatic initialization, import "github.com/tmc/misc/macgo/auto"
func init() {
	// Using debugf for visibility of initialization steps
	debugf("macgo: setting up configuration from environment...")

	// Initialize config from environment
	if name := os.Getenv("MACGO_APP_NAME"); name != "" {
		DefaultConfig.ApplicationName = name
	}

	if id := os.Getenv("MACGO_BUNDLE_ID"); id != "" {
		DefaultConfig.BundleID = id
	}

	if os.Getenv("MACGO_NO_RELAUNCH") == "1" {
		DefaultConfig.Relaunch = false
	}

	if os.Getenv("MACGO_KEEP_TEMP") == "1" {
		DefaultConfig.KeepTemp = true
	}

	// Check if dock icon should be shown
	if os.Getenv("MACGO_SHOW_DOCK_ICON") == "1" {
		if DefaultConfig.PlistEntries == nil {
			DefaultConfig.PlistEntries = make(map[string]any)
		}
		DefaultConfig.PlistEntries["LSUIElement"] = true
	}
}

// Start initializes macgo and creates the app bundle if needed.
// This should be called explicitly in your main() function after any configuration.
// It's safe to call multiple times; only the first call will take effect.
//
// Example:
//
//	func main() {
//	    // Configure macgo (optional)
//	    macgo.RequestEntitlements(macgo.EntCamera, macgo.EntMicrophone)
//
//	    // Start macgo - this creates the app bundle and relaunches if needed
//	    macgo.Start()
//
//	    // Rest of your program
//	    // ...
//	}
func Start() {
	initOnce.Do(func() {
		debugf("macgo: initializing app bundle...")
		initializeMacGo()
	})
}

// Initialize is an alias for Start() for backward compatibility
// For new code, use Start() instead
func Initialize() {
	Start()
}

// DisableAutoInit is deprecated and no longer does anything.
// Auto-initialization is disabled by default, and you must call Start() manually.
//
// Example:
//
//	func init() {
//	    macgo.RequestEntitlements(macgo.EntCamera)
//	    // ...configure all your settings...
//	    macgo.Start() // explicitly initialize when ready
//	}
func DisableAutoInit() {
	// No-op for backward compatibility
}

// initializeMacGo is called once to set up the app bundle
func initializeMacGo() {
	// Skip if already running inside an app bundle
	if isRunningInBundle() {
		debugf("Already running inside an app bundle")
		return
	}

	// Skip if relaunching is disabled
	if os.Getenv("MACGO_NO_RELAUNCH") == "1" {
		debugf("Relaunching disabled by environment variable")
		return
	}

	// By default, we always have app sandbox and user-selected file access
	// No need to skip anymore, because DefaultConfig initializes with entitlements

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		debugf("Failed to get executable path: %v", err)
		return
	}

	// Create app bundle
	appPath, err := createBundle(execPath)
	if err != nil {
		debugf("Failed to create app bundle: %v", err)
		return
	}

	// Only relaunch if enabled and not running in a test
	if DefaultConfig.Relaunch && !isTestMode() {
		// Determine which relaunch method to use
		if customReLaunchFunction != nil {
			// Prepare open command arguments
			args := []string{
				"-a", appPath,
				"--wait-apps",
			}

			// Pass original arguments
			if len(os.Args) > 1 {
				args = append(args, "--args")
				args = append(args, os.Args[1:]...)
			}

			// Use the custom relaunch function if available
			customReLaunchFunction(appPath, execPath, args)
		} else {
			// Use the default relaunch implementation
			relaunch(appPath, execPath)
		}
	}
}

// isRunningInBundle checks if the current process is already running
// inside a macOS application bundle.
func isRunningInBundle() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	// Check for .app/Contents/MacOS/ in the path
	return strings.Contains(execPath, ".app/Contents/MacOS/")
}

// IsInAppBundle returns true if the current process is running
// inside a macOS application bundle.
func IsInAppBundle() bool {
	return isRunningInBundle()
}

// Debug prints debug messages to stderr if MACGO_DEBUG=1 is set in the environment.
// This is a public version of debugf that can be used by extension modules.
func Debug(format string, args ...any) {
	debugf(format, args...)
}

// ReLaunchFunction is a function type for custom app relaunching
type ReLaunchFunction func(appPath, execPath string, args []string)

// Custom relaunch function that can be set by extension modules
var customReLaunchFunction ReLaunchFunction

// SetReLaunchFunction allows setting a custom relaunch function
// This is used by extension modules to provide custom functionality
func SetReLaunchFunction(fn ReLaunchFunction) {
	customReLaunchFunction = fn
}

// IsAutoInit is deprecated and always returns false.
// This function is kept for backward compatibility.
func IsAutoInit() bool {
	return false
}

// isTestMode checks if the process is running in a Go test
func isTestMode() bool {
	// Check if we're running as part of 'go test'
	for _, arg := range os.Args {
		if strings.Contains(arg, "go-build") && strings.Contains(arg, "test") {
			return true
		}
	}

	// Check for specific test environment variables
	if os.Getenv("MACGO_TEST") == "1" || os.Getenv("GO_TEST") == "1" {
		return true
	}

	// Check if TEST_TMPDIR is set (used by Go tests)
	if os.Getenv("TEST_TMPDIR") != "" {
		return true
	}

	return false
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		Relaunch:     true,
		Entitlements: map[Entitlement]bool{},
		PlistEntries: map[string]any{
			"LSUIElement": true, // Hide dock icon and app menu by default
		},
		AutoSign: true,
	}
}

// Configure applies the given configuration
func Configure(cfg *Config) {
	// Copy entitlements
	if cfg.Entitlements != nil {
		if DefaultConfig.Entitlements == nil {
			DefaultConfig.Entitlements = make(map[Entitlement]bool)
		}
		for k, v := range cfg.Entitlements {
			DefaultConfig.Entitlements[k] = v
		}
	}

	// Copy other fields
	if cfg.ApplicationName != "" {
		DefaultConfig.ApplicationName = cfg.ApplicationName
	}

	if cfg.BundleID != "" {
		DefaultConfig.BundleID = cfg.BundleID
	}

	// Set relaunch flag
	DefaultConfig.Relaunch = cfg.Relaunch

	// Copy plist entries
	if cfg.PlistEntries != nil {
		if DefaultConfig.PlistEntries == nil {
			DefaultConfig.PlistEntries = make(map[string]any)
		}
		for k, v := range cfg.PlistEntries {
			DefaultConfig.PlistEntries[k] = v
		}
	}

	// Set app template if provided
	if cfg.AppTemplate != nil {
		DefaultConfig.AppTemplate = cfg.AppTemplate
	}

	// Set custom app path if provided
	if cfg.CustomDestinationAppPath != "" {
		DefaultConfig.CustomDestinationAppPath = cfg.CustomDestinationAppPath
	}

	// Set auto-sign options
	DefaultConfig.AutoSign = cfg.AutoSign
	if cfg.SigningIdentity != "" {
		DefaultConfig.SigningIdentity = cfg.SigningIdentity
	}

	// Set keep temp flag
	DefaultConfig.KeepTemp = cfg.KeepTemp
}
