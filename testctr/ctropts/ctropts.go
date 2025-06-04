// Package ctropts provides additional options for testctr containers
package ctropts

import (
	"github.com/tmc/misc/testctr"
)

// WithBindMounts adds multiple bind mounts at once.
// Each key-value pair in the mounts map represents a hostPath-containerPath binding.
func WithBindMounts(mounts map[string]string) testctr.Option {
	opts := make([]testctr.Option, 0, len(mounts))
	for hostPath, containerPath := range mounts {
		opts = append(opts, WithBindMount(hostPath, containerPath))
	}
	return testctr.Options(opts...)
}

// WithEnvMap adds multiple environment variables at once from a map.
func WithEnvMap(env map[string]string) testctr.Option {
	opts := make([]testctr.Option, 0, len(env))
	for key, value := range env {
		opts = append(opts, testctr.WithEnv(key, value))
	}
	return testctr.Options(opts...)
}

// WithPodman configures the container to use Podman instead of Docker.
// This is a convenience wrapper around WithRuntime("podman").
func WithPodman() testctr.Option {
	return WithRuntime("podman")
}

// WithPlatform specifies the target platform/architecture for the container.
// This enables running containers for different architectures than the host.
//
// Common platform values:
//   - "linux/amd64" - x86_64 architecture
//   - "linux/arm64" - ARM64 architecture  
//   - "linux/arm/v7" - ARM v7 architecture
//   - "linux/386" - 32-bit x86 architecture
//
// Example:
//   // Run ARM64 container on any host
//   container := testctr.New(t, "nginx", ctropts.WithPlatform("linux/arm64"))
//
// Platform emulation is handled automatically by Docker/Podman when the target
// platform differs from the host architecture.
// WithLabel adds a label to the container.
func WithLabel(key, value string) testctr.Option {
	// For now, store as environment variable since we don't have direct label support
	// Backends can read this and convert to actual labels
	return testctr.WithEnv("LABEL_"+key, value)
}

func WithPlatform(platform string) testctr.Option {
	// Store platform information as a special environment variable that
	// backends can recognize and use when creating containers with --platform
	return testctr.WithEnv("TESTCTR_PLATFORM", platform)
}

// WithNerdctl configures the container to use nerdctl (containerd CLI).
// This is a convenience wrapper around WithRuntime("nerdctl").
func WithNerdctl() testctr.Option {
	return WithRuntime("nerdctl")
}

// WithFinch configures the container to use AWS Finch.
// This is a convenience wrapper around WithRuntime("finch").
func WithFinch() testctr.Option {
	return WithRuntime("finch")
}

// WithLima configures the container to use Lima (Linux VMs on macOS).
// This is a convenience wrapper around WithRuntime("lima").
func WithLima() testctr.Option {
	return WithRuntime("lima")
}

// WithColima configures the container to use Colima (Containers on macOS with Lima).
// This is a convenience wrapper around WithRuntime("colima").
func WithColima() testctr.Option {
	return WithRuntime("colima")
}
