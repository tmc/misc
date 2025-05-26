package testctr

// Option configures a Container.
type Option interface {
	// contains filtered or unexported methods
	apply(*containerConfig)
}

// optionFunc wraps a function to satisfy the Option interface
type optionFunc func(*containerConfig)

// apply implements the Option interface
func (f optionFunc) apply(cfg *containerConfig) {
	f(cfg)
}

// Options combines multiple options into a single Option
func Options(opts ...Option) Option {
	return optionFunc(func(cfg *containerConfig) {
		for _, opt := range opts {
			if opt != nil {
				opt.apply(cfg)
			}
		}
	})
}

// WithEnv returns an Option that adds an environment variable
func WithEnv(key, value string) Option {
	return optionFunc(func(cfg *containerConfig) {
		if cfg.dockerRun != nil {
			cfg.dockerRun.env[key] = value
		}
	})
}

// WithPort returns an Option that exposes a port
func WithPort(port string) Option {
	return optionFunc(func(cfg *containerConfig) {
		if cfg.dockerRun != nil {
			cfg.dockerRun.ports = append(cfg.dockerRun.ports, port)
		}
	})
}

// WithCommand returns an Option that sets the container command
func WithCommand(cmd ...string) Option {
	return optionFunc(func(cfg *containerConfig) {
		if cfg.dockerRun != nil {
			cfg.dockerRun.cmd = cmd
		}
	})
}

// OptionFunc allows external packages to create options without accessing unexported types
func OptionFunc(fn func(interface{})) Option {
	return optionFunc(func(cfg *containerConfig) {
		fn(cfg)
	})
}
