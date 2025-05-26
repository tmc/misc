package ctropts

import (
	"time"

	"github.com/tmc/misc/testctr"
)

// WithBindMount returns an Option that mounts a host directory
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

// WithNetwork returns an Option that sets the container network
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

// WithUser returns an Option that sets the container user
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

// WithWorkingDir returns an Option that sets the working directory
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

// WithPrivileged returns an Option that runs the container in privileged mode
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

// WithLogs returns an Option that streams container logs to t.Logf
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

// WithStartupTimeout returns an Option that sets the startup timeout
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

// WithWaitForHTTP returns an Option that waits for an HTTP endpoint
func WithWaitForHTTP(path string) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type httpWaiter interface {
			SetWaitForHTTP(path string)
		}
		if w, ok := cfg.(httpWaiter); ok {
			w.SetWaitForHTTP(path)
		}
	})
}

// WithWaitForExec returns an option that waits until a command succeeds in the container
func WithWaitForExec(cmd []string, timeout time.Duration) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type execWaiter interface {
			SetWaitForExec(cmd []string, timeout time.Duration)
		}
		if w, ok := cfg.(execWaiter); ok {
			w.SetWaitForExec(cmd, timeout)
		}
	})
}

// WithWaitForLog returns an option that waits for a specific log line
func WithWaitForLog(logLine string, timeout time.Duration) testctr.Option {
	return testctr.OptionFunc(func(cfg interface{}) {
		type logWaiter interface {
			SetWaitForLog(logLine string, timeout time.Duration)
		}
		if w, ok := cfg.(logWaiter); ok {
			w.SetWaitForLog(logLine, timeout)
		}
	})
}

// WithDSNProvider returns an Option that sets a custom DSN provider
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

// WithStartupDelay returns an Option that adds a delay before starting the container
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
