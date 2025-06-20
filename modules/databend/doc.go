// Code generated by generate-all-modules. DO NOT EDIT.

/*
Package databend provides testctr support for databend containers.

This package was auto-generated from the testcontainers-go project.

# Default Configuration

The default configuration uses:
  - Image: datafuselabs/databend:v1.0.0
  - Port: 8000
  - Wait Strategy: "Databend HTTP server listening"

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/modules/databend"
	)

	func TestWithDatabend(t *testing.T) {
		container := testctr.New(t, "datafuselabs/databend:v1.0.0", databend.Default())
		// Use container...
	}

# DSN Support

This module includes DSN (Data Source Name) support for easy database connections.
Full DSN provider implementation is pending.

*/
package databend
