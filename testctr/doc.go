// Package testctr provides zero-dependency test containers for Go.
//
// testctr runs containers for tests using docker/podman/nerdctl CLIs directly,
// with automatic cleanup and test isolation. The API is minimal by design,
// exporting only essential functionality.
//
// # Core API
//
// The main entry point is [New], which creates a [Container] with the given
// image and options. The [Container] provides methods for interacting with
// the running container, including [Container.Endpoint] for network access
// and [Container.Exec] for command execution.
//
//	func TestRedis(t *testing.T) {
//	    redis := testctr.New(t, "redis:7-alpine")
//	    
//	    // Get connection endpoint
//	    addr := redis.Endpoint("6379") // Returns "127.0.0.1:port"
//	    
//	    // Test with Redis...
//	}
//
// # Configuration Options
//
// Basic configuration is available through [Option] functions in this package:
//
//	container := testctr.New(t, "nginx",
//	    testctr.WithPort("80"),
//	    testctr.WithEnv("NGINX_HOST", "localhost"),
//	    testctr.WithCommand("nginx", "-g", "daemon off;"),
//	    testctr.WithFile("./config.conf", "/etc/nginx/nginx.conf"),
//	)
//
// Advanced options are available in the [github.com/tmc/misc/testctr/ctropts] package.
//
// # Database Testing
//
// testctr includes specialized support for database testing with automatic
// database creation and DSN generation:
//
//	import "github.com/tmc/misc/testctr/ctropts/postgres"
//	
//	func TestWithPostgres(t *testing.T) {
//	    pg := testctr.New(t, "postgres:15", postgres.Default())
//	    
//	    // DSN automatically creates test-isolated database
//	    db, err := sql.Open("postgres", pg.DSN(t))
//	    if err != nil {
//	        t.Fatal(err)
//	    }
//	    defer db.Close()
//	    
//	    // Test with database...
//	}
//
// The [DSNProvider] interface enables database-specific functionality.
// See [github.com/tmc/misc/testctr/ctropts/postgres], 
// [github.com/tmc/misc/testctr/ctropts/mysql], and
// [github.com/tmc/misc/testctr/ctropts/redis] for database-specific options.
//
// # Backend System
//
// testctr supports pluggable backends through the [github.com/tmc/misc/testctr/backend] package.
// The default CLI backend uses Docker/Podman/nerdctl commands, but alternative
// backends can be registered:
//
//	import "github.com/tmc/misc/testctr/backend"
//	
//	// Use alternative backend
//	container := testctr.New(t, "image", testctr.WithBackend(myBackend))
//
// # Test Flags and Environment
//
// testctr behavior can be controlled through command-line flags and environment variables:
//
//	-testctr.verbose         Stream container logs to test output
//	-testctr.keep-failed     Keep containers when tests fail (debugging)
//	-testctr.cleanup-old     Clean up old containers before test run
//	-testctr.warn-old        Warn about old containers
//	-testctr.cleanup-age     Age threshold for cleanup (default: 5m)
//	-testctr.label           Label prefix for containers (default: "testctr")
//	
//	TESTCTR_RUNTIME         Force specific runtime (docker/podman/nerdctl)
//	TESTCTR_VERBOSE         Enable verbose logging
//	TESTCTR_KEEP_FAILED     Keep failed test containers
//
// # Script Testing
//
// For script-based testing, see the [github.com/tmc/misc/testctr/testctrscript] 
// package, which provides [rsc.io/script] integration for running test scripts
// inside containers with Dockerfile support.
//
// # Thread Safety and Parallel Tests
//
// All testctr operations are thread-safe and designed for use with parallel tests.
// Each test gets isolated containers with unique names and ports:
//
//	func TestParallel(t *testing.T) {
//	    t.Parallel()
//	    redis := testctr.New(t, "redis:7-alpine") // Unique per test
//	    // ...
//	}
//
// # Cleanup and Resource Management
//
// testctr automatically cleans up containers when tests complete, handles
// container failures gracefully, and provides debugging support for failed tests.
// Old containers are automatically cleaned up based on the cleanup-age threshold.
package testctr
