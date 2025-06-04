// Package mongodb provides testctr support for mongodb containers.
// This package was generated and is ready for manual enhancement.
package mongodb

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DefaultDatabaseName is the default database name for mongodb.
const DefaultDatabaseName = "test"

// DefaultUsername is the default username for mongodb.
const DefaultUsername = "root"

// GetDefaultPassword returns the default password for mongodb.
func GetDefaultPassword() string {
	if pwd := os.Getenv("MONGODB_PASSWORD"); pwd != "" {
		return pwd
	}
	return "root"
}

// DSNProvider implements testctr.DSNProvider for MongoDB containers.
// It provides database lifecycle management and connection string formatting.
type DSNProvider struct{}

// CreateDatabase creates a new database within the MongoDB container for the current test.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	t.Helper()
	// MongoDB creates databases automatically when first accessed
	// Verify container is accessible by attempting a simple command
	exitCode, output, err := c.Exec(context.Background(), []string{
		"mongosh", "--eval", "db.runCommand('ping')",
	})
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to verify MongoDB connection (exit code: %d, output: %s): %w", exitCode, output, err)
	}
	return p.FormatDSN(c, dbName), nil
}

// DropDatabase removes the specified database from the MongoDB container.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	_, _, err := c.Exec(context.Background(), []string{
		"mongosh", dbName, "--eval", "db.dropDatabase()",
	})
	return err
}

// FormatDSN returns a DSN string for connecting to the specified database.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	auth := ""
	if DefaultUsername != "" && GetDefaultPassword() != "" {
		auth = fmt.Sprintf("%s:%s@", DefaultUsername, GetDefaultPassword())
	}
	return fmt.Sprintf("mongodb://%s%s/%s?authSource=admin", auth, c.Endpoint("27017"), dbName)
}

// Default returns the default configuration for mongodb containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("27017"),
		testctr.WithEnv("MONGO_INITDB_ROOT_USERNAME", "root"), testctr.WithEnv("MONGO_INITDB_ROOT_PASSWORD", "root"),
		ctropts.WithWaitForLog("Waiting for connections", 30*time.Second),
		ctropts.WithDSNProvider(DSNProvider{}),
	)
}

// WithUsername sets the MongoDB root username.
func WithUsername(value string) testctr.Option {
	return testctr.WithEnv("MONGO_INITDB_ROOT_USERNAME", value)
}

// WithPassword sets the MongoDB root password.
func WithPassword(value string) testctr.Option {
	return testctr.WithEnv("MONGO_INITDB_ROOT_PASSWORD", value)
}

// WithDatabase sets the initial database name.
func WithDatabase(value string) testctr.Option {
	return testctr.WithEnv("MONGO_INITDB_DATABASE", value)
}

// WithReplicaSet sets the replica set name.
func WithReplicaSet(value string) testctr.Option {
	return testctr.WithEnv("MONGO_REPLICA_SET_NAME", value)
}

// WithConfigFile mounts a MongoDB configuration file into the container.
func WithConfigFile(hostPath string) testctr.Option {
	return testctr.WithFile(hostPath, "/etc/mongod.conf")
}

// WithInitScript mounts an initialization script that runs when the container starts.
func WithInitScript(hostPath string) testctr.Option {
	return testctr.WithFile(hostPath, "/docker-entrypoint-initdb.d/init.js")
}

// WithAuthEnabled enables authentication for MongoDB.
func WithAuthEnabled() testctr.Option {
	return testctr.WithCommand("mongod", "--auth")
}

// WithJournaling enables journaling for MongoDB.
func WithJournaling(enabled bool) testctr.Option {
	if enabled {
		return testctr.WithCommand("mongod", "--journal")
	}
	return testctr.WithCommand("mongod", "--nojournal")
}

// WithOplogSize sets the size of the oplog in MB.
func WithOplogSize(sizeMB int) testctr.Option {
	return testctr.WithCommand("mongod", "--oplogSize", fmt.Sprintf("%d", sizeMB))
}

// ConnectionString builds a MongoDB connection string with custom parameters.
func ConnectionString(host, port, username, password, database string, params map[string]string) string {
	auth := ""
	if username != "" && password != "" {
		auth = fmt.Sprintf("%s:%s@", username, password)
	}
	
	dsn := fmt.Sprintf("mongodb://%s%s:%s/%s", auth, host, port, database)
	
	if len(params) > 0 {
		dsn += "?"
		var paramPairs []string
		for key, value := range params {
			paramPairs = append(paramPairs, fmt.Sprintf("%s=%s", key, value))
		}
		dsn += strings.Join(paramPairs, "&")
	}
	
	return dsn
}
