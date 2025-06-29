// Code generated by generate-all-modules. DO NOT EDIT.

/*
Package dolt provides testctr support for dolt containers.

This package was auto-generated from the testcontainers-go project.

# Default Configuration

The default configuration uses:
  - Image: dolthub/dolt-sql-server:latest
  - Port: 3306
  - Wait Strategy: "Server ready"

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/modules/dolt"
	)

	func TestWithDolt(t *testing.T) {
		container := testctr.New(t, "dolthub/dolt-sql-server:latest", dolt.Default())
		// Use container...
	}

# DSN Support

This module includes DSN (Data Source Name) support for easy database connections.
Full DSN provider implementation is pending.

*/
package dolt
