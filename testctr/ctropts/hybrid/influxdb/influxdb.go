// Package influxdb provides testctr support for InfluxDB containers.
// InfluxDB is a time series database designed for high-availability storage and retrieval of time series data.
package influxdb

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DefaultDatabaseName is the default database name for InfluxDB.
const DefaultDatabaseName = "test"

// DefaultUsername is the default username for InfluxDB.
const DefaultUsername = "admin"

// GetDefaultPassword returns the default password for InfluxDB.
func GetDefaultPassword() string {
	if pwd := os.Getenv("INFLUXDB_PASSWORD"); pwd != "" {
		return pwd
	}
	return "password"
}

// DSNProvider implements testctr.DSNProvider for InfluxDB containers.
// It provides database lifecycle management and connection string formatting.
type DSNProvider struct{}

// CreateDatabase creates a new database within the InfluxDB container.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	// TODO: Implement database creation logic
	return p.FormatDSN(c, dbName), nil
}

// DropDatabase removes the specified database from the InfluxDB container.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	// TODO: Implement database deletion logic
	return nil
}

// FormatDSN returns a DSN string for connecting to the specified database.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	return fmt.Sprintf("http://%s", c.Endpoint("8086"))
}

// Default returns the default configuration for InfluxDB containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("8086"),
		testctr.WithEnv("DOCKER_INFLUXDB_INIT_USERNAME", "admin"), testctr.WithEnv("DOCKER_INFLUXDB_INIT_PASSWORD", "password"),
		ctropts.WithWaitForLog("ready", 30*time.Second),
		ctropts.WithDSNProvider(DSNProvider{}),
	)
}

// WithUsername sets the InfluxDB admin username.
func WithUsername(value string) testctr.Option {
	return testctr.WithEnv("DOCKER_INFLUXDB_INIT_USERNAME", value)
}

// WithPassword sets the InfluxDB admin password.
func WithPassword(value string) testctr.Option {
	return testctr.WithEnv("DOCKER_INFLUXDB_INIT_PASSWORD", value)
}

// WithOrg sets the InfluxDB organization.
func WithOrg(value string) testctr.Option {
	return testctr.WithEnv("DOCKER_INFLUXDB_INIT_ORG", value)
}

// WithBucket sets the InfluxDB bucket.
func WithBucket(value string) testctr.Option {
	return testctr.WithEnv("DOCKER_INFLUXDB_INIT_BUCKET", value)
}

// Additional helper functions can be added here for advanced InfluxDB features:
// - Configuration file mounting
// - Initialization script support
// - Token-based authentication
// - Organization and bucket management
