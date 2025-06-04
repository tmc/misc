package testctr

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr/backend"
	"github.com/tmc/misc/testctr/backends/cli"
)

var coordinatorOnce sync.Once

// Container manages a test container instance created with New.
// All methods are safe for concurrent use. The container is automatically
// cleaned up when the test completes unless debugging flags are set.
type Container struct {
	id           string
	t            testing.TB
	backend      backend.Backend
	host         string
	ports        map[string]string // Mapped ports: containerPort -> hostPort
	image        string
	config       *containerConfig
	logStreaming bool
	logCancel    context.CancelFunc
}

// containerConfig holds container configuration.
type containerConfig struct {
	// Core configuration
	env            map[string]string
	ports          []string
	cmd            []string
	labels         map[string]string
	mounts         []string
	network        string
	user           string
	workdir        string
	memoryLimit    string
	privileged     bool
	files          []fileEntry
	
	// Behavior configuration
	logStreaming   bool
	logFilter      func(string) bool
	startupTimeout time.Duration
	startupDelay   time.Duration
	waitConditions []waitCondition
	dsnProvider    DSNProvider
	
	// Backend configuration
	backendName    string
	backendConfig  interface{} // Backend-specific configuration
}

// fileEntry represents a file to be copied into the container.
type fileEntry struct {
	Source interface{} // string (file path) or io.Reader
	Target string      // Target path in container
	Mode   os.FileMode // File permissions
}

// waitCondition represents a condition to wait for before considering container ready.
type waitCondition func(ctx context.Context, c *Container) error

// New creates and starts a new test container for the given image.
// The container is automatically stopped and removed when the test completes
// via t.Cleanup, unless the -testctr.keep-failed flag is set for debugging.
//
// The image parameter specifies the container image to use (e.g., "redis:7-alpine").
// If the image is not available locally, it will be pulled automatically.
//
// Options can be provided to configure the container. Core options like WithEnv,
// WithPort, and WithCommand are in this package. Advanced options are available
// in the ctropts subpackage.
//
// Example:
//
//	redis := testctr.New(t, "redis:7-alpine",
//	    testctr.WithPort("6379"),
//	    testctr.WithEnv("REDIS_PASSWORD", "secret"),
//	)
//	endpoint := redis.Endpoint("6379")
//	// Connect to Redis at endpoint...
func New(t testing.TB, image string, opts ...Option) *Container {
	t.Helper()

	// Update coordinator settings from flags (only happens once)
	coordinatorOnce.Do(updateCoordinatorFromFlags)

	// Check for old containers
	checkOldContainers(t)

	// Create config with defaults
	config := &containerConfig{
		env:            make(map[string]string),
		labels:         containerLabels(t, image),
		logStreaming:   *verbose,
		startupTimeout: 5 * time.Second,
		backendName:    "cli", // Default to CLI backend
	}

	// Apply options
	for _, opt := range opts {
		if opt != nil {
			opt.apply(config)
		}
	}

	// Get the backend
	be, err := backend.Get(config.backendName)
	if err != nil {
		t.Fatalf("failed to get backend %q: %v", config.backendName, err)
	}

	// Apply startup delay if configured
	if config.startupDelay > 0 {
		time.Sleep(config.startupDelay)
	}

	// Coordinate container creation
	globalContainerCoordinator.requestContainerSlot()
	defer globalContainerCoordinator.releaseContainerSlot()

	// Prepare backend-specific configuration
	backendConfig := prepareBackendConfig(config)

	// Ensure image exists
	if ensurer, ok := be.(interface{ EnsureImage(testing.TB, string) error }); ok {
		if err := ensurer.EnsureImage(t, image); err != nil {
			t.Fatalf("failed to ensure image exists: %v", err)
		}
	}

	// Create container via backend
	containerID, err := be.CreateContainer(t, image, backendConfig)
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	// Create container object
	c := &Container{
		id:           containerID,
		t:            t,
		backend:      be,
		host:         "127.0.0.1",
		image:        image,
		config:       config,
		logStreaming: config.logStreaming,
		ports:        make(map[string]string),
	}

	// Setup cleanup
	setupContainerCleanup(t, c)

	// Copy files if needed
	if err := c.copyFiles(); err != nil {
		t.Fatalf("failed to copy files: %v", err)
	}

	// Update port mappings
	if err := c.updatePortMappings(); err != nil {
		c.logVerbosef("initial port mapping update failed: %v", err)
	}

	// Run wait conditions
	if err := c.runWaitConditions(); err != nil {
		t.Fatalf("container failed to become ready: %v", err)
	}

	// Start log streaming if enabled
	if c.logStreaming {
		c.startLogStreaming()
	}

	return c
}

// prepareBackendConfig converts generic config to backend-specific config.
func prepareBackendConfig(cfg *containerConfig) interface{} {
	// For CLI backend, create CLIConfig
	if cfg.backendName == "cli" || cfg.backendName == "docker" {
		cliConfig := &cli.CLIConfig{
			Labels:      cfg.labels,
			Env:         cfg.env,
			Ports:       cfg.ports,
			Cmd:         cfg.cmd,
			Mounts:      cfg.mounts,
			Network:     cfg.network,
			User:        cfg.user,
			WorkDir:     cfg.workdir,
			MemoryLimit: cfg.memoryLimit,
			Privileged:  cfg.privileged,
		}
		
		// Convert file entries
		for _, f := range cfg.files {
			cliConfig.Files = append(cliConfig.Files, cli.FileEntry{
				Source: f.Source,
				Target: f.Target,
				Mode:   f.Mode,
			})
		}
		
		return cliConfig
	}
	
	// For other backends, return the custom config if set
	return cfg.backendConfig
}

// setupContainerCleanup registers cleanup handlers for the container.
func setupContainerCleanup(t testing.TB, c *Container) {
	t.Helper()

	t.Cleanup(func() {
		// Stop log streaming
		if c.logCancel != nil {
			c.logCancel()
		}

		// Check if test failed and we should keep the container
		if *keepOnFailure && t.Failed() {
			t.Logf("Test failed - keeping container %s (%s) for debugging", c.id[:12], c.image)
			t.Logf("To inspect: docker exec -it %s /bin/sh", c.id[:12])
			t.Logf("To view logs: docker logs %s", c.id[:12])
			t.Logf("To remove: docker rm -f %s", c.id[:12])
			return
		}

		// Stop container
		if err := c.backend.StopContainer(c.id); err != nil {
			c.logVerbosef("failed to stop container: %v", err)
		}

		// Remove container
		if err := c.backend.RemoveContainer(c.id); err != nil {
			c.logVerbosef("failed to remove container: %v", err)
		}
	})
}

// copyFiles copies configured files into the container.
func (c *Container) copyFiles() error {
	if len(c.config.files) == 0 {
		return nil
	}

	// Check if backend supports file copying
	copier, ok := c.backend.(interface {
		CopyFilesToContainer(string, []cli.FileEntry, testing.TB) error
	})
	if !ok {
		return fmt.Errorf("backend %q does not support file copying", c.config.backendName)
	}

	// Convert file entries
	var files []cli.FileEntry
	for _, f := range c.config.files {
		files = append(files, cli.FileEntry{
			Source: f.Source,
			Target: f.Target,
			Mode:   f.Mode,
		})
	}

	return copier.CopyFilesToContainer(c.id, files, c.t)
}

// updatePortMappings updates the container's port mappings from inspection.
func (c *Container) updatePortMappings() error {
	info, err := c.backend.InspectContainer(c.id)
	if err != nil {
		return err
	}

	c.ports = extractPorts(info)
	return nil
}

// extractPorts extracts port mappings from container info.
func extractPorts(info *backend.ContainerInfo) map[string]string {
	ports := make(map[string]string)
	for port, bindings := range info.NetworkSettings.Ports {
		if len(bindings) > 0 && bindings[0].HostPort != "" {
			// Extract just the port number from "3306/tcp"
			portNum := strings.Split(port, "/")[0]
			ports[portNum] = bindings[0].HostPort
		}
	}
	return ports
}

// runWaitConditions executes all configured wait conditions.
func (c *Container) runWaitConditions() error {
	c.t.Helper()

	// Default wait condition if none specified
	if len(c.config.waitConditions) == 0 {
		c.config.waitConditions = []waitCondition{c.waitForRunning}
	}

	// Create context with timeout
	ctx, cancel := c.contextWithTimeout(context.Background(), c.config.startupTimeout, "container startup")
	defer cancel()

	// Run each wait condition
	for i, condition := range c.config.waitConditions {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled before wait condition %d: %w", i+1, context.Cause(ctx))
		default:
		}

		if err := condition(ctx, c); err != nil {
			return fmt.Errorf("wait condition %d failed: %w", i+1, err)
		}
	}

	return nil
}

// waitForRunning waits for the container to be in running state.
func (c *Container) waitForRunning(ctx context.Context, _ *Container) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-ticker.C:
			info, err := c.backend.InspectContainer(c.id)
			if err != nil {
				return fmt.Errorf("failed to inspect container: %w", err)
			}
			if info.State.Running {
				c.logVerbosef("container is running")
				return nil
			}
			if info.State.Status == "exited" {
				return fmt.Errorf("container exited with code %d", info.State.ExitCode)
			}
		}
	}
}

// startLogStreaming starts streaming container logs to the test output.
func (c *Container) startLogStreaming() {
	// Get current logs first
	if logs, err := c.backend.GetContainerLogs(c.id); err == nil && logs != "" {
		for _, line := range strings.Split(logs, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				c.logFiltered(line)
			}
		}
	}

	// Start streaming if backend supports it
	streamer, ok := c.backend.(interface {
		StreamLogs(context.Context, string, func(string)) error
	})
	if !ok {
		c.logVerbosef("backend does not support log streaming")
		return
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	c.logCancel = cancel

	// Start streaming in background
	go func() {
		if err := streamer.StreamLogs(ctx, c.id, func(line string) {
			c.logFiltered(line)
		}); err != nil && !strings.Contains(err.Error(), "context canceled") {
			c.logVerbosef("log streaming error: %v", err)
		}
	}()
}

// contextWithTimeout creates a context that respects test deadlines.
func (c *Container) contextWithTimeout(ctx context.Context, timeout time.Duration, operation string, details ...string) (context.Context, context.CancelFunc) {
	c.t.Helper()
	
	// Build detailed context message
	contextMsg := operation
	if len(details) > 0 {
		contextMsg = fmt.Sprintf("%s (%s)", operation, strings.Join(details, ", "))
	}

	// Check if test has a deadline
	if td, ok := c.t.(interface{ Deadline() (time.Time, bool) }); ok {
		if deadline, hasDeadline := td.Deadline(); hasDeadline {
			// Include a shutdown buffer
			shutdownBuffer := 5 * time.Second
			deadline = deadline.Add(-shutdownBuffer)
			remaining := time.Until(deadline)

			// If the test deadline is sooner than our timeout, use it
			if remaining > 0 && remaining < timeout {
				c.logVerbosef("%s limited by test deadline (%v remaining)", contextMsg, remaining)
				return context.WithDeadlineCause(ctx, deadline,
					fmt.Errorf("test deadline exceeded during %s", contextMsg))
			}
		}
	}

	// Use the requested timeout
	return context.WithTimeoutCause(ctx, timeout,
		fmt.Errorf("timeout (%v) exceeded during %s", timeout, contextMsg))
}

// Host returns the host address for accessing the container, typically "127.0.0.1".
func (c *Container) Host() string {
	return c.host
}

// Port returns the host port mapped to the given container port.
func (c *Container) Port(containerPort string) string {
	return c.ports[containerPort]
}

// Endpoint returns the complete network endpoint (host:port) for accessing
// a service running on the specified container port.
func (c *Container) Endpoint(containerPort string) string {
	return fmt.Sprintf("%s:%s", c.host, c.Port(containerPort))
}

// ID returns the full container ID.
func (c *Container) ID() string {
	return c.id
}

// Inspect returns detailed container information.
func (c *Container) Inspect() (*backend.ContainerInfo, error) {
	return c.backend.InspectContainer(c.id)
}

// Exec executes a command inside the running container.
func (c *Container) Exec(ctx context.Context, cmd []string) (int, string, error) {
	return c.backend.ExecInContainer(c.id, cmd)
}

// ExecSimple executes a command inside the container and returns the output.
// It calls t.Fatal if the command fails or returns a non-zero exit code.
func (c *Container) ExecSimple(cmd ...string) string {
	c.t.Helper()
	
	ctx, cancel := c.contextWithTimeout(context.Background(), 30*time.Second, "exec", strings.Join(cmd, " "))
	defer cancel()

	exitCode, output, err := c.Exec(ctx, cmd)
	if err != nil {
		c.t.Fatalf("command failed: %v (command: %s, output: %s)", err, strings.Join(cmd, " "), output)
	}
	if exitCode != 0 {
		c.t.Fatalf("command exited with code %d (command: %s, output: %s)", exitCode, strings.Join(cmd, " "), output)
	}
	return strings.TrimSpace(output)
}

// logf logs a message with the testctr prefix and image name.
func (c *Container) logf(format string, args ...interface{}) {
	c.t.Helper()
	id := c.ID()
	if len(id) > 12 {
		id = id[:12]
	}
	prefix := fmt.Sprintf("testctr: [%s] %s", c.image, id)
	c.t.Logf("%s %s", prefix, fmt.Sprintf(format, args...))
}

// logFiltered logs a message after applying the configured log filter.
func (c *Container) logFiltered(msg string) {
	c.t.Helper()
	if c.config.logFilter != nil && !c.config.logFilter(msg) {
		return
	}
	c.logf("%s", msg)
}

// logVerbosef logs a message only if verbose mode is enabled.
func (c *Container) logVerbosef(format string, args ...interface{}) {
	c.t.Helper()
	if *verbose {
		c.logf(format, args...)
	}
}

// parseImageBasename extracts the container type from image string.
func parseImageBasename(image string) string {
	// Simple parsing - just get the first part before :
	for i, ch := range image {
		if ch == ':' || ch == '/' {
			return image[:i]
		}
	}
	return image
}