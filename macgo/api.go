// Simplified API for macgo
package macgo

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
)

// WithEntitlements adds multiple entitlements at once
// All entitlements will be enabled (set to true)
//
// Example:
//
//	macgo.WithEntitlements(
//	    macgo.EntCamera,
//	    macgo.EntMicrophone,
//	    macgo.EntAppSandbox,
//	    "com.apple.security.virtualization",
//	)
func WithEntitlements(entitlements ...string) {
	DefaultConfig.WithEntitlements(entitlements...)
}

// WithEntitlement adds a single entitlement
// The entitlement will be enabled (set to true)
//
// Example:
//
//	macgo.WithEntitlement(macgo.EntCamera)
//
func WithEntitlement(entitlement string) {
	if DefaultConfig.Entitlements == nil {
		DefaultConfig.Entitlements = make(map[Entitlement]bool)
	}
	DefaultConfig.Entitlements[Entitlement(entitlement)] = true
}

// WithAppName sets the app name
func WithAppName(name string) {
	DefaultConfig.ApplicationName = name
}

// WithBundleID sets the bundle identifier
func WithBundleID(bundleID string) {
	DefaultConfig.BundleID = bundleID
}

// WithKeepTemp enables keeping temporary app bundles
func WithKeepTemp() {
	DefaultConfig.KeepTemp = true
}

// WithNoRelaunch disables auto-relaunching
func WithNoRelaunch() {
	DefaultConfig.Relaunch = false
}

// WithDebug enables debug output
func WithDebug() {
	os.Setenv("MACGO_DEBUG", "1")
}

// WithCustomAppBundle sets a custom app bundle template from embedded filesystem
//
// Example with go:embed:
//
//	//go:embed template/*
//	var appTemplate embed.FS
//
//	func init() {
//	    macgo.WithCustomAppBundle(appTemplate)
//	}
func WithCustomAppBundle(template fs.FS) {
	DefaultConfig.AppTemplate = template
}

// WithSigning enables app bundle signing with an optional identity
// If identity is empty, the default identity will be used
func WithSigning(identity string) {
	DefaultConfig.AutoSign = true
	if identity != "" {
		DefaultConfig.SigningIdentity = identity
	}
}

// WithEntitlementsFromJSON registers entitlements from JSON data
// This is useful with go:embed for embedding entitlements configuration
//
// Example:
//
//	//go:embed entitlements.json
//	var entitlementsData []byte
//
//	func init() {
//	    macgo.WithEntitlementsFromJSON(entitlementsData)
//	}
func WithEntitlementsFromJSON(data []byte) error {
	var entitlements map[string]bool
	if err := json.Unmarshal(data, &entitlements); err != nil {
		return fmt.Errorf("macgo: parse entitlements JSON: %w", err)
	}

	for key, value := range entitlements {
		DefaultConfig.Entitlements[Entitlement(key)] = value
	}

	return nil
}

// WithPlistEntry adds a custom entry to the Info.plist file
func WithPlistEntry(key string, value any) {
	DefaultConfig.WithPlistEntry(key, value)
}

// WithPlistEntries adds multiple custom entries to the Info.plist file
func (c *Config) WithPlistEntry(key string, value any) {
	if c.PlistEntries == nil {
		c.PlistEntries = make(map[string]any)
	}
	c.PlistEntries[key] = value
}

// WithEntitlements adds multiple entitlements at once
func (c *Config) WithEntitlements(entitlements ...string) {
	if c.Entitlements == nil {
		c.Entitlements = make(map[Entitlement]bool)
	}

	for _, ent := range entitlements {
		c.Entitlements[Entitlement(ent)] = true
	}
}
