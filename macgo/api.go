// Package macgo provides a simplified API for creating macOS app bundles with entitlements.
package macgo

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
)

// RequestEntitlements adds multiple entitlements at once.
// All entitlements will be enabled (set to true).
//
// Example:
//
//	macgo.RequestEntitlements(
//	    macgo.EntCamera,
//	    macgo.EntMicrophone,
//	    macgo.EntAppSandbox,
//	    "com.apple.security.virtualization",
//	)
//
// This is the preferred method for requesting entitlements.
func RequestEntitlements(entitlements ...interface{}) {
	for _, ent := range entitlements {
		var entStr string
		switch e := ent.(type) {
		case string:
			entStr = e
		case Entitlement:
			entStr = string(e)
		default:
			continue
		}
		if DefaultConfig.Entitlements == nil {
			DefaultConfig.Entitlements = make(map[Entitlement]bool)
		}
		DefaultConfig.Entitlements[Entitlement(entStr)] = true
	}
}

// RequestEntitlement adds a single entitlement.
// The entitlement will be enabled (set to true).
//
// Example:
//
//	macgo.RequestEntitlement(macgo.EntCamera)
func RequestEntitlement(entitlement interface{}) {
	var entStr string
	switch e := entitlement.(type) {
	case string:
		entStr = e
	case Entitlement:
		entStr = string(e)
	default:
		return
	}

	if DefaultConfig.Entitlements == nil {
		DefaultConfig.Entitlements = make(map[Entitlement]bool)
	}
	DefaultConfig.Entitlements[Entitlement(entStr)] = true
}

// EnableDockIcon enables showing the application in the dock and app switcher
// By default, macgo applications run as background applications (LSUIElement=true)
func EnableDockIcon() {
	if DefaultConfig.PlistEntries == nil {
		DefaultConfig.PlistEntries = make(map[string]any)
	}
	DefaultConfig.PlistEntries["LSUIElement"] = false
}

// SetAppName sets the app name
func SetAppName(name string) {
	DefaultConfig.ApplicationName = name
}

// SetBundleID sets the bundle identifier
func SetBundleID(bundleID string) {
	DefaultConfig.BundleID = bundleID
}

// EnableKeepTemp enables keeping temporary app bundles
func EnableKeepTemp() {
	DefaultConfig.KeepTemp = true
}

// DisableRelaunch disables auto-relaunching
func DisableRelaunch() {
	DefaultConfig.Relaunch = false
}

// EnableDebug enables debug output
func EnableDebug() {
	os.Setenv("MACGO_DEBUG", "1")
}

// SetCustomAppBundle sets a custom app bundle template from embedded filesystem
//
// Example with go:embed:
//
//	//go:embed template/*
//	var appTemplate embed.FS
//
//	func init() {
//	    macgo.SetCustomAppBundle(appTemplate)
//	}
func SetCustomAppBundle(template fs.FS) {
	DefaultConfig.AppTemplate = template
}

// EnableSigning enables app bundle signing with an optional identity.
// If identity is empty, ad-hoc signing ("-") will be used.
func EnableSigning(identity string) {
	DefaultConfig.AutoSign = true
	if identity != "" {
		DefaultConfig.SigningIdentity = identity
	}
}

// LoadEntitlementsFromJSON registers entitlements from JSON data.
// This is useful with go:embed for embedding entitlements configuration.
//
// Example:
//
//	//go:embed entitlements.json
//	var entitlementsData []byte
//
//	func init() {
//	    err := macgo.LoadEntitlementsFromJSON(entitlementsData)
//	    if err != nil {
//	        log.Printf("Failed to load entitlements: %v", err)
//	    }
//	}
func LoadEntitlementsFromJSON(data []byte) error {
	var entitlements map[string]bool
	if err := json.Unmarshal(data, &entitlements); err != nil {
		return fmt.Errorf("macgo: parse entitlements JSON: %w", err)
	}

	for key, value := range entitlements {
		DefaultConfig.Entitlements[Entitlement(key)] = value
	}

	return nil
}

// AddPlistEntry adds a custom entry to the Info.plist file
func AddPlistEntry(key string, value any) {
	DefaultConfig.AddPlistEntry(key, value)
}

// SetIconFile sets a custom icon file for the app bundle
// If not set, it defaults to "/System/./Library/CoreServices/CoreTypes.bundle/Contents/Resources/ExecutableBinaryIcon.icns"
func SetIconFile(iconPath string) {
	DefaultConfig.AddPlistEntry("CFBundleIconFile", iconPath)
}

// RequestEntitlements adds multiple entitlements at once to a Config
func (c *Config) RequestEntitlements(entitlements ...interface{}) {
	if c.Entitlements == nil {
		c.Entitlements = make(map[Entitlement]bool)
	}

	for _, ent := range entitlements {
		var entStr string
		switch e := ent.(type) {
		case string:
			entStr = e
		case Entitlement:
			entStr = string(e)
		default:
			continue
		}
		c.Entitlements[Entitlement(entStr)] = true
	}
}
