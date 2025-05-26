package testctr

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Backend represents a container runtime backend
type Backend interface {
	// CreateContainer creates a new container and returns its ID
	CreateContainer(t testing.TB, image string, config interface{}) (string, error)

	// StartContainer starts a container
	StartContainer(containerID string) error

	// StopContainer stops a container
	StopContainer(containerID string) error

	// RemoveContainer removes a container
	RemoveContainer(containerID string) error

	// InspectContainer returns container information
	InspectContainer(containerID string) (*ContainerInfo, error)

	// ExecInContainer executes a command in the container
	ExecInContainer(containerID string, cmd []string) (int, string, error)

	// GetContainerLogs retrieves container logs
	GetContainerLogs(containerID string) (string, error)

	// WaitForLog waits for a specific log line
	WaitForLog(containerID string, logLine string, timeout time.Duration) error
}

var (
	backendsMu sync.RWMutex
	backends   = make(map[string]Backend)
)

// RegisterBackend registers a new backend implementation
func RegisterBackend(name string, backend Backend) {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	backends[name] = backend
}

// GetBackend retrieves a registered backend
func GetBackend(name string) (Backend, error) {
	backendsMu.RLock()
	defer backendsMu.RUnlock()

	backend, ok := backends[name]
	if !ok {
		return nil, fmt.Errorf("backend %q not registered", name)
	}
	return backend, nil
}

// WithBackend returns an option that selects a specific backend
func WithBackend(name string) Option {
	return OptionFunc(func(cfg interface{}) {
		type backendSetter interface {
			SetBackend(name string)
		}
		if s, ok := cfg.(backendSetter); ok {
			s.SetBackend(name)
		}
	})
}
