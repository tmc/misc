// Package camera provides camera access entitlement for macOS apps.
// Import this package with the blank identifier to enable camera access:
//
//	import _ "github.com/tmc/misc/macgo/entitlements/camera"
package camera

import "github.com/tmc/misc/macgo/entitlements"

func init() {
	entitlements.Register(entitlements.EntCamera, true)
}
