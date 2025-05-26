// Package testctr provides a minimal, zero-dependency wrapper for container-based testing in Go.
//
// testctr uses the Docker CLI directly instead of the Docker API, resulting in:
//   - Zero external dependencies
//   - Faster startup times
//   - Simpler debugging
//   - Support for multiple container runtimes (Docker, Podman, nerdctl, etc.)
//
// # Basic Usage
//
// Create a container with automatic cleanup:
//
//	func TestMySQL(t *testing.T) {
//	    mysql := testctr.New(t, "mysql:8")
//	    dsn := mysql.DSN(t)  // Get test-specific database
//	    
//	    db, err := sql.Open("mysql", dsn)
//	    // ... use database
//	}
//
// # Options
//
// Configure containers using the options pattern:
//
//	redis := testctr.New(t, "redis:7-alpine",
//	    testctr.WithEnv("REDIS_ARGS", "--maxmemory 256mb"),
//	    testctr.WithPort("6379"),
//	)
//
// Use backend-specific options for databases:
//
//	import "github.com/tmc/misc/testctr/ctropts/mysql"
//	
//	mysql := testctr.New(t, "mysql:8",
//	    mysql.WithPassword("secret"),
//	    mysql.WithDatabase("myapp"),
//	)
//
// Copy files into containers:
//
//	c := testctr.New(t, "alpine:latest",
//	    testctr.WithFile("./config.json", "/app/config.json"),
//	    testctr.WithFileMode("./script.sh", "/app/script.sh", 0755),
//	)
//
// # Container Methods
//
//	c.Host()           // Get container host (usually 127.0.0.1)
//	c.Port("3306")     // Get mapped port for container port
//	c.Endpoint("3306") // Get "host:port" string
//	c.Exec(ctx, cmd)   // Execute command in container
//	c.DSN(t)           // Get database connection string (if supported)
//
// # Command-Line Flags
//
//	-testctr.verbose         Stream container logs to test output
//	-testctr.keep-failed     Keep containers when tests fail (for debugging)
//	-testctr.warn-old        Warn about old testctr containers (default: true)
//	-testctr.cleanup-old     Clean up old testctr containers
//	-testctr.cleanup-age     Age threshold for cleanup/warning (default: 5m)
//	-testctr.max-concurrent  Max containers starting simultaneously (default: 20)
//	-testctr.create-delay    Delay between container creations (default: 200ms)
//
// # Environment Variables
//
// All flags can also be set via environment variables:
//
//	TESTCTR_VERBOSE=true
//	TESTCTR_KEEP_FAILED=true
//	TESTCTR_CLEANUP_OLD=true
//	TESTCTR_CLEANUP_AGE=10m
//
// # Parallel Testing
//
// All tests using testctr can safely use t.Parallel(). The library includes:
//   - Mutex-based coordination for database containers
//   - Configurable concurrent container limits
//   - Per-test database isolation via DSN()
//
// # Backend Support
//
// testctr includes a backend abstraction for pluggable container runtimes.
// The default implementation uses Docker/Podman CLI directly. A testcontainers-go
// backend is available as a separate module for those who need it.
//
// When using the testcontainers backend, you can access testcontainers-specific features:
//
//	import "github.com/tmc/misc/testctr/ctropts"
//	
//	c := testctr.New(t, "redis:7",
//	    ctropts.WithBackend("testcontainers"),
//	    ctropts.WithTestcontainersPrivileged(),
//	    ctropts.WithTestcontainersCustomizer(func(req interface{}) {
//	        // Full access to customize GenericContainerRequest
//	    }),
//	)
package testctr