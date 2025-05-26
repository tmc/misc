package testctr_tests_test

import (
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts/mysql"
)

func TestMySQLWithOptions(t *testing.T) {
	t.Parallel()
	c := testctr.New(t, "mysql:8",
		mysql.Default(),
		mysql.WithDatabase("myapp"),
		mysql.WithUser("appuser", "apppass"),
	)

	// Test connection
	dsn := c.DSN(t)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Verify we can query
	var version string
	err = db.QueryRow("SELECT VERSION()").Scan(&version)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("MySQL version: %s", version)
}
