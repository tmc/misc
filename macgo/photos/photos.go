// Package photos provides photos library access entitlement for macOS apps.
// Import this package with the blank identifier to enable photos library access:
//
//	import _ "github.com/tmc/misc/macgo/photos"
package photos

import "github.com/tmc/misc/macgo"

func init() {
	macgo.RegisterEntitlement(string(macgo.PermPhotos), true)
}