package testctr

import (
	"testing"
)

// DSN creates a test-isolated database and returns its connection string.
// The database name is derived from the test name. Requires a DSNProvider
// (configured via database options like postgres.Default()).
//
//	pg := testctr.New(t, "postgres:15", postgres.Default())
//	db, _ := sql.Open("postgres", pg.DSN(t))
func (c *Container) DSN(t testing.TB) string {
	t.Helper()

	if c.config == nil || c.config.dsnProvider == nil {
		t.Fatalf("DSN() not supported for image %s (no DSN provider configured via options)", c.image)
		return ""
	}

	// Generate database name from test name
	dbName := sanitizeDBName(t.Name())

	// Create the database
	dsn, err := c.config.dsnProvider.CreateDatabase(c, t, dbName)
	if err != nil {
		t.Fatalf("Failed to create database %q for test %q on image %s: %v", dbName, t.Name(), c.image, err)
		return "" // Should be unreachable due to t.Fatalf
	}

	// Register cleanup
	t.Cleanup(func() {
		t.Helper()
		if err := c.config.dsnProvider.DropDatabase(c, dbName); err != nil {
			// Logf is appropriate here as test might have already failed or passed.
			t.Logf("Failed to drop database %q for test %q on image %s: %v", dbName, t.Name(), c.image, err)
		}
	})

	return dsn
}
