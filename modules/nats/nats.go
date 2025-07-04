// Code generated by generate-all-modules. DO NOT EDIT.

package nats

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for nats containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("4222"),
		ctropts.WithWaitForLog("Server is ready", 30*time.Second),
	)
}
