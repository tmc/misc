// Package camera provides camera access entitlement for macOS apps.
// Import this package with the blank identifier to enable camera access:
//
//	import _ "github.com/tmc/misc/macgo/camera"
package camera

import "github.com/tmc/misc/macgo"

func init() {
	macgo.RegisterEntitlement(string(macgo.PermCamera), true)
}