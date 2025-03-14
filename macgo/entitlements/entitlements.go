// Package entitlements provides macOS entitlements for app sandbox and TCC permissions.
// This package centralizes all entitlement-related functionality for the macgo library.
package entitlements

import (
	"io"
	"os"

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
	if value {
		macgo.RequestEntitlement(ent)
	}
}

// TCC Permission functions

// SetCamera enables camera access
func SetCamera() {
	macgo.RequestEntitlement(EntCamera)
}

// SetMic enables microphone access
func SetMic() {
	macgo.RequestEntitlement(EntMicrophone)
}

// SetLocation enables location access
func SetLocation() {
	macgo.RequestEntitlement(EntLocation)
}

// SetContacts enables contacts access
func SetContacts() {
	macgo.RequestEntitlement(EntAddressBook)
}

// SetPhotos enables photos library access
func SetPhotos() {
	macgo.RequestEntitlement(EntPhotos)
}

// SetCalendar enables calendar access
func SetCalendar() {
	macgo.RequestEntitlement(EntCalendars)
}

// SetReminders enables reminders access
func SetReminders() {
	macgo.RequestEntitlement(EntReminders)
}

// App Sandbox functions

// SetAppSandbox enables App Sandbox
func SetAppSandbox() {
	macgo.RequestEntitlement(EntAppSandbox)
}

// SetNetworkClient enables outgoing network connections
// NOTE: This only affects Objective-C/Swift network APIs. Go's standard networking
// (net/http, etc.) bypasses these restrictions and will work regardless of this
// entitlement being present or not.
func SetNetworkClient() {
	macgo.RequestEntitlement(EntNetworkClient)
}

// SetNetworkServer enables incoming network connections
// NOTE: This only affects Objective-C/Swift network APIs. Go's standard networking
// (net.Listen, etc.) bypasses these restrictions and will work regardless of this
// entitlement being present or not.
func SetNetworkServer() {
	macgo.RequestEntitlement(EntNetworkServer)
}

// Device access functions

// SetBluetooth enables Bluetooth access
func SetBluetooth() {
	macgo.RequestEntitlement(EntBluetooth)
}

// SetUSB enables USB device access
func SetUSB() {
	macgo.RequestEntitlement(EntUSB)
}

// SetAudioInput enables audio input access
func SetAudioInput() {
	macgo.RequestEntitlement(EntAudioInput)
}

// SetPrinting enables printing capabilities
func SetPrinting() {
	macgo.RequestEntitlement(EntPrint)
}

// Hardened Runtime functions

// SetAllowJIT enables JIT compilation
func SetAllowJIT() {
	macgo.RequestEntitlement(EntAllowJIT)
}

// SetAllowUnsignedMemory allows unsigned executable memory
func SetAllowUnsignedMemory() {
	macgo.RequestEntitlement(EntAllowUnsignedExecutableMemory)
}

// SetAllowDyldEnvVars allows DYLD environment variables
func SetAllowDyldEnvVars() {
	macgo.RequestEntitlement(EntAllowDyldEnvVars)
}

// SetDisableLibraryValidation disables library validation
func SetDisableLibraryValidation() {
	macgo.RequestEntitlement(EntDisableLibraryValidation)
}

// SetDisableExecutablePageProtection disables executable memory page protection
func SetDisableExecutablePageProtection() {
	macgo.RequestEntitlement(EntDisableExecutablePageProtection)
}

// SetDebugger enables attaching to other processes as a debugger
func SetDebugger() {
	macgo.RequestEntitlement(EntDebugger)
}

// SetVirtualization enables virtualization support
func SetVirtualization() {
	macgo.RequestEntitlement(EntVirtualization)
}

// SetCustomEntitlement enables any arbitrary entitlement by its key string
func SetCustomEntitlement(key string, value bool) {
	if value {
		macgo.RequestEntitlement(key)
	}
}

// RegisterEntitlements registers entitlements directly from a JSON byte array
func RegisterEntitlements(data []byte) error {
	return macgo.LoadEntitlementsFromJSON(data)
}

// RegisterEntitlementsFromReader loads entitlements from a JSON reader and registers them
func RegisterEntitlementsFromReader(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return RegisterEntitlements(data)
}

// RegisterEntitlementsFromFile loads entitlements from a JSON file and registers them
func RegisterEntitlementsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return RegisterEntitlements(data)
}

// Convenience functions

// SetAllTCCPermissions enables all TCC permissions
func SetAllTCCPermissions() {
	SetCamera()
	SetMic()
	SetLocation()
	SetContacts()
	SetPhotos()
	SetCalendar()
	SetReminders()
}

// SetAllDeviceAccess enables all device access permissions
func SetAllDeviceAccess() {
	SetCamera()
	SetMic()
	SetBluetooth()
	SetUSB()
	SetAudioInput()
	SetPrinting()
}

// SetAllNetworking enables all networking permissions
func SetAllNetworking() {
	SetNetworkClient()
	SetNetworkServer()
}

// SetAll enables all basic TCC permissions
func SetAll() {
	SetAllTCCPermissions()
}

// RequestEntitlements requests multiple entitlements at once
func RequestEntitlements(entitlements ...interface{}) {
	macgo.RequestEntitlements(entitlements...)
}
