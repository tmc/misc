// Package readonly provides automatic initialization for macgo with app sandboxing
// and read-only file access to user-selected files.
//
// Import this package to automatically set up app sandboxing with read-only access:
//
//	import (
//	    _ "github.com/tmc/misc/macgo/auto/sandbox/readonly"
//	)
//
// This will automatically enable app sandboxing with read-only access to
// user-selected files and create the app bundle.
package readonly

import (
	"github.com/tmc/misc/macgo"
)

func init() {
	// Enable app sandbox with read-only file access
	macgo.RequestEntitlements(
		macgo.EntAppSandbox,
		macgo.EntUserSelectedReadOnly,
	)
	// Start macgo
	macgo.Start()
}
