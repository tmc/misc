// Package location provides location services access entitlement for macOS apps.
// Import this package with the blank identifier to enable location services access:
//
//	import _ "github.com/tmc/misc/macgo/location"
package location

import "github.com/tmc/misc/macgo"

func init() {
	macgo.RegisterEntitlement(string(macgo.PermLocation), true)
}