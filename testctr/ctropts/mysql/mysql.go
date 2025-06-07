// Package mysql provides MySQL-specific options for testctr
package mysql

import (
	"fmt"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DefaultMySQLImage is the default MySQL image used by `mysql.Default()`.
const DefaultMySQLImage = "mysql:8.0" // Using a more specific version

// WithDatabase sets the initial database name (MYSQL_DATABASE environment variable).
func WithDatabase(name string) testctr.Option {
	return testctr.WithEnv("MYSQL_DATABASE", name)
}

// WithRootPassword sets the root password (MYSQL_ROOT_PASSWORD environment variable).
func WithRootPassword(password string) testctr.Option {
	return testctr.WithEnv("MYSQL_ROOT_PASSWORD", password)
}

// WithUser creates a non-root user with the given username and password.
// Sets MYSQL_USER and MYSQL_PASSWORD environment variables.
func WithUser(username, password string) testctr.Option {
	return testctr.Options(
		testctr.WithEnv("MYSQL_USER", username),
		testctr.WithEnv("MYSQL_PASSWORD", password),
	)
}

// WithRandomRootPassword enables generating a random root password.
// Sets MYSQL_RANDOM_ROOT_PASSWORD=yes.
func WithRandomRootPassword() testctr.Option {
	return testctr.WithEnv("MYSQL_RANDOM_ROOT_PASSWORD", "yes")
}

// WithAllowEmptyPassword allows connections with an empty password for the root user.
// Sets MYSQL_ALLOW_EMPTY_PASSWORD=yes.
func WithAllowEmptyPassword() testctr.Option {
	return testctr.WithEnv("MYSQL_ALLOW_EMPTY_PASSWORD", "yes")
}

// WithCharacterSet sets the default character set and collation for the server.
// Example: WithCharacterSet("utf8mb4")
func WithCharacterSet(charset string) testctr.Option {
	return testctr.WithCommand("--character-set-server="+charset, "--collation-server="+charset+"_unicode_ci")
}

// WithTimezone sets the timezone for the MySQL server (TZ environment variable).
func WithTimezone(tz string) testctr.Option {
	return testctr.WithEnv("TZ", tz)
}

// WithInitScript mounts an initialization SQL script to be run on first startup.
// The script is mounted into /docker-entrypoint-initdb.d/.
func WithInitScript(hostPath string) testctr.Option {
	return ctropts.WithBindMount(hostPath, "/docker-entrypoint-initdb.d/init.sql")
}

// WithConfig mounts a custom MySQL configuration file (e.g., my.cnf).
// The file is mounted into /etc/mysql/conf.d/custom.cnf.
func WithConfig(hostPath string) testctr.Option {
	return ctropts.WithBindMount(hostPath, "/etc/mysql/conf.d/custom.cnf")
}

// WithSlowQueryLog enables the slow query log with a given threshold.
// Example: WithSlowQueryLog("1s")
func WithSlowQueryLog(threshold string) testctr.Option {
	return testctr.WithCommand(
		"--slow-query-log=1",
		"--slow-query-log-file=/var/log/mysql/slow.log",
		"--long-query-time="+threshold,
	)
}

// WithBinLog enables binary logging with ROW format.
func WithBinLog() testctr.Option {
	return testctr.WithCommand("--log-bin=mysql-bin", "--binlog-format=ROW")
}

// WithMaxConnections sets the maximum number of connections for the MySQL server.
func WithMaxConnections(max int) testctr.Option {
	return testctr.WithCommand("--max-connections=" + fmt.Sprintf("%d", max))
}

// Default returns a testctr.Option that configures a MySQL container with sensible defaults for testing.
// It includes common settings, DSN support, and wait strategies.
// Image used: mysql.DefaultMySQLImage.
func Default() testctr.Option {
	// Lock startup to prevent race conditions when multiple MySQL containers are started in parallel tests.
	lockStartup()

	return testctr.Options(
		WithRootPassword("test"), // Default root password
		WithDatabase("test"),     // Default database name
		WithCharacterSet("utf8mb4"),
		WithRootHost("%"), // Allow root connections from any host
		WithDSN(),         // Enable DSN support for test-specific databases

		// Resource and performance tuning for test environments
		ctropts.WithMemoryLimit("512m"),
		testctr.WithCommand("--log-error-verbosity=1"), // Reduce log verbosity
		testctr.WithCommand("--max-connections=200"),
		testctr.WithCommand("--innodb-buffer-pool-size=128M"),
		testctr.WithCommand("--innodb-log-file-size=64M"),
		testctr.WithCommand("--innodb-flush-log-at-trx-commit=2"), // Less durable but faster for tests

		// Wait strategies to ensure MySQL is ready
		ctropts.WithWaitForLog("ready for connections. Version", 45*time.Second),
		ctropts.WithWaitForExec([]string{"mysqladmin", "-uroot", "-ptest", "ping"}, 15*time.Second),

		testctr.WithPort("3306"), // Expose the standard MySQL port

		// Unlock startup mutex after options are applied and container is being created
		testctr.OptionFunc(func(cfg interface{}) {
			go unlockStartup()
		}),
	)
}

// WithRootHost sets the host from which root user can connect (MYSQL_ROOT_HOST environment variable).
// Use "%" to allow connections from any host.
func WithRootHost(host string) testctr.Option {
	return testctr.WithEnv("MYSQL_ROOT_HOST", host)
}

// WithPassword is an alias for WithRootPassword for consistency and ease of use.
func WithPassword(password string) testctr.Option {
	return WithRootPassword(password)
}
