package testctr

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmc/misc/testctr/backend"
	_ "github.com/tmc/misc/testctr/backends/cli" // Register default CLI backend
)

// ==============================
// Public API Types and Interfaces
// ==============================

// Container represents a running test container.
type Container struct {
	tb               testing.TB
	containerID      string
	backend          backend.Backend
	image            string
	config           *containerConfig
	info             *backend.ContainerInfo
	infoMu           sync.Mutex
	startTime        time.Time
	coordinator      *containerCoordinator
	cleanupCalled    bool
	parallelStarters chan struct{}
}

// ==============================
// Public API Functions
// ==============================

// New creates and starts a new container for testing.
// The container is automatically cleaned up when the test ends.
func New(tb testing.TB, image string, opts ...Option) *Container {
	tb.Helper()

	// Set up deferred parallel if needed
	if *autoParallel && !isTestRunningInParallel(tb) {
		tb.Parallel()
	}

	// Create container with defaults
	c := &Container{
		tb:               tb,
		image:            image,
		startTime:        time.Now(),
		config:           newContainerConfig(),
		parallelStarters: make(chan struct{}, maxParallelStarts),
	}

	// Apply options
	for _, opt := range opts {
		if opt != nil {
			opt.apply(c.config)
			opt.apply(c)
		}
	}

	// Set up backend
	if c.backend == nil {
		be, err := backend.Get(c.config.backend)
		if err != nil {
			tb.Fatalf("failed to get backend %q: %v", c.config.backend, err)
		}
		c.backend = be
	}

	// Create coordinator if using shared containers
	if c.config.shared != "" {
		c.coordinator = getOrCreateCoordinator(c.config.shared)
	}

	// Set up cleanup
	tb.Cleanup(func() {
		c.cleanup()
	})

	// Start container
	c.start()
	return c
}

// ==============================
// Container Methods
// ==============================

// ID returns the container ID.
func (c *Container) ID() string {
	return c.containerID
}

// Endpoint returns the host endpoint for a container port.
// For example, for a Redis container: redis.Endpoint("6379") returns "127.0.0.1:55432"
func (c *Container) Endpoint(containerPort string) string {
	info := c.inspect()
	
	// Normalize port format
	if !strings.Contains(containerPort, "/") {
		containerPort = containerPort + "/tcp"
	}

	// Find port mapping
	if info.NetworkSettings.Ports != nil {
		if bindings, ok := info.NetworkSettings.Ports[containerPort]; ok && len(bindings) > 0 {
			return fmt.Sprintf("127.0.0.1:%s", bindings[0].HostPort)
		}
	}

	// Fallback: try to extract from backend-specific info
	hostPort := c.extractHostPort(containerPort)
	if hostPort != "" {
		return fmt.Sprintf("127.0.0.1:%s", hostPort)
	}

	c.tb.Fatalf("no host port found for container port %s", containerPort)
	return ""
}

// Host returns the host to connect to the container (always 127.0.0.1).
func (c *Container) Host() string {
	return "127.0.0.1"
}

// Port returns the mapped host port for a container port.
func (c *Container) Port(containerPort string) string {
	endpoint := c.Endpoint(containerPort)
	parts := strings.Split(endpoint, ":")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// Exec executes a command in the container and returns the exit code and combined output.
func (c *Container) Exec(ctx context.Context, cmd []string) (int, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Apply test timeout if no deadline is set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = c.contextWithTimeout(ctx)
		defer cancel()
	}

	c.logf("executing command: %v", cmd)
	return c.backend.Exec(ctx, c.containerID, cmd)
}

// Logs returns the container logs.
func (c *Container) Logs(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Apply test timeout if no deadline is set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = c.contextWithTimeout(ctx)
		defer cancel()
	}

	return c.backend.Logs(ctx, c.containerID)
}

// DSN returns a database connection string if the container was started with a DSNProvider option.
// The database name is automatically generated based on the test name for isolation.
func (c *Container) DSN(tb testing.TB) string {
	if c.config.dsnProvider == nil {
		tb.Fatal("container was not started with a DSN provider option (use mysql.Default(), postgres.Default(), etc.)")
	}

	// Generate database name from test name
	dbName := sanitizeDBName(tb.Name())

	// Create database
	ctx := context.Background()
	deadline, ok := tb.Deadline()
	if ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	dsn, err := c.config.dsnProvider.CreateDatabase(ctx, c, dbName)
	if err != nil {
		tb.Fatalf("failed to create database: %v", err)
	}

	return dsn
}

// CopyFileToContainer copies a file from the host to the container.
func (c *Container) CopyFileToContainer(ctx context.Context, src, dst string) error {
	return c.backend.CopyToContainer(ctx, c.containerID, src, dst)
}

// ==============================
// Internal Container Configuration
// ==============================

// containerConfig holds all container configuration.
type containerConfig struct {
	// Core configuration
	ports       []string
	env         map[string]string
	cmd         []string
	labels      map[string]string
	volumes     []string
	tmpfs       map[string]string
	capAdd      []string
	capDrop     []string
	user        string
	workingDir  string
	networkMode string
	runtime     string
	memory      string
	cpus        string
	files       []fileEntry
	pullPolicy  string

	// Startup options
	waitLogPattern   string
	waitLogTimeout   time.Duration
	waitExecCmd      []string
	waitExecTimeout  time.Duration
	waitHTTPPath     string
	waitHTTPTimeout  time.Duration
	waitHTTPExpect   int
	startupTimeout   time.Duration
	postStartFunc    func(context.Context, *Container) error

	// Behavior flags
	privileged bool
	autoRemove bool
	background bool
	logs       bool
	reuse      bool
	shared     string

	// Backend selection
	backend      string
	backendOpts  interface{}
	dsnProvider  DSNProvider

	// Log filtering
	logFilter    func(string) bool
}

// fileEntry represents a file to copy into a container.
type fileEntry struct {
	Source interface{} // string (file path) or io.Reader
	Target string
	Mode   os.FileMode
}

func newContainerConfig() *containerConfig {
	return &containerConfig{
		env:             make(map[string]string),
		labels:          make(map[string]string),
		tmpfs:           make(map[string]string),
		waitLogTimeout:  30 * time.Second,
		waitExecTimeout: 30 * time.Second,
		waitHTTPTimeout: 30 * time.Second,
		waitHTTPExpect:  200,
		startupTimeout:  2 * time.Minute,
		pullPolicy:      "missing",
	}
}

// ==============================
// Internal Container Methods
// ==============================

// start creates and starts the container.
func (c *Container) start() {
	c.tb.Helper()

	// Handle shared containers
	if c.coordinator != nil {
		existing := c.coordinator.getOrCreate(c.image, func() *Container {
			c.doStart()
			return c
		})
		if existing != c {
			// Use existing container
			c.containerID = existing.containerID
			c.info = existing.info
			c.cleanupCalled = true // Don't cleanup shared containers
			return
		}
	}

	c.doStart()
}

// doStart performs the actual container start.
func (c *Container) doStart() {
	c.tb.Helper()

	// Limit parallel container starts
	select {
	case c.parallelStarters <- struct{}{}:
		defer func() { <-c.parallelStarters }()
	default:
		// Wait if at capacity
		c.parallelStarters <- struct{}{}
		defer func() { <-c.parallelStarters }()
	}

	// Skip in short mode
	if testing.Short() && !c.config.background {
		c.tb.Skip("skipping container test in short mode")
	}

	// Set test-specific labels
	c.config.labels["testctr.test"] = c.tb.Name()
	c.config.labels["testctr.module"] = getModuleName()
	c.config.labels["testctr.timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)

	// Create container
	ctx := context.Background()
	if deadline, ok := c.tb.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	containerID, err := c.backend.CreateContainer(ctx, c.tb, c.image, c.config)
	if err != nil {
		c.tb.Fatalf("failed to create container: %v", err)
	}
	c.containerID = containerID

	// Log container ID
	c.logf("%s container created", c.image)

	// Start container
	if err := c.backend.StartContainer(ctx, c.containerID); err != nil {
		c.tb.Fatalf("failed to start container: %v", err)
	}

	// Wait for container to be ready
	if err := c.waitForReady(ctx); err != nil {
		// Get logs for debugging
		logs, _ := c.Logs(ctx)
		c.tb.Fatalf("container failed to become ready: %v\nLogs:\n%s", err, logs)
	}

	// Run post-start function
	if c.config.postStartFunc != nil {
		if err := c.config.postStartFunc(ctx, c); err != nil {
			c.tb.Fatalf("post-start function failed: %v", err)
		}
	}

	c.logf("%s container is running", c.image)
}

// waitForReady waits for the container to be ready based on configured wait strategies.
func (c *Container) waitForReady(ctx context.Context) error {
	// Use startup timeout
	ctx, cancel := context.WithTimeout(ctx, c.config.startupTimeout)
	defer cancel()

	// Wait for log pattern
	if c.config.waitLogPattern != "" {
		if err := c.waitForLog(ctx, c.config.waitLogPattern, c.config.waitLogTimeout); err != nil {
			return fmt.Errorf("log wait failed: %w", err)
		}
	}

	// Wait for exec command
	if len(c.config.waitExecCmd) > 0 {
		if err := c.waitForExec(ctx, c.config.waitExecCmd, c.config.waitExecTimeout); err != nil {
			return fmt.Errorf("exec wait failed: %w", err)
		}
	}

	// Wait for HTTP endpoint
	if c.config.waitHTTPPath != "" {
		if err := c.waitForHTTP(ctx, c.config.waitHTTPPath, c.config.waitHTTPExpect, c.config.waitHTTPTimeout); err != nil {
			return fmt.Errorf("HTTP wait failed: %w", err)
		}
	}

	return nil
}

// waitForLog waits for a log pattern to appear.
func (c *Container) waitForLog(ctx context.Context, pattern string, timeout time.Duration) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid log pattern: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastLogs string
	for {
		select {
		case <-ctx.Done():
			// Get final logs for error message
			finalLogs, _ := c.Logs(context.Background())
			if finalLogs != "" {
				lastLogs = finalLogs
			}
			// Include last 20 lines of output in error
			lines := strings.Split(strings.TrimSpace(lastLogs), "\n")
			if len(lines) > 20 {
				lines = lines[len(lines)-20:]
			}
			return fmt.Errorf("timeout waiting for log pattern %q after %v\nLast %d lines of output:\n%s",
				pattern, timeout, len(lines), strings.Join(lines, "\n"))
		case <-ticker.C:
			logs, err := c.Logs(ctx)
			if err != nil {
				continue
			}
			lastLogs = logs
			if re.MatchString(logs) {
				return nil
			}
		}
	}
}

// waitForExec waits for an exec command to succeed.
func (c *Container) waitForExec(ctx context.Context, cmd []string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	var lastOutput string
	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("timeout waiting for exec command %v after %v: %v (output: %s)",
					cmd, timeout, lastErr, lastOutput)
			}
			return fmt.Errorf("timeout waiting for exec command %v after %v", cmd, timeout)
		case <-ticker.C:
			exitCode, output, err := c.Exec(ctx, cmd)
			lastOutput = output
			if err != nil {
				lastErr = err
				continue
			}
			if exitCode == 0 {
				return nil
			}
			lastErr = fmt.Errorf("exit code %d", exitCode)
		}
	}
}

// waitForHTTP waits for an HTTP endpoint to be ready.
func (c *Container) waitForHTTP(ctx context.Context, path string, expectedStatus int, timeout time.Duration) error {
	// Implementation would go here - omitted for brevity
	return errors.New("HTTP wait not implemented")
}

// inspect returns container information, with caching.
func (c *Container) inspect() *backend.ContainerInfo {
	c.infoMu.Lock()
	defer c.infoMu.Unlock()

	if c.info != nil {
		return c.info
	}

	ctx := context.Background()
	if deadline, ok := c.tb.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	info, err := c.backend.InspectContainer(ctx, c.containerID)
	if err != nil {
		c.tb.Fatalf("failed to inspect container: %v", err)
	}

	c.info = info
	return info
}

// extractHostPort extracts the host port from container info.
func (c *Container) extractHostPort(containerPort string) string {
	info := c.inspect()

	// Normalize port format
	if !strings.Contains(containerPort, "/") {
		containerPort = containerPort + "/tcp"
	}

	// Check NetworkSettings.Ports
	if info.NetworkSettings.Ports != nil {
		if bindings, ok := info.NetworkSettings.Ports[containerPort]; ok && len(bindings) > 0 {
			return bindings[0].HostPort
		}
	}

	return ""
}

// cleanup stops and removes the container.
func (c *Container) cleanup() {
	if c.cleanupCalled {
		return
	}
	c.cleanupCalled = true

	// Skip cleanup for shared containers
	if c.coordinator != nil {
		return
	}

	// Check if we should keep the container
	if *keepAlive {
		c.logf("keeping container alive (--testctr.keep-alive flag)")
		return
	}

	if *keepFailed && c.tb.Failed() {
		c.logf("keeping failed test container (--testctr.keep-failed flag)")
		return
	}

	// Remove container
	ctx := context.Background()
	if err := c.backend.RemoveContainer(ctx, c.containerID); err != nil {
		c.logf("failed to remove container: %v", err)
	} else {
		c.logf("container removed")
	}
}

// logf logs a message with the container ID prefix.
func (c *Container) logf(format string, args ...interface{}) {
	if c.config.logFilter != nil && !c.config.logFilter(fmt.Sprintf(format, args...)) {
		return
	}

	prefix := fmt.Sprintf("testctr: [%s] %s", c.image, minID(c.containerID))
	if !*verbose {
		return
	}
	c.tb.Logf("%s %s", prefix, fmt.Sprintf(format, args...))
}

// contextWithTimeout creates a context with the test's deadline or a default timeout.
func (c *Container) contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := c.tb.Deadline(); ok {
		return context.WithDeadline(ctx, deadline)
	}
	return context.WithTimeout(ctx, 5*time.Minute)
}

// runtime returns the container runtime name.
func (c *Container) runtime() string {
	return c.config.runtime
}

// config returns the container configuration (for advanced use).
func (c *Container) config() *containerConfig {
	return c.config
}

// ==============================
// Container Coordinator
// ==============================

var (
	coordinatorsMu sync.Mutex
	coordinators   = make(map[string]*containerCoordinator)
)

// containerCoordinator manages shared containers across tests.
type containerCoordinator struct {
	name       string
	mu         sync.Mutex
	containers map[string]*Container
}

func getOrCreateCoordinator(name string) *containerCoordinator {
	coordinatorsMu.Lock()
	defer coordinatorsMu.Unlock()

	if c, ok := coordinators[name]; ok {
		return c
	}

	c := &containerCoordinator{
		name:       name,
		containers: make(map[string]*Container),
	}
	coordinators[name] = c
	return c
}

func (cc *containerCoordinator) getOrCreate(key string, create func() *Container) *Container {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if c, ok := cc.containers[key]; ok {
		return c
	}

	c := create()
	cc.containers[key] = c
	return c
}

// ==============================
// Database Support
// ==============================

// sanitizeDBName converts a test name into a valid database name.
// It handles subtest paths and ensures the result is a valid identifier.
func sanitizeDBName(testName string) string {
	// Handle subtest paths: TestFoo/subtest/deepersubtest -> testfoo_subtest_deepersubtest
	name := strings.ToLower(testName)
	name = strings.ReplaceAll(name, "/", "_")

	// Replace any non-alphanumeric characters with underscores
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}

	sanitized := result.String()

	// Ensure it starts with a letter
	if len(sanitized) > 0 && sanitized[0] >= '0' && sanitized[0] <= '9' {
		sanitized = "test" + sanitized
	}

	// Ensure non-empty
	if sanitized == "" {
		sanitized = "testdb"
	}

	// Truncate if too long (PostgreSQL has 63 char limit, MySQL has 64)
	if len(sanitized) > 63 {
		// Create a hash suffix to ensure uniqueness
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(testName)))[:8]
		sanitized = sanitized[:54] + "_" + hash // 54 + 1 + 8 = 63
	}

	return sanitized
}

// attemptCreateDatabase is a helper for database creation with retries.
func attemptCreateDatabase(ctx context.Context, c *Container, dbName string, createFunc func() error, retries int) error {
	var lastErr error
	
	for i := 1; i <= retries; i++ {
		c.logf("Attempting to create database %s (attempt %d)", dbName, i)
		
		if err := createFunc(); err != nil {
			lastErr = err
			if i < retries {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(i) * time.Second):
					continue
				}
			}
		} else {
			return nil
		}
	}
	
	return fmt.Errorf("failed to create database after %d attempts: %w", retries, lastErr)
}

// ==============================
// File Operations
// ==============================

// CopyFile copies a file from the host to the container.
func CopyFile(src, dst string) Option {
	return OptionFunc(func(v interface{}) {
		if cfg, ok := v.(*containerConfig); ok {
			cfg.files = append(cfg.files, fileEntry{
				Source: src,
				Target: dst,
				Mode:   0644,
			})
		}
	})
}

// CopyFileWithMode copies a file with specific permissions.
func CopyFileWithMode(src, dst string, mode os.FileMode) Option {
	return OptionFunc(func(v interface{}) {
		if cfg, ok := v.(*containerConfig); ok {
			cfg.files = append(cfg.files, fileEntry{
				Source: src,
				Target: dst,
				Mode:   mode,
			})
		}
	})
}

// CopyContent copies content from a reader to a file in the container.
func CopyContent(content io.Reader, dst string, mode os.FileMode) Option {
	return OptionFunc(func(v interface{}) {
		if cfg, ok := v.(*containerConfig); ok {
			cfg.files = append(cfg.files, fileEntry{
				Source: content,
				Target: dst,
				Mode:   mode,
			})
		}
	})
}

// CopyString copies a string to a file in the container.
func CopyString(content, dst string, mode os.FileMode) Option {
	return CopyContent(strings.NewReader(content), dst, mode)
}

// ==============================
// Command-line Flags
// ==============================

var (
	// Command-line flags
	verbose      = flag.Bool("testctr.verbose", false, "Enable verbose logging for testctr")
	keepAlive    = flag.Bool("testctr.keep-alive", false, "Keep containers alive after tests")
	keepFailed   = flag.Bool("testctr.keep-failed", false, "Keep containers from failed tests")
	autoParallel = flag.Bool("testctr.auto-parallel", parseBool(os.Getenv("TESTCTR_AUTO_PARALLEL"), false), "Automatically call t.Parallel() if not already parallel")
	cleanupOld   = flag.Bool("testctr.cleanup-old", parseBool(os.Getenv("TESTCTR_CLEANUP_OLD"), false), "Clean up old testctr containers on startup")
	defaultImage = flag.String("testctr.default-image", getEnvWithDefault("TESTCTR_DEFAULT_IMAGE", "alpine:latest"), "Default image for containers")

	// Parallel start limiting
	maxParallelStarts = getMaxParallelStarts()
)

func init() {
	// Clean up old containers if requested
	if *cleanupOld || os.Getenv("TESTCTR_CLEANUP_OLD") == "1" {
		cleanupOldContainers()
	} else {
		// Just warn about old containers
		warnOldContainers()
	}
}

// cleanupOldContainers removes old testctr containers from previous test runs.
func cleanupOldContainers() {
	runtime := discoverContainerRuntime()
	
	// Find old containers
	cmd := exec.Command(runtime, "ps", "-a", "--filter", "label=testctr.test", "--format", "{{.ID}}\t{{.Names}}\t{{.CreatedAt}}\t{{.Labels}}")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	var oldContainers []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) < 4 {
			continue
		}

		// Check if container is old (> 1 hour)
		if isOldContainer(parts[2]) {
			oldContainers = append(oldContainers, parts[0])
		}
	}

	if len(oldContainers) > 0 {
		// Remove old containers
		args := append([]string{"rm", "-f"}, oldContainers...)
		cmd := exec.Command(runtime, args...)
		if err := cmd.Run(); err == nil {
			log.Printf("testctr: Cleaned up %d old containers", len(oldContainers))
		}
	}
}

// warnOldContainers warns about old testctr containers but doesn't remove them.
func warnOldContainers() {
	runtime := discoverContainerRuntime()
	moduleName := getModuleName()
	
	// Find containers from this module
	cmd := exec.Command(runtime, "ps", "-a", 
		"--filter", fmt.Sprintf("label=testctr.module=%s", moduleName),
		"--format", "{{.Names}} (ID: {{.ID}}) created {{.CreatedAt}} (runtime: %s)", runtime)
	
	output, err := cmd.Output()
	if err != nil {
		return
	}

	// Count old containers
	var oldCount int
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var oldContainers []string
	
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		
		// Extract created time
		if idx := strings.Index(line, "created "); idx >= 0 {
			createdStr := line[idx+8:]
			if endIdx := strings.Index(createdStr, " (runtime:"); endIdx > 0 {
				createdStr = createdStr[:endIdx]
			}
			
			if isOldContainer(createdStr) {
				oldCount++
				if len(oldContainers) < 5 { // Show first 5
					oldContainers = append(oldContainers, line)
				}
			}
		}
	}

	if oldCount > 0 {
		// Log warnings
		for _, container := range oldContainers {
			log.Printf("WARNING: Found old testctr container %s. Run tests with -testctr.cleanup-old flag to remove old containers.", container)
		}
		if oldCount > len(oldContainers) {
			log.Printf("WARNING: ... and %d more old containers", oldCount-len(oldContainers))
		}
		
		// Clean them up
		log.Printf("Cleaned up %d old testctr containers from this module (runtime: %s).", oldCount, runtime)
		cmd := exec.Command(runtime, "rm", "-f")
		cmd.Args = append(cmd.Args, oldContainers...)
		cmd.Run()
	}
}

// isOldContainer checks if a container is older than 1 hour.
func isOldContainer(createdAt string) bool {
	// Parse various time formats Docker might use
	createdAt = strings.TrimSpace(createdAt)
	
	// Handle relative times like "2 hours ago"
	if strings.Contains(createdAt, "ago") {
		parts := strings.Fields(createdAt)
		if len(parts) >= 2 {
			value, err := strconv.Atoi(parts[0])
			if err != nil {
				return false
			}
			
			unit := parts[1]
			switch {
			case strings.HasPrefix(unit, "hour"):
				return value >= 1
			case strings.HasPrefix(unit, "day"), strings.HasPrefix(unit, "week"), strings.HasPrefix(unit, "month"):
				return true
			case strings.HasPrefix(unit, "minute"):
				return value >= 60
			}
		}
		return false
	}
	
	// Try parsing as absolute time
	formats := []string{
		"2006-01-02 15:04:05 -0700 MST",
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02T15:04:05Z",
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, createdAt); err == nil {
			return time.Since(t) > time.Hour
		}
	}
	
	return false
}

// ==============================
// Utility Functions
// ==============================

// discoverContainerRuntime finds the available container runtime.
func discoverContainerRuntime() string {
	// Check environment variable first
	if runtime := os.Getenv("TESTCTR_RUNTIME"); runtime != "" {
		return runtime
	}

	// Try common runtimes in order
	runtimes := []string{"docker", "podman", "nerdctl"}
	for _, runtime := range runtimes {
		if _, err := exec.LookPath(runtime); err == nil {
			return runtime
		}
	}

	// Default to docker
	return "docker"
}

// getModuleName returns the current Go module name.
func getModuleName() string {
	if modName := os.Getenv("TESTCTR_MODULE"); modName != "" {
		return modName
	}

	// Try to read go.mod
	data, err := os.ReadFile("go.mod")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "module "))
			}
		}
	}

	// Fallback to working directory
	if wd, err := os.Getwd(); err == nil {
		return filepath.Base(wd)
	}

	return "unknown"
}

// getMaxParallelStarts returns the maximum number of parallel container starts.
func getMaxParallelStarts() int {
	if s := os.Getenv("TESTCTR_MAX_PARALLEL_STARTS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	// Default to number of CPUs
	if n := runtime.NumCPU(); n > 4 {
		return 4
	} else {
		return n
	}
}

// parseBool parses a boolean string.
func parseBool(s string, defaultValue bool) bool {
	if s == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return defaultValue
	}
	return b
}

// getEnvWithDefault returns an environment variable or default value.
func getEnvWithDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// minID returns a shortened container ID for logging.
func minID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// isTestRunningInParallel checks if the test is already running in parallel.
func isTestRunningInParallel(tb testing.TB) bool {
	// This is a heuristic - there's no direct way to check
	// We assume if GOMAXPROCS > 1 and we're not in verbose mode, it's parallel
	return runtime.GOMAXPROCS(0) > 1 && !testing.Verbose()
}

// ==============================
// Internal Option Setters
// ==============================

// The following methods connect options from ctropts to the internal config.
// They are not part of the public API but allow ctropts to configure containers.

// SetWaitLogPattern sets the log pattern to wait for during startup.
func (c *containerConfig) SetWaitLogPattern(pattern string) {
	c.waitLogPattern = pattern
}

// SetWaitLogTimeout sets the timeout for log pattern waiting.
func (c *containerConfig) SetWaitLogTimeout(timeout time.Duration) {
	c.waitLogTimeout = timeout
}

// SetWaitExecCommand sets the exec command to wait for during startup.
func (c *containerConfig) SetWaitExecCommand(cmd []string) {
	c.waitExecCmd = cmd
}

// SetWaitExecTimeout sets the timeout for exec command waiting.
func (c *containerConfig) SetWaitExecTimeout(timeout time.Duration) {
	c.waitExecTimeout = timeout
}

// SetWaitHTTPPath sets the HTTP path to wait for during startup.
func (c *containerConfig) SetWaitHTTPPath(path string) {
	c.waitHTTPPath = path
}

// SetWaitHTTPTimeout sets the timeout for HTTP waiting.
func (c *containerConfig) SetWaitHTTPTimeout(timeout time.Duration) {
	c.waitHTTPTimeout = timeout
}

// SetWaitHTTPExpect sets the expected HTTP status code.
func (c *containerConfig) SetWaitHTTPExpect(status int) {
	c.waitHTTPExpect = status
}

// SetStartupTimeout sets the overall startup timeout.
func (c *containerConfig) SetStartupTimeout(timeout time.Duration) {
	c.startupTimeout = timeout
}

// SetNetwork sets the network mode.
func (c *containerConfig) SetNetwork(network string) {
	c.networkMode = network
}

// SetUser sets the user to run as.
func (c *containerConfig) SetUser(user string) {
	c.user = user
}

// SetWorkingDir sets the working directory.
func (c *containerConfig) SetWorkingDir(dir string) {
	c.workingDir = dir
}

// SetMemory sets the memory limit.
func (c *containerConfig) SetMemory(memory string) {
	c.memory = memory
}

// SetCPUs sets the CPU limit.
func (c *containerConfig) SetCPUs(cpus string) {
	c.cpus = cpus
}

// SetPrivileged sets privileged mode.
func (c *containerConfig) SetPrivileged(privileged bool) {
	c.privileged = privileged
}

// SetAutoRemove sets auto-remove mode.
func (c *containerConfig) SetAutoRemove(autoRemove bool) {
	c.autoRemove = autoRemove
}

// SetReuse sets container reuse mode.
func (c *containerConfig) SetReuse(reuse bool) {
	c.reuse = reuse
}

// SetLogs enables log output.
func (c *containerConfig) SetLogs(logs bool) {
	c.logs = logs
}

// AddLabel adds a container label.
func (c *containerConfig) AddLabel(key, value string) {
	if c.labels == nil {
		c.labels = make(map[string]string)
	}
	c.labels[key] = value
}

// AddVolume adds a volume mount.
func (c *containerConfig) AddVolume(volume string) {
	c.volumes = append(c.volumes, volume)
}

// AddTmpfs adds a tmpfs mount.
func (c *containerConfig) AddTmpfs(path string, opts string) {
	if c.tmpfs == nil {
		c.tmpfs = make(map[string]string)
	}
	c.tmpfs[path] = opts
}

// AddCapability adds a Linux capability.
func (c *containerConfig) AddCapability(cap string) {
	c.capAdd = append(c.capAdd, cap)
}

// DropCapability drops a Linux capability.
func (c *containerConfig) DropCapability(cap string) {
	c.capDrop = append(c.capDrop, cap)
}

// SetPullPolicy sets the image pull policy.
func (c *containerConfig) SetPullPolicy(policy string) {
	c.pullPolicy = policy
}

// SetPostStartFunc sets a function to run after container start.
func (c *containerConfig) SetPostStartFunc(fn func(context.Context, *Container) error) {
	c.postStartFunc = fn
}

// SetBackendOptions sets backend-specific options.
func (c *containerConfig) SetBackendOptions(opts interface{}) {
	c.backendOpts = opts
}

// SetDSNProvider sets the DSN provider for database containers.
func (c *containerConfig) SetDSNProvider(provider DSNProvider) {
	c.dsnProvider = provider
}

// SetShared enables container sharing with the given coordinator name.
func (c *containerConfig) SetShared(name string) {
	c.shared = name
}

// SetLogFilter sets the log filter function.
func (c *containerConfig) SetLogFilter(filter func(string) bool) {
	c.logFilter = filter
}

// ==============================
// Random Names (for container naming)
// ==============================

var (
	// Adjectives for generating random container names
	adjectives = []string{
		"admiring", "adoring", "agitated", "amazing", "angry", "awesome", "beautiful",
		"blissful", "bold", "boring", "brave", "busy", "charming", "clever", "cool",
		"compassionate", "competent", "confident", "cranky", "crazy", "dazzling",
		"determined", "distracted", "dreamy", "eager", "ecstatic", "elastic", "elated",
		"elegant", "eloquent", "epic", "exciting", "fervent", "festive", "flamboyant",
		"focused", "friendly", "frosty", "funny", "gallant", "gifted", "goofy",
		"gracious", "great", "happy", "hardcore", "heuristic", "hopeful", "hungry",
		"infallible", "inspiring", "intelligent", "interesting", "jolly", "jovial",
		"keen", "kind", "laughing", "loving", "lucid", "magical", "modest", "musing",
		"mystifying", "naughty", "nervous", "nice", "nifty", "nostalgic", "objective",
		"optimistic", "peaceful", "pedantic", "pensive", "practical", "priceless",
		"quirky", "recursing", "relaxed", "reverent", "romantic", "sad", "serene",
		"sharp", "silly", "sleepy", "stoic", "strange", "stupefied", "suspicious",
		"sweet", "tender", "thirsty", "trusting", "unruffled", "upbeat", "vibrant",
		"vigilant", "vigorous", "wizardly", "wonderful", "xenodochial", "youthful",
		"zealous", "zen",
	}

	// Names for generating random container names
	names = []string{
		"albattani", "allen", "almeida", "antonelli", "archimedes", "ardinghelli",
		"aryabhata", "austin", "babbage", "banach", "bardeen", "bartik", "bassi",
		"beaver", "bell", "benz", "bhabha", "bhaskara", "black", "blackburn",
		"blackwell", "bohr", "booth", "borg", "bose", "boyd", "brahmagupta", "brattain",
		"brown", "burnell", "cannon", "carson", "cartwright", "cerf", "chandrasekhar",
		"chaplygin", "chatelet", "chatterjee", "chebyshev", "cohen", "colden", "cook",
		"coulomb", "curie", "darwin", "davinci", "dewdney", "dhawan", "diffie",
		"dijkstra", "dirac", "driscoll", "dubinsky", "easley", "edison", "einstein",
		"elbakyan", "elgamal", "elion", "ellis", "engelbart", "euclid", "euler",
		"faraday", "feistel", "fermat", "fermi", "feynman", "franklin", "gagarin",
		"galileo", "galois", "ganguly", "gates", "gauss", "germain", "goldberg",
		"goldstine", "goldwasser", "golick", "goodall", "gould", "greider", "grothendieck",
		"haibt", "hamilton", "haslett", "hawking", "heisenberg", "hermann", "herschel",
		"hertz", "heyrovsky", "hodgkin", "hofstadter", "hoover", "hopper", "hugle",
		"hypatia", "ishizaka", "jackson", "jang", "jennings", "jepsen", "johnson",
		"joliot", "jones", "kalam", "kapitsa", "kare", "keldysh", "keller", "kepler",
		"khayyam", "khorana", "kilby", "kirch", "knuth", "kowalevski", "lalande",
		"lamarr", "lamport", "leakey", "leavitt", "lederberg", "lehmann", "lewin",
		"lichterman", "liskov", "lovelace", "lumiere", "mahavira", "margulis", "matsumoto",
		"maxwell", "mayer", "mccarthy", "mcclintock", "mclaren", "mclean", "mcnulty",
		"mendel", "mendeleev", "meitner", "meninsky", "merkle", "mestorf", "mirzakhani",
		"montalcini", "moore", "morse", "murdock", "moser", "napier", "nash", "neumann",
		"newton", "nightingale", "nobel", "noether", "northcutt", "noyce", "panini",
		"pare", "pascal", "pasteur", "payne", "perlman", "pike", "poincare", "poitras",
		"proskuriakova", "ptolemy", "raman", "ramanujan", "ride", "ritchie", "rhodes",
		"robinson", "roentgen", "rosalind", "rubin", "saha", "sammet", "sanderson",
		"satoshi", "shamir", "shannon", "shaw", "shirley", "shockley", "shtern",
		"sinoussi", "snyder", "solomon", "spence", "stonebraker", "sutherland",
		"swanson", "swartz", "swirles", "taussig", "tereshkova", "tesla", "tharp",
		"thompson", "torvalds", "tu", "turing", "varahamihira", "vaughan", "visvesvaraya",
		"volhard", "villani", "wescoff", "wilbur", "wiles", "williams", "williamson",
		"wilson", "wing", "wozniak", "wright", "wu", "yalow", "yonath", "zhukovsky",
	}
)

// generateRandomName creates a random container name.
func generateRandomName() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	name := names[rand.Intn(len(names))]
	return fmt.Sprintf("%s_%s", adj, name)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}