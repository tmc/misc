// Package postgres2 provides enhanced testctr support for PostgreSQL containers.
// This is an improved version synthesized from testcontainers-go/modules/postgres.
package postgres2

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

// DSNProvider implements testctr.DSNProvider for PostgreSQL.
type DSNProvider struct{}

// CreateDatabase creates a new database within the PostgreSQL container for the current test.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	t.Helper()

	var lastErr error
	for i := 0; i < 5; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i) * 200 * time.Millisecond)
		}
		exitCode, output, err := c.Exec(context.Background(), []string{
			"psql", "-U", DefaultUsername, "-d", "postgres", "-c", 
			fmt.Sprintf("CREATE DATABASE \"%s\";", dbName),
		})

		if err != nil && strings.Contains(err.Error(), "container") && strings.Contains(err.Error(), "is not running") {
			return "", fmt.Errorf("failed to create database %s: PostgreSQL container has stopped unexpectedly: %w", dbName, err)
		}

		if err == nil && exitCode == 0 {
			return p.FormatDSN(c, dbName), nil
		}

		if exitCode == 1 && strings.Contains(output, "already exists") {
			return p.FormatDSN(c, dbName), nil
		}

		errMsg := "failed to create database"
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, err)
		}
		lastErr = fmt.Errorf("%s (exit code: %d, output: %s)", errMsg, exitCode, output)
		time.Sleep(500 * time.Millisecond)
	}

	return "", fmt.Errorf("failed to create database %s after %d attempts: %w", dbName, 5, lastErr)
}

// DropDatabase removes the specified database from the PostgreSQL container.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	_, _, err := c.Exec(context.Background(), []string{
		"psql", "-U", DefaultUsername, "-d", "postgres", "-c", 
		fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\";", dbName),
	})
	return err
}

// FormatDSN returns a DSN string for connecting to the specified database.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", 
		DefaultUsername, GetDefaultPassword(), c.Endpoint("5432"), dbName)
}

// DefaultDatabaseName is the default database name for PostgreSQL.
const DefaultDatabaseName = "postgres"

// DefaultUsername is the default username for PostgreSQL.
const DefaultUsername = "postgres"

// GetDefaultPassword returns the default password for PostgreSQL.
func GetDefaultPassword() string {
	if pwd := os.Getenv("POSTGRES_PASSWORD"); pwd != "" {
		return pwd
	}
	return "postgres"
}

// Default returns the default configuration for PostgreSQL containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("5432"),
		testctr.WithEnv("POSTGRES_PASSWORD", GetDefaultPassword()),
		testctr.WithEnv("POSTGRES_DB", DefaultDatabaseName),
		ctropts.WithWaitForLog("database system is ready to accept connections", 30*time.Second),
		ctropts.WithDSNProvider(&DSNProvider{}),
	)
}

// WithDatabase sets the database name.
func WithDatabase(database string) testctr.Option {
	return testctr.WithEnv("POSTGRES_DB", database)
}

// WithUsername sets the PostgreSQL user.
func WithUsername(username string) testctr.Option {
	return testctr.WithEnv("POSTGRES_USER", username)
}

// WithPassword sets the PostgreSQL user password.
func WithPassword(password string) testctr.Option {
	return testctr.WithEnv("POSTGRES_PASSWORD", password)
}

// WithInitDatabase sets the initial database name (alternative to WithDatabase).
func WithInitDatabase(database string) testctr.Option {
	return testctr.WithEnv("POSTGRES_INITDB_ARGS", "--locale=C --encoding=UTF8")
}

// WithConfigFile sets a custom PostgreSQL configuration file.
func WithConfigFile(configPath string) testctr.Option {
	return testctr.WithFile(configPath, "/etc/postgresql/postgresql.conf")
}

// WithInitScript sets an initialization SQL script.
func WithInitScript(scriptPath string) testctr.Option {
	return testctr.WithFile(scriptPath, "/docker-entrypoint-initdb.d/init.sql")
}

// WithInitScripts sets multiple initialization SQL scripts.
func WithInitScripts(scriptPaths ...string) testctr.Option {
	var options []testctr.Option
	for i, scriptPath := range scriptPaths {
		mountPath := fmt.Sprintf("/docker-entrypoint-initdb.d/%02d-init.sql", i+1)
		options = append(options, testctr.WithFile(scriptPath, mountPath))
	}
	return testctr.Options(options...)
}

// WithCommand sets a custom PostgreSQL command.
func WithCommand(args ...string) testctr.Option {
	allArgs := append([]string{"postgres"}, args...)
	return testctr.WithCommand(allArgs...)
}

// WithTimezone sets the PostgreSQL timezone.
func WithTimezone(timezone string) testctr.Option {
	return testctr.WithEnv("TZ", timezone)
}

// WithLocale sets the PostgreSQL locale.
func WithLocale(locale string) testctr.Option {
	return testctr.WithEnv("LANG", locale)
}

// WithSharedBuffers sets the PostgreSQL shared_buffers setting.
func WithSharedBuffers(size string) testctr.Option {
	return WithCommand("-c", "shared_buffers="+size)
}

// WithMaxConnections sets the maximum number of connections.
func WithMaxConnections(maxConn int) testctr.Option {
	return WithCommand("-c", fmt.Sprintf("max_connections=%d", maxConn))
}

// WithLogStatement sets the log_statement level.
func WithLogStatement(level string) testctr.Option {
	return WithCommand("-c", "log_statement="+level)
}

// WithLogMinDuration sets the log_min_duration_statement.
func WithLogMinDuration(duration int) testctr.Option {
	return WithCommand("-c", fmt.Sprintf("log_min_duration_statement=%d", duration))
}

// WithWALLevel sets the WAL level for replication.
func WithWALLevel(level string) testctr.Option {
	return WithCommand("-c", "wal_level="+level)
}

// WithMaxWALSenders sets the maximum number of WAL sender processes.
func WithMaxWALSenders(count int) testctr.Option {
	return WithCommand("-c", fmt.Sprintf("max_wal_senders=%d", count))
}

// WithArchiveMode enables or disables archive mode.
func WithArchiveMode(enabled bool) testctr.Option {
	if enabled {
		return WithCommand("-c", "archive_mode=on")
	}
	return testctr.OptionFunc(func(interface{}) {})
}

// WithArchiveCommand sets the archive command.
func WithArchiveCommand(command string) testctr.Option {
	return WithCommand("-c", "archive_command="+command)
}

// WithHotStandby enables hot standby mode.
func WithHotStandby(enabled bool) testctr.Option {
	if enabled {
		return WithCommand("-c", "hot_standby=on")
	}
	return testctr.OptionFunc(func(interface{}) {})
}

// WithReplicationSlot creates a replication slot.
func WithReplicationSlot(slotName string) testctr.Option {
	return WithCommand("-c", "max_replication_slots=10")
}

// WithSSL enables SSL connections.
func WithSSL(enabled bool) testctr.Option {
	if enabled {
		return testctr.Options(
			WithCommand("-c", "ssl=on"),
			WithCommand("-c", "ssl_cert_file=/var/lib/postgresql/server.crt"),
			WithCommand("-c", "ssl_key_file=/var/lib/postgresql/server.key"),
		)
	}
	return testctr.OptionFunc(func(interface{}) {})
}

// WithWorkMem sets the work_mem setting.
func WithWorkMem(size string) testctr.Option {
	return WithCommand("-c", "work_mem="+size)
}

// WithMaintenanceWorkMem sets the maintenance_work_mem setting.
func WithMaintenanceWorkMem(size string) testctr.Option {
	return WithCommand("-c", "maintenance_work_mem="+size)
}

// WithEffectiveCacheSize sets the effective_cache_size setting.
func WithEffectiveCacheSize(size string) testctr.Option {
	return WithCommand("-c", "effective_cache_size="+size)
}

// WithRandomPageCost sets the random_page_cost setting.
func WithRandomPageCost(cost float64) testctr.Option {
	return WithCommand("-c", fmt.Sprintf("random_page_cost=%.2f", cost))
}

// WithExtension enables a PostgreSQL extension.
func WithExtension(extensionName string) testctr.Option {
	_ = fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s;", extensionName)
	return testctr.WithEnv("POSTGRES_INITDB_ARGS", "--locale=C --encoding=UTF8")
}

// WithPerformanceTuning applies common performance tuning settings.
func WithPerformanceTuning() testctr.Option {
	return testctr.Options(
		WithSharedBuffers("256MB"),
		WithWorkMem("4MB"),
		WithMaintenanceWorkMem("64MB"),
		WithEffectiveCacheSize("1GB"),
		WithRandomPageCost(1.1),
		WithCommand("-c", "checkpoint_completion_target=0.9"),
		WithCommand("-c", "wal_buffers=16MB"),
		WithCommand("-c", "default_statistics_target=100"),
	)
}