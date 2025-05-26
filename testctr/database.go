package testctr

import (
	"testing"
)

// DSN returns a database connection string with an auto-namespaced database based on the test name
// The database is automatically created and cleaned up after the test
func (c *Container) DSN(t testing.TB) string {
	t.Helper()

	cfg, ok := c.config.(*containerConfig)
	if !ok || cfg.dsnProvider == nil {
		t.Fatalf("DSN() not supported for %s (no DSN provider configured)", c.image)
		return ""
	}

	// Generate database name from test name
	dbName := sanitizeDBName(t.Name())

	// Create the database
	dsn, err := cfg.dsnProvider.CreateDatabase(c, t, dbName)
	if err != nil {
		t.Fatalf("Failed to create database %s: %v", dbName, err)
	}

	// Register cleanup
	t.Cleanup(func() {
		if err := cfg.dsnProvider.DropDatabase(c, dbName); err != nil {
			t.Logf("Failed to drop database %s: %v", dbName, err)
		}
	})

	return dsn
}
