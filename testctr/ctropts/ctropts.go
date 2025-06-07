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
