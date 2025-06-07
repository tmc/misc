package testctr

// Option configures container creation. Use the With* functions to create options.
// Multiple options can be combined with Options(). Advanced options are in ctropts.
type Option interface {
	apply(*containerConfig)
}

// optionFunc wraps a function to satisfy the Option interface.
// This is the standard way to create new Option implementations.
type optionFunc func(*containerConfig)

// apply implements the Option interface for optionFunc.
func (f optionFunc) apply(cfg *containerConfig) {
	f(cfg)
}

// Options combines multiple options into one. Nil options are ignored.
//
//	defaults := testctr.Options(
//	    testctr.WithPort("6379"),
//	    testctr.WithEnv("REDIS_PASSWORD", "test"),
//	)
func Options(opts ...Option) Option {
	return optionFunc(func(cfg *containerConfig) {
		for _, opt := range opts {
			if opt != nil {
				opt.apply(cfg)
			}
		}
	})
}

// WithEnv sets an environment variable in the container.
func WithEnv(key, value string) Option {
	return optionFunc(func(cfg *containerConfig) {
		if cfg.dockerRun != nil { // Applicable to default CLI backend
			cfg.dockerRun.env[key] = value
		}
		// For other backends, this would be translated by the adapter
		// by storing on a generic part of containerConfig if needed,
		// or directly if the adapter can access dockerRun-like fields.
	})
}

// WithPort exposes a container port. The port can be "8080" or "8080/tcp".
// Use Port() or Endpoint() to get the mapped host port after creation.
func WithPort(port string) Option {
	return optionFunc(func(cfg *containerConfig) {
		if cfg.dockerRun != nil { // Applicable to default CLI backend
			cfg.dockerRun.ports = append(cfg.dockerRun.ports, port)
		}
	})
}

// WithCommand overrides the container's default command (CMD).
func WithCommand(cmd ...string) Option {
	return optionFunc(func(cfg *containerConfig) {
		if cfg.dockerRun != nil { // Applicable to default CLI backend
			cfg.dockerRun.cmd = cmd
		}
	})
}

// OptionFunc creates custom options. The function receives the internal config
// and should use type assertions to call setter methods. Used by ctropts.
func OptionFunc(fn func(interface{})) Option {
	return optionFunc(func(cfg *containerConfig) {
		fn(cfg) // Pass the concrete *containerConfig to the provided function
	})
}

// WithBackend selects a container backend. Default uses docker/podman CLI.
// Alternative backends must be imported separately.
func WithBackend(name string) Option {
	return OptionFunc(func(cfgRaw interface{}) {
		// cfgRaw is *containerConfig
		type backendSetter interface {
			SetBackend(name string)
		}
		if s, ok := cfgRaw.(backendSetter); ok {
			s.SetBackend(name)
		}
	})
}
