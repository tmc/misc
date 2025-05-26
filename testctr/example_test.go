package testctr_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
	"github.com/tmc/misc/testctr/ctropts/postgres"
)

// Example demonstrates basic container creation.
func Example() {
	// In a real test, use t from testing.T
	t := &testing.T{}

	// Create a Redis container
	redis := testctr.New(t, "redis:7-alpine")

	// Get connection endpoint
	endpoint := redis.Endpoint("6379")
	_ = endpoint // Use with Redis client
}

// Example_parallel shows parallel test execution.
func Example_parallel() {
	// Tests using testctr are safe for t.Parallel()
	t := &testing.T{}
	t.Parallel()

	// Each test gets its own container instance
	redis := testctr.New(t, "redis:7-alpine")

	// Containers are automatically cleaned up
	_ = redis.Port("6379")
}

// Example_database demonstrates PostgreSQL with test isolation.
func Example_database() {
	t := &testing.T{}

	// Create PostgreSQL with optimized defaults
	pg := testctr.New(t, "postgres:15", postgres.Default())

	// Get test-specific database
	dsn := pg.DSN(t) // e.g., "postgres://postgres:password@127.0.0.1:32768/testexample_database"

	db, _ := sql.Open("postgres", dsn)
	defer db.Close()

	// Database is automatically dropped during cleanup
}

// Example_advanced shows advanced configuration.
func Example_advanced() {
	t := &testing.T{}

	// Create container with multiple options
	app := testctr.New(t, "myapp:latest",
		// Basic configuration
		testctr.WithPort("8080"),
		testctr.WithEnv("APP_ENV", "test"),

		// Advanced options from ctropts
		ctropts.WithWaitForHTTP("/health", "8080", 200, 30*time.Second),
		ctropts.WithLogs(), // Stream logs to test output
	)

	// Execute commands in container
	version := app.ExecSimple("myapp", "--version")
	_ = version
}

// Example_exec demonstrates command execution.
func Example_exec() {
	t := &testing.T{}
	pg := testctr.New(t, "postgres:15", postgres.Default())

	// Simple command execution
	version := pg.ExecSimple("postgres", "--version")
	_ = version

	// Full control with context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exitCode, output, err := pg.Exec(ctx, []string{"psql", "-U", "postgres", "-c", "SELECT 1"})
	if err != nil || exitCode != 0 {
		t.Fatalf("Command failed: %v, output: %s", err, output)
	}
}
