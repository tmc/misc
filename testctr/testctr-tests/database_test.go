package testctr_tests

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts/mysql"
	"github.com/tmc/misc/testctr/ctropts/postgres"
)

func TestCleanDSNAPI(t *testing.T) {
	t.Parallel()

	// Create a MySQL container once
	c := testctr.New(t, "mysql:8", mysql.Default())

	// Each test/subtest automatically gets its own database
	t.Run("test1", func(t *testing.T) {
		t.Parallel()

		// Automatically creates database named after the test
		dsn := c.DSN(t)
		t.Logf("Test 1 DSN: %s", dsn)

		// Use it with database/sql
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			t.Skip("MySQL driver not available")
		}
		defer db.Close()

		// Create a table in our isolated database
		_, err = db.Exec("CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(50))")
		if err != nil {
			t.Fatalf("Failed to create table: %v", err)
		}

		// Insert data
		_, err = db.Exec("INSERT INTO users (id, name) VALUES (1, 'test1')")
		if err != nil {
			t.Fatalf("Failed to insert: %v", err)
		}
	})

	t.Run("test2", func(t *testing.T) {
		t.Parallel()

		// Different test gets different database automatically
		dsn := c.DSN(t)
		t.Logf("Test 2 DSN: %s", dsn)

		db, err := sql.Open("mysql", dsn)
		if err != nil {
			t.Skip("MySQL driver not available")
		}
		defer db.Close()

		// This database is isolated - no users table here
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'users'").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}
		if count != 0 {
			t.Error("users table should not exist in test2's database")
		}
	})

	// Nested subtests also work
	t.Run("parent", func(t *testing.T) {
		t.Parallel()

		dsn := c.DSN(t)
		t.Logf("Parent DSN: %s", dsn)

		t.Run("child1", func(t *testing.T) {
			t.Parallel()

			// Child gets its own database too
			childDSN := c.DSN(t)
			t.Logf("Child 1 DSN: %s", childDSN)

			if dsn == childDSN {
				t.Error("Child should have different database than parent")
			}
		})

		t.Run("child2", func(t *testing.T) {
			t.Parallel()

			childDSN := c.DSN(t)
			t.Logf("Child 2 DSN: %s", childDSN)
		})
	})
}

func TestPostgreSQLDSN(t *testing.T) {
	t.Parallel()

	pg := testctr.New(t, "postgres:15", postgres.Default())

	t.Run("auto_database", func(t *testing.T) {
		t.Parallel()
		// Don't run subtest in parallel - container is shared with parent

		dsn := pg.DSN(t)
		t.Logf("PostgreSQL DSN: %s", dsn)

		// Give PostgreSQL a moment to stabilize after database creation
		time.Sleep(2 * time.Second)

		db, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Skip("PostgreSQL driver not available")
		}
		defer db.Close()

		// Verify we're in the right database
		var dbName string
		err = db.QueryRow("SELECT current_database()").Scan(&dbName)
		if err != nil {
			t.Fatalf("Failed to get database name: %v", err)
		}

		t.Logf("Connected to database: %s", dbName)

		// Should be named after our test
		if dbName != "testpostgresqldsn_auto_database" {
			t.Errorf("Expected database name based on test, got %s", dbName)
		}
	})
}

// Example of how clean the API is now
func TestRealWorldExample(t *testing.T) {
	t.Parallel()

	c := testctr.New(t, "mysql:8", mysql.Default())

	// That's it! Just call DSN(t) anywhere you need a database
	dsn := c.DSN(t)

	// Use it...
	_ = dsn

	// Each parallel test gets its own database automatically
	for i := 0; i < 3; i++ {
		i := i
		t.Run(fmt.Sprintf("worker_%d", i), func(t *testing.T) {
			t.Parallel()

			// Each worker gets its own isolated database
			workerDSN := c.DSN(t)
			t.Logf("Worker %d DSN: %s", i, workerDSN)
		})
	}
}
