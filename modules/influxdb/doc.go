// Code generated by generate-all-modules. DO NOT EDIT.

/*
Package influxdb provides testctr support for influxdb containers.

This package was auto-generated from the testcontainers-go project.

# Default Configuration

The default configuration uses:
  - Image: influxdb:2.7
  - Port: 8086
  - Wait Strategy: "started"

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/modules/influxdb"
	)

	func TestWithInfluxdb(t *testing.T) {
		container := testctr.New(t, "influxdb:2.7", influxdb.Default())
		// Use container...
	}


*/
package influxdb
