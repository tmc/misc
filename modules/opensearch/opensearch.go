// Code generated by generate-all-modules. DO NOT EDIT.

package opensearch

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for opensearch containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("9200/tcp"),
		ctropts.WithWaitForLog("started", 30*time.Second),
		// TODO: Add DSN provider when fully implemented
	)
}

// WithPassword WithPassword sets the password for the OpenSearch container.
func WithPassword(value string) testctr.Option {
	return testctr.WithEnv("OPENSEARCH_PASSWORD", value)
}

// WithUsername WithUsername sets the username for the OpenSearch container.
func WithUsername(value string) testctr.Option {
	return testctr.WithEnv("OPENSEARCH_USER", value)
}
