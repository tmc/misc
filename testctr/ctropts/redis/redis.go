// Package redis provides Redis-specific options for testctr.
package redis

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DefaultRedisImage is the default Redis image used by `redis.Default()`.
const DefaultRedisImage = "redis:7-alpine"

// Default returns a testctr.Option that configures a Redis container with sensible defaults for testing.
// It includes a wait strategy for readiness and exposes the standard Redis port.
// Image used: redis.DefaultRedisImage.
func Default() testctr.Option {
	return testctr.Options(
		// Wait for Redis to be ready
		ctropts.WithWaitForLog("Ready to accept connections", 10*time.Second),
		testctr.WithPort("6379"), // Expose the standard Redis port
	)
}
