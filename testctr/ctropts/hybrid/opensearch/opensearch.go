// Package opensearch provides testctr support for OpenSearch containers.
// OpenSearch is a distributed, RESTful search and analytics engine.
package opensearch

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for OpenSearch containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("9200"),

		ctropts.WithWaitForLog("started", 30*time.Second),
	)
}

// WithUsername sets the admin username.
func WithUsername(value string) testctr.Option {
	return testctr.WithEnv("OPENSEARCH_INITIAL_ADMIN_PASSWORD", value)
}

// WithPassword sets the admin password.
func WithPassword(value string) testctr.Option {
	return testctr.WithEnv("OPENSEARCH_INITIAL_ADMIN_PASSWORD", value)
}

// WithClusterName sets the cluster name.
func WithClusterName(value string) testctr.Option {
	return testctr.WithEnv("cluster.name", value)
}

// WithNodeName sets the node name.
func WithNodeName(value string) testctr.Option {
	return testctr.WithEnv("node.name", value)
}

// Additional helper functions can be added here for advanced OpenSearch features:
// - Configuration file mounting
// - Index template management
// - Cluster configuration
// - Security and authentication settings
