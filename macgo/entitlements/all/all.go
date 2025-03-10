// Package all imports all macgo entitlements.
// Import this package with the blank identifier to enable all supported permissions:
//
//	import _ "github.com/tmc/misc/macgo/entitlements/all"
package all

import (
	_ "github.com/tmc/misc/macgo/entitlements/calendar"
	_ "github.com/tmc/misc/macgo/entitlements/camera"
	_ "github.com/tmc/misc/macgo/entitlements/contacts"
	_ "github.com/tmc/misc/macgo/entitlements/location"
	_ "github.com/tmc/misc/macgo/entitlements/mic"
	_ "github.com/tmc/misc/macgo/entitlements/photos"
	_ "github.com/tmc/misc/macgo/entitlements/reminders"
)
