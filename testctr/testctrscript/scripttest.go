package testctrscript

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr"
	"golang.org/x/tools/txtar"
	"rsc.io/script"
)

var (
	cleanupImages  = flag.Bool("testctr.cleanup-images", true, "Clean up testctr script-built images older than cleanup-age before test run.")
	warnOldImages  = flag.Bool("testctr.warn-images", true, "Warn about testctr script-built images older than cleanup-age.")
	cleanupOrphans = flag.Bool("testctr.cleanup-orphans", true, "Clean up orphaned testctr images (not used by any containers).")
	imageCheckOnce sync.Once
)

func TestWithContainer(t *testing.T, ctx context.Context, engine *script.Engine, pattern string, opts ...ContainerOption) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC in TestWithContainer: %v", r)
		}
	}()
	checkOldImages(t)
	config := &containerConfig{image: "ubuntu:latest"}
	for _, opt := range // Package testctrscript provides rsc.io/script/scripttest compatible commands
	// for testing with testctr containers.
	// Additional flags specific to testctrscript image management
	// imageCheckOnce ensures we only check for old images once per test run
	// TestWithContainer runs script tests inside a container, mirroring scripttest.Test().
	// It follows the same pattern as scripttest.Test() but executes the test scripts
	// within a containerized environment.
	//
	// Parameters:
	//   - t: The testing.T instance
	//   - ctx: Context for cancellation and timeouts
	//   - engine: Script engine with commands and conditions
	//   - pattern: File pattern to match test files (e.g., "testdata/*.txt")
	//   - opts: Container options (WithImage, WithEnv)
	//
	// Behavior:
	//   - If WithImage is not specified, defaults to "ubuntu:latest"
	//   - If a test contains a Dockerfile, it will be built and used instead of the base image
	//   - All txtar archive contents are cleanly transferred to containers
	//
	// Usage:
	//
	//	func TestMyContainerizedScripts(t *testing.T) {
	//	    TestWithContainer(t, context.Background(),
	//	        &script.Engine{
	//	            Cmds:  DefaultCmds(t),
	//	            Conds: DefaultConds(),
	//	        },
	//	        "testdata/*.txt",
	//	        WithImage("alpine:latest"),
	//	        WithEnv("DEBUG=1"))
	//	}
	//
	//	// Or with default image:
	//	func TestWithDefaultImage(t *testing.T) {
	//	    TestWithContainer(t, context.Background(), engine, "testdata/*.txt")
	//	}
	// Panic recovery for the entire test container setup
	// Check for old images (like the root package does for containers)
	// Apply configuration options
	// default
	// Log if using default image
	// Set up timeout handling like scripttest.Test()
	// If time allows, increase the termination grace period to 5% of the
	// remaining time.
	// Reserve grace periods for cleanup
	// Find test files matching the pattern
	// Run each test file
	// Parse the txtar file
	// Check if there's a Dockerfile in the archive
	// Create a container for running this specific test
	// Keep container alive
	// Add Docker-in-Docker support if requested
	// Set up the container environment
	// Run the script test in the container
	// getLabelPrefix returns the configured label prefix for containers
	// Look up the flag from the root testctr package
	// fallback default
	// sanitizeImageName ensures the value is valid for a Docker image name.
	// This uses the same logic as the root testctr package's sanitizeLabelValue.
	// Colons can be problematic
	// Docker image names should be lowercase
	// sanitizeLabelValue ensures the value is valid for a Docker label.
	// This mirrors the function from the root testctr package.
	// Colons can be problematic
	// imageLabels returns the labels to apply to a built image (similar to container labels).
	// These labels help identify images managed by testctr.
	// Base label to identify all testctr images
	// Distinguish from regular containers
	// shouldCleanupImages determines if images should be cleaned up based on flags.
	// Images are cleaned up by default unless explicitly disabled.
	// Check for testctr.keep-failed flag
	// Use the testctr.cleanup-images flag
	// checkOldImages warns about or cleans up old images based on flags.
	// This mirrors the container cleanup logic from the root package.
	// Also clean up orphaned images if cleanup is enabled
	// doCheckOldImages performs the actual checking and cleanup of old images using Docker CLI.
	// List images with our label
	// Get cleanup age threshold
	// default
	// cleanupOrphanedImages removes images that are no longer used by any containers
	// Get all testctr images
	// Get all containers (including stopped ones)
	// Create set of images in use by containers
	// Find orphaned images
	// Check if image is used by any container
	// Get image names/tags for this ID
	// Parse repo tags and check if any are in use
	// Also check by image ID
	// Clean up orphaned images
	// Helper function for min
	// SimpleCmd creates a testctr command for use in script tests.
	// The returned command manages containers using the provided testing.T.
	// exec needs to return a WaitFunc to capture stdout
	// port writes directly to stdout
	// endpoint writes directly to stdout
	// containerRegistry is shared between commands and conditions
	// getRegistry returns the shared container registry for a test
	// Use test cleanup to ensure we get a fresh registry per test
	// Store in global map keyed by test name
	// Global registry map (simplified approach for demo)
	// Panic recovery for container start operations
	// Check if already exists
	// Parse options
	// Collect remaining args as command
	// Return a WaitFunc that will start the container asynchronously
	// Create container
	// Synchronous start (default behavior)
	// Return WaitFunc that executes the command and returns stdout
	// Put output in stderr if command failed
	// Return output as stdout for script matching
	// Return WaitFunc that returns the port as stdout
	// Return WaitFunc that returns the endpoint as stdout
	// Container is already ready (testctr.New waits)
	// DefaultCmds returns the default set of commands for script tests with testctr.
	// This includes the standard rsc.io/script commands plus testctr container management.
	// DefaultConds returns the default set of conditions for script tests with testctr.
	// This includes the standard rsc.io/script conditions plus testctr container conditions.
	// Add dynamic container conditions
	// Since we can't predict container names, we'll use a generic approach
	// ContainerCond returns a condition that checks if a container exists and is running.
	// Usage:
	//
	//	[container name]    # Container exists and is running
	//	[!container name]   # Container does not exist or is not running
	// Check all registries for this container (simplified approach)
	// Container doesn't exist
	// ContainerCondForName returns a condition for a specific container name.
	// This is used for dynamic registration of container conditions.
	// Check all registries for this specific container
	// ContainerOption configures TestWithContainer behavior
	// Additional docker build arguments
	// WithImage specifies the base container image to use
	// WithEnv adds environment variables to the container test
	// WithDockerInDocker enables Docker-in-Docker support by mounting the host Docker socket
	// WithBuildArgs adds additional arguments to docker build command
	// Examples:
	//   WithBuildArgs("--platform=linux/amd64")          // Set platform
	//   WithBuildArgs("--build-arg", "VERSION=1.0")      // Build arguments
	//   WithBuildArgs("--builder=mybuilder")             // Use specific builder (buildx)
	//   WithBuildArgs("--cache-from=type=gha")           // GitHub Actions cache
	// WithBuildx enables Docker buildx for advanced build features
	// WithPlatform sets the target platform for multi-platform builds
	// setupContainerEnvironment prepares the container with necessary tools and directories
	// Create working directories
	// Install basic tools if needed (for alpine)
	// runScriptInContainer executes a single script test inside the container
	// Panic recovery for script execution
	// Create unique workspace for this test
	// Create workspace in container
	// Extract files from archive into container
	// Set up environment in container
	// Create a script that sets up environment and runs commands
	// Wrap the script content with environment setup
	// Write script to container
	// Make script executable and run it
	// Execute the script in the container
	// Cleanup workspace
	// extractFilesToContainer copies files from txtar archive to container
	// Create directory for file if needed
	// Write file content to container
	// writeFileToContainer writes data to a file in the container
	// For simple text content, use cat with here-doc (most efficient)
	// Limit size for here-doc
	// For binary or large content, use base64 encoding for safe transfer
	// writeScriptToContainer writes a script file to the container
	// isPrintableText checks if content is safe to transfer as text
	// setupDockerInDocker configures Docker-in-Docker support
	// Check if Docker is available and get context info
	// Future: Add volume mounting support
	// *containerOpts = append(*containerOpts, testctr.WithVolume("/var/run/docker.sock:/var/run/docker.sock"))
	// detectDockerHost attempts to detect the Docker host from various sources
	// Check DOCKER_HOST environment variable first
	// Try to get Docker context information
	// Default Unix socket if nothing else found
	// hasDockerfile checks if the txtar archive contains a Dockerfile
	// buildImageFromArchive builds a Docker image from the txtar archive containing a Dockerfile
	// Panic recovery for Docker build operations
	// Create a temporary directory for the build context
	// Extract all files from the archive to the build directory
	// Create directory if needed
	// Write file content with appropriate permissions
	// Make scripts executable
	// Generate a unique image name for this test using the same naming convention as the root package
	// Build the Docker image using host docker command with labels
	// Create labels for the image
	// Run docker build on the host with labels and custom build args
	// Add any custom build arguments
	// Clean up the image when the test completes (respecting flags)
	opts {
		opt(config)
	}
	if config.image == "ubuntu:latest" && len(opts) == 0 {
		t.Logf("No container image specified, defaulting to %s", config.image)
	}
	gracePeriod := 100 * time.Millisecond
	if deadline, ok := t.Deadline(); ok {
		timeout := time.Until(deadline)
		if gp := timeout / 20; gp > gracePeriod {
			gracePeriod = gp
		}
		timeout -= 2 * gracePeriod
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		t.Cleanup(cancel)
	}
	files, _ := filepath.Glob(pattern)
	if len(files) == 0 {
		t.Fatal("no testdata")
	}
	for _, file := range files {
		file := file
		name := strings.TrimSuffix(filepath.Base(file), ".txt")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			archive, err := txtar.ParseFile(file)
			if err != nil {
				t.Fatal(err)
			}
			finalImage := config.image
			if hasDockerfile(archive) {
				builtImage := buildImageFromArchive(t, ctx, archive, name, config)
				finalImage = builtImage
			}
			var containerOpts []testctr.Option
			containerOpts = append(containerOpts, testctr.WithCommand("sleep", "3600"))
			if config.dockerInDocker {
				setupDockerInDocker(t, &containerOpts)
			}
			container := testctr.New(t, finalImage, containerOpts...)
			setupContainerEnvironment(t, container)
			runScriptInContainer(t, ctx, engine, config.env, container, file, archive)
		})
	}
}

func getLabelPrefix() string {
	labelFlag := flag.Lookup("testctr.label")
	if labelFlag != nil {
		return labelFlag.Value.String()
	}
	return "testctr"
}

func sanitizeImageName(value string) string {
	sanitized := strings.ReplaceAll(value, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ToLower(sanitized)
	const maxImageNameLength = 63
	if len(sanitized) > maxImageNameLength {
		sanitized = sanitized[:maxImageNameLength]
	}
	return sanitized
}

func sanitizeLabelValue(value string) string {
	sanitized := strings.ReplaceAll(value, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	const maxLabelValueLength = 63
	if len(sanitized) > maxLabelValueLength {
		sanitized = sanitized[:maxLabelValueLength]
	}
	return sanitized
}

func imageLabels(t testing.TB, testName string) map[string]string {
	labelPrefix := getLabelPrefix()
	labels := map[string]string{labelPrefix: "true", labelPrefix + ".testname": sanitizeLabelValue(t.Name()), labelPrefix + ".script": sanitizeLabelValue(testName), labelPrefix + ".timestamp": time.Now().Format(time.RFC3339), labelPrefix + ".type": "script-built", labelPrefix + ".created-by": "testctrscript"}
	return labels
}

func shouldCleanupImages() bool {
	keepFailedFlag := flag.Lookup("testctr.keep-failed")
	if keepFailedFlag != nil && keepFailedFlag.Value.String() == "true" {
		return false
	}
	return *cleanupImages
}

func checkOldImages(t testing.TB) {
	if !*warnOldImages && !*cleanupImages {
		return
	}
	imageCheckOnce.Do(func() {
		doCheckOldImages(t, "docker", *cleanupImages, *warnOldImages)
		if *cleanupOrphans {
			cleanupOrphanedImages(t, "docker")
		}
	})
}

func doCheckOldImages(t testing.TB, runtime string, cleanupOld, warnOld bool) {
	labelPrefix := getLabelPrefix()
	filter := fmt.Sprintf("label=%s", labelPrefix)
	cmdImages := exec.Command(runtime, "images", "--filter", filter, "--format", "{{.ID}}")
	outputImages, errImages := cmdImages.Output()
	if errImages != nil {
		t.Logf("Failed to list images for cleanup check (runtime: %s): %v", runtime, errImages)
		return
	}
	imageIDs := strings.Fields(string(outputImages))
	if len(imageIDs) == 0 {
		return
	}
	var oldImagesToClean []string
	now := time.Now()
	cleanupAgeFlag := flag.Lookup("testctr.cleanup-age")
	cleanupAge := 5 * time.Minute
	if cleanupAgeFlag != nil {
		if parsed, err := time.ParseDuration(cleanupAgeFlag.Value.String()); err == nil {
			cleanupAge = parsed
		}
	}
	for _, id := range imageIDs {
		cmdInspect := exec.Command(runtime, "inspect", id, "--format", "{{.Created}} {{.RepoTags}} {{.Config.Labels}}")
		outputInspect, errInspect := cmdInspect.Output()
		if errInspect != nil {
			t.Logf("Failed to inspect image %s for cleanup (runtime: %s): %v", id, runtime, errInspect)
			continue
		}
		parts := strings.Fields(string(outputInspect))
		if len(parts) < 1 {
			continue
		}
		createdTimeStr := parts[0]
		var imageName string
		if len(parts) > 1 && parts[1] != "[<none>:<none>]" {
			imageName = strings.Trim(parts[1], "[]")
		} else {
			imageName = id[:min(12, len(id))]
		}
		createdTime, err := time.Parse(time.RFC3339Nano, createdTimeStr)
		if err != nil {
			t.Logf("Failed to parse creation time '%s' for image %s (runtime: %s): %v", createdTimeStr, id, runtime, err)
			continue
		}
		age := now.Sub(createdTime)
		if age > cleanupAge {
			if warnOld {
				t.Logf("WARNING: Found old testctr image %s (ID: %s) created %v ago (runtime: %s)", imageName, id[:min(12, len(id))], age.Round(time.Second), runtime)
			}
			if cleanupOld {
				oldImagesToClean = append(oldImagesToClean, id)
			}
		}
	}
	if cleanupOld && len(oldImagesToClean) > 0 {
		rmiArgs := append([]string{"rmi", "-f"}, oldImagesToClean...)
		cmdRmi := exec.Command(runtime, rmiArgs...)
		if errRmi := cmdRmi.Run(); errRmi == nil {
			t.Logf("Cleaned up %d old testctr images (runtime: %s).", len(oldImagesToClean), runtime)
		} else {
			t.Logf("Failed to clean up old testctr images (runtime: %s): %v", runtime, errRmi)
		}
	}
}

func cleanupOrphanedImages(t testing.TB, runtime string) {
	labelPrefix := getLabelPrefix()
	filter := fmt.Sprintf("label=%s", labelPrefix)
	cmdImages := exec.Command(runtime, "images", "--filter", filter, "--format", "{{.ID}}")
	outputImages, err := cmdImages.Output()
	if err != nil {
		t.Logf("Failed to list images for orphan cleanup (runtime: %s): %v", runtime, err)
		return
	}
	imageIDs := strings.Fields(string(outputImages))
	if len(imageIDs) == 0 {
		return
	}
	cmdContainers := exec.Command(runtime, "ps", "-a", "--format", "{{.Image}}")
	outputContainers, err := cmdContainers.Output()
	if err != nil {
		t.Logf("Failed to list containers for orphan cleanup (runtime: %s): %v", runtime, err)
		return
	}
	usedImages := make(map[string]bool)
	for _, line := range strings.Split(string(outputContainers), "\n") {
		if line != "" {
			usedImages[strings.TrimSpace(line)] = true
		}
	}
	var orphanedImages []string
	for _, imageID := range imageIDs {
		inUse := false
		cmdImageInfo := exec.Command(runtime, "inspect", imageID, "--format", "{{.RepoTags}}")
		outputImageInfo, err := cmdImageInfo.Output()
		if err != nil {
			continue
		}
		repoTags := strings.Trim(string(outputImageInfo), "[] \n")
		if repoTags != "<none>:<none>" {
			for _, tag := range strings.Split(repoTags, " ") {
				tag = strings.Trim(tag, " ")
				if tag != "" && usedImages[tag] {
					inUse = true
					break
				}
			}
		}
		if usedImages[imageID] || usedImages[imageID[:12]] {
			inUse = true
		}
		if !inUse {
			orphanedImages = append(orphanedImages, imageID)
		}
	}
	if len(orphanedImages) > 0 {
		rmiArgs := append([]string{"rmi", "-f"}, orphanedImages...)
		cmdRmi := exec.Command(runtime, rmiArgs...)
		if err := cmdRmi.Run(); err == nil {
			t.Logf("Cleaned up %d orphaned testctr images (runtime: %s).", len(orphanedImages), runtime)
		} else {
			t.Logf("Failed to clean up orphaned testctr images (runtime: %s): %v", runtime, err)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SimpleCmd creates a testctr command that uses environment variables for state sharing.
// This eliminates global state and allows commands and conditions to share container information.
func SimpleCmd(t *testing.T) script.Cmd {
	return SimpleCmdEnv(t)
}

// createTestctrCommand builds the actual command implementation
func createTestctrCommand(mgr *simpleManager) script.Cmd {
	return script.Command(script.CmdUsage{Summary: "manage test containers", Args: "start|stop|exec|port|endpoint|wait image-or-name [name] [args...]", Detail: []string{"The testctr command manages containers for testing.", "", "Subcommands:", "  start image name [opts...]  - Start a container", "  stop name                   - Stop a container", "  exec name cmd [args...]     - Execute a command", "  port name port              - Get the host port", "  endpoint name port          - Get the endpoint", "  wait name                   - Wait for container"}}, func(s *script.State, args ...string) (script.WaitFunc, error) {
		if len(args) < 1 {
			return nil, script.ErrUsage
		}
		switch args[0] {
		case "start":
			return mgr.start(s, args[1:])
		case "stop":
			return nil, mgr.stop(s, args[1:])
		case "exec":
			return mgr.exec(s, args[1:])
		case "port":
			return mgr.port(s, args[1:])
		case "endpoint":
			return mgr.endpoint(s, args[1:])
		case "wait":
			return mgr.wait(s, args[1:])
		default:
			return nil, fmt.Errorf("unknown subcommand: %s", args[0])
		}
	})
}

type containerRegistry struct {
	mu         sync.RWMutex
	containers map[string]*testctr.Container
}

type simpleManager struct {
	t        *testing.T
	registry *containerRegistry
}

// TestEnvironment contains commands and conditions that share container state.
// This design eliminates global state and ensures perfect test isolation.
type TestEnvironment struct {
	Cmds  map[string]script.Cmd
	Conds map[string]script.Cond
}

// NewTestEnvironment creates a test environment with shared container registry.
// Commands and conditions created by this function share the same registry,
// allowing conditions to check for containers created by commands.
//
// Usage:
//
//	env := testctrscript.NewTestEnvironment(t)
//	engine := &script.Engine{
//	    Cmds:  env.Cmds,
//	    Conds: env.Conds,
//	}
//	scripttest.Run(t, engine, "testdata/*.txt")
func NewTestEnvironment(t *testing.T) *TestEnvironment {
	// Create a registry local to this test environment
	registry := &containerRegistry{
		containers: make(map[string]*testctr.Container),
	}
	
	// Build commands with the registry
	cmds := script.DefaultCmds()
	mgr := &simpleManager{t: t, registry: registry}
	cmds["testctr"] = createTestctrCommand(mgr)
	
	
	// Build conditions with the same registry  
	conds := script.DefaultConds()
	conds["container"] = makeContainerCondition(registry)
	
	return &TestEnvironment{
		Cmds:  cmds,
		Conds: conds,
	}
}

func (m *simpleManager) start(s *script.State, args []string) (waitFunc script.WaitFunc, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC in container start: %v", r)
		}
	}()
	if len(args) < 2 {
		return nil, fmt.Errorf("start requires image and name")
	}
	image := args[0]
	name := args[1]
	m.registry.mu.RLock()
	_, exists := m.registry.containers[name]
	m.registry.mu.RUnlock()
	if exists {
		return nil, fmt.Errorf("container %q already exists", name)
	}
	var opts []testctr.Option
	var async bool
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "-p":
			if i+1 < len(args) {
				i++
				opts = append(opts, testctr.WithPort(args[i]))
			}
		case "-e":
			if i+1 < len(args) {
				i++
				parts := strings.SplitN(args[i], "=", 2)
				if len(parts) == 2 {
					opts = append(opts, testctr.WithEnv(parts[0], parts[1]))
				}
			}
		case "--async":
			async = true
		case "--cmd":
			if i+1 < len(args) {
				opts = append(opts, testctr.WithCommand(args[i+1:]...))
			}
			break
		}
	}
	if async {
		return func(s *script.State) (stdout, stderr string, err error) {
			container := testctr.New(m.t, image, opts...)
			m.registry.mu.Lock()
			m.registry.containers[name] = container
			m.registry.mu.Unlock()
			return fmt.Sprintf("started container %s asynchronously\n", name), "", nil
		}, nil
	}
	container := testctr.New(m.t, image, opts...)
	m.registry.mu.Lock()
	m.registry.containers[name] = container
	m.registry.mu.Unlock()
	s.Logf("started container %s\n", name)
	return nil, nil
}
func (m *simpleManager) stop(s *script.State, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("stop requires name")
	}
	name := args[0]
	m.registry.mu.RLock()
	_, ok := m.registry.containers[name]
	m.registry.mu.RUnlock()
	if !ok {
		return fmt.Errorf("container %s not found", name)
	}
	m.registry.mu.Lock()
	delete(m.registry.containers, name)
	m.registry.mu.Unlock()
	s.Logf("stopped container %s\n", name)
	return nil
}
func (m *simpleManager) exec(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("exec requires name and command")
	}
	name := args[0]
	m.registry.mu.RLock()
	container, ok := m.registry.containers[name]
	m.registry.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("container %s not found", name)
	}
	return func(s *script.State) (stdout, stderr string, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		exitCode, output, err := container.Exec(ctx, args[1:])
		if err != nil {
			return "", "", err
		}
		if exitCode != 0 {
			return "", output, fmt.Errorf("exit code %d", exitCode)
		}
		return output, "", nil
	}, nil
}
func (m *simpleManager) port(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("port requires name and port")
	}
	name := args[0]
	port := args[1]
	m.registry.mu.RLock()
	container, ok := m.registry.containers[name]
	m.registry.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("container %s not found", name)
	}
	return func(s *script.State) (stdout, stderr string, err error) {
		hostPort := container.Port(port)
		if hostPort == "" {
			return "", "", fmt.Errorf("port %s not mapped", port)
		}
		return hostPort + "\n", "", nil
	}, nil
}
func (m *simpleManager) endpoint(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("endpoint requires name and port")
	}
	name := args[0]
	port := args[1]
	m.registry.mu.RLock()
	container, ok := m.registry.containers[name]
	m.registry.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("container %s not found", name)
	}
	return func(s *script.State) (stdout, stderr string, err error) {
		endpoint := container.Endpoint(port)
		return endpoint + "\n", "", nil
	}, nil
}
func (m *simpleManager) wait(s *script.State, args []string) (script.WaitFunc, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("wait requires name [timeout]")
	}
	name := args[0]
	m.registry.mu.RLock()
	_, ok := m.registry.containers[name]
	m.registry.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("container %s not found", name)
	}
	return func(s *script.State) (string, string, error) {
		s.Logf("container %s ready\n", name)
		return "", "", nil
	}, nil
}

// DefaultCmds returns commands using environment variable state sharing
func DefaultCmds(t *testing.T) map[string]script.Cmd {
	cmds := DefaultCmdsEnv(t)
	return cmds
}

// DefaultConds returns conditions using environment variable state sharing
func DefaultConds() map[string]script.Cond {
	return DefaultCondsEnv()
}

// makeContainerCondition creates a container condition with access to the registry
func makeContainerCondition(registry *containerRegistry) script.Cond {
	return script.PrefixCondition("test container existence", func(s *script.State, suffix string) (bool, error) {
		if suffix == "" {
			return false, fmt.Errorf("container condition requires a container name")
		}
		name := strings.TrimSpace(suffix)
		registry.mu.RLock()
		defer registry.mu.RUnlock()
		_, exists := registry.containers[name]
		return exists, nil
	})
}

// Deprecated: ContainerCondForName functionality is now internal.
// Use NewTestEnvironment() to create conditions with proper registry access.

type ContainerOption func(*containerConfig)
type containerConfig struct {
	image          string
	env            []string
	dockerInDocker bool
	buildArgs      []string
}

func WithImage(image string) ContainerOption {
	return func(c *containerConfig) {
		c.image = image
	}
}

func WithEnv(env ...string) ContainerOption {
	return func(c *containerConfig) {
		c.env = append(c.env, env...)
	}
}

func WithDockerInDocker() ContainerOption {
	return func(c *containerConfig) {
		c.dockerInDocker = true
	}
}

func WithBuildArgs(args ...string) ContainerOption {
	return func(c *containerConfig) {
		c.buildArgs = append(c.buildArgs, args...)
	}
}

func WithBuildx() ContainerOption {
	return WithBuildArgs("--builder=default")
}

func WithPlatform(platform string) ContainerOption {
	return WithBuildArgs("--platform=" + platform)
}

func setupContainerEnvironment(t *testing.T, container *testctr.Container) {
	t.Helper()
	_, _, err := container.Exec(context.Background(), []string{"sh", "-c", "mkdir -p /tmp/testwork /tmp/scripts"})
	if err != nil {
		t.Logf("warning: failed to create directories: %v", err)
	}
	_, _, err = container.Exec(context.Background(), []string{"sh", "-c", "command -v wget >/dev/null || (apk update && apk add --no-cache wget curl)"})
	if err != nil {
		t.Logf("note: could not install additional tools: %v", err)
	}
}

func runScriptInContainer(t *testing.T, ctx context.Context, engine *script.Engine, env []string, container *testctr.Container, filename string, archive *txtar.Archive) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC in runScriptInContainer: %v", r)
		}
	}()
	workspaceID := fmt.Sprintf("test_%s_%d", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	workdir := fmt.Sprintf("/tmp/testwork/%s", workspaceID)
	_, _, err := container.Exec(ctx, []string{"mkdir", "-p", workdir})
	if err != nil {
		t.Fatalf("failed to create workspace in container: %v", err)
	}
	extractFilesToContainer(t, ctx, container, workdir, archive)
	envSetup := "export WORK=" + workdir
	for _, envVar := range env {
		envSetup += " && export " + envVar
	}
	scriptContent := string(archive.Comment)
	wrappedScript := fmt.Sprintf(`#!/bin/sh
set -e
cd %s
%s

# Original script content:
%s
`, workdir, envSetup, scriptContent)
	scriptPath := workdir + "/test.sh"
	writeScriptToContainer(t, ctx, container, scriptPath, wrappedScript)
	_, _, err = container.Exec(ctx, []string{"chmod", "+x", scriptPath})
	if err != nil {
		t.Fatalf("failed to make script executable: %v", err)
	}
	t.Logf("Running script %s in container at %s", filename, workdir)
	exitCode, output, err := container.Exec(ctx, []string{"sh", scriptPath})
	if err != nil {
		t.Fatalf("failed to execute script in container: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("script failed with exit code %d\nOutput:\n%s", exitCode, output)
	} else {
		t.Logf("Script completed successfully\nOutput:\n%s", output)
	}
	_, _, _ = container.Exec(context.Background(), []string{"rm", "-rf", workdir})
}

func extractFilesToContainer(t *testing.T, ctx context.Context, container *testctr.Container, workdir string, archive *txtar.Archive) {
	t.Helper()
	t.Logf("Extracting %d files from txtar archive to container workspace %s", len(archive.Files), workdir)
	for _, file := range archive.Files {
		targetPath := workdir + "/" + file.Name
		dir := filepath.Dir(targetPath)
		if dir != workdir {
			exitCode, output, err := container.Exec(ctx, []string{"mkdir", "-p", dir})
			if err != nil || exitCode != 0 {
				t.Logf("warning: failed to create directory %s (exit: %d, err: %v): %s", dir, exitCode, err, output)
				continue
			}
		}
		writeFileToContainer(t, ctx, container, targetPath, file.Data)
		t.Logf("Extracted to container: %s (%d bytes)", file.Name, len(file.Data))
	}
}

func writeFileToContainer(t *testing.T, ctx context.Context, container *testctr.Container, path string, data []byte) {
	t.Helper()
	content := string(data)
	if isPrintableText(content) && len(content) < 32768 {
		exitCode, output, err := container.Exec(ctx, []string{"sh", "-c", fmt.Sprintf(`cat > %s << 'EOF'
%s
EOF`, path, content)})
		if err != nil || exitCode != 0 {
			t.Logf("warning: failed to write text file %s (exit: %d, err: %v): %s", path, exitCode, err, output)
		}
	} else {
		encoded := base64.StdEncoding.EncodeToString(data)
		exitCode, output, err := container.Exec(ctx, []string{"sh", "-c", fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, path)})
		if err != nil || exitCode != 0 {
			t.Logf("warning: failed to write binary/large file %s (exit: %d, err: %v): %s", path, exitCode, err, output)
		} else {
			t.Logf("Successfully transferred %s using base64 encoding (%d bytes)", path, len(data))
		}
	}
}

func writeScriptToContainer(t *testing.T, ctx context.Context, container *testctr.Container, path, content string) {
	t.Helper()
	_, _, err := container.Exec(ctx, []string{"sh", "-c", fmt.Sprintf(`cat > %s << 'SCRIPT_EOF'
%s
SCRIPT_EOF`, path, content)})
	if err != nil {
		t.Fatalf("failed to write script to %s: %v", path, err)
	}
}

func isPrintableText(s string) bool {
	for _, r := range s {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

func setupDockerInDocker(t *testing.T, containerOpts *[]testctr.Option) {
	t.Helper()
	dockerHost := detectDockerHost(t)
	if dockerHost != "" {
		t.Logf("Docker-in-Docker: Detected Docker host: %s", dockerHost)
		t.Logf("Docker-in-Docker: Would mount Docker socket for access")
	} else {
		t.Logf("Docker-in-Docker: No Docker context detected, running isolated")
	}
}

func detectDockerHost(t *testing.T) string {
	t.Helper()
	if dockerHost := os.Getenv("DOCKER_HOST"); dockerHost != "" {
		t.Logf("Docker host from DOCKER_HOST: %s", dockerHost)
		return dockerHost
	}
	cmd := exec.Command("docker", "context", "inspect", "-f", "{{.Endpoints.docker.Host}}")
	output, err := cmd.Output()
	if err != nil {
		t.Logf("Docker context inspection failed: %v", err)
		return ""
	}
	dockerHost := strings.TrimSpace(string(output))
	if dockerHost != "" {
		t.Logf("Docker host from context: %s", dockerHost)
		return dockerHost
	}
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return "unix:///var/run/docker.sock"
	}
	return ""
}

func hasDockerfile(archive *txtar.Archive) bool {
	for _, file := range archive.Files {
		if file.Name == "Dockerfile" || file.Name == "dockerfile" {
			return true
		}
	}
	return false
}

func buildImageFromArchive(t *testing.T, ctx context.Context, archive *txtar.Archive, testName string, config *containerConfig) (result string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC in buildImageFromArchive: %v", r)
		}
	}()
	buildDir, err := os.MkdirTemp("", fmt.Sprintf("testctr-build-%s-", testName))
	if err != nil {
		t.Fatalf("failed to create build directory: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(buildDir)
	})
	t.Logf("Extracting %d files to build context %s", len(archive.Files), buildDir)
	for _, file := range archive.Files {
		filePath := filepath.Join(buildDir, file.Name)
		if dir := filepath.Dir(filePath); dir != buildDir {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Logf("warning: failed to create directory %s: %v", dir, err)
				continue
			}
		}
		fileMode := os.FileMode(0644)
		if filepath.Ext(file.Name) == ".sh" || file.Name == "entrypoint" {
			fileMode = 0755
		}
		if err := os.WriteFile(filePath, file.Data, fileMode); err != nil {
			t.Logf("warning: failed to write file %s: %v", filePath, err)
			continue
		}
		t.Logf("Extracted file: %s (%d bytes, mode %o)", file.Name, len(file.Data), fileMode)
	}
	labelPrefix := getLabelPrefix()
	cleanTestName := sanitizeImageName(testName)
	imageName := fmt.Sprintf("%s-%s:%d", labelPrefix, cleanTestName, time.Now().UnixNano())
	t.Logf("Building custom Docker image %s from Dockerfile...", imageName)
	labels := imageLabels(t, testName)
	var labelArgs []string
	for key, value := range labels {
		labelArgs = append(labelArgs, "--label", fmt.Sprintf("%s=%s", key, value))
	}
	buildArgs := append([]string{"build", "-t", imageName}, labelArgs...)
	if len(config.buildArgs) > 0 {
		buildArgs = append(buildArgs, config.buildArgs...)
		t.Logf("Using custom build args: %v", config.buildArgs)
	}
	buildArgs = append(buildArgs, buildDir)
	cmd := exec.CommandContext(ctx, "docker", buildArgs...)
	t.Logf("Docker build command: docker %s", strings.Join(buildArgs, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Docker build failed: %v\nOutput: %s", err, output)
		t.Logf("Falling back to alpine:latest")
		return "alpine:latest"
	}
	t.Logf("Successfully built image %s", imageName)
	t.Cleanup(func() {
		shouldCleanup := shouldCleanupImages()
		if shouldCleanup {
			cmd := exec.Command("docker", "rmi", imageName)
			if err := cmd.Run(); err == nil {
				t.Logf("Cleaned up image %s", imageName)
			} else {
				t.Logf("Failed to clean up image %s: %v", imageName, err)
			}
		} else {
			t.Logf("Keeping image %s (cleanup disabled)", imageName)
		}
	})
	return imageName
}
