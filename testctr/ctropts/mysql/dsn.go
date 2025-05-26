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

// DSNProvider implements testctr.DSNProvider for MySQL
type DSNProvider struct{}

// dbCreationMutex serializes database creation to prevent overwhelming MySQL
var dbCreationMutex sync.Mutex

func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	// Serialize database creation to prevent MySQL from being overwhelmed
	dbCreationMutex.Lock()
	defer dbCreationMutex.Unlock()
	t.Helper()
	
	// Try connecting with a retry loop since MySQL might need more time
	var lastErr error
	for i := 0; i < 10; i++ {
		t.Logf("Attempting to create database %s (attempt %d)", dbName, i+1)
		exitCode, output, err := c.Exec(context.Background(), []string{
			"mysql", "-uroot", "-ptest", "-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName),
		})
		
		// Check for container availability issues
		if err != nil && strings.Contains(err.Error(), "container") && strings.Contains(err.Error(), "is not running") {
			return "", fmt.Errorf("failed to create database %s: MySQL container has stopped unexpectedly", dbName)
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
			return p.FormatDSN(c, dbName), nil
		}
		
		lastErr = fmt.Errorf("failed to create database: exit status %d (exit code: %d, output: %s)", err, exitCode, output)
		
		// Wait before retrying, but use shorter delays for faster recovery
		time.Sleep(500 * time.Millisecond)
	}

	return "", fmt.Errorf("failed after %d attempts: %v", 10, lastErr)
}

func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	_, _, err := c.Exec(context.Background(), []string{
		"mysql", "-uroot", "-ptest", "-e", fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName),
	})
	return err
}

func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	return fmt.Sprintf("root:test@tcp(%s)/%s", c.Endpoint("3306"), dbName)
}

// WithDSN returns an option that enables DSN functionality for MySQL
func WithDSN() testctr.Option {
	return ctropts.WithDSNProvider(DSNProvider{})
}
