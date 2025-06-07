package testctr

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr/backend"
)

var coordinatorOnce sync.Once

// discoverContainerRuntime discovers the available container runtime
func discoverContainerRuntime() string {
	// Check environment variable first
	if runtime := os.Getenv("TESTCTR_RUNTIME"); runtime != "" {
		return runtime
	}

	// Try to find available runtimes in order of preference
	runtimes := []string{"docker", "podman", "nerdctl"}
	for _, runtime := range runtimes {
		if _, err := exec.LookPath(runtime); err == nil {
			return runtime
		}
	}

	// Default to docker if nothing found (will likely fail later)
	return "docker"
}

// Container manages a test container instance created with New.
// All methods are safe for concurrent use. The container is automatically
// cleaned up when the test completes unless debugging flags are set.
type Container struct {
	id           string
	t            testing.TB
	host         string
	ports        map[string]string // Mapped ports: containerPort -> hostPort
	container    string
	image        string
	logStreaming bool
	localRuntime string // name of the container runtime used (e.g. "docker", "podman") (for direct implementation)
	config       *containerConfig

	// nil for default implementation, otherwise set to a specific backend via options
	be backend.Backend // Backend interface for container management
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
	privileged  bool
}

// fileEntry represents a file to be copied into the container.
type fileEntry struct {
	Source any         // string (file path) or io.Reader
	Target string      // Target path in container
	Mode   os.FileMode // File permissions
}

// waitCondition represents a condition to wait for before considering container ready.
// The context includes any test deadline and should be respected to allow clean shutdown.
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

	// Parse image to get container type
	_ = parseImageBasename(image)

	// Build docker run command
	dr := &dockerRun{
		image:  image,
		env:    make(map[string]string),
		labels: containerLabels(t, image),
	}

	// Create config
	config := &containerConfig{
		dockerRun:      dr,
		logStreaming:   *verbose,        // Use global flag by default
		startupTimeout: 5 * time.Second, // Default startup timeout
	}

	// Apply options first to get backend
	for _, opt := range opts {
		if opt != nil {
			opt.apply(config)
		}
	}

	// Apply defaults based on backend
	applyDefaults(dr, config)

	// No longer add default wait condition here - runWaitConditions handles it

	// TODO: Consider treating this API more like exec.Command, where the container
	// configuration is built up and then started separately, allowing for more
	// flexibility in how containers are created and managed.

	// Determine which runtime to use
	runtime := config.localRuntime
	if runtime == "" {
		runtime = discoverContainerRuntime()
	}

	// Apply startup delay if configured
	if config.startupDelay > 0 {
		time.Sleep(config.startupDelay)
	}

	// Coordinate container creation to prevent resource contention
	globalContainerCoordinator.requestContainerSlot()
	defer globalContainerCoordinator.releaseContainerSlot()

	// Create the container
	container, err := createContainer(t, runtime, config)
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	// Setup cleanup
	setupContainerCleanup(t, container)

	// Wait for container to be ready and perform initialization
	if err := waitForContainerReady(t, container); err != nil {
		t.Fatalf("container failed to become ready: %v", err)
	}

	return container
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

// createContainer handles the container creation process
func createContainer(t testing.TB, runtime string, config *containerConfig) (*Container, error) {
	t.Helper()

	// Pull image if needed
	if err := ensureImageExists(t, runtime, config.dockerRun.image); err != nil {
		return nil, fmt.Errorf("failed to ensure image exists: %w", err)
	}

	// Create a context for container creation
	ctx, cancel := prepareWaitContext(t, config.dockerRun.image)
	defer cancel()

	// Build and run the container
	containerID, err := runContainer(ctx, t, runtime, config.dockerRun)
	if err != nil {
		return nil, fmt.Errorf("failed to run container: %w", err)
	}

	// Create container object
	c := &Container{
		id:           containerID,
		t:            t,
		host:         "127.0.0.1",
		container:    parseImageBasename(config.dockerRun.image),
		image:        config.dockerRun.image,
		logStreaming: config.logStreaming,
		localRuntime: runtime,
		config:       config,
	}

	// Get initial container info
	if err := c.updatePortMappings(); err != nil {
		// Log error but don't fail - ports might not be ready yet
		c.logVerbosef("initial port mapping update failed: %v", err)
	}

	return c, nil
}

// runContainer executes the docker/podman run command
func runContainer(ctx context.Context, t testing.TB, runtime string, dr *dockerRun) (string, error) {
	t.Helper()

	args := buildDockerRunArgs(dr)
	cmd := exec.CommandContext(ctx, runtime, args...)

	if *verbose {
		t.Logf("testctr: Running: %s %s", runtime, strings.Join(args, " "))
	}

	o := new(bytes.Buffer)
	cmd.Stdout = o

	if *verbose {
		stderr, err := cmd.StderrPipe()
		if err != nil {
			t.Logf("testctr: runContainer: Failed to get stderr pipe: %v", err)
		}

		// copy stderr to t.Logf:
		logf := t.Logf
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				logf(scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				logf("testctr: runContainer: Failed to read stderr: %v", err)
			}
		}()
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}
	containerID := strings.TrimSpace(o.String())
	if containerID == "" {
		return "", fmt.Errorf("no container ID returned")
	}

	return containerID, nil
}

// extractPorts extracts port mappings from container info
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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// contextWithTimeout creates a context that respects test deadlines.
// It automatically checks for test deadlines and applies the requested timeout
// within those constraints. The cause will indicate whether the deadline or
// timeout was hit.
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
			// Include a shutdown buffer:
			shutdownBuffer := 5 * time.Second
			deadline = deadline.Add(-shutdownBuffer)
			remaining := time.Until(deadline)

			// If the test deadline is sooner than our timeout, use it
			if remaining > 0 && remaining < timeout {
				if c.logStreaming {
					c.logf("%s limited by test deadline (%v remaining)", contextMsg, remaining)
				}
				return context.WithDeadlineCause(ctx, deadline,
					fmt.Errorf("test deadline exceeded during %s", contextMsg))
			}
		}
	}

	// Use the requested timeout
	return context.WithTimeoutCause(ctx, timeout,
		fmt.Errorf("timeout (%v) exceeded during %s", timeout, contextMsg))
}

// prepareWaitContext creates a context for wait operations with a cleanup buffer.
// This is specifically for container startup where we need to leave time for cleanup.
func prepareWaitContext(t testing.TB, containerID string) (context.Context, context.CancelFunc) {
	ctx := context.Background()
	cleanupBuffer := 5 * time.Second

	// Check if test has a deadline
	if td, ok := t.(interface{ Deadline() (time.Time, bool) }); ok {
		if deadline, hasDeadline := td.Deadline(); hasDeadline {
			// Leave buffer for cleanup
			adjustedDeadline := deadline.Add(-cleanupBuffer)
			if time.Until(adjustedDeadline) > 0 {
				// if *verbose {
				// 	t.Logf("testctr: using test deadline %v for container %s (adjusted from %v for cleanup, %v remaining)",
				// 		adjustedDeadline, containerID[:min(12, len(containerID))], deadline, time.Until(adjustedDeadline))
				// }
				return context.WithDeadlineCause(ctx, adjustedDeadline,
					fmt.Errorf("go test overall timeout exceeded while waiting for container %s", containerID[:min(12, len(containerID))]))
			}
		}
	}

	// No deadline or deadline already passed, use a reasonable default
	defaultTimeout := 60 * time.Second
	if *verbose {
		t.Logf("testctr: no test deadline found, using default timeout of %v for container %s", defaultTimeout, containerID[:min(12, len(containerID))])
	}
	return context.WithTimeoutCause(ctx, defaultTimeout,
		fmt.Errorf("default timeout exceeded while waiting for container %s", containerID[:min(12, len(containerID))]))
}

// waitForContainer waits for the container to be ready
func waitForContainer(t testing.TB, c *Container) error {
	t.Helper()

	// Prepare context with test deadline
	ctx, cancel := prepareWaitContext(t, c.id)
	defer cancel()

	// Use the new runWaitConditions method
	return c.runWaitConditions(ctx)
}

// runWaitConditions executes all configured wait conditions for the container.
// It respects the provided overall context for timeouts.
func (c *Container) runWaitConditions(ctx context.Context) error {
	c.t.Helper()
	if len(c.config.waitConditions) == 0 {
		// Apply a default minimal wait if no conditions are specified.
		c.config.waitConditions = []waitCondition{ensureRunning}
		if c.logStreaming {
			c.logf("applying default 'running' check")
		}
	}

	for i, condFunc := range c.config.waitConditions {
		if c.logStreaming {
			c.logf("running wait condition %d/%d", i+1, len(c.config.waitConditions))
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("overall context done before running wait condition %d: %w", i+1, context.Cause(ctx))
		default:
		}
		// Each condFunc is responsible for its own internal timeout logic using the passed context.
		if err := condFunc(ctx, c); err != nil {
			return fmt.Errorf("wait condition %d failed: %w", i+1, err)
		}
	}
	return nil
}

// ensureRunning performs a basic check that the container is running
func ensureRunning(ctx context.Context, c *Container) error {
	c.t.Helper()
	if c.be != nil {
		// Backend handles its own inspection
		return nil
	}

	// Use contextWithTimeout to respect test deadline
	waitCtx, cancel := c.contextWithTimeout(ctx, c.config.startupTimeout, "container startup check")
	defer cancel()

	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Check immediately before waiting
	info, err := c.inspectContainer(waitCtx)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}
	if info.State.Running {
		c.logf("container is running after %v", time.Since(start))
		return nil
	}

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("failed to verify container running after %v: %w", time.Since(start), context.Cause(waitCtx))
		case <-ticker.C:
			info, err := c.inspectContainer(waitCtx)
			if err != nil {
				return fmt.Errorf("failed to inspect container: %w", err)
			}
			if info.State.Running {
				c.logf("container is running after %v", time.Since(start))
				return nil
			}
			c.logVerbosef("container not running yet (status: %s), waiting...", info.State.Status)
			// Check if container exited
			if info.State.Status == "exited" {
				return fmt.Errorf("container exited with status: %s", info.State.Status)
			}
		}
	}
}

// waitForLogWithContext waits for a specific log line with context support
func waitForLogWithContext(ctx context.Context, containerID, logLine string) error {

	start := time.Now()
	attempts := 0
	var lastOutput []byte

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Check immediately before waiting
	cmd := exec.CommandContext(ctx, discoverContainerRuntime(), "logs", containerID)
	output, _ := cmd.CombinedOutput()
	lastOutput = output
	attempts++
	if strings.Contains(string(output), logLine) {
		return nil
	}

	// Then check periodically
	for {
		select {
		case <-ctx.Done():
			// Get fresh logs one more time in case we missed something
			cmd := exec.CommandContext(context.Background(), discoverContainerRuntime(), "logs", containerID)
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
		case <-ticker.C:
			attempts++
			cmd := exec.CommandContext(ctx, discoverContainerRuntime(), "logs", containerID)
			output, _ := cmd.CombinedOutput() // Get both stdout and stderr
			lastOutput = output
			if strings.Contains(string(output), logLine) {
				return nil
			}
		}
	}
}

// applyDefaults applies generic default configuration
func applyDefaults(dr *dockerRun, cfg *containerConfig) {
	// No defaults - let users specify everything
}

// Host returns the host address for accessing the container, typically "127.0.0.1".
// This is guaranteed to be a valid IP address for connecting to exposed ports.
func (c *Container) Host() string {
	return c.host
}

// Port returns the host port mapped to the given container port.
// Returns empty string if the port is not exposed. The port must have been
// exposed using WithPort during container creation.
//
//	redis := testctr.New(t, "redis:7", testctr.WithPort("6379"))
//	port := redis.Port("6379") // e.g., "32768"
func (c *Container) Port(containerPort string) string {
	return c.ports[containerPort]
}

// Endpoint returns the complete network endpoint (host:port) for accessing
// a service running on the specified container port. Convenience method
// equivalent to Host() + ":" + Port(containerPort).
//
//	endpoint := pg.Endpoint("5432") // e.g., "127.0.0.1:32769"
//	db, _ := sql.Open("postgres", "postgres://user:pass@" + endpoint + "/db")
func (c *Container) Endpoint(containerPort string) string {
	return fmt.Sprintf("%s:%s", c.host, c.Port(containerPort))
}

// ID returns the full container ID.
// Useful for debugging or direct CLI commands.
func (c *Container) ID() string {
	return c.id
}

// runtime returns the container runtime name: "docker", "podman", or "nerdctl".
func (c *Container) runtime() string {
	return c.localRuntime
}

// logf logs a message with the testctr prefix and image name
func (c *Container) logf(format string, args ...interface{}) {
	c.t.Helper()
	id := c.ID()[:min(12, len(c.ID()))]
	prefix := fmt.Sprintf("testctr: [%s] %s", c.image, id)
	c.t.Logf("%s %s", prefix, fmt.Sprintf(format, args...))
}

// logFiltered logs a message after applying the configured log filter
func (c *Container) logFiltered(format string, args ...interface{}) {
	c.t.Helper()
	msg := fmt.Sprintf(format, args...)

	// Apply log filter if configured
	if c.config.logFilter != nil && !c.config.logFilter(msg) {
		return
	}

	c.logf("%s", msg)
}

// logVerbosef logs a message only if verbose mode is enabled
func (c *Container) logVerbosef(format string, args ...interface{}) {
	c.t.Helper()
	if *verbose {
		c.logf(format, args...)
	}
}

// inspectContainer gets container information and logs any issues
func (c *Container) inspectContainer(ctx context.Context) (*backend.ContainerInfo, error) {
	c.t.Helper()
	// Execute the inspect command
	cmd := exec.CommandContext(ctx, c.localRuntime, "inspect", c.id)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			c.logf("failed to inspect container: %v (stderr: %s)", err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Parse the JSON output
	var infos []backend.ContainerInfo
	if err := json.Unmarshal(output, &infos); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	if len(infos) == 0 {
		return nil, fmt.Errorf("no container info found")
	}

	info := &infos[0]

	// Log container state if verbose
	c.logVerbosef("state: running=%v, status=%s, exitCode=%d",
		info.State.Running, info.State.Status, info.State.ExitCode)

	// Log port mappings if any
	if len(info.NetworkSettings.Ports) > 0 {
		for port, bindings := range info.NetworkSettings.Ports {
			if len(bindings) > 0 && bindings[0].HostPort != "" {
				c.logVerbosef("port mapping: %s -> %s", port, bindings[0].HostPort)
			}
		}
	}

	return info, nil
}

// Inspect returns detailed container information including state,
// network settings, and other metadata.
//
//	info, _ := container.Inspect()
//	t.Logf("State: %s, IP: %s", info.State.Status, info.NetworkSettings.IPAddress)
func (c *Container) Inspect() (*backend.ContainerInfo, error) {
	c.t.Helper()
	ctx, cancel := c.contextWithTimeout(context.Background(), 5*time.Second, "inspect container")
	defer cancel()
	return c.inspectContainer(ctx)
}

// Exec executes a command inside the running container.
// It returns the exit code, combined stdout/stderr output, and any execution error.
// The provided context can be used to set timeouts or cancel the operation.
//
// This is the low-level exec method. For simpler cases where you expect
// the command to succeed, use ExecSimple instead.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//	defer cancel()
//	exitCode, output, err := container.Exec(ctx, []string{"psql", "-c", "SELECT 1"})
//	if err != nil {
//	    t.Fatalf("exec failed: %v", err)
//	}
//	if exitCode != 0 {
//	    t.Fatalf("command failed with exit code %d: %s", exitCode, output)
//	}
func (c *Container) Exec(ctx context.Context, cmd []string) (int, string, error) {
	args := append([]string{"exec", c.id}, cmd...)
	command := exec.CommandContext(ctx, c.localRuntime, args...)

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

// ExecSimple executes a command inside the container and returns the output.
// It calls t.Fatal if the command fails or returns a non-zero exit code.
// This is a convenience method for commands that are expected to succeed.
//
// The method automatically applies a 30-second timeout (respecting test deadlines)
// and trims whitespace from the output.
//
// Example:
//
//	version := container.ExecSimple("redis-cli", "--version")
//	result := container.ExecSimple("redis-cli", "PING") // Returns "PONG"
//
// For commands that might fail or when you need the exit code, use Exec instead.
func (c *Container) ExecSimple(cmd ...string) string {
	c.t.Helper()
	// Default exec timeout of 30s, respecting test deadlines
	cmdStr := strings.Join(cmd, " ")
	ctx, cancel := c.contextWithTimeout(context.Background(), 30*time.Second, "exec", cmdStr)
	defer cancel()

	exitCode, output, err := c.Exec(ctx, cmd)
	if err != nil {
		c.t.Fatalf("command failed: %v (command: %s, output: %s)", err, cmdStr, output)
	}
	if exitCode != 0 {
		c.t.Fatalf("command exited with code %d (command: %s, output: %s)", exitCode, cmdStr, output)
	}
	return strings.TrimSpace(output)
}

// parseImageBasename extracts the container type from image string
func parseImageBasename(image string) string {
	// Simple parsing - just get the first part before :
	for i, ch := range image {
		if ch == ':' || ch == '/' {
			return image[:i]
		}
	}
	return image
}

// containerConfig for the direct implementation
type containerConfig struct {
	dockerRun *dockerRun

	startupTimeout time.Duration // Timeout for container startup
	startupDelay   time.Duration // Delay before starting the container

	logStreaming bool        // Whether to stream logs to testing.T
	localRuntime string      // Override for container runtime (e.g. "docker", "podman", "nerdctl")
	dsnProvider  DSNProvider // Function to provide DSN for the container
	files        []fileEntry // Files to copy into the container
	privileged   bool        // Whether to run the container in privileged mode

	// Wait conditions
	waitConditions []waitCondition

	// Log filtering
	logFilter func(string) bool // Optional filter function for log lines

	// Testcontainers-specific fields
	testcontainersCustomizers []interface{} // Store as interface{} to avoid import cycle
	tcPrivileged              bool
	tcAutoRemove              bool
	tcWaitStrategy            interface{}
	tcHostConfigModifier      func(interface{})
	tcReuse                   bool
}

// AddWaitCondition adds a wait condition to the container config
func (cc *containerConfig) AddWaitCondition(cond waitCondition) {
	cc.waitConditions = append(cc.waitConditions, cond)
}

// getLogs fetches container logs once without following
func (c *Container) getLogs() {
	c.t.Helper()
	ctx, cancel := c.contextWithTimeout(context.Background(), 5*time.Second, "get logs")
	defer cancel()

	cmd := exec.CommandContext(ctx, c.localRuntime, "logs", c.id)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check the output for known error messages
		outputStr := string(output)
		if !strings.Contains(outputStr, "No such container") &&
			!strings.Contains(outputStr, "dead or marked for removal") &&
			!strings.Contains(outputStr, "Error response from daemon") {
			c.logf("Failed to get logs: %v", err)
		}
		return
	}

	if len(output) > 0 {
		for _, line := range strings.Split(string(output), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				c.logFiltered("%s", line)
			}
		}
	}
}

// streamLogs streams container logs to testing.T (for long-running containers)
func (c *Container) streamLogs() {
	// Use a long timeout for streaming logs (basically until test ends)
	ctx, cancel := c.contextWithTimeout(context.Background(), 1*time.Hour, "stream logs")
	defer cancel()

	cmd := exec.CommandContext(ctx, c.localRuntime, "logs", "-f", c.id)
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
				c.logFiltered("[stdout] %s", strings.TrimSpace(string(buf[:n])))
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
				// if !strings.Contains(msg, "dead or marked for removal") &&
				// 	!strings.Contains(msg, "No such container") &&
				// 	!strings.Contains(msg, "can not get logs from container") {
				c.logFiltered("[stderr] %s", msg)
			}
			if err != nil {
				break
			}
		}
	}()
}

// ensureImageExists checks if the image exists locally and pulls it if needed
func ensureImageExists(t testing.TB, runtime, image string) error {
	t.Helper()

	// Create a context that respects test deadlines
	ctx, cancel := prepareWaitContext(t, image)
	defer cancel()

	// Check if image exists locally
	cmd := exec.CommandContext(ctx, runtime, "image", "inspect", image)
	if err := cmd.Run(); err == nil {
		// Image exists
		return nil
	}

	// Image doesn't exist, try to pull it
	// TODO: Consider coordinating pull operations across parallel tests to:
	//   1. Avoid duplicate pulls when multiple tests need the same image
	//   2. Reduce noise in test output by having only one test log the pull progress
	//   3. Potentially use a mutex or channel to serialize pulls of the same image
	//   4. Consider caching pull results to avoid repeated network checks
	t.Logf("testctr: Pulling image %s (this may take a moment)...", image)

	pullCmd := exec.CommandContext(ctx, runtime, "pull", image)

	if *verbose {
		// Stream output to t.Logf in verbose mode
		stdout, err := pullCmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to get stdout pipe: %w", err)
		}
		stderr, err := pullCmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("failed to get stderr pipe: %w", err)
		}

		if err := pullCmd.Start(); err != nil {
			return fmt.Errorf("failed to start pull command: %w", err)
		}

		// Use WaitGroup to ensure we read all output
		var wg sync.WaitGroup
		wg.Add(2)

		// Stream stdout
		go func() {
			defer wg.Done()
			stdoutScanner := bufio.NewScanner(stdout)
			for stdoutScanner.Scan() {
				t.Logf("testctr: [pull %s] %s", image, stdoutScanner.Text())
			}
		}()

		// Stream stderr
		go func() {
			defer wg.Done()
			stderrScanner := bufio.NewScanner(stderr)
			for stderrScanner.Scan() {
				t.Logf("testctr: [pull %s] %s", image, stderrScanner.Text())
			}
		}()

		// Wait for command to finish
		err = pullCmd.Wait()

		// Wait for all output to be read
		wg.Wait()

		if err != nil {
			return fmt.Errorf("failed to pull image %s: %w", image, err)
		}
	} else {
		// Non-verbose mode - just capture output
		output, err := pullCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to pull image %s: %w\nOutput: %s", image, err, output)
		}
	}

	t.Logf("testctr: Successfully pulled image %s", image)
	return nil
}

// updatePortMappings updates the container's port mappings from inspection
func (c *Container) updatePortMappings() error {
	c.t.Helper()

	ctx, cancel := c.contextWithTimeout(context.Background(), 5*time.Second, "port mapping update")
	defer cancel()

	info, err := c.inspectContainer(ctx)
	if err != nil {
		return err
	}

	c.ports = extractPorts(info)
	return nil
}

// setupContainerCleanup registers cleanup handlers for the container
func setupContainerCleanup(t testing.TB, c *Container) {
	t.Helper()

	t.Cleanup(func() {
		t.Helper()

		// Check if test failed and we should keep the container
		if *keepOnFailure && t.Failed() {
			t.Logf("Test failed - keeping container %s (%s) for debugging", c.id[:12], c.image)
			t.Logf("To inspect: %s exec -it %s /bin/sh", c.localRuntime, c.id[:12])
			t.Logf("To view logs: %s logs %s", c.localRuntime, c.id[:12])
			t.Logf("To remove: %s rm -f %s", c.localRuntime, c.id[:12])
			return
		}

		// Stop the container first with a timeout
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()

		stopCmd := exec.CommandContext(stopCtx, c.localRuntime, "stop", c.id)
		if output, err := stopCmd.CombinedOutput(); err != nil {
			// Only log if it's not a "container not found" error
			outputStr := string(output)
			if !strings.Contains(outputStr, "No such container") &&
				!strings.Contains(outputStr, "can not get logs from container") {
				c.logVerbosef("failed to stop container: %v", err)
			}
		}

		// Remove the container
		rmCmd := exec.CommandContext(stopCtx, c.localRuntime, "rm", "-f", c.id)
		if output, err := rmCmd.CombinedOutput(); err != nil {
			// Only log if it's not a "container not found" error
			outputStr := string(output)
			if !strings.Contains(outputStr, "No such container") {
				c.logf("failed to remove container: %v\nOutput: %s", err, output)
			}
		}
	})
}

// waitForContainerReady waits for the container to be ready according to wait conditions
func waitForContainerReady(t testing.TB, c *Container) error {
	t.Helper()

	// Copy files if needed
	if err := copyFilesIfNeeded(t, c); err != nil {
		return fmt.Errorf("failed to copy files: %w", err)
	}

	// Wait for container to be ready
	if err := waitForContainer(t, c); err != nil {
		return fmt.Errorf("container failed to start: %w", err)
	}

	// Start log streaming if enabled
	if c.logStreaming {
		// Get current logs first
		go func() {
			time.Sleep(50 * time.Millisecond)
			c.getLogs()
		}()

		// Start streaming logs
		if !c.config.tcAutoRemove {
			go c.streamLogs()
		}
	}

	return nil
}

// copyFilesIfNeeded copies any configured files into the container
func copyFilesIfNeeded(t testing.TB, c *Container) error {
	t.Helper()

	if len(c.config.files) == 0 {
		return nil
	}

	if *verbose {
		t.Logf("testctr: Copying %d files into container", len(c.config.files))
	}

	return copyFilesToContainerCLI(c.id, c.localRuntime, c.config.files, t)
}
