// Package location provides location access entitlement for macOS apps.
// Import this package with the blank identifier to enable location access:
//
//	import _ "github.com/tmc/misc/macgo/entitlements/location"
package location

import "github.com/tmc/misc/macgo/entitlements"

func init() {
	entitlements.Register(entitlements.EntLocation, true)
}
