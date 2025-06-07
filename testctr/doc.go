// Package testctr provides zero-dependency test containers for Go.
//
// testctr runs containers for tests using docker/podman/nerdctl CLIs directly,
// with automatic cleanup and test isolation. The API is minimal by design.
//
// Quick start:
//
//	func TestRedis(t *testing.T) {
//	    redis := testctr.New(t, "redis:7-alpine")
//
//	    client, _ := redis.NewClient(&redis.Options{
//	        Addr: redis.Endpoint("6379"),
//	    })
//
//	    // Test with Redis...
//	}
//
// Basic options:
//
//	container := testctr.New(t, "nginx",
//	    testctr.WithPort("80"),
//	    testctr.WithEnv("NGINX_HOST", "localhost"),
//	    testctr.WithCommand("nginx", "-g", "daemon off;"),
//	)
//
// Advanced options are in the ctropts package.
//
// Database testing:
//
//	import "github.com/tmc/misc/testctr/ctropts/postgres"
//
//	pg := testctr.New(t, "postgres:15", postgres.Default())
//	db, _ := sql.Open("postgres", pg.DSN(t)) // Creates test-isolated database
//
// Debugging:
//
//	-testctr.verbose         Detailed logging
//	-testctr.keep-failed     Keep failed test containers
//	TESTCTR_RUNTIME         Force specific runtime
//
// The API is intentionally minimal. Only essential functionality is exported.
// See the ctropts package for advanced options.
package testctr
