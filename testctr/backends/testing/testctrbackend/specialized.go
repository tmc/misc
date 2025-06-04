package testctrbackendtest

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/testctr/backend"
)

// TestBackendWithConfig tests a backend with various configurations.
func RunBackendConfigTests(t *testing.T, b backend.Backend) {
	t.Helper()

	testCases := []struct {
		name   string
		image  string
		config *containerConfig
		test   func(t *testing.T, b backend.Backend, containerID string)
	}{
		{
			name:  "EnvironmentVariables",
			image: "alpine:latest",
			config: NewTestConfig(
				WithEnv("TEST_VAR", "test_value"),
				WithEnv("ANOTHER_VAR", "another_value"),
			),
			test: func(t *testing.T, b backend.Backend, id string) {
				// Check environment variables are set
				exitCode, output, err := b.ExecInContainer(id, []string{"sh", "-c", "echo $TEST_VAR"})
				if err != nil || exitCode != 0 {
					t.Errorf("Failed to get env var: %v", err)
				}
				if !strings.Contains(output, "test_value") {
					t.Errorf("TEST_VAR not set correctly, got: %q", output)
				}
			},
		},
		{
			name:  "CustomCommand",
			image: "alpine:latest",
			config: NewTestConfig(
				WithCommand("sh", "-c", "echo 'custom command' && sleep 30"),
			),
			test: func(t *testing.T, b backend.Backend, id string) {
				// Check logs contain custom command output
				time.Sleep(500 * time.Millisecond) // Give command time to run
				logs, err := b.GetContainerLogs(id)
				if err != nil {
					t.Errorf("Failed to get logs: %v", err)
				}
				if !strings.Contains(logs, "custom command") {
					t.Errorf("Custom command output not found in logs: %q", logs)
				}
			},
		},
		{
			name:  "FileOperations",
			image: "alpine:latest",
			config: NewTestConfig(
				WithFile(CreateTempFile(t, "test file content"), "/test.txt"),
			),
			test: func(t *testing.T, b backend.Backend, id string) {
				// Verify file was copied
				exitCode, output, err := b.ExecInContainer(id, []string{"cat", "/test.txt"})
				if err != nil || exitCode != 0 {
					t.Errorf("Failed to read file: %v", err)
				}
				if !strings.Contains(output, "test file content") {
					t.Errorf("File content incorrect, got: %q", output)
				}
			},
		},
		{
			name:  "Labels",
			image: "alpine:latest",
			config: NewTestConfig(
				WithLabel("test.label", "test-value"),
				WithLabel("another.label", "another-value"),
			),
			test: func(t *testing.T, b backend.Backend, id string) {
				info, err := b.InspectContainer(id)
				if err != nil {
					t.Fatalf("Failed to inspect container: %v", err)
				}
				if info.Config.Labels == nil {
					t.Error("No labels found")
					return
				}
				if val, ok := info.Config.Labels["test.label"]; !ok || val != "test-value" {
					t.Errorf("Label test.label not found or incorrect: %v", info.Config.Labels)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := b.CreateContainer(t, tc.image, tc.config)
			if err != nil {
				t.Fatalf("CreateContainer failed: %v", err)
			}
			defer b.RemoveContainer(id)

			if err := b.StartContainer(id); err != nil {
				t.Fatalf("StartContainer failed: %v", err)
			}

			tc.test(t, b, id)
		})
	}
}

// TestBackendDatabaseContainers tests common database containers.
func RunBackendDatabaseTests(t *testing.T, b backend.Backend) {
	t.Helper()

	databases := []struct {
		name     string
		image    string
		port     string
		readyLog string
		readyCmd []string
		timeout  time.Duration
	}{
		{
			name:     "Redis",
			image:    "redis:7-alpine",
			port:     "6379",
			readyLog: "Ready to accept connections",
			readyCmd: []string{"redis-cli", "PING"},
			timeout:  30 * time.Second,
		},
		{
			name:     "PostgreSQL",
			image:    "postgres:15-alpine",
			port:     "5432",
			readyLog: "database system is ready to accept connections",
			readyCmd: []string{"pg_isready"},
			timeout:  60 * time.Second,
		},
		// MySQL takes longer and might need special handling
		// Uncomment if needed:
		// {
		//     name:     "MySQL",
		//     image:    "mysql:8",
		//     port:     "3306",
		//     readyLog: "ready for connections",
		//     readyCmd: []string{"mysqladmin", "ping"},
		//     timeout:  90 * time.Second,
		// },
	}

	for _, db := range databases {
		t.Run(db.name, func(t *testing.T) {
			// Skip if short tests
			if testing.Short() {
				t.Skipf("Skipping %s test in short mode", db.name)
			}

			cfg := NewTestConfig(
				WithPort(db.port),
			)

			// Add environment variables for databases
			if db.name == "PostgreSQL" {
				cfg = NewTestConfig(
					WithPort(db.port),
					WithEnv("POSTGRES_PASSWORD", "test"),
					WithEnv("POSTGRES_DB", "test"),
				)
			} else if db.name == "MySQL" {
				cfg = NewTestConfig(
					WithPort(db.port),
					WithEnv("MYSQL_ROOT_PASSWORD", "test"),
					WithEnv("MYSQL_DATABASE", "test"),
				)
			}

			id, err := b.CreateContainer(t, db.image, cfg)
			if err != nil {
				t.Fatalf("CreateContainer failed: %v", err)
			}
			defer b.RemoveContainer(id)

			if err := b.StartContainer(id); err != nil {
				t.Fatalf("StartContainer failed: %v", err)
			}

			// Wait for database to be ready
			if db.readyLog != "" {
				err := b.WaitForLog(id, db.readyLog, db.timeout)
				if err != nil {
					logs, _ := b.GetContainerLogs(id)
					t.Fatalf("Database not ready after %v: %v\nLogs:\n%s", db.timeout, err, logs)
				}
			}

			// Verify with ready command
			if len(db.readyCmd) > 0 {
				// Give it a moment after log appears
				time.Sleep(1 * time.Second)

				exitCode, output, err := b.ExecInContainer(id, db.readyCmd)
				if err != nil || exitCode != 0 {
					t.Errorf("%s ready check failed: exit=%d, err=%v, output=%q",
						db.name, exitCode, err, output)
				}
			}

			// Check port is exposed
			info, err := b.InspectContainer(id)
			if err != nil {
				t.Fatalf("InspectContainer failed: %v", err)
			}

			portKey := db.port + "/tcp"
			if _, ok := info.NetworkSettings.Ports[portKey]; !ok {
				t.Errorf("Port %s not exposed for %s", db.port, db.name)
			}
		})
	}
}

// TestBackendStressTest performs stress testing on the backend.
func RunBackendStressTests(t *testing.T, b backend.Backend) {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const (
		numContainers = 10
		numOperations = 50
	)

	// Create multiple containers
	var containerIDs []string
	for i := 0; i < numContainers; i++ {
		id, err := b.CreateContainer(t, "alpine:latest", NewTestConfig(
			WithLabel("stress-test", "true"),
			WithLabel("index", string(rune(i))),
		))
		if err != nil {
			t.Errorf("Failed to create container %d: %v", i, err)
			continue
		}
		containerIDs = append(containerIDs, id)

		if err := b.StartContainer(id); err != nil {
			t.Errorf("Failed to start container %d: %v", i, err)
		}
	}

	// Clean up all containers at the end
	defer func() {
		for _, id := range containerIDs {
			b.RemoveContainer(id)
		}
	}()

	// Perform many operations
	for i := 0; i < numOperations; i++ {
		// Pick a random container
		id := containerIDs[i%len(containerIDs)]

		// Perform various operations
		switch i % 4 {
		case 0:
			b.ExecInContainer(id, []string{"echo", "stress test"})
		case 1:
			b.GetContainerLogs(id)
		case 2:
			b.InspectContainer(id)
		case 3:
			b.InternalIP(id)
		}
	}
}

// TestBackendWithReader tests file operations using io.Reader.
func RunBackendReaderTests(t *testing.T, b backend.Backend) {
	t.Helper()

	content := "content from io.Reader"
	reader := bytes.NewReader([]byte(content))

	cfg := &containerConfig{
		files: []fileEntry{
			{
				Source: reader,
				Target: "/reader-test.txt",
				Mode:   0644,
			},
		},
	}

	id, err := b.CreateContainer(t, "alpine:latest", cfg)
	if err != nil {
		// Backend might not support io.Reader for files
		t.Skipf("Backend doesn't support io.Reader for files: %v", err)
	}
	defer b.RemoveContainer(id)

	if err := b.StartContainer(id); err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	// Verify file content
	exitCode, output, err := b.ExecInContainer(id, []string{"cat", "/reader-test.txt"})
	if err != nil || exitCode != 0 {
		t.Errorf("Failed to read file: %v", err)
	}
	if !strings.Contains(output, content) {
		t.Errorf("File content incorrect, expected %q, got %q", content, output)
	}
}

// RunBackendBenchmarks provides basic benchmarks for backend operations.
func RunBackendBenchmarks(b *testing.B, backend backend.Backend) {
	// Benchmark container creation
	b.Run("CreateContainer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			id, err := backend.CreateContainer(b, "alpine:latest", nil)
			if err != nil {
				b.Fatal(err)
			}
			backend.RemoveContainer(id)
		}
	})

	// Benchmark exec operations
	b.Run("ExecInContainer", func(b *testing.B) {
		id, err := backend.CreateContainer(b, "alpine:latest", nil)
		if err != nil {
			b.Fatal(err)
		}
		defer backend.RemoveContainer(id)

		if err := backend.StartContainer(id); err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			backend.ExecInContainer(id, []string{"echo", "benchmark"})
		}
	})
}
