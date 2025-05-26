package testctr_tests_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts/mysql"
	"github.com/tmc/misc/testctr/ctropts/postgres"
	"github.com/tmc/misc/testctr/ctropts/redis"
)

func TestMySQL(t *testing.T) {
	t.Parallel()
	// This is all you need!
	c := testctr.New(t, "mysql:8", mysql.Default())

	// Log connection details for debugging
	t.Logf("MySQL endpoint: %s", c.Endpoint("3306"))

	// Connect to the database
	dsn := c.DSN(t)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// MySQL should already be ready due to wait strategies

	// Run a query
	var version string
	err = db.QueryRow("SELECT VERSION()").Scan(&version)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("MySQL version: %s", version)
}

func TestRedis(t *testing.T) {
	t.Parallel()
	// Simple as that
	c := testctr.New(t, "redis:7", redis.Default())

	// Use the endpoint
	t.Logf("Redis available at: %s", c.Endpoint("6379"))

	// Or run commands
	ctx := context.Background()
	exitCode, output, err := c.Exec(ctx, []string{"redis-cli", "PING"})
	if err != nil || exitCode != 0 {
		t.Fatalf("redis ping failed: %v (exit: %d)", err, exitCode)
	}

	t.Logf("Redis PING response: %s", output)
}

func TestPostgres(t *testing.T) {
	t.Parallel()
	// Just works
	c := testctr.New(t, "postgres:15", postgres.Default())

	dsn := c.DSN(t)
	t.Logf("PostgreSQL DSN: %s", dsn)
}
