// Code generated by generate-all-modules. DO NOT EDIT.

/*
Package k6 provides testctr support for k6 containers.

This package was auto-generated from the testcontainers-go project.

# Default Configuration

The default configuration uses:
  - Image: grafana/k6:0.45.0
  - Port: 6565
  - Wait Strategy: "k6 archive server running"

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/modules/k6"
	)

	func TestWithK6(t *testing.T) {
		container := testctr.New(t, "grafana/k6:0.45.0", k6.Default())
		// Use container...
	}


*/
package k6
