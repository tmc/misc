package testctr_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts/mysql"
	"github.com/tmc/misc/testctr/ctropts/postgres"
)

func TestParallelRedis(t *testing.T) {
	t.Parallel()
	// Create a single Redis container for all parallel tests
	redis := testctr.New(t, "redis:7-alpine")

	// Run 10 parallel operations
	t.Run("parallel", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			i := i // capture loop variable
			t.Run(fmt.Sprintf("operation_%d", i), func(t *testing.T) {
				t.Parallel()

				key := fmt.Sprintf("key_%d", i)
				value := fmt.Sprintf("value_%d", i)

				// SET
				output := redis.ExecSimple("redis-cli", "SET", key, value)
				if !strings.Contains(output, "OK") {
					t.Errorf("SET failed: %s", output)
				}

				// GET
				result := redis.ExecSimple("redis-cli", "GET", key)
				if result != value {
					t.Errorf("Expected %s, got %s", value, result)
				}
			})
		}
	})
}

func TestConcurrentContainers(t *testing.T) {
	t.Parallel()
	// Test creating multiple containers concurrently
	var wg sync.WaitGroup
	containers := make([]*testctr.Container, 20)

	start := time.Now()
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			containers[i] = testctr.New(t, "alpine:latest",
				testctr.WithCommand("echo", fmt.Sprintf("container %d", i)),
			)
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Verify all containers were created
	successCount := 0
	for i, c := range containers {
		if c == nil {
			t.Errorf("Container %d was not created", i)
		} else {
			successCount++
		}
	}

	t.Logf("Created %d/%d containers in %v", successCount, len(containers), elapsed)
}

func TestDatabaseSubtests(t *testing.T) {
	t.Parallel()
	// Test database functionality with MySQL
	m := testctr.New(t, "mysql:8", mysql.Default())

	// Get DSN which creates a test-specific database
	dsn := m.DSN(t)
	t.Logf("MySQL DSN: %s", dsn)

	// Verify database was created
	dbName := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	_, output, _ := m.Exec(context.Background(), []string{
		"mysql", "-uroot", "-ptest", "-e", "SHOW DATABASES",
	})
	if !contains(output, dbName) {
		t.Errorf("Expected database %s to exist, databases: %s", dbName, output)
	}

	// Create a table in the test database
	_, _, err := m.Exec(context.Background(), []string{
		"mysql", "-uroot", "-ptest", dbName, "-e",
		"CREATE TABLE test_table (id INT PRIMARY KEY)",
	})
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
}

func TestPostgreSQLSubtests(t *testing.T) {
	t.Parallel()
	pg := testctr.New(t, "postgres:15", postgres.Default())

	// Use DSN functionality
	dsn := pg.DSN(t)
	t.Logf("PostgreSQL DSN: %s", dsn)

	// Verify database was created with test name
	dbName := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	_, output, _ := pg.Exec(context.Background(), []string{
		"psql", "-U", "postgres", "-t", "-c", "SELECT datname FROM pg_database WHERE datname LIKE 'test%'",
	})
	if !contains(output, dbName) {
		t.Errorf("Expected database %s to exist, databases: %s", dbName, output)
	}
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
