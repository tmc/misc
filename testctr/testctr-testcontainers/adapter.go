package testcontainers

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tmc/misc/testctr/backend"
)

func init() {
	// Register the testcontainers backend
	backend.Register("testcontainers", &TestcontainersBackend{})
}

// TestcontainersBackend implements the testctr.Backend interface using testcontainers-go
type TestcontainersBackend struct {
	containers map[string]testcontainers.Container
}

// CreateContainer creates a new container using testcontainers-go
func (b *TestcontainersBackend) CreateContainer(t testing.TB, image string, config interface{}) (string, error) {
	if b.containers == nil {
		b.containers = make(map[string]testcontainers.Container)
	}

	// Extract configuration
	// Try to extract the unexported type
	cfg, ok := config.(*containerConfig)
	if !ok {
		// Configuration might be nil or a different type
		cfg = nil
	}

	ctx := context.Background()

	// Build container request
	req := testcontainers.ContainerRequest{
		Image: image,
		Env:   make(map[string]string),
		Labels: map[string]string{
			"testctr":       "true",
			"testctr.test":  t.Name(),
			"testctr.image": image,
		},
	}

	// Extract configuration from dockerRun if available
	if cfg != nil && cfg.dockerRun != nil {
		dr := cfg.dockerRun
		req.Env = dr.env
		for k, v := range dr.labels {
			req.Labels[k] = v
		}
		req.Cmd = dr.cmd

		// Convert ports
		for _, port := range dr.ports {
			req.ExposedPorts = append(req.ExposedPorts, port)
		}

		// Set memory limit if specified
		// Note: testcontainers-go may handle this differently
		// This is a placeholder for memory limits
	}

	// Apply testcontainers-specific settings from config
	if cfg != nil {
		// Set privileged mode
		if cfg.privileged {
			req.Privileged = true
		}

		// Set auto-remove
		if cfg.autoRemove {
			req.AutoRemove = true
		}

		// Use custom wait strategy if provided
		if cfg.waitStrategy != nil {
			if ws, ok := cfg.waitStrategy.(wait.Strategy); ok {
				req.WaitingFor = ws
			}
		} else {
			// Default wait strategy based on image type
			var waitStrategy wait.Strategy
			if strings.Contains(image, "mysql") {
				waitStrategy = wait.ForLog("ready for connections. Version").
					WithStartupTimeout(60 * time.Second)
			} else if strings.Contains(image, "postgres") {
				waitStrategy = wait.ForLog("database system is ready to accept connections").
					WithStartupTimeout(60 * time.Second)
			} else if strings.Contains(image, "redis") {
				waitStrategy = wait.ForLog("Ready to accept connections").
					WithStartupTimeout(30 * time.Second)
			}

			if waitStrategy != nil {
				req.WaitingFor = waitStrategy
			}
		}

		// Apply host config modifier
		if cfg.hostConfigModifier != nil {
			req.HostConfigModifier = func(hc *container.HostConfig) {
				cfg.hostConfigModifier(hc)
			}
		}
	} else {
		// Default wait strategy when no config
		var waitStrategy wait.Strategy
		if strings.Contains(image, "mysql") {
			waitStrategy = wait.ForLog("ready for connections. Version").
				WithStartupTimeout(60 * time.Second)
		} else if strings.Contains(image, "postgres") {
			waitStrategy = wait.ForLog("database system is ready to accept connections").
				WithStartupTimeout(60 * time.Second)
		} else if strings.Contains(image, "redis") {
			waitStrategy = wait.ForLog("Ready to accept connections").
				WithStartupTimeout(30 * time.Second)
		}

		if waitStrategy != nil {
			req.WaitingFor = waitStrategy
		}
	}

	// Create GenericContainerRequest
	gcr := testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	}

	// Apply skip reaper setting
	if cfg != nil && cfg.skipReaper {
		gcr.Reuse = true // This skips the reaper
	}

	// Apply custom testcontainers customizers
	if cfg != nil && len(cfg.testcontainersCustomizers) > 0 {
		for _, customizer := range cfg.testcontainersCustomizers {
			if fn, ok := customizer.(func(interface{})); ok {
				fn(&gcr)
			}
		}
	}

	// Create and start container
	container, err := testcontainers.GenericContainer(ctx, gcr)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	containerID := container.GetContainerID()
	b.containers[containerID] = container

	return containerID, nil
}

// StartContainer starts a container (no-op for testcontainers as containers start automatically)
func (b *TestcontainersBackend) StartContainer(containerID string) error {
	// Testcontainers starts containers automatically
	return nil
}

// StopContainer stops a container
func (b *TestcontainersBackend) StopContainer(containerID string) error {
	container, ok := b.containers[containerID]
	if !ok {
		return fmt.Errorf("container %s not found", containerID)
	}

	ctx := context.Background()
	return container.Stop(ctx, nil)
}

// RemoveContainer removes a container
func (b *TestcontainersBackend) RemoveContainer(containerID string) error {
	container, ok := b.containers[containerID]
	if !ok {
		return fmt.Errorf("container %s not found", containerID)
	}

	ctx := context.Background()
	err := container.Terminate(ctx)
	delete(b.containers, containerID)
	return err
}

// InspectContainer returns container information
func (b *TestcontainersBackend) InspectContainer(containerID string) (*backend.ContainerInfo, error) {
	_, ok := b.containers[containerID]
	if !ok {
		return nil, fmt.Errorf("container %s not found", containerID)
	}

	// Get Docker client to inspect the container
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	inspect, err := dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Convert to backend.ContainerInfo
	info := &backend.ContainerInfo{
		State: struct {
			Running  bool   `json:"Running"`
			Status   string `json:"Status"`
			ExitCode int    `json:"ExitCode"`
		}{
			Running:  inspect.State.Running,
			Status:   inspect.State.Status,
			ExitCode: inspect.State.ExitCode,
		},
		ID:      containerID,
		Name:    inspect.Name,
		Created: inspect.Created,
	}

	// Initialize NetworkSettings
	info.NetworkSettings.Ports = make(map[string][]backend.PortBinding)

	// Convert port mappings
	for port, bindings := range inspect.NetworkSettings.Ports {
		var mappings []backend.PortBinding
		for _, binding := range bindings {
			mappings = append(mappings, backend.PortBinding{
				HostIP:   binding.HostIP,
				HostPort: binding.HostPort,
			})
		}
		info.NetworkSettings.Ports[string(port)] = mappings
	}

	return info, nil
}

// ExecInContainer executes a command in the container
func (b *TestcontainersBackend) ExecInContainer(containerID string, cmd []string) (int, string, error) {
	container, ok := b.containers[containerID]
	if !ok {
		return -1, "", fmt.Errorf("container %s not found", containerID)
	}

	ctx := context.Background()
	exitCode, reader, err := container.Exec(ctx, cmd)
	if err != nil {
		return -1, "", fmt.Errorf("exec failed: %w", err)
	}

	output := new(strings.Builder)
	_, err = io.Copy(output, reader)
	if err != nil {
		return exitCode, "", fmt.Errorf("failed to read output: %w", err)
	}

	return exitCode, output.String(), nil
}

// GetContainerLogs retrieves container logs
func (b *TestcontainersBackend) GetContainerLogs(containerID string) (string, error) {
	container, ok := b.containers[containerID]
	if !ok {
		return "", fmt.Errorf("container %s not found", containerID)
	}

	ctx := context.Background()
	reader, err := container.Logs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer reader.Close()

	output := new(strings.Builder)
	_, err = io.Copy(output, reader)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return output.String(), nil
}

// WaitForLog waits for a specific log line
func (b *TestcontainersBackend) WaitForLog(containerID string, logLine string, timeout time.Duration) error {
	_, ok := b.containers[containerID]
	if !ok {
		return fmt.Errorf("container %s not found", containerID)
	}

	// This is already handled by the WaitingFor strategy in CreateContainer
	// But we can implement it here for consistency
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for log: %s", logLine)
		case <-ticker.C:
			logs, err := b.GetContainerLogs(containerID)
			if err != nil {
				continue
			}
			if strings.Contains(logs, logLine) {
				return nil
			}
		}
	}
}

// InternalIP returns the internal IP address of the container
func (b *TestcontainersBackend) InternalIP(containerID string) (string, error) {
	tc, ok := b.containers[containerID]
	if !ok {
		return "", fmt.Errorf("container %s not found", containerID)
	}

	// Get container info
	inspect, err := tc.Inspect(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	// Return the IP address from the default network
	if inspect.NetworkSettings != nil && len(inspect.NetworkSettings.Networks) > 0 {
		// Get the first network's IP address
		for _, network := range inspect.NetworkSettings.Networks {
			if network.IPAddress != "" {
				return network.IPAddress, nil
			}
		}
	}

	return "", fmt.Errorf("no IP address found for container %s", containerID)
}

// Commit commits the current state of the container to a new image
func (b *TestcontainersBackend) Commit(containerID string, imageName string) error {
	tc, ok := b.containers[containerID]
	if !ok {
		return fmt.Errorf("container %s not found", containerID)
	}

	// Testcontainers doesn't directly expose a commit method, so we need to use the Docker client
	// Get container info to ensure it exists
	_, err := tc.Inspect(context.Background())
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	// Get the underlying Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Commit the container
	_, err = cli.ContainerCommit(ctx, containerID, container.CommitOptions{
		Reference: imageName,
	})
	if err != nil {
		return fmt.Errorf("failed to commit container: %w", err)
	}

	return nil
}

// Helper to access unexported fields
type containerConfig struct {
	dockerRun      *dockerRun
	startupTimeout time.Duration
	startupDelay   time.Duration
	logStreaming   bool
	forceRuntime   string
	dsnProvider    interface{}
	waitFunc       interface{}
	backend        string
	files          []interface{} // FileEntry type

	// Testcontainers-specific fields
	testcontainersCustomizers []interface{}
	privileged                bool
	autoRemove                bool
	waitStrategy              interface{}
	hostConfigModifier        func(interface{})
	skipReaper                bool
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
}

// Setter methods for testcontainers customizations
func (c *containerConfig) AddTestcontainersCustomizer(customizer interface{}) {
	c.testcontainersCustomizers = append(c.testcontainersCustomizers, customizer)
}

func (c *containerConfig) SetTestcontainersPrivileged(privileged bool) {
	c.privileged = privileged
}

func (c *containerConfig) SetAutoRemove(autoRemove bool) {
	c.autoRemove = autoRemove
}

func (c *containerConfig) SetWaitStrategy(strategy interface{}) {
	c.waitStrategy = strategy
}

func (c *containerConfig) SetHostConfigModifier(modifier func(interface{})) {
	c.hostConfigModifier = modifier
}

func (c *containerConfig) SetSkipReaper(skip bool) {
	c.skipReaper = skip
}
