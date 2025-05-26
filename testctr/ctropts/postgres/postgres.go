// Package postgres provides PostgreSQL-specific options for testctr
package postgres

import (
	"fmt"
	"time"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// WithDatabase sets the initial database name
func WithDatabase(name string) testctr.Option {
	return testctr.WithEnv("POSTGRES_DB", name)
}

// WithPassword sets the superuser password
func WithPassword(password string) testctr.Option {
	return testctr.WithEnv("POSTGRES_PASSWORD", password)
}

// WithUser creates the superuser with a specific name
func WithUser(username string) testctr.Option {
	return testctr.WithEnv("POSTGRES_USER", username)
}

// WithInitScript mounts an initialization SQL script
func WithInitScript(hostPath string) testctr.Option {
	return ctropts.WithBindMount(hostPath, "/docker-entrypoint-initdb.d/init.sql")
}

// WithConfig adds custom PostgreSQL configuration
func WithConfig(config map[string]string) testctr.Option {
	opts := make([]testctr.Option, 0, len(config))
	for key, value := range config {
		opts = append(opts, testctr.WithCommand("-c", fmt.Sprintf("%s=%s", key, value)))
	}
	return testctr.Options(opts...)
}

// WithLocale sets the database locale
func WithLocale(locale string) testctr.Option {
	return testctr.Options(
		testctr.WithEnv("LANG", locale),
		testctr.WithEnv("LC_COLLATE", locale),
		testctr.WithEnv("LC_CTYPE", locale),
	)
}

// WithTimezone sets the timezone
func WithTimezone(tz string) testctr.Option {
	return testctr.WithEnv("TZ", tz)
}

// WithMaxConnections sets the maximum number of connections
func WithMaxConnections(max int) testctr.Option {
	return testctr.WithCommand("-c", fmt.Sprintf("max_connections=%d", max))
}

// WithSharedBuffers sets the shared buffer size
func WithSharedBuffers(size string) testctr.Option {
	return testctr.WithCommand("-c", "shared_buffers="+size)
}

// WithLogging enables statement logging
func WithLogging() testctr.Option {
	return testctr.Options(
		testctr.WithCommand("-c", "log_statement=all"),
		testctr.WithCommand("-c", "log_connections=on"),
		testctr.WithCommand("-c", "log_disconnections=on"),
	)
}

// WithSSL enables SSL connections
func WithSSL(certPath, keyPath string) testctr.Option {
	return testctr.Options(
		ctropts.WithBindMount(certPath, "/var/lib/postgresql/server.crt"),
		ctropts.WithBindMount(keyPath, "/var/lib/postgresql/server.key"),
		testctr.WithCommand("-c", "ssl=on"),
		testctr.WithCommand("-c", "ssl_cert_file=/var/lib/postgresql/server.crt"),
		testctr.WithCommand("-c", "ssl_key_file=/var/lib/postgresql/server.key"),
	)
}

// WithExtensions preloads PostgreSQL extensions
func WithExtensions(extensions ...string) testctr.Option {
	opts := make([]testctr.Option, len(extensions))
	for i, ext := range extensions {
		opts[i] = testctr.WithCommand("-c", fmt.Sprintf("shared_preload_libraries=%s", ext))
	}
	return testctr.Options(opts...)
}

// WithDelayAfterLog adds a delay after the log wait completes
// This is a workaround for PostgreSQL's initialization process
// DEPRECATED: This function is no longer needed as PostgreSQL coordination
// is now handled via startup mutex in the Default() function
func WithDelayAfterLog(delay time.Duration) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		// No-op: PostgreSQL startup coordination is now handled via mutex
		// This function is kept for backward compatibility
	})
}

// Default returns sensible defaults for PostgreSQL testing
func Default() testctr.Option {
	// Lock startup to prevent race conditions
	lockStartup()
	
	return testctr.Options(
		testctr.WithEnv("POSTGRES_HOST_AUTH_METHOD", "trust"),
		WithDatabase("test"),
		WithMaxConnections(100),
		WithDSN(),
		// Wait for PostgreSQL to be ready with multiple checks
		ctropts.WithWaitForLog("database system is ready to accept connections", 20*time.Second),
		ctropts.WithWaitForExec([]string{"pg_isready", "-U", "postgres"}, 15*time.Second),
		testctr.WithPort("5432"),
		// Add a cleanup function to unlock after container is created
		testctr.OptionFunc(func(cfg interface{}) {
			// Schedule unlock after options are applied
			go unlockStartup()
		}),
	)
}
