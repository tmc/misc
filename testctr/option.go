package testctr

import (
	"io"
	"os"
)

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
		if cfg.env == nil {
			cfg.env = make(map[string]string)
		}
		cfg.env[key] = value
	})
}

// WithPort exposes a container port. The port can be "8080" or "8080/tcp".
// Use Port() or Endpoint() to get the mapped host port after creation.
func WithPort(port string) Option {
	return optionFunc(func(cfg *containerConfig) {
		cfg.ports = append(cfg.ports, port)
	})
}

// WithCommand overrides the container's default command (CMD).
func WithCommand(cmd ...string) Option {
	return optionFunc(func(cfg *containerConfig) {
		cfg.cmd = cmd
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
//
// Example:
//
//	// Use testcontainers backend
//	import _ "github.com/tmc/misc/testctr/testctr-testcontainers"
//	container := testctr.New(t, "redis:7", testctr.WithBackend("testcontainers"))
//
//	// Use docker client backend
//	import _ "github.com/tmc/misc/testctr/testctr-dockerclient"
//	container := testctr.New(t, "redis:7", testctr.WithBackend("dockerclient"))
func WithBackend(name string) Option {
	return optionFunc(func(cfg *containerConfig) {
		cfg.backendName = name
	})
}

// WithFile copies a file from the host into the container.
// The file is copied after the container starts but before wait conditions.
//
// Example:
//
//	container := testctr.New(t, "alpine:latest",
//	    testctr.WithFile("./config.json", "/app/config.json"),
//	)
func WithFile(hostPath, containerPath string) Option {
	return optionFunc(func(cfg *containerConfig) {
		cfg.files = append(cfg.files, fileEntry{
			Source: hostPath,
			Target: containerPath,
		})
	})
}

// WithFileMode copies a file with specific permissions.
//
// Example:
//
//	container := testctr.New(t, "alpine:latest",
//	    testctr.WithFileMode("./script.sh", "/app/script.sh", 0755),
//	)
func WithFileMode(hostPath, containerPath string, mode os.FileMode) Option {
	return optionFunc(func(cfg *containerConfig) {
		cfg.files = append(cfg.files, fileEntry{
			Source: hostPath,
			Target: containerPath,
			Mode:   mode,
		})
	})
}

// WithFileReader copies content from an io.Reader into the container.
//
// Example:
//
//	config := strings.NewReader(`{"key": "value"}`)
//	container := testctr.New(t, "alpine:latest",
//	    testctr.WithFileReader(config, "/app/config.json"),
//	)
func WithFileReader(reader io.Reader, containerPath string) Option {
	return optionFunc(func(cfg *containerConfig) {
		cfg.files = append(cfg.files, fileEntry{
			Source: reader,
			Target: containerPath,
		})
	})
}

// FileContent represents file content and permissions.
type FileContent struct {
	Content []byte
	Mode    os.FileMode
}

// WithFiles copies multiple files into the container.
//
// Example:
//
//	container := testctr.New(t, "alpine:latest",
//	    testctr.WithFiles(map[string]testctr.FileContent{
//	        "/app/config.json": {Content: []byte(`{"key": "value"}`)},
//	        "/app/script.sh":   {Content: []byte("#!/bin/sh\necho hello"), Mode: 0755},
//	    }),
//	)
func WithFiles(files map[string]FileContent) Option {
	return optionFunc(func(cfg *containerConfig) {
		for target, fc := range files {
			cfg.files = append(cfg.files, fileEntry{
				Source: fc.Content,
				Target: target,
				Mode:   fc.Mode,
			})
		}
	})
}
