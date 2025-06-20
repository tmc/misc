// Code generated by generate-all-modules. DO NOT EDIT.

/*
Package mariadb provides testctr support for mariadb containers.

This package was auto-generated from the testcontainers-go project.

# Default Configuration

The default configuration uses:
  - Image: mariadb:10
  - Port: 3306
  - Wait Strategy: "ready for connections"

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/modules/mariadb"
	)

	func TestWithMariadb(t *testing.T) {
		container := testctr.New(t, "mariadb:10", mariadb.Default())
		// Use container...
	}

# DSN Support

This module includes DSN (Data Source Name) support for easy database connections.
Full DSN provider implementation is pending.

*/
package mariadb
