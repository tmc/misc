// Package macgo automatically creates and launches macOS app bundles
// to gain TCC permissions for command-line Go programs.
//
// macgo solves a fundamental problem with Go programs on macOS: command-line
// binaries cannot access protected resources (camera, microphone, etc.) because
// macOS requires apps to be properly bundled and signed to request permissions.
// This package bridges that gap by automatically wrapping your Go binary in an
// app bundle with the necessary entitlements.
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
//
// The package works by:
// 1. Detecting if running as a regular binary
// 2. Creating an app bundle with requested entitlements
// 3. Relaunching the process inside the app bundle
// 4. Forwarding all I/O and signals between processes
package macgo

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"
)

// Entitlement is a type for macOS entitlement identifiers.
// Entitlements are special permissions that allow apps to access protected
// resources and perform privileged operations on macOS.
type Entitlement string

// Entitlements is a map of entitlement identifiers to boolean values.
// When true, the entitlement is granted; when false, it is explicitly denied.
type Entitlements map[Entitlement]bool

// Available app sandbox entitlements
const (
	// EntAppSandbox enables the macOS App Sandbox, which provides a secure environment
	// by restricting access to system resources. Required for many other entitlements.
	EntAppSandbox Entitlement = "com.apple.security.app-sandbox"

	// Network entitlements
	// NOTE: These network entitlements only affect Objective-C/Swift network APIs.
	// Go's standard networking (net/http, etc.) bypasses these restrictions and will
	// work regardless of these entitlements being present or not. To properly restrict
	// network access in Go applications, additional measures are required.

	// EntNetworkClient allows the app to make outgoing network connections
	EntNetworkClient Entitlement = "com.apple.security.network.client"
	// EntNetworkServer allows the app to listen for incoming network connections
	EntNetworkServer Entitlement = "com.apple.security.network.server"

	// Device entitlements - provide access to hardware devices

	// EntCamera allows access to the camera (requires user approval via TCC)
	EntCamera Entitlement = "com.apple.security.device.camera"
	// EntMicrophone allows access to the microphone (requires user approval via TCC)
	EntMicrophone Entitlement = "com.apple.security.device.microphone"
	// EntBluetooth allows access to Bluetooth devices
	EntBluetooth Entitlement = "com.apple.security.device.bluetooth"
	// EntUSB allows access to USB devices
	EntUSB Entitlement = "com.apple.security.device.usb"
	// EntAudioInput allows access to audio input devices
	EntAudioInput Entitlement = "com.apple.security.device.audio-input"
	// EntPrint allows access to printing services
	EntPrint Entitlement = "com.apple.security.print"

	// Personal information entitlements - provide access to user data

	// EntAddressBook allows access to the user's contacts (requires user approval via TCC)
	EntAddressBook Entitlement = "com.apple.security.personal-information.addressbook"
	// EntLocation allows access to location services (requires user approval via TCC)
	EntLocation Entitlement = "com.apple.security.personal-information.location"
	// EntCalendars allows access to calendar data (requires user approval via TCC)
	EntCalendars Entitlement = "com.apple.security.personal-information.calendars"
	// EntPhotos allows access to the Photos library (requires user approval via TCC)
	EntPhotos Entitlement = "com.apple.security.personal-information.photos-library"
	// EntReminders allows access to reminders (requires user approval via TCC)
	EntReminders Entitlement = "com.apple.security.personal-information.reminders"

	// File entitlements - control access to specific file system locations

	// EntUserSelectedReadOnly allows read-only access to files selected by the user
	EntUserSelectedReadOnly Entitlement = "com.apple.security.files.user-selected.read-only"
	// EntUserSelectedReadWrite allows read-write access to files selected by the user
	EntUserSelectedReadWrite Entitlement = "com.apple.security.files.user-selected.read-write"
	// EntDownloadsReadOnly allows read-only access to the Downloads folder
	EntDownloadsReadOnly Entitlement = "com.apple.security.files.downloads.read-only"
	// EntDownloadsReadWrite allows read-write access to the Downloads folder
	EntDownloadsReadWrite Entitlement = "com.apple.security.files.downloads.read-write"
	// EntPicturesReadOnly allows read-only access to the Pictures folder
	EntPicturesReadOnly Entitlement = "com.apple.security.assets.pictures.read-only"
	// EntPicturesReadWrite allows read-write access to the Pictures folder
	EntPicturesReadWrite Entitlement = "com.apple.security.assets.pictures.read-write"
	// EntMusicReadOnly allows read-only access to the Music folder
	EntMusicReadOnly Entitlement = "com.apple.security.assets.music.read-only"
	// EntMusicReadWrite allows read-write access to the Music folder
	EntMusicReadWrite Entitlement = "com.apple.security.assets.music.read-write"
	// EntMoviesReadOnly allows read-only access to the Movies folder
	EntMoviesReadOnly Entitlement = "com.apple.security.assets.movies.read-only"
	// EntMoviesReadWrite allows read-write access to the Movies folder
	EntMoviesReadWrite Entitlement = "com.apple.security.assets.movies.read-write"

	// Hardened Runtime entitlements - control code execution and security features

	// EntAllowJIT allows Just-In-Time compilation (required for some interpreters)
	EntAllowJIT Entitlement = "com.apple.security.cs.allow-jit"
	// EntAllowUnsignedExecutableMemory allows unsigned executable memory pages
	EntAllowUnsignedExecutableMemory Entitlement = "com.apple.security.cs.allow-unsigned-executable-memory"
	// EntAllowDyldEnvVars allows DYLD environment variables to be used
	EntAllowDyldEnvVars Entitlement = "com.apple.security.cs.allow-dyld-environment-variables"
	// EntDisableLibraryValidation disables library validation allowing unsigned libraries
	EntDisableLibraryValidation Entitlement = "com.apple.security.cs.disable-library-validation"
	// EntDisableExecutablePageProtection disables executable page protection
	EntDisableExecutablePageProtection Entitlement = "com.apple.security.cs.disable-executable-page-protection"
	// EntDebugger allows other processes to attach as a debugger
	EntDebugger Entitlement = "com.apple.security.cs.debugger"

	// Virtualization entitlements

	// EntVirtualization allows the app to use virtualization features
	EntVirtualization Entitlement = "com.apple.security.virtualization"

	// Custom entitlements: If you need to add more entitlements, you can simply cast string to Entitlement:
	// Example: Entitlement("com.apple.security.some.new.entitlement")
)

// DefaultConfig is the default configuration for macgo.
// This configuration is used unless overridden by the Configure function
// or environment variables. It sets reasonable defaults for most use cases.
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

// Config provides a way to customize the app bundle behavior.
// It controls all aspects of the app bundle creation, from naming
// and identification to security entitlements and visual appearance.
type Config struct {
	// ApplicationName overrides the default app name (executable name).
	// If empty, the base name of the executable will be used.
	ApplicationName string

	// BundleID overrides the default bundle identifier.
	// If empty, a default ID like "com.macgo.appname" will be generated.
	// Bundle IDs should follow reverse-DNS notation (e.g., "com.company.app").
	BundleID string

	// Entitlements contains the security entitlements to request.
	// These control what system resources and APIs the app can access.
	Entitlements Entitlements

	// PlistEntries contains additional Info.plist entries.
	// Use this to set app metadata, capabilities, and behavior flags.
	PlistEntries map[string]any

	// Relaunch controls whether to auto-relaunch (default: true).
	// When true, the process will relaunch inside the app bundle to gain
	// TCC permissions. Set to false to disable this behavior.
	// Note: Without relaunching, entitlements won't take effect.
	Relaunch bool

	// CustomDestinationAppPath specifies a custom path for the app bundle.
	// If empty, the bundle will be created in $GOPATH/bin or a temp directory.
	CustomDestinationAppPath string

	// KeepTemp prevents temporary bundles from being cleaned up.
	// Useful for debugging or when using 'go run' with persistent bundles.
	KeepTemp bool

	// AppTemplate provides a custom app bundle template.
	// This should be a directory structure with placeholder files
	// that will be filled in during app bundle creation.
	// Use with go:embed to embed an entire app structure.
	// Placeholders: {{BundleName}}, {{BundleExecutable}}, {{BundleIdentifier}}
	AppTemplate fs.FS

	// AutoSign enables automatic codesigning of the app bundle.
	// When true, the app bundle will be code signed to enable proper functionality
	// of entitlements. Required for many security features to work correctly.
	AutoSign bool

	// SigningIdentity specifies the identity to use for codesigning.
	// If empty, ad-hoc signing ("-") will be used when AutoSign is true.
	// Use "Developer ID Application: Your Name" for distribution.
	SigningIdentity string
}

// AddEntitlement adds an entitlement to the configuration.
// The entitlement will be enabled (set to true) in the entitlements plist.
func (c *Config) AddEntitlement(e Entitlement) {
	if c.Entitlements == nil {
		c.Entitlements = make(map[Entitlement]bool)
	}
	c.Entitlements[e] = true
}

// AddPermission adds a TCC permission to the configuration (legacy method).
// Deprecated: Use AddEntitlement instead for consistency.
func (c *Config) AddPermission(p Entitlement) {
	c.AddEntitlement(p)
}

// AddPlistEntry adds a custom entry to the Info.plist file.
// This allows customization of app metadata and behavior beyond
// what macgo provides through specific methods.
func (c *Config) AddPlistEntry(key string, value any) {
	if c.PlistEntries == nil {
		c.PlistEntries = make(map[string]any)
	}
	c.PlistEntries[key] = value
}

var (
	initOnce sync.Once
	// globalCtx is the context for the entire macgo lifecycle
	globalCtx context.Context
	// globalCancel allows cancellation of all macgo operations
	globalCancel context.CancelFunc
)

// init sets up the default configuration from environment variables.
// It does not create the app bundle or relaunch the application.
// For automatic initialization, import "github.com/tmc/misc/macgo/auto".
// This function runs automatically when the package is imported.
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
	StartWithContext(context.Background())
}

// StartWithContext initializes macgo with a custom context.
// This allows for better lifecycle management and cancellation support.
// The context will be used for all macgo operations.
func StartWithContext(ctx context.Context) {
	initOnce.Do(func() {
		// Create a cancellable context from the provided one
		globalCtx, globalCancel = context.WithCancel(ctx)
		debugf("macgo: initializing app bundle with context...")
		initializeMacGo()
	})
}

// Initialize is an alias for Start() for backward compatibility
// For new code, use Start() instead
func Initialize() {
	Start()
}

// Stop gracefully stops all macgo operations and cleans up resources.
// This cancels the global context and triggers cleanup of any running operations.
func Stop() {
	if globalCancel != nil {
		debugf("macgo: stopping all operations...")
		globalCancel()
		globalCancel = nil
		globalCtx = nil
	}
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

// initializeMacGo is called once to set up the app bundle.
// This is the main initialization function that orchestrates the entire
// bundle creation and relaunching process. It handles:
// - Detection of existing app bundle execution
// - Creation of new app bundles when needed
// - Process relaunching with proper I/O and signal forwarding
func initializeMacGo() {
	// Skip if already running inside an app bundle
	if isRunningInBundle() {
		debugf("Already running inside an app bundle")
		setupChildProcessTeeWriter()
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
			// Use robust signal handling by default with context
			debugf("Using robust signal handling (default)")
			if globalCtx != nil {
				relaunchWithIORedirectionContext(globalCtx, appPath, execPath)
			} else {
				relaunchWithIORedirection(appPath, execPath)
			}
		}
	}
}

// isRunningInBundle checks if the current process is already running
// inside a macOS application bundle by looking for the telltale
// ".app/Contents/MacOS/" path structure in the executable path.
func isRunningInBundle() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	// Check for .app/Contents/MacOS/ in the path
	return strings.Contains(execPath, ".app/Contents/MacOS/")
}

// IsInAppBundle returns true if the current process is running
// inside a macOS application bundle. This is a public API that
// applications can use to detect their execution context.
func IsInAppBundle() bool {
	return isRunningInBundle()
}

// Debug prints debug messages to stderr if MACGO_DEBUG=1 is set in the environment.
// This is a public version of debugf that can be used by extension modules
// and applications for consistent debug logging.
func Debug(format string, args ...any) {
	debugf(format, args...)
}

// ReLaunchFunction is a function type for custom app relaunching.
// It allows extension modules to provide alternative relaunch mechanisms
// while maintaining compatibility with the core macgo functionality.
// Parameters:
//   - appPath: Path to the created app bundle
//   - execPath: Path to the original executable
//   - args: Command-line arguments to pass to the relaunched process
type ReLaunchFunction func(appPath, execPath string, args []string)

// Custom relaunch function that can be set by extension modules
var customReLaunchFunction ReLaunchFunction

// SetReLaunchFunction allows setting a custom relaunch function.
// This is used by extension modules (like improvedsignals) to provide
// enhanced functionality while maintaining the core relaunch behavior.
func SetReLaunchFunction(fn ReLaunchFunction) {
	customReLaunchFunction = fn
}

// IsAutoInit is deprecated and always returns false.
// This function is kept for backward compatibility.
func IsAutoInit() bool {
	return false
}

// isTestMode checks if the process is running in a Go test.
// This prevents automatic relaunching during testing which could
// interfere with test execution and reporting.
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

// NewConfig creates a new configuration with default values.
// Use this when you need complete control over the configuration
// rather than modifying the DefaultConfig.
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

// Configure applies the given configuration to DefaultConfig.
// This merges the provided configuration with the existing defaults,
// allowing partial configuration updates. Call this before Start().
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

// setupChildProcessTeeWriter sets up TeeWriter for stdout/stderr in the child process.
// This is used for debugging to capture output from the relaunched process
// when running inside the app bundle.
func setupChildProcessTeeWriter() {
	if !isDebugEnabled() {
		return
	}

	debugf("Setting up TeeWriter for child process (PID: %d)", os.Getpid())

	// Create debug log files for the child process
	if stdoutFile, err := createChildDebugLogFile("stdout"); err == nil {
		// Note: We can't directly replace os.Stdout, but we create the debug log for manual testing
		debugf("Created stdout debug log file")
		defer stdoutFile.Close()
	} else {
		debugf("Failed to create stdout debug log: %v", err)
	}

	if stderrFile, err := createChildDebugLogFile("stderr"); err == nil {
		// Note: We can't directly replace os.Stderr, but we create the debug log for manual testing
		debugf("Created stderr debug log file")
		defer stderrFile.Close()
	} else {
		debugf("Failed to create stderr debug log: %v", err)
	}
}

// createChildDebugLogFile creates a debug log file for the child process
func createChildDebugLogFile(streamName string) (*os.File, error) {
	logPath := fmt.Sprintf("/tmp/macgo-debug-child-%s-%d.txt", streamName, os.Getpid())
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	debugf("Created child %s debug log: %s", streamName, logPath)

	// Write a header to the log file
	fmt.Fprintf(file, "=== macgo child process %s log (PID: %d) ===\n", streamName, os.Getpid())
	fmt.Fprintf(file, "Started at: %s\n", getCurrentTimestamp())
	fmt.Fprintf(file, "Args: %v\n", os.Args)
	fmt.Fprintf(file, "Working directory: %s\n", getCurrentWorkingDir())
	fmt.Fprintf(file, "=== Start of %s output ===\n", streamName)

	return file, nil
}

// Helper functions for child process logging
func getCurrentTimestamp() string {
	return time.Now().Format(time.RFC3339)
}

func getCurrentWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return dir
}
