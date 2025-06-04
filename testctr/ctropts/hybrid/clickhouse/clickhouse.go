// Package clickhouse provides testctr support for ClickHouse containers.
// ClickHouse is a column-oriented database management system for online analytical processing.
package clickhouse

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DefaultDatabaseName is the default database name for ClickHouse.
const DefaultDatabaseName = "default"

// DefaultUsername is the default username for ClickHouse.
const DefaultUsername = "default"

// GetDefaultPassword returns the default password for ClickHouse.
func GetDefaultPassword() string {
	if pwd := os.Getenv("CLICKHOUSE_PASSWORD"); pwd != "" {
		return pwd
	}
	return ""
}

// DSNProvider implements testctr.DSNProvider for ClickHouse containers.
// It provides database lifecycle management and connection string formatting.
type DSNProvider struct{}

// CreateDatabase creates a new database within the ClickHouse container.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	// TODO: Implement database creation logic
	return p.FormatDSN(c, dbName), nil
}

// DropDatabase removes the specified database from the ClickHouse container.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	// TODO: Implement database deletion logic
	return nil
}

// FormatDSN returns a DSN string for connecting to the specified database.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	return fmt.Sprintf("clickhouse://%s:%s@%s/%s", DefaultUsername, GetDefaultPassword(), c.Endpoint("8123"), dbName)
}

// Default returns the default configuration for ClickHouse containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("8123"),
		testctr.WithEnv("CLICKHOUSE_USER", "default"), testctr.WithEnv("CLICKHOUSE_PASSWORD", ""),
		ctropts.WithWaitForLog("Ready for connections", 30*time.Second),
		ctropts.WithDSNProvider(DSNProvider{}),
	)
}

// WithUsername sets the ClickHouse username.
func WithUsername(value string) testctr.Option {
	return testctr.WithEnv("CLICKHOUSE_USER", value)
}

// WithPassword sets the ClickHouse password.
func WithPassword(value string) testctr.Option {
	return testctr.WithEnv("CLICKHOUSE_PASSWORD", value)
}

// WithDatabase sets the database name.
func WithDatabase(value string) testctr.Option {
	return testctr.WithEnv("CLICKHOUSE_DB", value)
}

// Additional helper functions can be added here for advanced ClickHouse features:
// - Configuration file mounting
// - Initialization script support
// - Cluster configuration
// - Security and authentication settings
