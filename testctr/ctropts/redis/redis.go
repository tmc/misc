package redis

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns sensible defaults for Redis testing
func Default() testctr.Option {
	return testctr.Options(
		// Wait for Redis to be ready
		ctropts.WithWaitForLog("Ready to accept connections", 10*time.Second),
		testctr.WithPort("6379"),
	)
}
