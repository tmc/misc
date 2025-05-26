// Package mysql provides MySQL-specific options for testctr
package mysql

import (
	"fmt"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// WithDatabase sets the initial database name
func WithDatabase(name string) testctr.Option {
	return testctr.WithEnv("MYSQL_DATABASE", name)
}

// WithRootPassword sets the root password
func WithRootPassword(password string) testctr.Option {
	return testctr.WithEnv("MYSQL_ROOT_PASSWORD", password)
}

// WithUser creates a non-root user
func WithUser(username, password string) testctr.Option {
	return testctr.Options(
		testctr.WithEnv("MYSQL_USER", username),
		testctr.WithEnv("MYSQL_PASSWORD", password),
	)
}

// WithRandomRootPassword generates a random root password
func WithRandomRootPassword() testctr.Option {
	return testctr.WithEnv("MYSQL_RANDOM_ROOT_PASSWORD", "yes")
}

// WithAllowEmptyPassword allows connections with empty password
func WithAllowEmptyPassword() testctr.Option {
	return testctr.WithEnv("MYSQL_ALLOW_EMPTY_PASSWORD", "yes")
}

// WithCharacterSet sets the default character set
func WithCharacterSet(charset string) testctr.Option {
	return testctr.WithCommand("--character-set-server="+charset, "--collation-server="+charset+"_unicode_ci")
}

// WithTimezone sets the timezone
func WithTimezone(tz string) testctr.Option {
	return testctr.WithEnv("TZ", tz)
}

// WithInitScript mounts an initialization SQL script
func WithInitScript(hostPath string) testctr.Option {
	return ctropts.WithBindMount(hostPath, "/docker-entrypoint-initdb.d/init.sql")
}

// WithConfig mounts a custom MySQL configuration file
func WithConfig(hostPath string) testctr.Option {
	return ctropts.WithBindMount(hostPath, "/etc/mysql/conf.d/custom.cnf")
}

// WithSlowQueryLog enables the slow query log
func WithSlowQueryLog(threshold string) testctr.Option {
	return testctr.WithCommand(
		"--slow-query-log=1",
		"--slow-query-log-file=/var/log/mysql/slow.log",
		"--long-query-time="+threshold,
	)
}

// WithBinLog enables binary logging
func WithBinLog() testctr.Option {
	return testctr.WithCommand("--log-bin=mysql-bin", "--binlog-format=ROW")
}

// WithMaxConnections sets the maximum number of connections
func WithMaxConnections(max int) testctr.Option {
	return testctr.WithCommand("--max-connections=" + fmt.Sprintf("%d", max))
}

// Default returns sensible defaults for MySQL testing
func Default() testctr.Option {
	// Lock startup to prevent race conditions
	lockStartup()
	
	return testctr.Options(
		WithRootPassword("test"),
		WithDatabase("test"),
		WithCharacterSet("utf8mb4"),
		WithRootHost("%"),
		WithDSN(),
		// Set memory limit to prevent race condition issues
		ctropts.WithMemoryLimit("512m"),
		// Disable the MySQL error log to console to reduce I/O contention
		testctr.WithCommand("--log-error-verbosity=1"),
		// Add stability improvements for parallel workloads
		testctr.WithCommand("--max-connections=200"),
		testctr.WithCommand("--innodb-buffer-pool-size=128M"),
		testctr.WithCommand("--innodb-log-file-size=64M"),
		testctr.WithCommand("--innodb-flush-log-at-trx-commit=2"),
		// Wait for MySQL to be ready - look for the ready message that includes the port
		ctropts.WithWaitForLog("ready for connections. Version", 45*time.Second),
		// Then verify we can actually connect
		ctropts.WithWaitForExec([]string{"mysql", "-uroot", "-ptest", "-e", "SELECT 1"}, 15*time.Second),
		testctr.WithPort("3306"),
		// Add a cleanup function to unlock after container is created
		testctr.OptionFunc(func(cfg interface{}) {
			// Schedule unlock after options are applied
			go unlockStartup()
		}),
	)
}

// WithRootHost sets the host allowed for root connections
func WithRootHost(host string) testctr.Option {
	return testctr.WithEnv("MYSQL_ROOT_HOST", host)
}

// WithPassword is an alias for WithRootPassword for consistency
func WithPassword(password string) testctr.Option {
	return WithRootPassword(password)
}
