// Package sandbox provides automatic initialization for macgo with app sandboxing.
//
// Import this package to automatically set up app sandboxing on startup:
//
//	import (
//	    _ "github.com/tmc/misc/macgo/auto/sandbox"
//	)
//
// This will automatically enable app sandboxing and create the app bundle.
// No user-selected file access is enabled by default. To include read-only
// file access, use github.com/tmc/misc/macgo/auto/sandbox/readonly.
package sandbox

import (
	"github.com/tmc/misc/macgo"
)

func init() {
	// Only enable app sandbox for basic security
	macgo.RequestEntitlements(macgo.EntAppSandbox)

	// Enable auto-initialization in macgo
	macgo.EnableAutoInit()

	// Start macgo
	macgo.Start()
}
