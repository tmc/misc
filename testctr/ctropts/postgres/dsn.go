package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DSNProvider implements testctr.DSNProvider for PostgreSQL.
// It handles the creation and formatting of DSNs for test-specific databases.
type DSNProvider struct{}

// dbCreationMutex serializes database creation to prevent overwhelming PostgreSQL.
var dbCreationMutex sync.Mutex

// CreateDatabase creates a new database within the PostgreSQL container for the current test.
// It retries several times if PostgreSQL is not immediately ready.
// The database name is provided, typically generated from the test name.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	// Serialize database creation to prevent overwhelming PostgreSQL
	dbCreationMutex.Lock()
	defer dbCreationMutex.Unlock()
	t.Helper()

	// PostgreSQL sometimes needs multiple attempts due to initialization timing
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 { // Add a small delay before retrying, not for the first attempt
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
		t.Logf("Attempting to create PostgreSQL database %s (attempt %d)", dbName, attempt+1)

		// First try to drop if it exists (from previous failed run)
		// Use default "postgres" user for admin tasks
		// TODO(tmc): use user/pass from config rather than hardcoding "postgres"
		_, _, _ = c.Exec(context.Background(), []string{
			"dropdb", "-U", "postgres", "--if-exists", dbName,
		})

		// Create database using createdb command which handles connection better
		exitCode, output, err := c.Exec(context.Background(), []string{
			"createdb", "-U", "postgres", dbName,
		})

		if err != nil && strings.Contains(err.Error(), "container") && strings.Contains(err.Error(), "is not running") {
			return "", fmt.Errorf("failed to create database %s: PostgreSQL container has stopped unexpectedly: %w", dbName, err)
		}
		if strings.Contains(output, "container") && strings.Contains(output, "is not running") {
			return "", fmt.Errorf("failed to create database %s: PostgreSQL container has stopped unexpectedly (output indicates container failure)", dbName)
		}

		if err != nil { // General exec error
			lastErr = fmt.Errorf("failed to execute createdb for %s: %w (exit code: %d, output: %s)", dbName, err, exitCode, output)
			continue
		}
		if exitCode != 0 { // Command executed but failed
			// Check if DB already exists, which is fine
			if strings.Contains(output, "already exists") {
				t.Logf("Database %s already exists, proceeding.", dbName)
				// Verify connection to existing DB
				if verifyErr := p.verifyDatabaseConnection(c, t, dbName); verifyErr != nil {
					lastErr = fmt.Errorf("database %s exists but failed to verify connection: %w", dbName, verifyErr)
					continue
				}
				return p.FormatDSN(c, dbName), nil
			}
			lastErr = fmt.Errorf("createdb for %s failed with exit code %d: %s", dbName, exitCode, output)
			continue
		}

		// Verify the database was created and PostgreSQL is still alive
		if err := p.verifyDatabaseConnection(c, t, dbName); err != nil {
			lastErr = fmt.Errorf("failed to verify database %s creation: %w", dbName, err)
			continue
		}

		return p.FormatDSN(c, dbName), nil
	}

	return "", fmt.Errorf("failed to create PostgreSQL database %s after 5 attempts: %w", dbName, lastErr)
}

// verifyDatabaseConnection checks if a connection to the specified database can be established.
func (p DSNProvider) verifyDatabaseConnection(c *testctr.Container, t testing.TB, dbName string) error {
	t.Helper()
	// TODO(tmc): use user/pass from config rather than hardcoding "postgres"
	exitCode, output, err := c.Exec(context.Background(), []string{
		"psql", "-U", "postgres", "-d", dbName, "-c", "SELECT 1",
	})
	if err != nil {
		return fmt.Errorf("psql exec error verifying database %s: %w (exit code: %d, output: %s)", dbName, err, exitCode, output)
	}
	if exitCode != 0 {
		return fmt.Errorf("psql command failed verifying database %s (exit code %d): %s", dbName, exitCode, output)
	}
	return nil
}

// DropDatabase removes the specified database from the PostgreSQL container.
// This is typically called during test cleanup.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	// TODO(tmc): use user/pass from config rather than hardcoding "postgres"
	_, _, err := c.Exec(context.Background(), []string{
		"dropdb", "-U", "postgres", "--if-exists", dbName,
	})
	return err
}

// FormatDSN returns a DSN string for connecting to the specified database
// in the PostgreSQL container.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	// TODO(tmc): use user/pass from config rather than hardcoding "postgres"
	return fmt.Sprintf("postgresql://postgres@%s/%s?sslmode=disable", c.Endpoint("5432"), dbName)
}

// WithDSN returns a testctr.Option that configures the container to use this DSNProvider for PostgreSQL.
// This enables the `container.DSN(t)` method for PostgreSQL containers.
func WithDSN() testctr.Option {
	return ctropts.WithDSNProvider(DSNProvider{})
}
