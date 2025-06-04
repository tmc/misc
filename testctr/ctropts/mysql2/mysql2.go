// Package mysql2 provides enhanced testctr support for MySQL containers.
// This is an improved version synthesized from testcontainers-go/modules/mysql.
package mysql2

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DefaultDatabaseName is the default database name for MySQL.
const DefaultDatabaseName = "test"

// DefaultUsername is the default username for MySQL.
const DefaultUsername = "root"

// GetDefaultRootPassword returns the default root password for MySQL.
func GetDefaultRootPassword() string {
	if pwd := os.Getenv("MYSQL_ROOT_PASSWORD"); pwd != "" {
		return pwd
	}
	return "root"
}

// DSNProvider implements testctr.DSNProvider for MySQL.
type DSNProvider struct{}

// dbCreationMutex serializes database creation to prevent overwhelming MySQL.
var dbCreationMutex sync.Mutex

// CreateDatabase creates a new database within the MySQL container for the current test.
func (p DSNProvider) CreateDatabase(c *testctr.Container, t testing.TB, dbName string) (string, error) {
	dbCreationMutex.Lock()
	defer dbCreationMutex.Unlock()
	t.Helper()

	var lastErr error
	for i := 0; i < 10; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i) * 100 * time.Millisecond)
		}
		exitCode, output, err := c.Exec(context.Background(), []string{
			"mysql", "-uroot", "-p" + GetDefaultRootPassword(), "-e", 
			fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName),
		})

		if err != nil && strings.Contains(err.Error(), "container") && strings.Contains(err.Error(), "is not running") {
			return "", fmt.Errorf("failed to create database %s: MySQL container has stopped unexpectedly: %w", dbName, err)
		}

		if err == nil && exitCode == 0 {
			return p.FormatDSN(c, dbName), nil
		}

		if exitCode == 1 && strings.Contains(output, "database exists") {
			return p.FormatDSN(c, dbName), nil
		}

		errMsg := "failed to create database"
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, err)
		}
		lastErr = fmt.Errorf("%s (exit code: %d, output: %s)", errMsg, exitCode, output)
		time.Sleep(500 * time.Millisecond)
	}

	return "", fmt.Errorf("failed to create database %s after %d attempts: %w", dbName, 10, lastErr)
}

// DropDatabase removes the specified database from the MySQL container.
func (p DSNProvider) DropDatabase(c *testctr.Container, dbName string) error {
	_, _, err := c.Exec(context.Background(), []string{
		"mysql", "-uroot", "-p" + GetDefaultRootPassword(), "-e", 
		fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName),
	})
	return err
}

// FormatDSN returns a DSN string for connecting to the specified database.
func (p DSNProvider) FormatDSN(c *testctr.Container, dbName string) string {
	return fmt.Sprintf("root:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", 
		GetDefaultRootPassword(), c.Endpoint("3306"), dbName)
}

// Default returns the default configuration for MySQL containers.
func Default() testctr.Option {
	return testctr.Options(
		testctr.WithPort("3306"),
		testctr.WithEnv("MYSQL_ROOT_PASSWORD", GetDefaultRootPassword()),
		testctr.WithEnv("MYSQL_DATABASE", DefaultDatabaseName),
		ctropts.WithWaitForLog("ready for connections", 60*time.Second),
		ctropts.WithDSNProvider(&DSNProvider{}),
	)
}

// WithDatabase sets the database name.
func WithDatabase(database string) testctr.Option {
	return testctr.WithEnv("MYSQL_DATABASE", database)
}

// WithUsername sets the MySQL user.
func WithUsername(username string) testctr.Option {
	return testctr.WithEnv("MYSQL_USER", username)
}

// WithPassword sets the MySQL user password.
func WithPassword(password string) testctr.Option {
	return testctr.Options(
		testctr.WithEnv("MYSQL_PASSWORD", password),
		// Also set root password to the same value for simplicity
		testctr.WithEnv("MYSQL_ROOT_PASSWORD", password),
	)
}

// WithRootPassword sets the MySQL root password.
func WithRootPassword(password string) testctr.Option {
	return testctr.WithEnv("MYSQL_ROOT_PASSWORD", password)
}

// WithConfigFile sets a custom MySQL configuration file.
func WithConfigFile(configPath string) testctr.Option {
	return ctropts.WithBindMount(configPath, "/etc/mysql/conf.d/custom.cnf")
}

// WithInitScript sets an initialization SQL script.
func WithInitScript(scriptPath string) testctr.Option {
	return ctropts.WithBindMount(scriptPath, "/docker-entrypoint-initdb.d/init.sql")
}

// WithInitScripts sets multiple initialization SQL scripts.
func WithInitScripts(scriptPaths ...string) testctr.Option {
	var options []testctr.Option
	for i, scriptPath := range scriptPaths {
		mountPath := fmt.Sprintf("/docker-entrypoint-initdb.d/%02d-init.sql", i+1)
		options = append(options, ctropts.WithBindMount(scriptPath, mountPath))
	}
	return testctr.Options(options...)
}

// WithCommand sets a custom MySQL command.
func WithCommand(args ...string) testctr.Option {
	allArgs := append([]string{"mysqld"}, args...)
	return testctr.WithCommand(allArgs...)
}

// WithCharacterSet sets the MySQL character set and collation.
func WithCharacterSet(charset, collation string) testctr.Option {
	return testctr.Options(
		testctr.WithEnv("MYSQL_CHARSET", charset),
		testctr.WithEnv("MYSQL_COLLATION", collation),
	)
}

// WithTimezone sets the MySQL timezone.
func WithTimezone(timezone string) testctr.Option {
	return testctr.WithEnv("TZ", timezone)
}

// WithSlowQueryLog enables MySQL slow query logging.
func WithSlowQueryLog(enabled bool) testctr.Option {
	if enabled {
		return testctr.Options(
			WithCommand("--slow-query-log=1"),
			WithCommand("--slow-query-log-file=/var/log/mysql/slow.log"),
			WithCommand("--long_query_time=2"),
		)
	}
	return testctr.OptionFunc(func(interface{}) {})
}

// WithGeneralLog enables MySQL general query logging.
func WithGeneralLog(enabled bool) testctr.Option {
	if enabled {
		return testctr.Options(
			WithCommand("--general-log=1"),
			WithCommand("--general-log-file=/var/log/mysql/general.log"),
		)
	}
	return testctr.OptionFunc(func(interface{}) {})
}

// WithInnoDBBufferPoolSize sets the InnoDB buffer pool size.
func WithInnoDBBufferPoolSize(size string) testctr.Option {
	return WithCommand("--innodb-buffer-pool-size=" + size)
}

// WithMaxConnections sets the maximum number of connections.
func WithMaxConnections(maxConn int) testctr.Option {
	return WithCommand(fmt.Sprintf("--max-connections=%d", maxConn))
}

// WithSQLMode sets the SQL mode.
func WithSQLMode(mode string) testctr.Option {
	return WithCommand("--sql-mode=" + mode)
}

// WithBinlogFormat sets the binary log format.
func WithBinlogFormat(format string) testctr.Option {
	return testctr.Options(
		WithCommand("--log-bin=mysql-bin"),
		WithCommand("--binlog-format=" + format),
	)
}

// WithGTID enables GTID mode.
func WithGTID(enabled bool) testctr.Option {
	if enabled {
		return testctr.Options(
			WithCommand("--gtid-mode=ON"),
			WithCommand("--enforce-gtid-consistency=ON"),
		)
	}
	return testctr.OptionFunc(func(interface{}) {})
}