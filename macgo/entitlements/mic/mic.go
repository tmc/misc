// Package mic provides microphone access entitlement for macOS apps.
// Import this package with the blank identifier to enable microphone access:
//
//	import _ "github.com/tmc/misc/macgo/entitlements/mic"
package mic

import "github.com/tmc/misc/macgo/entitlements"

func init() {
	entitlements.Register(entitlements.EntMicrophone, true)
}
