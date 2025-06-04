// Package elasticsearch provides testctr support for Elasticsearch containers.
// Elasticsearch is a distributed, RESTful search and analytics engine.
package elasticsearch

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for Elasticsearch containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("9200"),

		ctropts.WithWaitForLog("started", 30*time.Second),
	)
}

// WithPassword sets the elastic user password.
func WithPassword(value string) testctr.Option {
	return testctr.WithEnv("ELASTIC_PASSWORD", value)
}

// WithClusterName sets the cluster name.
func WithClusterName(value string) testctr.Option {
	return testctr.WithEnv("cluster.name", value)
}

// WithNodeName sets the node name.
func WithNodeName(value string) testctr.Option {
	return testctr.WithEnv("node.name", value)
}

// Additional helper functions can be added here for advanced Elasticsearch features:
// - Configuration file mounting
// - Index template management
// - Cluster configuration
// - Security and authentication settings
