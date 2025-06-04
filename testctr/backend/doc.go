// Package backend provides a pluggable backend system for testctr container management.
//
// This package defines the [Backend] interface that container backends must implement,
// along with supporting types and a registration system. It enables testctr to work
// with different container runtimes through a common interface.
//
// # Backend Interface
//
// The [Backend] interface defines the contract for container backends:
//
//	type Backend interface {
//	    CreateContainer(ctx context.Context, opts CreateOptions) (string, error)
//	    StartContainer(ctx context.Context, id string) error
//	    StopContainer(ctx context.Context, id string) error
//	    RemoveContainer(ctx context.Context, id string) error
//	    ExecContainer(ctx context.Context, id string, cmd []string) (int, string, error)
//	    InspectContainer(ctx context.Context, id string) (*ContainerInfo, error)
//	}
//
// # Container Information
//
// The [ContainerInfo] struct provides standardized container metadata:
//
//	type ContainerInfo struct {
//	    ID     string            // Container ID
//	    Name   string            // Container name
//	    Image  string            // Container image
//	    Status string            // Container status
//	    Ports  map[string]string // Port mappings (container -> host)
//	    Labels map[string]string // Container labels
//	}
//
// # Backend Registration
//
// Backends register themselves using the package-level registry:
//
//	package mybackend
//	
//	import "github.com/tmc/misc/testctr/backend"
//	
//	func init() {
//	    backend.Register("mybackend", &MyBackend{})
//	}
//	
//	type MyBackend struct{}
//	
//	func (b *MyBackend) CreateContainer(ctx context.Context, opts backend.CreateOptions) (string, error) {
//	    // Implementation...
//	}
//
// # Using Backends in testctr
//
// Backends are selected using [github.com/tmc/misc/testctr.WithBackend]:
//
//	import (
//	    "github.com/tmc/misc/testctr"
//	    "github.com/tmc/misc/testctr/backend"
//	    _ "mypackage/mybackend" // Register backend
//	)
//	
//	func TestWithCustomBackend(t *testing.T) {
//	    myBackend, _ := backend.Get("mybackend")
//	    container := testctr.New(t, "redis:7-alpine", 
//	        testctr.WithBackend(myBackend))
//	    // ...
//	}
//
// # Custom Registries
//
// For advanced use cases, you can create isolated backend registries:
//
//	registry := backend.NewRegistry()
//	registry.Register("custom", myBackend)
//	
//	// Use with custom lookup
//	backend, exists := registry.Get("custom")
//	if !exists {
//	    log.Fatal("Backend not found")
//	}
//
// This is useful for testing backend implementations or when you need
// isolated backend namespaces.
//
// # Built-in Backends
//
// testctr includes several backend implementations:
//
//   - CLI Backend (default): Uses docker/podman/nerdctl commands
//   - Docker Client Backend: Uses Docker client library  
//   - Testcontainers Backend: Adapts testcontainers-go
//
// See [github.com/tmc/misc/testctr/testctr-dockerclient] and
// [github.com/tmc/misc/testctr/testctr-testcontainers] for alternative backends.
//
// # Error Handling
//
// Backends should return descriptive errors that can help users debug
// container issues. The [ContainerInfo] from [Backend.InspectContainer]
// provides context for error reporting.
package backend
