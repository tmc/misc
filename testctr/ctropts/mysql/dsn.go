package mysql

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

// DSNProvider implements testctr.DSNProvider for MySQL.
// It handles the creation and formatting of DSNs for test-specific databases.
type DSNProvider struct{}

// dbCreationMutex serializes database creation to prevent overwhelming MySQL.
var dbCreationMutex sync.Mutex

// CreateDatabase creates a new database within the MySQL container for the current test.
// It retries several times if MySQL is not immediately ready.
// The database name is provided, typically generated from the test name.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	// Serialize database creation to prevent MySQL from being overwhelmed
	dbCreationMutex.Lock()
	defer dbCreationMutex.Unlock()
	t.Helper()

	// Try connecting with a retry loop since MySQL might need more time
	var lastErr error
	for i := 0; i < 10; i++ {
		if i > 0 { // Add a small delay before retrying, not for the first attempt
			time.Sleep(time.Duration(i) * 100 * time.Millisecond)
		}
		t.Logf("Attempting to create database %s (attempt %d)", dbName, i+1)
		// TODO(tmc): use user/pass from config rather than hardcoding
		exitCode, output, err := c.Exec(context.Background(), []string{
			"mysql", "-uroot", "-ptest", "-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName),
		})

		// Check for container availability issues
		if err != nil && strings.Contains(err.Error(), "container") && strings.Contains(err.Error(), "is not running") {
			return "", fmt.Errorf("failed to create database %s: MySQL container has stopped unexpectedly: %w", dbName, err)
		}

		// Check output for container failure patterns
		if strings.Contains(output, "container") && strings.Contains(output, "is not running") {
			return "", fmt.Errorf("failed to create database %s: MySQL container has stopped unexpectedly (output indicates container failure)", dbName)
		}

		if err == nil && exitCode == 0 {
			return p.FormatDSN(c, dbName), nil
		}

		// For exit code 1, check if it's a connection issue vs database already exists
		if exitCode == 1 && strings.Contains(output, "database exists") {
			// Database already exists, that's okay
			t.Logf("Database %s already exists, proceeding.", dbName)
			return p.FormatDSN(c, dbName), nil
		}

		errMsg := "failed to create database"
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, err)
		}
		lastErr = fmt.Errorf("%s (exit code: %d, output: %s)", errMsg, exitCode, output)

		// Wait before retrying, but use shorter delays for faster recovery
		time.Sleep(500 * time.Millisecond)
	}

	return "", fmt.Errorf("failed to create database %s after %d attempts: %w", dbName, 10, lastErr)
}

// DropDatabase removes the specified database from the MySQL container.
// This is typically called during test cleanup.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	// TODO(tmc): use user/pass from config rather than hardcoding
	_, _, err := c.Exec(context.Background(), []string{
		"mysql", "-uroot", "-ptest", "-e", fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName),
	})
	return err
}

// FormatDSN returns a DSN string for connecting to the specified database
// in the MySQL container.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	// TODO(tmc): use user/pass from config rather than hardcoding
	// Add common DSN parameters for better compatibility.
	return fmt.Sprintf("root:test@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", c.Endpoint("3306"), dbName)
}

// WithDSN returns a testctr.Option that configures the container to use this DSNProvider for MySQL.
// This enables the `container.DSN(t)` method for MySQL containers.
func WithDSN() testctr.Option {
	return ctropts.WithDSNProvider(DSNProvider{})
}
