package testctr

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

var coordinatorOnce sync.Once

// getContainerRuntime returns the container runtime command to use
func getContainerRuntime() string {
	// Default to docker, can be overridden with options
	return "docker"
}

// Container wraps a docker container with useful methods
type Container struct {
	id           string
	t            testing.TB
	host         string
	ports        map[string]string
	container    string
	image        string
	logStreaming bool
	runtime      string      // docker or podman
	config       interface{} // Backend configuration
}

// dockerRun represents a minimal docker run command builder
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

// New creates a new test container with automatic cleanup
func New(t testing.TB, image string, opts ...Option) *Container {
	t.Helper()

	// Update coordinator settings from flags (only happens once)
	coordinatorOnce.Do(updateCoordinatorFromFlags)

	// Check for old containers
	checkOldContainers(t)

	// Parse image to get container type
	containerType := parseContainerType(image)


	// Build docker run command
	dr := &dockerRun{
		image:  image,
		env:    make(map[string]string),
		labels: containerLabels(t, image),
	}

	// Create config
	config := &containerConfig{
		dockerRun:    dr,
		logStreaming: *verbose, // Use global flag by default
	}

	// Apply options first to get backend
	for _, opt := range opts {
		if opt != nil {
			opt.apply(config)
		}
	}

	// Apply defaults based on backend
	applyDefaults(dr, config)

	// Check if we should use a backend
	if config.backend != "" {
		backend, err := GetBackend(config.backend)
		if err != nil {
			t.Fatalf("failed to get backend %q: %v", config.backend, err)
		}

		// Create container using backend
		containerID, err := backend.CreateContainer(t, image, config)
		if err != nil {
			t.Fatalf("backend failed to create container: %v", err)
		}

		// Get container info from backend
		info, err := backend.InspectContainer(containerID)
		if err != nil {
			t.Fatalf("backend failed to inspect container: %v", err)
		}

		c := &Container{
			id:           containerID,
			t:            t,
			host:         "127.0.0.1",
			ports:        extractPorts(info),
			container:    containerType,
			image:        image,
			logStreaming: config.logStreaming,
			runtime:      config.backend, // Store backend name as runtime
			config:       config,
		}

		// Cleanup
		t.Cleanup(func() {
			if *keepOnFailure && t.Failed() {
				t.Logf("Test failed - keeping container %s (%s) for debugging", containerID[:12], image)
				return
			}
			backend.RemoveContainer(containerID)
		})

		return c
	}

	// Default to direct Docker/Podman implementation
	// Determine which runtime to use
	runtime := getContainerRuntime()
	if config.forceRuntime != "" {
		runtime = config.forceRuntime
	}

	// Apply startup delay if configured
	if config.startupDelay > 0 {
		time.Sleep(config.startupDelay)
	}

	// Coordinate container creation to prevent resource contention
	globalContainerCoordinator.requestContainerSlot()
	defer globalContainerCoordinator.releaseContainerSlot()

	// Run container
	args := buildDockerRunArgs(dr)
	cmd := exec.Command(runtime, args...)

	// Debug: log the command being run
	if *verbose {
		t.Logf("Running: %s %s", runtime, strings.Join(args, " "))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create container: %v\nOutput: %s", err, output)
	}

	containerID := strings.TrimSpace(string(output))

	// Get container info
	info, err := inspectContainer(runtime, containerID)
	if err != nil {
		t.Fatalf("failed to inspect container: %v", err)
	}

	c := &Container{
		id:           containerID,
		t:            t,
		host:         "127.0.0.1",
		ports:        extractPorts(info),
		container:    containerType,
		image:        image,
		logStreaming: config.logStreaming,
		runtime:      runtime,
		config:       config,
	}

	// Auto cleanup (unless keepOnFailure is set and test failed)
	t.Cleanup(func() {
		t.Helper()
		// Check if test failed and we should keep the container
		if *keepOnFailure && t.Failed() {
			t.Logf("Test failed - keeping container %s (%s) for debugging", containerID[:12], image)
			t.Logf("To inspect: %s exec -it %s /bin/sh", c.runtime, containerID[:12])
			t.Logf("To remove: %s rm -f %s", c.runtime, containerID[:12])
			return
		}

		// Get final logs before cleanup if verbose
		if c.logStreaming {
			c.getLogs()
		}
		exec.Command(c.runtime, "rm", "-f", containerID).Run()
	})

	// Copy files into container if configured
	if len(config.files) > 0 {
		if *verbose {
			t.Logf("Copying %d files into container", len(config.files))
		}
		if err := copyFilesToContainer(containerID, runtime, config.files); err != nil {
			t.Fatalf("failed to copy files: %v", err)
		}
	}

	// Wait for container to be ready
	if err := waitForContainer(t, c); err != nil {
		t.Fatalf("container failed to start: %v", err)
	}

	// Get logs if enabled
	if c.logStreaming {
		// Always get current logs first
		go func() {
			// Small delay to let container produce output
			time.Sleep(50 * time.Millisecond)
			c.getLogs()
		}()
	}

	return c
}

// buildDockerRunArgs builds the docker run command arguments
func buildDockerRunArgs(dr *dockerRun) []string {
	args := []string{"run", "-d"}

	// Add labels
	for k, v := range dr.labels {
		args = append(args, "-l", fmt.Sprintf("%s=%s", k, v))
	}

	// Add environment variables
	for k, v := range dr.env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Add port mappings
	for _, port := range dr.ports {
		args = append(args, "-p", port)
	}

	// Add mounts
	for _, mount := range dr.mounts {
		args = append(args, "-v", mount)
	}

	// Add other options
	if dr.network != "" {
		args = append(args, "--network", dr.network)
	}
	if dr.user != "" {
		args = append(args, "--user", dr.user)
	}
	if dr.workdir != "" {
		args = append(args, "--workdir", dr.workdir)
	}
	if dr.memoryLimit != "" {
		args = append(args, "--memory", dr.memoryLimit)
	}

	// Add image
	args = append(args, dr.image)

	// Add command
	if len(dr.cmd) > 0 {
		args = append(args, dr.cmd...)
	}

	return args
}

// ContainerInfo represents minimal container inspection data
type ContainerInfo struct {
	NetworkSettings struct {
		Ports map[string][]struct {
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
	State struct {
		Running bool   `json:"Running"`
		Status  string `json:"Status"`
	} `json:"State"`
}

// inspectContainer gets container information
func inspectContainer(runtime, id string) (*ContainerInfo, error) {
	cmd := exec.Command(runtime, "inspect", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var infos []ContainerInfo
	if err := json.Unmarshal(output, &infos); err != nil {
		return nil, err
	}

	if len(infos) == 0 {
		return nil, fmt.Errorf("no container info found")
	}

	return &infos[0], nil
}

// extractPorts extracts port mappings from container info
func extractPorts(info *ContainerInfo) map[string]string {
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

// waitForContainer waits for the container to be ready
func waitForContainer(t testing.TB, c *Container) error {
	t.Helper()

	// Use wait function if provided
	if cfg, ok := c.config.(*containerConfig); ok && cfg.waitFunc != nil {
		if *verbose {
			t.Logf("Waiting for container %s to be ready...", c.id[:12])
		}
		start := time.Now()
		err := cfg.waitFunc(c.id, c.runtime)
		if err != nil {
			return err
		}
		if *verbose {
			t.Logf("Container %s ready after %v", c.id[:12], time.Since(start))
		}
		return nil
	}

	// No wait by default - container is assumed ready
	if *verbose {
		t.Logf("No wait strategy configured for container %s", c.id[:12])
	}
	return nil
}

// waitForLog waits for a specific log line
func waitForLog(containerID, logLine string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	attempts := 0
	var lastOutput []byte

	// Check both stdout and stderr
	for {
		select {
		case <-ctx.Done():
			// Get fresh logs one more time in case we missed something
			cmd := exec.Command(getContainerRuntime(), "logs", containerID)
			finalOutput, _ := cmd.CombinedOutput()
			if len(finalOutput) > 0 {
				lastOutput = finalOutput
			}

			// Include last 20 lines of output in error message to help debugging
			lines := strings.Split(string(lastOutput), "\n")
			if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
				return fmt.Errorf("timeout after %v waiting for log: %s (checked %d times)\nNo output from container",
					time.Since(start), logLine, attempts)
			}

			recentLines := lines
			if len(lines) > 20 {
				recentLines = lines[len(lines)-20:]
			}
			return fmt.Errorf("timeout after %v waiting for log: %s (checked %d times)\nLast %d lines of container output:\n%s",
				time.Since(start), logLine, attempts, len(recentLines), strings.Join(recentLines, "\n"))
		default:
			attempts++
			cmd := exec.Command(getContainerRuntime(), "logs", containerID)
			output, _ := cmd.CombinedOutput() // Get both stdout and stderr
			lastOutput = output
			if strings.Contains(string(output), logLine) {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// applyDefaults applies generic default configuration
func applyDefaults(dr *dockerRun, cfg *containerConfig) {
	// No defaults - let users specify everything
}

// Host returns the container host
func (c *Container) Host() string {
	return c.host
}

// Port returns the mapped port for a given container port
func (c *Container) Port(containerPort string) string {
	return c.ports[containerPort]
}

// Endpoint returns host:port for a given container port
func (c *Container) Endpoint(containerPort string) string {
	return fmt.Sprintf("%s:%s", c.host, c.Port(containerPort))
}

// ID returns the container ID
func (c *Container) ID() string {
	return c.id
}

// Runtime returns the container runtime being used
func (c *Container) Runtime() string {
	return c.runtime
}

// Config returns the container configuration
func (c *Container) Config() interface{} {
	return c.config
}

// Exec runs a command in the container
func (c *Container) Exec(ctx context.Context, cmd []string) (int, string, error) {
	args := append([]string{"exec", c.id}, cmd...)
	command := exec.CommandContext(ctx, c.runtime, args...)

	// Combine stdout and stderr for simpler handling
	output, err := command.CombinedOutput()

	// Get exit code
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = -1
	}

	return exitCode, string(output), err
}

// ExecSimple runs a command and returns just the output, panicking on error
// This is useful for tests where you expect the command to succeed
func (c *Container) ExecSimple(cmd ...string) string {
	ctx := context.Background()
	if tb, ok := c.t.(interface{ Context() context.Context }); ok {
		ctx = tb.Context()
	}

	exitCode, output, err := c.Exec(ctx, cmd)
	if err != nil {
		c.t.Fatalf("Command failed: %v (output: %s)", err, output)
	}
	if exitCode != 0 {
		c.t.Fatalf("Command exited with code %d: %s", exitCode, output)
	}
	return strings.TrimSpace(output)
}

// parseContainerType extracts the container type from image string
func parseContainerType(image string) string {
	// Simple parsing - just get the first part before :
	for i, ch := range image {
		if ch == ':' || ch == '/' {
			return image[:i]
		}
	}
	return image
}

// waitFunc is a function that waits for a container to be ready
type waitFunc func(containerID, runtime string) error

// containerConfig for the direct implementation
type containerConfig struct {
	dockerRun      *dockerRun
	startupTimeout time.Duration
	startupDelay   time.Duration
	logStreaming   bool
	forceRuntime   string
	dsnProvider    DSNProvider
	waitFunc       waitFunc
	backend        string
	files          []FileEntry
}

// Helper functions for options
func newBindMount(hostPath, containerPath string) string {
	return fmt.Sprintf("%s:%s", hostPath, containerPath)
}

// getLogs fetches container logs once without following
func (c *Container) getLogs() {
	output, err := exec.Command(c.runtime, "logs", c.id).CombinedOutput()
	if err != nil {
		// Check the output for known error messages
		outputStr := string(output)
		if !strings.Contains(outputStr, "No such container") &&
			!strings.Contains(outputStr, "dead or marked for removal") &&
			!strings.Contains(outputStr, "Error response from daemon") {
			c.t.Logf("Failed to get logs: %v", err)
		}
		return
	}

	if len(output) > 0 {
		for _, line := range strings.Split(string(output), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				c.t.Logf("[%s] %s", c.image, line)
			}
		}
	}
}

// streamLogs streams container logs to testing.T (for long-running containers)
func (c *Container) streamLogs() {
	cmd := exec.Command(c.runtime, "logs", "-f", c.id)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		// Don't log error if container is already stopped - this is expected
		if !strings.Contains(err.Error(), "dead") && !strings.Contains(err.Error(), "removal") {
			c.t.Logf("Failed to start log streaming: %v", err)
		}
		return
	}

	// Read both stdout and stderr
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				c.t.Logf("[%s stdout] %s", c.container, strings.TrimSpace(string(buf[:n])))
			}
			if err != nil {
				break
			}
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				msg := strings.TrimSpace(string(buf[:n]))
				// Filter out expected errors for stopped containers
				if !strings.Contains(msg, "dead or marked for removal") &&
					!strings.Contains(msg, "No such container") &&
					!strings.Contains(msg, "can not get logs from container") {
					c.t.Logf("[%s stderr] %s", c.container, msg)
				}
			}
			if err != nil {
				break
			}
		}
	}()
}
