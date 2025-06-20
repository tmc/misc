// Code generated by generate-all-modules. DO NOT EDIT.

package yugabytedb

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for yugabytedb containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("5433"),
		ctropts.WithWaitForLog("Started YugabyteDB", 30*time.Second),
		// TODO: Add DSN provider when fully implemented
	)
}
