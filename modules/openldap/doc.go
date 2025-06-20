// Code generated by generate-all-modules. DO NOT EDIT.

/*
Package openldap provides testctr support for openldap containers.

This package was auto-generated from the testcontainers-go project.

# Default Configuration

The default configuration uses:
  - Image: osixia/openldap:1.5.0
  - Port: 389
  - Wait Strategy: "slapd starting"

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/modules/openldap"
	)

	func TestWithOpenldap(t *testing.T) {
		container := testctr.New(t, "osixia/openldap:1.5.0", openldap.Default())
		// Use container...
	}


*/
package openldap
