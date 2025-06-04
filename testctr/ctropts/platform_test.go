package ctropts_test

import (
	"context"
	"testing"

	"github.com/tmc/misc/testctr"
	"github.com/tmc/misc/testctr/ctropts"
)

// TestPlatformSupport demonstrates running containers with different architectures
func TestPlatformSupport(t *testing.T) {
	t.Parallel()

	// Test AMD64 platform
	t.Run("AMD64", func(t *testing.T) {
		t.Parallel()
		testPlatform(t, "linux/amd64")
	})

	// Test ARM64 platform - this will use emulation if running on different architecture
	t.Run("ARM64", func(t *testing.T) {
		t.Parallel()
		testPlatform(t, "linux/arm64")
	})
}

func testPlatform(t *testing.T, platform string) {
	// Create container with specific platform
	container := testctr.New(t, "alpine:latest",
		ctropts.WithPlatform(platform))

	// Test that the container runs (platform emulation should work)
	exitCode, output, err := container.Exec(context.Background(), []string{"uname", "-m"})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Expected exit code 0, got %d", exitCode)
	}

	t.Logf("Platform %s: Architecture reported as %s", platform, output)

	// Verify we can run basic commands
	exitCode, output, err = container.Exec(context.Background(), []string{"echo", "platform test successful"})
	if err != nil {
		t.Fatalf("Platform test exec failed: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Expected exit code 0, got %d", exitCode)
	}

	expected := "platform test successful\n"
	if output != expected {
		t.Fatalf("Expected output %q, got %q", expected, output)
	}

	t.Logf("✅ Platform %s test successful", platform)
}