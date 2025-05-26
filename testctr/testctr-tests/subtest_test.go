package testctr_tests_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
	mysqlpkg "github.com/tmc/misc/testctr/ctropts/mysql"
	postgrespkg "github.com/tmc/misc/testctr/ctropts/postgres"
)

func TestSubtestSupport(t *testing.T) {
	t.Parallel()
	// Create a MySQL container with log streaming
	mysql := testctr.New(t, "mysql:8",
		mysqlpkg.Default(),
		ctropts.WithLogs(), // This will stream logs if -testctr.verbose is set
	)

	// Run subtests with isolated databases
	t.Run("UserService", func(t *testing.T) {
		t.Parallel()
		dsn := mysql.DSN(t)
		t.Logf("Using DSN: %s", dsn)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		// Create tables in isolated database
		_, err = db.Exec(`CREATE TABLE users (
			id INT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(100)
		)`)
		if err != nil {
			t.Fatal(err)
		}

		// Run tests...
		_, err = db.Exec("INSERT INTO users (name) VALUES (?)", "Alice")
		if err != nil {
			t.Fatal(err)
		}

		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("expected 1 user, got %d", count)
		}
	})

	t.Run("ProductService", func(t *testing.T) {
		t.Parallel()
		dsn := mysql.DSN(t)
		t.Logf("Using DSN: %s", dsn)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		// This is a completely isolated database
		_, err = db.Exec(`CREATE TABLE products (
			id INT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(100)
		)`)
		if err != nil {
			t.Fatal(err)
		}

		// The users table doesn't exist here
		_, err = db.Exec("SELECT * FROM users")
		if err == nil {
			t.Error("expected error querying users table in isolated database")
		}
	})
}

func TestPostgreSQLSubtest(t *testing.T) {
	t.Parallel()
	pg := testctr.New(t, "postgres:15", postgrespkg.Default())

	// Use the automatic database creation
	dsn := pg.DSN(t)

	// Give PostgreSQL a moment to stabilize after database creation
	time.Sleep(2 * time.Second)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Verify we're connected
	var dbName string
	err = db.QueryRow("SELECT current_database()").Scan(&dbName)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Connected to database: %s", dbName)
}

func TestExecWithLogs(t *testing.T) {
	t.Parallel()
	// This will show logs if run with -testctr.verbose
	c := testctr.New(t, "alpine:latest",
		testctr.WithCommand("sh", "-c", "echo 'Starting...'; sleep 2; echo 'Done!'"),
		ctropts.WithLogs(),
	)

	ctx := context.Background()
	exitCode, output, err := c.Exec(ctx, []string{"echo", "Hello from exec"})
	if err != nil || exitCode != 0 {
		t.Fatalf("exec failed: %v (exit: %d)", err, exitCode)
	}

	t.Logf("Exec output: %s", output)
}
