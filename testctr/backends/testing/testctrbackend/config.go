package testctrbackendtest

import (
	"os"
	"path/filepath"
	"testing"
)

// containerConfig represents the minimal config structure that backends need to handle
type containerConfig struct {
	dockerRun      *dockerRun
	startupTimeout int
	logStreaming   bool
	files          []fileEntry
	env            map[string]string
	ports          []string
	cmd            []string
}

type dockerRun struct {
	image       string
	env         map[string]string
	ports       []string
	cmd         []string
	mounts      []string
	network     string
	user        string
	workdir     string
	labels      map[string]string
	memoryLimit string
	privileged  bool
}

type fileEntry struct {
	Source interface{} // string (file path) or io.Reader
	Target string
	Mode   os.FileMode
}

// ConfigOption modifies a test container configuration
type ConfigOption func(*containerConfig)

// WithEnv adds an environment variable to the test config
func WithEnv(key, value string) ConfigOption {
	return func(c *containerConfig) {
		if c.env == nil {
			c.env = make(map[string]string)
		}
		c.env[key] = value
		if c.dockerRun != nil {
			if c.dockerRun.env == nil {
				c.dockerRun.env = make(map[string]string)
			}
			c.dockerRun.env[key] = value
		}
	}
}

// WithPort exposes a port in the test config
func WithPort(port string) ConfigOption {
	return func(c *containerConfig) {
		c.ports = append(c.ports, port)
		if c.dockerRun != nil {
			c.dockerRun.ports = append(c.dockerRun.ports, port)
		}
	}
}

// WithCommand sets the container command
func WithCommand(cmd ...string) ConfigOption {
	return func(c *containerConfig) {
		c.cmd = cmd
		if c.dockerRun != nil {
			c.dockerRun.cmd = cmd
		}
	}
}

// WithFile adds a file to be copied into the container
func WithFile(source, target string) ConfigOption {
	return func(c *containerConfig) {
		c.files = append(c.files, fileEntry{
			Source: source,
			Target: target,
			Mode:   0644,
		})
	}
}

// WithLabel adds a label to the container
func WithLabel(key, value string) ConfigOption {
	return func(c *containerConfig) {
		if c.dockerRun == nil {
			c.dockerRun = &dockerRun{
				labels: make(map[string]string),
			}
		}
		if c.dockerRun.labels == nil {
			c.dockerRun.labels = make(map[string]string)
		}
		c.dockerRun.labels[key] = value
	}
}

// NewTestConfig creates a test configuration with the given options
func NewTestConfig(opts ...ConfigOption) *containerConfig {
	cfg := &containerConfig{
		dockerRun: &dockerRun{
			env:    make(map[string]string),
			labels: make(map[string]string),
		},
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// CreateTempFile creates a temporary file for testing and returns its path.
// The file is automatically cleaned up when the test ends.
func CreateTempFile(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	return tmpFile
}
