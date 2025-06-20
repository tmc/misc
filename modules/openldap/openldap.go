// Code generated by generate-all-modules. DO NOT EDIT.

package openldap

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for openldap containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("389"),
		ctropts.WithWaitForLog("slapd starting", 30*time.Second),
	)
}
