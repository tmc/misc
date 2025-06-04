// Package backend defines the interface for pluggable container runtime backends
// used by the testctr library. This allows testctr to interact with different
// container management systems (like Testcontainers-Go)
// through a consistent API, as an alternative to its default CLI-based operations.
package backend

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Backend represents an alternative container runtime backend.
// Implementations of this interface provide the underlying mechanism for
// creating, managing, and interacting with containers, used when testctr.New
// is configured with testctr.WithBackend(...).
type Backend interface {
	// CreateContainer creates a new container with the given image and configuration.
	// It returns the container ID or an error.
	// The `config` argument is the `*testctr.containerConfig` from the main package,
	// passed as `interface{}` to avoid direct import cycles if Backend implementations
	// are in separate modules. Implementations should type-assert it.
	CreateContainer(t testing.TB, image string, config interface{}) (string, error)

	// StartContainer starts a previously created container.
	// Note: Some backends might start containers immediately upon creation, making this a no-op.
	StartContainer(containerID string) error

	// StopContainer stops a running container.
	StopContainer(containerID string) error

	// RemoveContainer removes a container. This typically also stops it if running.
	RemoveContainer(containerID string) error

	// InspectContainer returns information about a container.
	InspectContainer(containerID string) (*ContainerInfo, error)

	// ExecInContainer executes a command inside a running container.
	// It returns the exit code, combined stdout/stderr output, and any error.
	ExecInContainer(containerID string, cmd []string) (int, string, error)

	// GetContainerLogs retrieves the logs of a container.
	GetContainerLogs(containerID string) (string, error)

	// WaitForLog waits for a specific log line to appear in the container's logs,
	// or until the timeout is reached.
	WaitForLog(containerID string, logLine string, timeout time.Duration) error

	// InternalIP returns the IP address of the container within its primary Docker network.
	InternalIP(containerID string) (string, error)

	// Commit commits the current state of the container to a new image.
	Commit(containerID string, imageName string) error
}

// Registry manages backend registrations.
type Registry struct {
	mu       sync.RWMutex
	backends map[string]Backend
}

// NewRegistry creates a new backend registry.
func NewRegistry() *Registry {
	return &Registry{
		backends: make(map[string]Backend),
	}
}

// Register registers a new backend implementation with the given name.
// It panics if the backend or name is nil, or if the name is already registered.
func (r *Registry) Register(name string, b Backend) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" {
		panic("backend: Register backend name cannot be empty")
	}
	if b == nil {
		panic("backend: Register backend is nil")
	}
	if _, dup := r.backends[name]; dup {
		panic("backend: Register called twice for backend " + name)
	}
	r.backends[name] = b
}

// Get retrieves a registered backend by name.
// It returns an error if the backend is not found.
func (r *Registry) Get(name string) (Backend, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.backends[name]
	if !ok {
		return nil, fmt.Errorf("backend: backend %q not registered", name)
	}
	return b, nil
}

// List returns all registered backend names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.backends))
	for name := range r.backends {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry is the global backend registry used by Register and Get.
var DefaultRegistry = NewRegistry()

// Register registers a new backend implementation with the given name in the default registry.
// This is typically called from an init() function in a backend adapter package
// (e.g., in the testctr-testcontainers module).
// It panics if the backend or name is nil, or if the name is already registered.
func Register(name string, b Backend) {
	DefaultRegistry.Register(name, b)
}

// Get retrieves a registered backend by name from the default registry.
// It returns an error if the backend is not found.
func Get(name string) (Backend, error) {
	return DefaultRegistry.Get(name)
}
