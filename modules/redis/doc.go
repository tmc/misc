// Code generated by generate-all-modules. DO NOT EDIT.

/*
Package redis provides testctr support for redis containers.

This package was auto-generated from the testcontainers-go project.

# Default Configuration

The default configuration uses:
  - Image: redis:7
  - Port: 6379/tcp
  - Wait Strategy: "Ready to accept connections"

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/modules/redis"
	)

	func TestWithRedis(t *testing.T) {
		container := testctr.New(t, "redis:7", redis.Default())
		// Use container...
	}


# Configuration Options

  - WithTLS: WithTLS sets the TLS configuration for the redis container, setting the 6380/tcp port to listen on for TLS connections and using a secure URL (rediss://).
  - WithConfigFile: WithConfigFile sets the config file to be used for the redis container, and sets the command to run the redis server using the passed config file
  - WithLogLevel: WithLogLevel sets the log level for the redis server process See "[RedisModule_Log]" for more information. [RedisModule_Log]: https://redis.io/docs/reference/modules/modules-api-ref/#redismodule_log
  - WithSnapshotting: WithSnapshotting sets the snapshotting configuration for the redis server process. You can configure Redis to have it save the dataset every N seconds if there are at least M changes in the dataset. This method allows Redis to benefit from copy-on-write semantics. See [Snapshotting] for more information. [Snapshotting]: https://redis.io/docs/management/persistence/#snapshotting
*/
package redis
