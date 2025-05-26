package testcontainers

import "strings"

// Option configures a Container
type Option interface {
	apply(*Container)
}

// funcOption implements Option with a function
type funcOption struct {
	fn func(*Container)
}

func (f funcOption) apply(c *Container) {
	f.fn(c)
}

// WithEnv adds an environment variable
func WithEnv(key, value string) Option {
	return funcOption{fn: func(c *Container) {
		c.env[key] = value
	}}
}

// WithPort exposes a port
func WithPort(port string) Option {
	return funcOption{fn: func(c *Container) {
		// Normalize to include /tcp if not specified
		if !strings.Contains(port, "/") {
			port = port + "/tcp"
		}
		c.ports = append(c.ports, port)
	}}
}

// WithCommand sets the container command
func WithCommand(cmd ...string) Option {
	return funcOption{fn: func(c *Container) {
		c.cmd = cmd
	}}
}
