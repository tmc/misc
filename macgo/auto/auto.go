// Package auto provides automatic initialization for macgo.
//
// Import this package to automatically initialize macgo on startup:
//
//	import (
//	    _ "github.com/tmc/misc/macgo/auto"
//	)
package auto

import (
	"github.com/tmc/misc/macgo"
)

func init() {
	// Start macgo
	// This will create the app bundle and relaunch the application if necessary.
	macgo.Start()
}
