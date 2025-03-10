// Package calendar provides calendar access entitlement for macOS apps.
// Import this package with the blank identifier to enable calendar access:
//
//	import _ "github.com/tmc/misc/macgo/entitlements/calendar"
package calendar

import "github.com/tmc/misc/macgo/entitlements"

func init() {
	entitlements.Register(entitlements.EntCalendars, true)
}
