// Code generated by generate-all-modules. DO NOT EDIT.

/*
Package kafka provides testctr support for kafka containers.

This package was auto-generated from the testcontainers-go project.

# Default Configuration

The default configuration uses:
  - Image: confluentinc/cp-kafka:7.5.0
  - Port: 9092
  - Wait Strategy: "started"

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/modules/kafka"
	)

	func TestWithKafka(t *testing.T) {
		container := testctr.New(t, "confluentinc/cp-kafka:7.5.0", kafka.Default())
		// Use container...
	}


*/
package kafka
