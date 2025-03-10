// Package entitlements provides macOS entitlements for app sandbox and TCC permissions.
// This package centralizes all entitlement-related functionality for the macgo library.
package entitlements

import (
	"io"

	"github.com/tmc/misc/macgo"
)

// Entitlement types from the main macgo package
type Entitlement = macgo.Entitlement

// Entitlement constants from the main macgo package
const (
	// App Sandbox entitlements
	EntAppSandbox    = macgo.EntAppSandbox
	EntNetworkClient = macgo.EntNetworkClient
	EntNetworkServer = macgo.EntNetworkServer

	// Device entitlements
	EntCamera     = macgo.EntCamera
	EntMicrophone = macgo.EntMicrophone
	EntBluetooth  = macgo.EntBluetooth
	EntUSB        = macgo.EntUSB
	EntAudioInput = macgo.EntAudioInput
	EntPrint      = macgo.EntPrint

	// Personal information entitlements
	EntAddressBook = macgo.EntAddressBook
	EntLocation    = macgo.EntLocation
	EntCalendars   = macgo.EntCalendars
	EntPhotos      = macgo.EntPhotos
	EntReminders   = macgo.EntReminders // Updated to use consistent naming

	// File entitlements
	EntUserSelectedReadOnly  = macgo.EntUserSelectedReadOnly
	EntUserSelectedReadWrite = macgo.EntUserSelectedReadWrite
	EntDownloadsReadOnly     = macgo.EntDownloadsReadOnly
	EntDownloadsReadWrite    = macgo.EntDownloadsReadWrite
	EntPicturesReadOnly      = macgo.EntPicturesReadOnly
	EntPicturesReadWrite     = macgo.EntPicturesReadWrite
	EntMusicReadOnly         = macgo.EntMusicReadOnly
	EntMusicReadWrite        = macgo.EntMusicReadWrite
	EntMoviesReadOnly        = macgo.EntMoviesReadOnly
	EntMoviesReadWrite       = macgo.EntMoviesReadWrite

	// Hardened Runtime entitlements
	EntAllowJIT                        = macgo.EntAllowJIT
	EntAllowUnsignedExecutableMemory   = macgo.EntAllowUnsignedExecutableMemory
	EntAllowDyldEnvVars                = macgo.EntAllowDyldEnvVars
	EntDisableLibraryValidation        = macgo.EntDisableLibraryValidation
	EntDisableExecutablePageProtection = macgo.EntDisableExecutablePageProtection
	EntDebugger                        = macgo.EntDebugger

	// Virtualization entitlements
	EntVirtualization = macgo.EntVirtualization
)

// Register registers an entitlement with the macgo system
func Register(ent Entitlement, value bool) {
	macgo.RegisterEntitlement(string(ent), value)
}

// TCC Permission functions - these map to the macgo functions

// SetCamera enables camera access
func SetCamera() {
	macgo.SetCamera()
}

// SetMic enables microphone access
func SetMic() {
	macgo.SetMic()
}

// SetLocation enables location access
func SetLocation() {
	macgo.SetLocation()
}

// SetContacts enables contacts access
func SetContacts() {
	macgo.SetContacts()
}

// SetPhotos enables photos library access
func SetPhotos() {
	macgo.SetPhotos()
}

// SetCalendar enables calendar access
func SetCalendar() {
	macgo.SetCalendar()
}

// SetReminders enables reminders access
func SetReminders() {
	macgo.SetReminders()
}

// App Sandbox functions

// SetAppSandbox enables App Sandbox
func SetAppSandbox() {
	macgo.SetAppSandbox()
}

// SetNetworkClient enables outgoing network connections
func SetNetworkClient() {
	macgo.SetNetworkClient()
}

// SetNetworkServer enables incoming network connections
func SetNetworkServer() {
	macgo.SetNetworkServer()
}

// Device access functions

// SetBluetooth enables Bluetooth access
func SetBluetooth() {
	macgo.SetBluetooth()
}

// SetUSB enables USB device access
func SetUSB() {
	macgo.SetUSB()
}

// SetAudioInput enables audio input access
func SetAudioInput() {
	macgo.SetAudioInput()
}

// SetPrinting enables printing capabilities
func SetPrinting() {
	macgo.SetPrinting()
}

// Hardened Runtime functions

// SetAllowJIT enables JIT compilation
func SetAllowJIT() {
	macgo.SetAllowJIT()
}

// SetAllowUnsignedMemory allows unsigned executable memory
func SetAllowUnsignedMemory() {
	macgo.SetAllowUnsignedMemory()
}

// SetAllowDyldEnvVars allows DYLD environment variables
func SetAllowDyldEnvVars() {
	macgo.SetAllowDyldEnvVars()
}

// SetDisableLibraryValidation disables library validation
func SetDisableLibraryValidation() {
	macgo.SetDisableLibraryValidation()
}

// SetDisableExecutablePageProtection disables executable memory page protection
func SetDisableExecutablePageProtection() {
	macgo.SetDisableExecutablePageProtection()
}

// SetDebugger enables attaching to other processes as a debugger
func SetDebugger() {
	macgo.SetDebugger()
}

// SetVirtualization enables virtualization support
func SetVirtualization() {
	macgo.SetVirtualization()
}

// SetCustomEntitlement enables any arbitrary entitlement by its key string
func SetCustomEntitlement(key string, value bool) {
	macgo.SetCustomEntitlement(key, value)
}

// RegisterEntitlementsFromReader loads entitlements from a JSON reader and registers them
func RegisterEntitlementsFromReader(r io.Reader) error {
	return macgo.RegisterEntitlementsFromReader(r)
}

// RegisterEntitlementsFromFile loads entitlements from a JSON file and registers them
func RegisterEntitlementsFromFile(path string) error {
	return macgo.RegisterEntitlementsFromFile(path)
}

// RegisterEntitlements registers entitlements directly from a JSON byte array
func RegisterEntitlements(data []byte) error {
	return macgo.RegisterEntitlements(data)
}

// Convenience functions

// SetAllTCCPermissions enables all TCC permissions
func SetAllTCCPermissions() {
	macgo.SetAllTCCPermissions()
}

// SetAllDeviceAccess enables all device access permissions
func SetAllDeviceAccess() {
	macgo.SetAllDeviceAccess()
}

// SetAllNetworking enables all networking permissions
func SetAllNetworking() {
	macgo.SetAllNetworking()
}

// SetAll enables all basic TCC permissions (legacy function)
func SetAll() {
	macgo.SetAll()
}

// WithEntitlements provides a simplified API for registering multiple entitlements at once
func WithEntitlements(entitlements map[string]bool) {
	macgo.WithEntitlements(entitlements)
}

// WithEntitlementList provides a simplified API for enabling a list of entitlements
func WithEntitlementList(entitlements ...string) {
	macgo.WithEntitlementList(entitlements...)
}
