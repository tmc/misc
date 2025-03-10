// Package contacts provides contacts/addressbook access entitlement for macOS apps.
// Import this package with the blank identifier to enable contacts access:
//
//	import _ "github.com/tmc/misc/macgo/entitlements/contacts"
package contacts

import "github.com/tmc/misc/macgo/entitlements"

func init() {
	entitlements.Register(entitlements.EntAddressBook, true)
}
