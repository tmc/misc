package testctr_tests

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts/mysql"
	"github.com/tmc/misc/testctr/ctropts/postgres"
	"github.com/tmc/misc/testctr/ctropts/redis"
)

// TestMySQLParallel tests MySQL with 10 parallel subtests and nested sub-subtests
func TestMySQLParallel(t *testing.T) {
	t.Parallel()

	// Create a single MySQL container to be shared by all subtests
	c := testctr.New(t, "mysql:8", mysql.Default())

	// Create 10 parallel subtests
	for i := 0; i < 10; i++ {
		i := i // capture loop variable
		t.Run(fmt.Sprintf("worker_%d", i), func(t *testing.T) {
			t.Parallel()

			// Each worker gets its own database
			dsn := c.DSN(t)
			t.Logf("Worker %d DSN: %s", i, dsn)

			// Connect to the database
			db, err := sql.Open("mysql", dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			// Create a table
			_, err = db.Exec(fmt.Sprintf("CREATE TABLE worker_%d (id INT PRIMARY KEY, data VARCHAR(100))", i))
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}

			// Insert some data
			_, err = db.Exec(fmt.Sprintf("INSERT INTO worker_%d (id, data) VALUES (?, ?)", i), i, fmt.Sprintf("data_%d", i))
			if err != nil {
				t.Fatalf("Failed to insert data: %v", err)
			}

			// Verify the data
			var data string
			err = db.QueryRow(fmt.Sprintf("SELECT data FROM worker_%d WHERE id = ?", i), i).Scan(&data)
			if err != nil {
				t.Fatalf("Failed to query data: %v", err)
			}
			if data != fmt.Sprintf("data_%d", i) {
				t.Errorf("Expected data_%d, got %s", i, data)
			}

			// Create sub-subtests
			for j := 0; j < 3; j++ {
				j := j
				t.Run(fmt.Sprintf("task_%d", j), func(t *testing.T) {
					t.Parallel()

					// Sub-subtests get their own database too
					subDSN := c.DSN(t)
					t.Logf("Worker %d Task %d DSN: %s", i, j, subDSN)

					// Ensure it's different from parent
					if subDSN == dsn {
						t.Error("Sub-subtest should have different database than parent")
					}

					subDB, err := sql.Open("mysql", subDSN)
					if err != nil {
						t.Fatal(err)
					}
					defer subDB.Close()

					// Create a task-specific table
					_, err = subDB.Exec(fmt.Sprintf("CREATE TABLE task_%d_%d (id INT PRIMARY KEY, result TEXT)", i, j))
					if err != nil {
						t.Fatalf("Failed to create task table: %v", err)
					}

					// Simulate some work
					time.Sleep(10 * time.Millisecond)

					// Write result
					_, err = subDB.Exec(fmt.Sprintf("INSERT INTO task_%d_%d (id, result) VALUES (?, ?)", i, j), j, fmt.Sprintf("task_%d_%d_complete", i, j))
					if err != nil {
						t.Fatalf("Failed to insert task result: %v", err)
					}
				})
			}
		})
	}
}

// TestPostgreSQLParallel tests PostgreSQL with 10 parallel subtests and nested sub-subtests
func TestPostgreSQLParallel(t *testing.T) {
	t.Parallel()

	// Create a single PostgreSQL container to be shared by all subtests
	c := testctr.New(t, "postgres:15", postgres.Default())

	// Use a WaitGroup to track when all databases are created
	var wg sync.WaitGroup

	// Create 10 parallel subtests
	for i := 0; i < 10; i++ {
		i := i
		wg.Add(1)
		t.Run(fmt.Sprintf("worker_%d", i), func(t *testing.T) {
			t.Parallel()
			defer wg.Done()

			// Each worker gets its own database
			dsn := c.DSN(t)
			t.Logf("Worker %d DSN: %s", i, dsn)

			// Give PostgreSQL a moment to stabilize
			time.Sleep(100 * time.Millisecond)

			// Connect to the database
			db, err := sql.Open("postgres", dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			// Verify connection
			err = db.Ping()
			if err != nil {
				t.Fatalf("Failed to ping database: %v", err)
			}

			// Create a schema
			_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA worker_%d", i))
			if err != nil {
				t.Fatalf("Failed to create schema: %v", err)
			}

			// Create a table in the schema
			_, err = db.Exec(fmt.Sprintf("CREATE TABLE worker_%d.data (id SERIAL PRIMARY KEY, value TEXT)", i))
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}

			// Insert data
			_, err = db.Exec(fmt.Sprintf("INSERT INTO worker_%d.data (value) VALUES ($1)", i), fmt.Sprintf("worker_%d_data", i))
			if err != nil {
				t.Fatalf("Failed to insert data: %v", err)
			}

			// Create sub-subtests
			for j := 0; j < 3; j++ {
				j := j
				t.Run(fmt.Sprintf("task_%d", j), func(t *testing.T) {
					t.Parallel()

					// Sub-subtests get their own database
					subDSN := c.DSN(t)
					t.Logf("Worker %d Task %d DSN: %s", i, j, subDSN)

					// Give PostgreSQL a moment
					time.Sleep(100 * time.Millisecond)

					subDB, err := sql.Open("postgres", subDSN)
					if err != nil {
						t.Fatal(err)
					}
					defer subDB.Close()

					// Create task table
					_, err = subDB.Exec(fmt.Sprintf("CREATE TABLE task_%d_%d (id SERIAL PRIMARY KEY, completed_at TIMESTAMP DEFAULT NOW())", i, j))
					if err != nil {
						t.Fatalf("Failed to create task table: %v", err)
					}

					// Record completion
					var id int
					err = subDB.QueryRow(fmt.Sprintf("INSERT INTO task_%d_%d DEFAULT VALUES RETURNING id", i, j)).Scan(&id)
					if err != nil {
						t.Fatalf("Failed to insert task completion: %v", err)
					}
					t.Logf("Task %d_%d completed with id %d", i, j, id)
				})
			}
		})
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
	}()
}

// TestRedisParallel tests Redis with 10 parallel subtests and nested sub-subtests
func TestRedisParallel(t *testing.T) {
	t.Parallel()

	// Create a single Redis container to be shared by all subtests
	c := testctr.New(t, "redis:7-alpine", redis.Default())

	// Create 10 parallel subtests
	for i := 0; i < 10; i++ {
		i := i
		t.Run(fmt.Sprintf("worker_%d", i), func(t *testing.T) {
			t.Parallel()

			// Use Redis CLI to set and get values
			ctx := context.Background()

			// Set a key specific to this worker
			key := fmt.Sprintf("worker:%d:data", i)
			value := fmt.Sprintf("worker_%d_value", i)

			exitCode, output, err := c.Exec(ctx, []string{"redis-cli", "SET", key, value})
			if err != nil || exitCode != 0 {
				t.Fatalf("Failed to SET key: %v (exit: %d, output: %s)", err, exitCode, output)
			}

			// Verify the value
			exitCode, output, err = c.Exec(ctx, []string{"redis-cli", "GET", key})
			if err != nil || exitCode != 0 {
				t.Fatalf("Failed to GET key: %v (exit: %d)", err, exitCode)
			}
			// Trim newline from redis-cli output
			output = strings.TrimSpace(output)
			if output != value {
				t.Errorf("Expected %s, got %s", value, output)
			}

			// Increment a counter
			counterKey := fmt.Sprintf("worker:%d:counter", i)
			exitCode, _, err = c.Exec(ctx, []string{"redis-cli", "INCR", counterKey})
			if err != nil || exitCode != 0 {
				t.Fatalf("Failed to INCR counter: %v (exit: %d)", err, exitCode)
			}

			// Create sub-subtests
			for j := 0; j < 3; j++ {
				j := j
				t.Run(fmt.Sprintf("task_%d", j), func(t *testing.T) {
					t.Parallel()

					// Set task-specific data
					taskKey := fmt.Sprintf("worker:%d:task:%d", i, j)
					taskValue := fmt.Sprintf("task_%d_%d_data", i, j)

					exitCode, _, err := c.Exec(ctx, []string{"redis-cli", "SET", taskKey, taskValue})
					if err != nil || exitCode != 0 {
						t.Fatalf("Failed to SET task key: %v (exit: %d)", err, exitCode)
					}

					// Add to a sorted set for tracking completion order
					score := fmt.Sprintf("%d.%d", i, j)
					exitCode, _, err = c.Exec(ctx, []string{"redis-cli", "ZADD", "task_completion", score, fmt.Sprintf("worker_%d_task_%d", i, j)})
					if err != nil || exitCode != 0 {
						t.Fatalf("Failed to ZADD: %v (exit: %d)", err, exitCode)
					}

					// Use a list to track task execution
					exitCode, _, err = c.Exec(ctx, []string{"redis-cli", "LPUSH", fmt.Sprintf("worker:%d:tasks", i), fmt.Sprintf("task_%d", j)})
					if err != nil || exitCode != 0 {
						t.Fatalf("Failed to LPUSH: %v (exit: %d)", err, exitCode)
					}

					// Simulate some work
					time.Sleep(5 * time.Millisecond)
				})
			}
		})
	}
}

// TestMixedParallel tests all three databases running in parallel
func TestMixedParallel(t *testing.T) {
	t.Parallel()

	// Create containers for all three databases
	mysqlC := testctr.New(t, "mysql:8", mysql.Default())
	postgresC := testctr.New(t, "postgres:15", postgres.Default())
	redisC := testctr.New(t, "redis:7-alpine", redis.Default())

	// Test MySQL
	t.Run("mysql", func(t *testing.T) {
		t.Parallel()
		for i := 0; i < 3; i++ {
			i := i
			t.Run(fmt.Sprintf("test_%d", i), func(t *testing.T) {
				t.Parallel()
				dsn := mysqlC.DSN(t)
				db, err := sql.Open("mysql", dsn)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()

				var version string
				err = db.QueryRow("SELECT VERSION()").Scan(&version)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("MySQL test %d connected to version: %s", i, version)
			})
		}
	})

	// Test PostgreSQL
	t.Run("postgres", func(t *testing.T) {
		t.Parallel()
		for i := 0; i < 3; i++ {
			i := i
			t.Run(fmt.Sprintf("test_%d", i), func(t *testing.T) {
				t.Parallel()
				dsn := postgresC.DSN(t)
				time.Sleep(100 * time.Millisecond) // Give PostgreSQL a moment
				db, err := sql.Open("postgres", dsn)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()

				var version string
				err = db.QueryRow("SELECT version()").Scan(&version)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("PostgreSQL test %d connected: %s", i, version[:50]+"...")
			})
		}
	})

	// Test Redis
	t.Run("redis", func(t *testing.T) {
		t.Parallel()
		for i := 0; i < 3; i++ {
			i := i
			t.Run(fmt.Sprintf("test_%d", i), func(t *testing.T) {
				t.Parallel()
				ctx := context.Background()
				exitCode, output, err := redisC.Exec(ctx, []string{"redis-cli", "INFO", "server"})
				if err != nil || exitCode != 0 {
					t.Fatalf("Failed to get Redis info: %v (exit: %d)", err, exitCode)
				}
				t.Logf("Redis test %d connected, output length: %d bytes", i, len(output))
			})
		}
	})
}