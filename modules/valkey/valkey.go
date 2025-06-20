// Code generated by generate-all-modules. DO NOT EDIT.

package valkey

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for valkey containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("6379"),
		ctropts.WithWaitForLog("Ready to accept connections", 30*time.Second),
	)
}
