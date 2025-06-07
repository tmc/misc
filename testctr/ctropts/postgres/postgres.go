// Package postgres provides PostgreSQL-specific options for testctr
package postgres

import (
	"fmt"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// DefaultPostgresImage is the default PostgreSQL image used by `postgres.Default()`.
const DefaultPostgresImage = "postgres:15-alpine"

// WithDatabase sets the initial database name (POSTGRES_DB environment variable).
// If not set, defaults to the value of POSTGRES_USER.
func WithDatabase(name string) testctr.Option {
	return testctr.WithEnv("POSTGRES_DB", name)
}

// WithPassword sets the superuser password (POSTGRES_PASSWORD environment variable).
// Required by the official PostgreSQL image.
func WithPassword(password string) testctr.Option {
	return testctr.WithEnv("POSTGRES_PASSWORD", password)
}

// WithUser sets the superuser username (POSTGRES_USER environment variable).
// If not set, defaults to "postgres".
func WithUser(username string) testctr.Option {
	return testctr.WithEnv("POSTGRES_USER", username)
}

// WithInitScript mounts an initialization SQL or shell script to be run on first startup.
// The script is mounted into /docker-entrypoint-initdb.d/.
// To control execution order for multiple scripts, name them like 01-script.sql, 02-another.sh.
func WithInitScript(hostPath string) testctr.Option {
	return ctropts.WithBindMount(hostPath, "/docker-entrypoint-initdb.d/init.sql") // Consider dynamic target name if multiple scripts
}

// WithConfig passes custom PostgreSQL configuration parameters as command-line arguments.
// Example: WithConfig(map[string]string{"shared_buffers": "128MB", "max_connections": "50"})
func WithConfig(config map[string]string) testctr.Option {
	opts := make([]testctr.Option, 0, len(config))
	for key, value := range config {
		opts = append(opts, testctr.WithCommand("-c", fmt.Sprintf("%s=%s", key, value)))
	}
	return testctr.Options(opts...)
}

// WithLocale sets the database locale using LANG, LC_COLLATE, and LC_CTYPE environment variables.
func WithLocale(locale string) testctr.Option {
	return testctr.Options(
		testctr.WithEnv("LANG", locale),
		testctr.WithEnv("LC_COLLATE", locale),
		testctr.WithEnv("LC_CTYPE", locale),
	)
}

// WithTimezone sets the timezone for the PostgreSQL server (TZ environment variable).
func WithTimezone(tz string) testctr.Option {
	return testctr.WithEnv("TZ", tz)
}

// WithMaxConnections sets the maximum number of connections for the PostgreSQL server.
func WithMaxConnections(max int) testctr.Option {
	return testctr.WithCommand("-c", fmt.Sprintf("max_connections=%d", max))
}

// WithSharedBuffers sets the shared_buffers size for PostgreSQL.
// Example: WithSharedBuffers("128MB")
func WithSharedBuffers(size string) testctr.Option {
	return testctr.WithCommand("-c", "shared_buffers="+size)
}

// WithLogging enables detailed statement, connection, and disconnection logging.
func WithLogging() testctr.Option {
	return testctr.Options(
		testctr.WithCommand("-c", "log_statement=all"),
		testctr.WithCommand("-c", "log_connections=on"),
		testctr.WithCommand("-c", "log_disconnections=on"),
	)
}

// WithSSL enables SSL connections by mounting server certificate and key, and setting SSL parameters.
// Assumes server.crt and server.key are standard names used in PostgreSQL for SSL.
func WithSSL(certPath, keyPath string) testctr.Option {
	return testctr.Options(
		ctropts.WithBindMount(certPath, "/var/lib/postgresql/server.crt"),
		ctropts.WithBindMount(keyPath, "/var/lib/postgresql/server.key"),
		testctr.WithCommand("-c", "ssl=on"),
		testctr.WithCommand("-c", "ssl_cert_file=/var/lib/postgresql/server.crt"),
		testctr.WithCommand("-c", "ssl_key_file=/var/lib/postgresql/server.key"),
	)
}

// WithExtensions preloads PostgreSQL extensions by setting shared_preload_libraries.
func WithExtensions(extensions ...string) testctr.Option {
	opts := make([]testctr.Option, len(extensions))
	for i, ext := range extensions {
		opts[i] = testctr.WithCommand("-c", fmt.Sprintf("shared_preload_libraries=%s", ext)) // Note: This overrides, usually append via comma
	}
	return testctr.Options(opts...)
}

// WithDelayAfterLog adds a delay after the log wait completes.
// This is a workaround for PostgreSQL's initialization process.
// DEPRECATED: This function is no longer needed as PostgreSQL coordination
// is now handled via startup mutex in the Default() function. Its presence
// is for backward compatibility and it's a no-op.
func WithDelayAfterLog(delay time.Duration) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		// No-op: PostgreSQL startup coordination is now handled via mutex.
		// This function is kept for backward compatibility.
	})
}

// Default returns a testctr.Option that configures a PostgreSQL container with sensible defaults for testing.
// It includes common settings, DSN support, and wait strategies.
// Image used: postgres.DefaultPostgresImage.
func Default() testctr.Option {
	// Lock startup to prevent race conditions when multiple PostgreSQL containers are started in parallel tests.
	lockStartup()

	return testctr.Options(
		// Default user is "postgres", password set by POSTGRES_PASSWORD (or trust if not set)
		// Default database is "postgres" or same as user if POSTGRES_USER is set.
		// We will set specific defaults here for clarity in tests.
		WithUser("postgres"), // Default user, makes DSN formatting easier.
		WithPassword("test"), // Default password.
		WithDatabase("test"), // Default database name for tests.
		testctr.WithEnv("POSTGRES_HOST_AUTH_METHOD", "trust"), // Simplifies connections for testing
		WithMaxConnections(100),
		WithDSN(), // Enable DSN support for test-specific databases

		// Wait strategies to ensure PostgreSQL is ready
		ctropts.WithWaitForLog("database system is ready to accept connections", 20*time.Second),
		ctropts.WithWaitForExec([]string{"pg_isready", "-U", "postgres", "-d", "test"}, 15*time.Second), // Check specific user/db

		testctr.WithPort("5432"), // Expose the standard PostgreSQL port

		// Unlock startup mutex after options are applied and container is being created
		testctr.OptionFunc(func(cfg interface{}) {
			go unlockStartup()
		}),
	)
}
