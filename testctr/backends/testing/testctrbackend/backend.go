package testctrbackendtest

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tmc/misc/testctr/backend"
)

// RunBackendTests runs a comprehensive test suite against a backend implementation.
// This ensures the backend correctly implements all required Backend interface methods.
func RunBackendTests(t *testing.T, b backend.Backend) {
	t.Helper()

	// Run all test suites
	t.Run("Lifecycle", func(t *testing.T) { RunBackendLifecycleTests(t, b) })
	t.Run("Networking", func(t *testing.T) { RunBackendNetworkingTests(t, b) })
	t.Run("Execution", func(t *testing.T) { RunBackendExecutionTests(t, b) })
	t.Run("Logs", func(t *testing.T) { RunBackendLogsTests(t, b) })
	t.Run("Inspection", func(t *testing.T) { RunBackendInspectionTests(t, b) })
	t.Run("ErrorHandling", func(t *testing.T) { RunBackendErrorHandlingTests(t, b) })
	t.Run("Concurrent", func(t *testing.T) { RunBackendConcurrentTests(t, b) })
}

// TestBackendLifecycle tests basic container lifecycle operations.
func RunBackendLifecycleTests(t *testing.T, b backend.Backend) {
	t.Helper()

	// Test basic create, start, stop, remove cycle
	t.Run("BasicLifecycle", func(t *testing.T) {
		id, err := b.CreateContainer(t, "alpine:latest", nil)
		if err != nil {
			t.Fatalf("CreateContainer failed: %v", err)
		}
		if id == "" {
			t.Fatal("CreateContainer returned empty ID")
		}

		// Start container
		if err := b.StartContainer(id); err != nil {
			t.Fatalf("StartContainer failed: %v", err)
		}

		// Verify it's running
		info, err := b.InspectContainer(id)
		if err != nil {
			t.Fatalf("InspectContainer failed: %v", err)
		}
		if !info.State.Running {
			t.Error("Container should be running after start")
		}

		// Stop container
		if err := b.StopContainer(id); err != nil {
			t.Fatalf("StopContainer failed: %v", err)
		}

		// Verify it's stopped
		info, err = b.InspectContainer(id)
		if err != nil {
			t.Fatalf("InspectContainer after stop failed: %v", err)
		}
		if info.State.Running {
			t.Error("Container should not be running after stop")
		}

		// Remove container
		if err := b.RemoveContainer(id); err != nil {
			t.Fatalf("RemoveContainer failed: %v", err)
		}

		// Verify it's gone
		_, err = b.InspectContainer(id)
		if err == nil {
			t.Error("InspectContainer should fail after remove")
		}
	})

	// Test auto-start behavior
	t.Run("AutoStart", func(t *testing.T) {
		id, err := b.CreateContainer(t, "alpine:latest", nil)
		if err != nil {
			t.Fatalf("CreateContainer failed: %v", err)
		}
		defer b.RemoveContainer(id)

		// Some backends auto-start containers
		info, err := b.InspectContainer(id)
		if err != nil {
			t.Fatalf("InspectContainer failed: %v", err)
		}

		// If not running, start it
		if !info.State.Running {
			if err := b.StartContainer(id); err != nil {
				t.Fatalf("StartContainer failed: %v", err)
			}
		}

		// Double-start should be idempotent
		if err := b.StartContainer(id); err != nil {
			t.Errorf("Double StartContainer failed: %v", err)
		}
	})
}

// TestBackendNetworking tests port mapping and network configuration.
func RunBackendNetworkingTests(t *testing.T, b backend.Backend) {
	t.Helper()

	t.Run("PortMapping", func(t *testing.T) {
		// Create config that requests port mapping
		// Note: This assumes the backend can handle basic config
		id, err := b.CreateContainer(t, "nginx:alpine", nil)
		if err != nil {
			t.Fatalf("CreateContainer failed: %v", err)
		}
		defer b.RemoveContainer(id)

		if err := b.StartContainer(id); err != nil {
			t.Fatalf("StartContainer failed: %v", err)
		}

		// Check internal IP
		ip, err := b.InternalIP(id)
		if err != nil {
			// Some backends might not support this
			t.Logf("InternalIP not supported: %v", err)
		} else if ip == "" {
			t.Error("InternalIP returned empty string")
		} else {
			// Validate IP format
			parts := strings.Split(ip, ".")
			if len(parts) != 4 {
				t.Errorf("Invalid IP format: %s", ip)
			}
		}
	})
}

// TestBackendExecution tests command execution in containers.
func RunBackendExecutionTests(t *testing.T, b backend.Backend) {
	t.Helper()

	id, err := b.CreateContainer(t, "alpine:latest", nil)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer b.RemoveContainer(id)

	if err := b.StartContainer(id); err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	// Test successful command
	t.Run("SuccessfulExec", func(t *testing.T) {
		exitCode, output, err := b.ExecInContainer(id, []string{"echo", "hello"})
		if err != nil {
			t.Fatalf("ExecInContainer failed: %v", err)
		}
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}
		if !strings.Contains(output, "hello") {
			t.Errorf("Expected output to contain 'hello', got %q", output)
		}
	})

	// Test failing command
	t.Run("FailingExec", func(t *testing.T) {
		exitCode, output, err := b.ExecInContainer(id, []string{"sh", "-c", "exit 42"})
		// Error might be nil even with non-zero exit code
		if exitCode != 42 {
			t.Errorf("Expected exit code 42, got %d", exitCode)
		}
		t.Logf("Failing exec: err=%v, output=%q", err, output)
	})

	// Test command not found
	t.Run("CommandNotFound", func(t *testing.T) {
		exitCode, output, err := b.ExecInContainer(id, []string{"nonexistentcommand"})
		if exitCode == 0 {
			t.Error("Expected non-zero exit code for non-existent command")
		}
		t.Logf("Command not found: exitCode=%d, err=%v, output=%q", exitCode, err, output)
	})
}

// TestBackendLogs tests log retrieval and waiting.
func RunBackendLogsTests(t *testing.T, b backend.Backend) {
	t.Helper()

	// Test log retrieval with containers that actually generate logs
	t.Run("GetLogs", func(t *testing.T) {
		testMessage := "test-log-message-" + time.Now().Format("20060102150405")
		
		// Create container with command that outputs the test message
		id, err := b.CreateContainer(t, "alpine:latest", NewTestConfig(
			WithCommand("sh", "-c", "echo '"+testMessage+"' && sleep 30"),
		))
		if err != nil {
			t.Fatalf("CreateContainer failed: %v", err)
		}
		defer b.RemoveContainer(id)

		if err := b.StartContainer(id); err != nil {
			t.Fatalf("StartContainer failed: %v", err)
		}

		// Give container time to output the message
		time.Sleep(200 * time.Millisecond)

		logs, err := b.GetContainerLogs(id)
		if err != nil {
			t.Fatalf("GetContainerLogs failed: %v", err)
		}
		if !strings.Contains(logs, testMessage) {
			t.Errorf("Logs should contain %q, got: %q", testMessage, logs)
		}
	})

	// Test wait for log with a container that generates delayed output
	t.Run("WaitForLog", func(t *testing.T) {
		futureMessage := "future-message-" + time.Now().Format("20060102150405")
		
		// Create container with command that outputs after a delay
		id, err := b.CreateContainer(t, "alpine:latest", NewTestConfig(
			WithCommand("sh", "-c", "sleep 0.5 && echo '"+futureMessage+"' && sleep 30"),
		))
		if err != nil {
			t.Fatalf("CreateContainer failed: %v", err)
		}
		defer b.RemoveContainer(id)

		if err := b.StartContainer(id); err != nil {
			t.Fatalf("StartContainer failed: %v", err)
		}

		// Wait for the delayed message
		err = b.WaitForLog(id, futureMessage, 5*time.Second)
		if err != nil {
			t.Errorf("WaitForLog failed: %v", err)
		}
	})

	// Test wait timeout
	t.Run("WaitTimeout", func(t *testing.T) {
		// Create container that doesn't output the expected message
		id, err := b.CreateContainer(t, "alpine:latest", NewTestConfig(
			WithCommand("sh", "-c", "echo 'different message' && sleep 30"),
		))
		if err != nil {
			t.Fatalf("CreateContainer failed: %v", err)
		}
		defer b.RemoveContainer(id)

		if err := b.StartContainer(id); err != nil {
			t.Fatalf("StartContainer failed: %v", err)
		}

		err = b.WaitForLog(id, "this-will-never-appear", 500*time.Millisecond)
		if err == nil {
			t.Error("WaitForLog should timeout for non-existent log")
		}
		if !strings.Contains(err.Error(), "timeout") {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	})
}

// TestBackendInspection tests container inspection capabilities.
func RunBackendInspectionTests(t *testing.T, b backend.Backend) {
	t.Helper()

	id, err := b.CreateContainer(t, "alpine:latest", nil)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	defer b.RemoveContainer(id)

	if err := b.StartContainer(id); err != nil {
		t.Fatalf("StartContainer failed: %v", err)
	}

	info, err := b.InspectContainer(id)
	if err != nil {
		t.Fatalf("InspectContainer failed: %v", err)
	}

	// Validate required fields
	if info.ID == "" {
		t.Error("ContainerInfo.ID should not be empty")
	}
	if !strings.HasPrefix(info.ID, id) && info.ID != id {
		t.Errorf("ContainerInfo.ID %q doesn't match container ID %q", info.ID, id)
	}
	if info.Created == "" {
		t.Error("ContainerInfo.Created should not be empty")
	}
	if info.State.Status == "" {
		t.Error("ContainerInfo.State.Status should not be empty")
	}

	// Test Commit (if supported)
	t.Run("Commit", func(t *testing.T) {
		// Make a change to the container
		_, _, err := b.ExecInContainer(id, []string{"touch", "/test-file"})
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Commit the container
		imageName := fmt.Sprintf("testctr-commit-test:%d", time.Now().Unix())
		err = b.Commit(id, imageName)
		if err != nil {
			// Commit might not be supported by all backends
			t.Logf("Commit not supported or failed: %v", err)
			return
		}

		// Try to create a new container from the committed image
		id2, err := b.CreateContainer(t, imageName, nil)
		if err != nil {
			t.Errorf("Failed to create container from committed image: %v", err)
			return
		}
		defer b.RemoveContainer(id2)

		// Verify the file exists in the new container
		if err := b.StartContainer(id2); err != nil {
			t.Fatalf("Failed to start container from committed image: %v", err)
		}

		exitCode, _, _ := b.ExecInContainer(id2, []string{"test", "-f", "/test-file"})
		if exitCode != 0 {
			t.Error("Test file should exist in committed image")
		}
	})
}

// TestBackendErrorHandling tests error conditions.
func RunBackendErrorHandlingTests(t *testing.T, b backend.Backend) {
	t.Helper()

	// Test invalid image
	t.Run("InvalidImage", func(t *testing.T) {
		_, err := b.CreateContainer(t, "this-image-definitely-does-not-exist:latest", nil)
		if err == nil {
			t.Error("CreateContainer should fail with non-existent image")
		}
	})

	// Test operations on non-existent container
	t.Run("NonExistentContainer", func(t *testing.T) {
		fakeID := "nonexistent-container-id"

		if err := b.StartContainer(fakeID); err == nil {
			t.Error("StartContainer should fail with non-existent container")
		}

		if err := b.StopContainer(fakeID); err == nil {
			t.Error("StopContainer should fail with non-existent container")
		}

		if _, err := b.InspectContainer(fakeID); err == nil {
			t.Error("InspectContainer should fail with non-existent container")
		}

		if _, _, err := b.ExecInContainer(fakeID, []string{"echo", "test"}); err == nil {
			t.Error("ExecInContainer should fail with non-existent container")
		}
	})

	// Test double remove
	t.Run("DoubleRemove", func(t *testing.T) {
		id, err := b.CreateContainer(t, "alpine:latest", nil)
		if err != nil {
			t.Fatalf("CreateContainer failed: %v", err)
		}

		// First remove should succeed
		if err := b.RemoveContainer(id); err != nil {
			t.Fatalf("First RemoveContainer failed: %v", err)
		}

		// Second remove should fail
		if err := b.RemoveContainer(id); err == nil {
			t.Error("Second RemoveContainer should fail")
		}
	})
}

// TestBackendConcurrent tests concurrent operations.
func RunBackendConcurrentTests(t *testing.T, b backend.Backend) {
	t.Helper()

	const numContainers = 5

	// Create multiple containers concurrently
	t.Run("ConcurrentCreate", func(t *testing.T) {
		type result struct {
			id  string
			err error
		}
		results := make(chan result, numContainers)

		for i := 0; i < numContainers; i++ {
			go func(i int) {
				id, err := b.CreateContainer(t, "alpine:latest", nil)
				results <- result{id, err}
			}(i)
		}

		// Collect results
		var ids []string
		for i := 0; i < numContainers; i++ {
			r := <-results
			if r.err != nil {
				t.Errorf("Concurrent CreateContainer %d failed: %v", i, r.err)
				continue
			}
			ids = append(ids, r.id)
		}

		// Clean up
		for _, id := range ids {
			b.RemoveContainer(id)
		}
	})

	// Test concurrent operations on same container
	t.Run("ConcurrentOpsOnSameContainer", func(t *testing.T) {
		id, err := b.CreateContainer(t, "alpine:latest", nil)
		if err != nil {
			t.Fatalf("CreateContainer failed: %v", err)
		}
		defer b.RemoveContainer(id)

		if err := b.StartContainer(id); err != nil {
			t.Fatalf("StartContainer failed: %v", err)
		}

		// Run multiple execs concurrently
		done := make(chan bool, 3)

		// Exec operations
		go func() {
			for i := 0; i < 5; i++ {
				b.ExecInContainer(id, []string{"echo", fmt.Sprintf("exec-%d", i)})
			}
			done <- true
		}()

		// Log operations
		go func() {
			for i := 0; i < 5; i++ {
				b.GetContainerLogs(id)
			}
			done <- true
		}()

		// Inspect operations
		go func() {
			for i := 0; i < 5; i++ {
				b.InspectContainer(id)
			}
			done <- true
		}()

		// Wait for all operations to complete
		for i := 0; i < 3; i++ {
			<-done
		}
	})
}

// BackendConfig provides configuration for backend testing.
// Backends can implement this interface to provide custom configuration
// for the test suite.
type BackendConfig interface {
	// SkipTests returns a list of test names to skip for this backend.
	SkipTests() []string

	// TestImage returns the image to use for testing.
	// Default is "alpine:latest".
	TestImage() string

	// Timeout returns the timeout for operations.
	// Default is 30 seconds.
	Timeout() time.Duration
}
