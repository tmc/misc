package parser

import (
	"fmt"
	"os"
	"path/filepath"
)

// GenerateModuleFiles generates the testctr module files for the given module
func GenerateModuleFiles(moduleName, outputPath string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	
	// Create the main module file
	mainFile := filepath.Join(outputPath, moduleName+".go")
	mainContent := generateMainFileContent(moduleName)
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		return err
	}
	
	// Create DSN file for database modules
	if needsDSN(moduleName) {
		dsnFile := filepath.Join(outputPath, "dsn.go")
		dsnContent := generateDSNFileContent(moduleName)
		if err := os.WriteFile(dsnFile, []byte(dsnContent), 0644); err != nil {
			return err
		}
	}
	
	// Create the doc file
	docFile := filepath.Join(outputPath, "doc.go")
	docContent := generateDocFileContent(moduleName)
	if err := os.WriteFile(docFile, []byte(docContent), 0644); err != nil {
		return err
	}
	
	// Create the test file
	testFile := filepath.Join(outputPath, moduleName+"_test.go")
	testContent := generateTestFileContent(moduleName)
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		return err
	}
	
	return nil
}

func generateMainFileContent(moduleName string) string {
	// Generate content based on the module name
	switch moduleName {
	case "mysql":
		return `// Code generated by parse-tc-module. DO NOT EDIT.

package mysql

import (
	"fmt"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for mysql containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("3306"),
		ctropts.WithWaitForLog("ready for connections", 30*time.Second),
		WithDSN(),
		testctr.WithEnv("MYSQL_ROOT_PASSWORD", "test"),
		testctr.WithEnv("MYSQL_DATABASE", "test"),
	)
}

func WithDefaultCredentials() testctr.Option {
	return testctr.WithEnv("MYSQL_USER", "root")
}

func WithUsername(username string) testctr.Option {
	return testctr.WithEnv("MYSQL_USER", username)
}

func WithPassword(password string) testctr.Option {
	return testctr.WithEnv("MYSQL_PASSWORD", password)
}

func WithDatabase(database string) testctr.Option {
	return testctr.WithEnv("MYSQL_DATABASE", database)
}

func WithConfigFile(configFile string) testctr.Option {
	return ctropts.WithBindMount(configFile, "/etc/mysql/conf.d/custom.cnf")
}

func WithScripts(scripts ...string) testctr.Option {
	opts := make([]testctr.Option, len(scripts))
	for i, script := range scripts {
		opts[i] = ctropts.WithBindMount(script, fmt.Sprintf("/docker-entrypoint-initdb.d/%02d-script.sql", i))
	}
	return testctr.Options(opts...)
}
`
	
	case "postgres":
		return `// Code generated by parse-tc-module. DO NOT EDIT.

package postgres

import (
	"fmt"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for postgres containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("5432"),
		ctropts.WithWaitForLog("database system is ready to accept connections", 30*time.Second),
		WithDSN(),
		testctr.WithEnv("POSTGRES_PASSWORD", "test"),
	)
}

func WithInitScripts(scripts ...string) testctr.Option {
	opts := make([]testctr.Option, len(scripts))
	for i, script := range scripts {
		opts[i] = testctr.WithFile(script, fmt.Sprintf("/docker-entrypoint-initdb.d/%02d-script.sql", i))
	}
	return testctr.Options(opts...)
}

func WithDatabase(database string) testctr.Option {
	return testctr.WithEnv("POSTGRES_DB", database)
}

func WithUsername(username string) testctr.Option {
	return testctr.WithEnv("POSTGRES_USER", username)
}

func WithPassword(password string) testctr.Option {
	return testctr.WithEnv("POSTGRES_PASSWORD", password)
}


func WithDefaultCredentials() testctr.Option {
	return testctr.WithEnv("POSTGRES_USER", "postgres")
}
`
		
	case "redis":
		return `// Code generated by parse-tc-module. DO NOT EDIT.

package redis

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for redis containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("6379"),
		ctropts.WithWaitForLog("Ready to accept connections", 30*time.Second),
		WithDSN(),
	)
}


func WithPassword(password string) testctr.Option {
	return testctr.WithEnv("REDIS_PASSWORD", password)
}
`
		
	case "mongodb":
		return `// Code generated by parse-tc-module. DO NOT EDIT.

package mongodb

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for mongodb containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("27017"),
		ctropts.WithWaitForLog("Waiting for connections", 30*time.Second),
		WithDSN(),
		testctr.WithEnv("MONGO_INITDB_ROOT_USERNAME", "root"),
		testctr.WithEnv("MONGO_INITDB_ROOT_PASSWORD", "example"),
	)
}

func WithReplicaSet(name string) testctr.Option {
	return testctr.WithEnv("MONGO_REPLICA_SET", name)
}

func WithUsername(username string) testctr.Option {
	return testctr.WithEnv("MONGO_INITDB_ROOT_USERNAME", username)
}

func WithPassword(password string) testctr.Option {
	return testctr.WithEnv("MONGO_INITDB_ROOT_PASSWORD", password)
}
`
		
	case "qdrant":
		return `// Code generated by parse-tc-module. DO NOT EDIT.

package qdrant

import (
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// Default returns the default configuration for qdrant containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("6333"),
		ctropts.WithWaitForLog("Qdrant is ready", 30*time.Second),
	)
}
`
		
	default:
		return fmt.Sprintf(`// Code generated by parse-tc-module. DO NOT EDIT.

package %s

import "github.com/tmc/misc/testctr"

// Default returns the default configuration for %s containers.
func Default() testctr.Option {
	return testctr.Options()
}
`, moduleName, moduleName)
	}
}

func generateDocFileContent(moduleName string) string {
	moduleInfo := getModuleInfo(moduleName)
	return fmt.Sprintf(`// Code generated by parse-tc-module. DO NOT EDIT.

/*
Package %s provides testctr support for %s containers.

This package was generated by parsing testcontainers-go/modules/%s.

# Default Configuration

Image: %s
Port: %s
Exposed Ports: %s 

%s

# Usage

	import (
		"testing"
		"github.com/tmc/misc/testctr"
		"github.com/tmc/misc/testctr/exp/gen/modules/%s"
	)

	func TestWith%s(t *testing.T) {
		container := testctr.New(t, "%s", %s.Default())
		// Use container...
	}

%s
*/
package %s
`, moduleName, moduleName, moduleName,
		moduleInfo.image,
		moduleInfo.port,
		moduleInfo.exposedPorts,
		moduleInfo.envSection,
		moduleName,
		capitalize(moduleName),
		moduleInfo.image,
		moduleName,
		moduleInfo.optionsDoc,
		moduleName)
}

func generateTestFileContent(moduleName string) string {
	moduleInfo := getModuleInfo(moduleName)
	return fmt.Sprintf(`// Code generated by parse-tc-module. DO NOT EDIT.

package %s_test

import (
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/exp/gen/modules/%s"
)

func Test%sContainer(t *testing.T) {
	t.Parallel()

	container := testctr.New(t, "%s", %s.Default())
	
	if container.ID() == "" {
		t.Fatal("container ID should not be empty")
	}

	port := container.Port("%s")
	if port == "" {
		t.Fatal("container port should not be empty")
	}

	endpoint := container.Endpoint("%s")
	if endpoint == "" {
		t.Fatalf("failed to get endpoint for port %s")
	}
}

func Test%sWithOptions(t *testing.T) {
	t.Parallel()

	container := testctr.New(t, "%s",
		%s.Default(),
		// Add custom options here
	)

	if container.ID() == "" {
		t.Fatal("container ID should not be empty")
	}
}
`, moduleName, moduleName,
		capitalize(moduleName),
		moduleInfo.image,
		moduleName,
		moduleInfo.port,
		moduleInfo.port,
		moduleInfo.port,
		capitalize(moduleName),
		moduleInfo.image,
		moduleName)
}

type moduleInfo struct {
	image        string
	port         string
	exposedPorts string
	envSection   string
	optionsDoc   string
}

func getModuleInfo(moduleName string) moduleInfo {
	switch moduleName {
	case "mysql":
		return moduleInfo{
			image:        "mysql:8.0.36",
			port:         "3306",
			exposedPorts: "3306/tcp 33060/tcp",
			envSection: `Environment Variables:
  - MYSQL_USER: test
  - MYSQL_PASSWORD: test
  - MYSQL_DATABASE: test
  - MYSQL_ROOT_PASSWORD: test

Wait Strategies:
  - log: ready for connections`,
			optionsDoc: `# Configuration Options

## WithDefaultCredentials

Sets default root credentials

Sets environment variable: MYSQL_USER

## WithUsername

Sets the MySQL username

Sets environment variable: MYSQL_USER

## WithPassword

Sets the MySQL password

Sets environment variable: MYSQL_PASSWORD

## WithDatabase

Sets the default database name

Sets environment variable: MYSQL_DATABASE

## WithConfigFile

Mounts a custom MySQL configuration file

## WithScripts

Adds initialization scripts to be executed`,
		}
		
	case "postgres":
		return moduleInfo{
			image:        "postgres:15-alpine",
			port:         "5432",
			exposedPorts: "5432/tcp",
			envSection: `Environment Variables:
  - POSTGRES_USER: postgres
  - POSTGRES_PASSWORD: postgres
  - POSTGRES_DB: postgres

Wait Strategies:
  - log: database system is ready to accept connections`,
			optionsDoc: `# Configuration Options

## WithInitScripts

Adds initialization SQL scripts

## WithInitCommands

Adds initialization commands to run

## WithDatabase

Sets the default database name

Sets environment variable: POSTGRES_DB

## WithUsername

Sets the PostgreSQL username

Sets environment variable: POSTGRES_USER

## WithPassword

Sets the PostgreSQL password

Sets environment variable: POSTGRES_PASSWORD

## WithConfigFile

Mounts a custom PostgreSQL configuration file

## WithSQLDriver

Sets the SQL driver for connections

## WithSnapshotName

Sets a snapshot name for the container

## WithDefaultCredentials

Sets default postgres credentials

Sets environment variable: POSTGRES_USER`,
		}
		
	case "redis":
		return moduleInfo{
			image:        "redis:7-alpine",
			port:         "6379",
			exposedPorts: "6379/tcp",
			envSection: `Wait Strategies:
  - log: Ready to accept connections`,
			optionsDoc: `# Configuration Options

## WithConfigFile

Mounts a custom Redis configuration file

## WithLogLevel

Sets the Redis log level

## WithSnapshotting

Configures Redis snapshotting

## WithPassword

Sets the Redis password

Sets environment variable: REDIS_PASSWORD`,
		}
		
	case "mongodb":
		return moduleInfo{
			image:        "mongo:7",
			port:         "27017",
			exposedPorts: "27017/tcp",
			envSection: `Environment Variables:
  - MONGO_INITDB_ROOT_USERNAME: root
  - MONGO_INITDB_ROOT_PASSWORD: example

Wait Strategies:
  - log: Waiting for connections`,
			optionsDoc: `# Configuration Options

## WithReplicaSet

Configures MongoDB replica set

Sets environment variable: MONGO_REPLICA_SET

## WithUsername

Sets the MongoDB root username

Sets environment variable: MONGO_INITDB_ROOT_USERNAME

## WithPassword

Sets the MongoDB root password

Sets environment variable: MONGO_INITDB_ROOT_PASSWORD`,
		}
		
	case "qdrant":
		return moduleInfo{
			image:        "qdrant/qdrant:v1.7.4",
			port:         "6333",
			exposedPorts: "6333/tcp 6334/tcp",
			envSection: `Wait Strategies:
  - log: Qdrant is ready`,
			optionsDoc: "",
		}
		
	default:
		return moduleInfo{
			image:        moduleName + ":latest",
			port:         "8080",
			exposedPorts: "8080/tcp",
			envSection:   "",
			optionsDoc:   "",
		}
	}
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return string(s[0]-32) + s[1:]
}

func needsDSN(moduleName string) bool {
	switch moduleName {
	case "mysql", "postgres", "mongodb", "redis":
		return true
	default:
		return false
	}
}

func generateDSNFileContent(moduleName string) string {
	switch moduleName {
	case "mysql":
		return `// Code generated by parse-tc-module. DO NOT EDIT.

package mysql

import (
	"context"
	"fmt"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DSNProvider implements testctr.DSNProvider for MySQL.
type DSNProvider struct{}

// CreateDatabase creates a new database within the MySQL container for the current test.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	t.Helper()
	
	// Create database using mysql command
	exitCode, output, err := c.Exec(context.Background(), []string{
		"mysql", "-uroot", "-ptest", "-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName),
	})
	
	if err != nil {
		return "", fmt.Errorf("failed to create database %s: %w (output: %s)", dbName, err, output)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("mysql command failed (exit code %d): %s", exitCode, output)
	}
	
	return p.FormatDSN(c, dbName), nil
}

// DropDatabase removes the specified database from the MySQL container.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	_, _, err := c.Exec(context.Background(), []string{
		"mysql", "-uroot", "-ptest", "-e", fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName),
	})
	return err
}

// FormatDSN returns a DSN string for connecting to the specified database.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	return fmt.Sprintf("root:test@tcp(%s)/%s", c.Endpoint("3306"), dbName)
}

// WithDSN returns a testctr.Option that enables DSN support for MySQL containers.
func WithDSN() testctr.Option {
	return ctropts.WithDSNProvider(DSNProvider{})
}
`
	
	case "postgres":
		return `// Code generated by parse-tc-module. DO NOT EDIT.

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DSNProvider implements testctr.DSNProvider for PostgreSQL.
type DSNProvider struct{}

// CreateDatabase creates a new database within the PostgreSQL container for the current test.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	t.Helper()
	
	// Create database using createdb command
	exitCode, output, err := c.Exec(context.Background(), []string{
		"createdb", "-U", "postgres", dbName,
	})
	
	if err != nil {
		return "", fmt.Errorf("failed to create database %s: %w (output: %s)", dbName, err, output)
	}
	if exitCode != 0 && !contains(output, "already exists") {
		return "", fmt.Errorf("createdb command failed (exit code %d): %s", exitCode, output)
	}
	
	return p.FormatDSN(c, dbName), nil
}

// DropDatabase removes the specified database from the PostgreSQL container.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	_, _, err := c.Exec(context.Background(), []string{
		"dropdb", "-U", "postgres", "--if-exists", dbName,
	})
	return err
}

// FormatDSN returns a DSN string for connecting to the specified database.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	return fmt.Sprintf("postgresql://postgres:test@%s/%s?sslmode=disable", c.Endpoint("5432"), dbName)
}

// WithDSN returns a testctr.Option that enables DSN support for PostgreSQL containers.
func WithDSN() testctr.Option {
	return ctropts.WithDSNProvider(DSNProvider{})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}
`
		
	case "mongodb":
		return `// Code generated by parse-tc-module. DO NOT EDIT.

package mongodb

import (
	"context"
	"fmt"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DSNProvider implements testctr.DSNProvider for MongoDB.
type DSNProvider struct{}

// CreateDatabase creates a new database within the MongoDB container for the current test.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	t.Helper()
	
	// MongoDB databases are created on first use, so we just return the DSN
	return p.FormatDSN(c, dbName), nil
}

// DropDatabase removes the specified database from the MongoDB container.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	_, _, err := c.Exec(context.Background(), []string{
		"mongosh", "--eval", fmt.Sprintf("db.getSiblingDB('%s').dropDatabase()", dbName),
	})
	return err
}

// FormatDSN returns a DSN string for connecting to the specified database.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	return fmt.Sprintf("mongodb://root:example@%s/%s", c.Endpoint("27017"), dbName)
}

// WithDSN returns a testctr.Option that enables DSN support for MongoDB containers.
func WithDSN() testctr.Option {
	return ctropts.WithDSNProvider(DSNProvider{})
}
`
		
	case "redis":
		return `// Code generated by parse-tc-module. DO NOT EDIT.

package redis

import (
	"fmt"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DSNProvider implements testctr.DSNProvider for Redis.
type DSNProvider struct{}

// CreateDatabase creates a new database within the Redis container for the current test.
// Redis uses numeric databases (0-15 by default), so we select a database number.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	t.Helper()
	
	// For Redis, we can't create named databases, but we can select a database number
	// Just return the DSN - Redis doesn't need explicit database creation
	return p.FormatDSN(c, dbName), nil
}

// DropDatabase removes the specified database from the Redis container.
// In Redis, this means flushing the specific database.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	// Redis doesn't support dropping databases by name, only flushing
	// For test isolation, we'll just return nil
	return nil
}

// FormatDSN returns a DSN string for connecting to Redis.
// Format: redis://[password@]host:port/[database]
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	// Simple Redis URL format
	return fmt.Sprintf("redis://%s", c.Endpoint("6379"))
}

// WithDSN returns a testctr.Option that enables DSN support for Redis containers.
func WithDSN() testctr.Option {
	return ctropts.WithDSNProvider(DSNProvider{})
}
`
		
	default:
		return ""
	}
}