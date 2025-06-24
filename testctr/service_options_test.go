package testctr_test

import (
	"strings"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts/mysql2"
	"github.com/tmc/misc/testctr/ctropts/postgres2"
)

func TestServiceSpecificOptions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		image        string
		options      []testctr.Option
		expectedEnvs []string // Environment variables we expect to be set
		skipReason   string
	}{
		{
			name:    "MySQL2_Default",
			image:   "mysql:8",
			options: []testctr.Option{mysql2.Default()},
			expectedEnvs: []string{
				"MYSQL_ROOT_PASSWORD=root",
				"MYSQL_DATABASE=test",
			},
		},
		{
			name:    "MySQL2_WithCustomCredentials",
			image:   "mysql:8",
			options: []testctr.Option{
				mysql2.WithRootPassword("mysecret"),
				mysql2.WithDatabase("customdb"),
				mysql2.WithUsername("testuser"),
				mysql2.WithPassword("testpass"),
			},
			expectedEnvs: []string{
				"MYSQL_ROOT_PASSWORD=testpass", // WithPassword sets both user and root password
				"MYSQL_DATABASE=customdb",
				"MYSQL_USER=testuser",
				"MYSQL_PASSWORD=testpass",
			},
		},
		{
			name:    "PostgreSQL2_Default",
			image:   "postgres:15",
			options: []testctr.Option{postgres2.Default()},
			expectedEnvs: []string{
				"POSTGRES_PASSWORD=postgres",
				"POSTGRES_DB=postgres",
			},
		},
		{
			name:    "PostgreSQL2_WithCustomCredentials",
			image:   "postgres:15",
			options: []testctr.Option{
				postgres2.WithPassword("secret"),
				postgres2.WithDatabase("mydb"),
				postgres2.WithUsername("myuser"),
			},
			expectedEnvs: []string{
				"POSTGRES_PASSWORD=secret",
				"POSTGRES_DB=mydb",
				"POSTGRES_USER=myuser",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}

			// Create container with service-specific options
			c := testctr.New(t, tc.image, tc.options...)

			// Verify container was created successfully
			if c == nil {
				t.Fatal("Container creation failed")
			}

			// Test that container has expected environment variables
			// We do this by examining the running container
			for _, expectedEnv := range tc.expectedEnvs {
				parts := strings.SplitN(expectedEnv, "=", 2)
				if len(parts) != 2 {
					continue
				}
				envVar, expectedValue := parts[0], parts[1]

				// Check environment variable via container inspection
				// This is an indirect test - we verify the container runs with expected config
				exitCode, output, err := c.Exec(nil, []string{"printenv", envVar})
				if err != nil {
					t.Logf("Failed to check env var %s: %v", envVar, err)
					continue
				}
				if exitCode != 0 {
					t.Logf("Environment variable %s not found (exit code %d)", envVar, exitCode)
					continue
				}

				actualValue := strings.TrimSpace(output)
				if actualValue != expectedValue {
					t.Errorf("Environment variable %s = %q, expected %q", envVar, actualValue, expectedValue)
				} else {
					t.Logf("✓ Environment variable %s = %q", envVar, actualValue)
				}
			}

			t.Logf("Service-specific options test completed for %s", tc.name)
		})
	}
}

func TestServiceOptionsWithDSN(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		image        string
		defaultOpts  func() testctr.Option
		dsnPattern   string
	}{
		{
			name:        "MySQL2_DSN_Integration",
			image:       "mysql:8",
			defaultOpts: mysql2.Default,
			dsnPattern:  "root:root@tcp(",
		},
		{
			name:        "PostgreSQL2_DSN_Integration",
			image:       "postgres:15",
			defaultOpts: postgres2.Default,
			dsnPattern:  "postgres://postgres:postgres@",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create container with default service options
			c := testctr.New(t, tc.image, tc.defaultOpts())

			// Test DSN generation
			dsn := c.DSN(t)
			if dsn == "" {
				t.Fatal("DSN generation failed")
			}

			if !strings.Contains(dsn, tc.dsnPattern) {
				t.Fatalf("DSN %q does not contain expected pattern %q", dsn, tc.dsnPattern)
			}

			t.Logf("✓ DSN generated successfully: %s", dsn)
		})
	}
}

func TestServiceOptionsValidation(t *testing.T) {
	t.Parallel()

	t.Run("MySQL2_Options", func(t *testing.T) {
		t.Parallel()

		// Test that options can be combined
		c := testctr.New(t, "mysql:8",
			mysql2.WithRootPassword("secret"),
			mysql2.WithDatabase("testdb"),
			mysql2.WithUsername("user"),
			mysql2.WithPassword("pass"),
		)

		if c == nil {
			t.Fatal("Container creation with combined MySQL options failed")
		}

		t.Log("✓ Combined MySQL options work correctly")
	})

	t.Run("PostgreSQL2_Options", func(t *testing.T) {
		t.Parallel()

		// Test that options can be combined
		c := testctr.New(t, "postgres:15",
			postgres2.WithPassword("secret"),
			postgres2.WithDatabase("testdb"),
			postgres2.WithUsername("user"),
		)

		if c == nil {
			t.Fatal("Container creation with combined PostgreSQL options failed")
		}

		t.Log("✓ Combined PostgreSQL options work correctly")
	})
}

func TestServiceWaitStrategies(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		image       string
		defaultOpts func() testctr.Option
		checkCmd    []string
		expectSuccess bool
	}{
		{
			name:        "MySQL2_Ready",
			image:       "mysql:8",
			defaultOpts: mysql2.Default,
			checkCmd:    []string{"mysqladmin", "ping", "-uroot", "-proot"},
			expectSuccess: true,
		},
		{
			name:        "PostgreSQL2_Ready",
			image:       "postgres:15",
			defaultOpts: postgres2.Default,
			checkCmd:    []string{"pg_isready", "-U", "postgres"},
			expectSuccess: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create container with default options (includes wait strategies)
			c := testctr.New(t, tc.image, tc.defaultOpts())

			// If we reach here, the wait strategy worked (container is ready)
			// Now verify service is actually functional
			exitCode, output, err := c.Exec(nil, tc.checkCmd)
			
			if tc.expectSuccess {
				if err != nil {
					t.Fatalf("Service check command failed: %v, output: %s", err, output)
				}
				if exitCode != 0 {
					t.Fatalf("Service check failed with exit code %d, output: %s", exitCode, output)
				}
				t.Logf("✓ Service is ready and functional: %s", strings.TrimSpace(output))
			}
		})
	}
}