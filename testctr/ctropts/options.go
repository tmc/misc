// Package ctropts provides advanced, general-purpose options for testctr containers.
// These options are not specific to any particular service (like MySQL or Redis)
// but offer more fine-grained control over container runtime behavior.
package ctropts

import (
	"time"

	"github.com/tmc/misc/testctr"
)

// WithBindMount returns an Option that mounts a host directory or file
// into the container at the specified container path.
func WithBindMount(hostPath, containerPath string) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type bindMounter interface {
			SetBindMount(hostPath, containerPath string)
		}
		if m, ok := cfg.(bindMounter); ok {
			m.SetBindMount(hostPath, containerPath)
		}
	})
}

// WithNetwork returns an Option that sets the container network mode
// (e.g., "host", "bridge", or a custom network name).
func WithNetwork(network string) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type networkSetter interface {
			SetNetwork(network string)
		}
		if s, ok := cfg.(networkSetter); ok {
			s.SetNetwork(network)
		}
	})
}

// WithUser returns an Option that sets the user for commands executed within the container.
// Format: "user", "user:group", "uid", "uid:gid".
func WithUser(user string) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type userSetter interface {
			SetUser(user string)
		}
		if s, ok := cfg.(userSetter); ok {
			s.SetUser(user)
		}
	})
}

// WithLogFilter returns an Option that sets a filter function for container logs.
// The filter function receives each log line and returns true if the line should
// be displayed, false otherwise. This is useful for suppressing noisy or irrelevant
// log output during tests.
func WithLogFilter(filter func(string) bool) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type logFilterSetter interface {
			SetLogFilter(filter func(string) bool)
		}
		if s, ok := cfg.(logFilterSetter); ok {
			s.SetLogFilter(filter)
		}
	})
}

// WithWorkingDir returns an Option that sets the working directory inside the container.
func WithWorkingDir(dir string) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type workdirSetter interface {
			SetWorkingDir(dir string)
		}
		if s, ok := cfg.(workdirSetter); ok {
			s.SetWorkingDir(dir)
		}
	})
}

// WithPrivileged returns an Option that runs the container in privileged mode.
// Use with caution as this gives the container extended permissions on the host.
func WithPrivileged() testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type privilegedSetter interface {
			SetPrivileged()
		}
		if s, ok := cfg.(privilegedSetter); ok {
			s.SetPrivileged()
		}
	})
}

// WithLogs returns an Option that enables streaming of container logs to t.Logf.
// This is equivalent to setting the -testctr.verbose flag for the specific container.
func WithLogs() testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type logSetter interface {
			SetLogs()
		}
		if s, ok := cfg.(logSetter); ok {
			s.SetLogs()
		}
	})
}

// WithRuntime returns an Option that forces the use of a specific container runtime
// (e.g., "docker", "podman", "nerdctl").
func WithRuntime(runtime string) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type runtimeSetter interface {
			SetRuntime(runtime string)
		}
		if s, ok := cfg.(runtimeSetter); ok {
			s.SetRuntime(runtime)
		}
	})
}

// WithStartupTimeout returns an Option that sets the overall startup timeout for the container.
// This includes time for pulling the image, creating, and starting the container,
// as well as any wait strategies.
func WithStartupTimeout(timeout time.Duration) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type timeoutSetter interface {
			SetStartupTimeout(timeout time.Duration)
		}
		if s, ok := cfg.(timeoutSetter); ok {
			s.SetStartupTimeout(timeout)
		}
	})
}

// WithWaitForHTTP returns an Option that waits for an HTTP endpoint to become available.
// It polls the specified path on the container's internalPort until it receives the expectedStatus.
// This relies on `curl` or `wget` being available in the container.
//
// Example: WithWaitForHTTP("/healthz", "8080", 200, 30*time.Second)
func WithWaitForHTTP(path, internalPort string, expectedStatus int, timeout time.Duration) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type httpWaiter interface {
			// SetWaitForHTTP is a simplified signature for the internal implementation.
			// The internal implementation will need more parameters.
			// This option will add a wait condition that captures all necessary params.
			SetWaitForHTTP(path string) // This signature is for the config setter.
			// The actual wait function will need internalPort, expectedStatus, timeout.
		}

		// Use the proper method name that matches internal.go
		type httpWaitSetter interface {
			SetWaitForHTTPOpt(path, internalPort string, expectedStatus int, timeout time.Duration)
		}
		if w, ok := cfg.(httpWaitSetter); ok {
			w.SetWaitForHTTPOpt(path, internalPort, expectedStatus, timeout)
		}
	})
}

// WithWaitForExec returns an option that waits until a command succeeds (exit code 0)
// when executed inside the container.
func WithWaitForExec(cmd []string, timeout time.Duration) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type execWaiter interface {
			SetWaitForExecOpt(cmd []string, timeout time.Duration)
		}
		if w, ok := cfg.(execWaiter); ok {
			w.SetWaitForExecOpt(cmd, timeout)
		}
	})
}

// WithWaitForLog returns an option that waits for a specific log line to appear
// in the container's logs.
func WithWaitForLog(logLine string, timeout time.Duration) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type logWaiter interface {
			SetWaitForLogOpt(logLine string, timeout time.Duration)
		}
		if w, ok := cfg.(logWaiter); ok {
			w.SetWaitForLogOpt(logLine, timeout)
		}
	})
}

// WithDSNProvider returns an Option that sets a custom DSNProvider for the container.
// This is used by database modules (e.g., mysql, postgres) to enable the
// `container.DSN(t)` method for generating test-specific database connection strings.
func WithDSNProvider(provider testctr.DSNProvider) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type dsnSetter interface {
			SetDSNProvider(provider testctr.DSNProvider)
		}
		if s, ok := cfg.(dsnSetter); ok {
			s.SetDSNProvider(provider)
		}
	})
}

// WithMemoryLimit returns an Option that sets the memory limit for the container
// (e.g., "512m", "1g").
func WithMemoryLimit(limit string) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type memoryLimiter interface {
			SetMemoryLimit(limit string)
		}
		if m, ok := cfg.(memoryLimiter); ok {
			m.SetMemoryLimit(limit)
		}
	})
}

// WithStartupDelay returns an Option that adds a fixed delay before
// the container creation process begins.
func WithStartupDelay(delay time.Duration) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type startupDelayer interface {
			SetStartupDelay(delay time.Duration)
		}
		if d, ok := cfg.(startupDelayer); ok {
			d.SetStartupDelay(delay)
		}
	})
}
