// Code generated by generate-all-modules. DO NOT EDIT.

package scylladb

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for scylladb containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("9042"),
		ctropts.WithWaitForLog("Started ScyllaDB", 30*time.Second),
		// TODO: Add DSN provider when fully implemented
	)
}
