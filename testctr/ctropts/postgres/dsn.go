package postgres

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DSNProvider implements testctr.DSNProvider for PostgreSQL
type DSNProvider struct{}

// dbCreationMutex serializes database creation to prevent PostgreSQL overload
var dbCreationMutex sync.Mutex

func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	// Serialize database creation to prevent overwhelming PostgreSQL
	dbCreationMutex.Lock()
	defer dbCreationMutex.Unlock()
	// PostgreSQL sometimes needs multiple attempts due to initialization timing
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		// First try to drop if it exists (from previous failed run)
		c.Exec(context.Background(), []string{
			"dropdb", "-U", "postgres", "--if-exists", dbName,
		})

		// Create database using createdb command which handles connection better
		exitCode, output, err := c.Exec(context.Background(), []string{
			"createdb", "-U", "postgres", dbName,
		})
		if err != nil {
			lastErr = fmt.Errorf("failed to create database: %v (exit code: %d, output: %s)", err, exitCode, output)
			continue
		}
		if exitCode != 0 {
			lastErr = fmt.Errorf("createdb failed with exit code %d: %s", exitCode, output)
			continue
		}

		// Verify the database was created and PostgreSQL is still alive
		exitCode, output, err = c.Exec(context.Background(), []string{
			"psql", "-U", "postgres", "-d", dbName, "-c", "SELECT 1",
		})
		if err != nil || exitCode != 0 {
			lastErr = fmt.Errorf("failed to verify database creation: %v (exit code: %d, output: %s)", err, exitCode, output)
			continue
		}

		return p.FormatDSN(c, dbName), nil
	}

	return "", fmt.Errorf("failed after 5 attempts: %v", lastErr)
}

func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	_, _, err := c.Exec(context.Background(), []string{
		"dropdb", "-U", "postgres", "--if-exists", dbName,
	})
	return err
}

func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	return fmt.Sprintf("postgresql://postgres@%s/%s?sslmode=disable", c.Endpoint("5432"), dbName)
}

// WithDSN returns an option that enables DSN functionality for PostgreSQL
func WithDSN() testctr.Option {
	return ctropts.WithDSNProvider(DSNProvider{})
}
