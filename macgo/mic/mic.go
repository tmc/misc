// Package mic provides microphone access entitlement for macOS apps.
// Import this package with the blank identifier to enable microphone access:
//
//	import _ "github.com/tmc/misc/macgo/mic"
package mic

import "github.com/tmc/misc/macgo"

func init() {
	macgo.RegisterEntitlement(string(macgo.PermMic), true)
}