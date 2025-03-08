// Package reminders provides reminders access entitlement for macOS apps.
// Import this package with the blank identifier to enable reminders access:
//
//	import _ "github.com/tmc/misc/macgo/reminders"
package reminders

import "github.com/tmc/misc/macgo"

func init() {
	macgo.RegisterEntitlement(string(macgo.PermReminders), true)
}