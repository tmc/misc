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
	"io/fs"
	"os"
	"strings"
)

// Entitlement is a type for macOS entitlement identifiers
type Entitlement string

// Entitlements is a map of entitlement identifiers to boolean values
type Entitlements map[Entitlement]bool

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
	Entitlements: make(map[Entitlement]bool),
	PlistEntries: make(map[string]any),
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
	// This requires the developer to have appropriate certificates
	// installed and will use the default signing identity if not specified
	AutoSign bool

	// SigningIdentity specifies the identity to use for codesigning
	// If empty, the default identity will be used when AutoSign is true
	SigningIdentity string
}

// AddEntitlement adds an entitlement to the configuration
func (c *Config) AddEntitlement(e Entitlement) {
	c.Entitlements[e] = true
}

// AddPermission adds a TCC permission to the configuration (legacy method)
func (c *Config) AddPermission(p Entitlement) {
	c.AddEntitlement(p)
}

// AddPlistEntry adds a custom entry to the Info.plist file
func (c *Config) AddPlistEntry(key string, value any) {
	c.PlistEntries[key] = value
}

// init is the default initialization function, called on import
// It sets up the default configuration and initializes the app bundle.
// It also reads environment variables to configure the app bundle.
// This function is called only once when the package is imported.
func init() {

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

	// Initialize once on import
	initializeMacGo()
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

	// Skip if no entitlements are requested
	if DefaultConfig.Entitlements == nil || len(DefaultConfig.Entitlements) == 0 {
		debugf("No entitlements requested, skipping app bundle creation")
		return
	}

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
		relaunch(appPath, execPath)
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
		Entitlements: make(map[Entitlement]bool),
		PlistEntries: make(map[string]any),
		AutoSign:     true,
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
