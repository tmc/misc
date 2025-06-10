package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmc/misc/testctr/backend"
)

func init() {
	// Register the CLI backend
	backend.Register("local", New())
	backend.Register("cli", New()) // Alias for backwards compatibility
	backend.Register("docker", New()) // Alias for compatibility
}

// Backend implements the Backend interface using Docker/Podman CLI.
type Backend struct {
	runtime string // Cached runtime discovery
	mu      sync.Mutex
}

// New creates a new CLI backend instance.
func New() *Backend {
	return &Backend{}
}

// CreateContainer creates a new container using docker/podman CLI.
func (b *Backend) CreateContainer(t testing.TB, image string, config interface{}) (string, error) {
	t.Helper()

	runtime := b.getRuntime()
	
	// Build docker run arguments
	args := []string{"run", "-d"}
	
	// Parse config if provided
	if cfg, ok := config.(*CLIConfig); ok && cfg != nil {
		// Add labels
		for k, v := range cfg.Labels {
			args = append(args, "-l", fmt.Sprintf("%s=%s", k, v))
		}
		
		// Add environment variables
		for k, v := range cfg.Env {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
		
		// Add port mappings
		for _, port := range cfg.Ports {
			args = append(args, "-p", port)
		}
		
		// Add mounts
		for _, mount := range cfg.Mounts {
			args = append(args, "-v", mount)
		}
		
		// Add other options
		if cfg.Network != "" {
			args = append(args, "--network", cfg.Network)
		}
		if cfg.User != "" {
			args = append(args, "--user", cfg.User)
		}
		if cfg.WorkDir != "" {
			args = append(args, "--workdir", cfg.WorkDir)
		}
		if cfg.MemoryLimit != "" {
			args = append(args, "--memory", cfg.MemoryLimit)
		}
		if cfg.Privileged {
			args = append(args, "--privileged")
		}
		
		// Add image
		args = append(args, image)
		
		// Add command
		if len(cfg.Cmd) > 0 {
			args = append(args, cfg.Cmd...)
		} else {
			// No explicit command - add a default only for alpine images
			// Other images (like redis, postgres, etc.) should use their default commands
			if strings.Contains(image, "alpine") && !strings.Contains(image, "redis") && !strings.Contains(image, "postgres") && !strings.Contains(image, "mysql") {
				// Add default command to keep alpine containers running for testing
				args = append(args, "sh", "-c", "echo 'Container started' && sleep 3600")
			}
		}
	} else if testCfg := extractTestConfig(config); testCfg != nil {
		// Handle test config from testctrbackendtest package
		
		// Add environment variables
		for k, v := range testCfg.env {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
		
		// Add port mappings
		for _, port := range testCfg.ports {
			args = append(args, "-p", port)
		}
		
		// Add image
		args = append(args, image)
		
		// Add command
		if len(testCfg.cmd) > 0 {
			args = append(args, testCfg.cmd...)
		} else {
			// Add default command only for alpine containers
			if strings.Contains(image, "alpine") && !strings.Contains(image, "redis") && !strings.Contains(image, "postgres") && !strings.Contains(image, "mysql") {
				args = append(args, "sh", "-c", "echo 'Container started' && sleep 3600")
			}
		}
	} else {
		// Minimal container with just the image and a long-running command
		args = append(args, image)
		// Add a default command only for alpine containers
		if strings.Contains(image, "alpine") && !strings.Contains(image, "redis") && !strings.Contains(image, "postgres") && !strings.Contains(image, "mysql") {
			args = append(args, "sh", "-c", "echo 'Container started' && sleep 3600")
		}
	}
	
	// Execute docker run
	cmd := exec.Command(runtime, args...)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("failed to create container: %w (stderr: %s)", err, exitErr.Stderr)
		}
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	
	containerID := strings.TrimSpace(string(output))
	if containerID == "" {
		return "", fmt.Errorf("no container ID returned")
	}
	
	return containerID, nil
}

// StartContainer starts a container.
func (b *Backend) StartContainer(containerID string) error {
	runtime := b.getRuntime()
	cmd := exec.Command(runtime, "start", containerID)
	return cmd.Run()
}

// StopContainer stops a container with a timeout.
func (b *Backend) StopContainer(containerID string) error {
	runtime := b.getRuntime()
	// Use a 2-second timeout for faster cleanup
	cmd := exec.Command(runtime, "stop", "--time", "2", containerID)
	if err := cmd.Run(); err != nil {
		// Try harder with kill
		killCmd := exec.Command(runtime, "kill", containerID)
		return killCmd.Run()
	}
	return nil
}

// RemoveContainer removes a container.
func (b *Backend) RemoveContainer(containerID string) error {
	runtime := b.getRuntime()
	
	// Stop the container first if it's running
	b.StopContainer(containerID)
	
	// Now remove the container
	cmd := exec.Command(runtime, "rm", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if container doesn't exist
		outputStr := string(output)
		if strings.Contains(outputStr, "No such container") {
			return fmt.Errorf("container not found")
		}
		return err
	}
	return nil
}

// InspectContainer returns container information.
func (b *Backend) InspectContainer(containerID string) (*backend.ContainerInfo, error) {
	runtime := b.getRuntime()
	cmd := exec.Command(runtime, "inspect", containerID)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("failed to inspect container: %w (stderr: %s)", err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Parse the JSON output - Docker returns an array
	var dockerInfos []map[string]interface{}
	if err := json.Unmarshal(output, &dockerInfos); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	if len(dockerInfos) == 0 {
		return nil, fmt.Errorf("no container info found")
	}

	dockerInfo := dockerInfos[0]
	
	// Convert Docker inspect format to our ContainerInfo format
	info := &backend.ContainerInfo{}
	
	// Basic fields
	if id, ok := dockerInfo["Id"].(string); ok {
		info.ID = id
	}
	if name, ok := dockerInfo["Name"].(string); ok {
		info.Name = name
	}
	if created, ok := dockerInfo["Created"].(string); ok {
		info.Created = created
	}
	
	// State information
	if stateMap, ok := dockerInfo["State"].(map[string]interface{}); ok {
		if running, ok := stateMap["Running"].(bool); ok {
			info.State.Running = running
		}
		if status, ok := stateMap["Status"].(string); ok {
			info.State.Status = status
		}
		if exitCode, ok := stateMap["ExitCode"].(float64); ok {
			info.State.ExitCode = int(exitCode)
		}
	}
	
	// Network settings
	if networkMap, ok := dockerInfo["NetworkSettings"].(map[string]interface{}); ok {
		// Extract internal IP - try IPAddress first, then Networks.bridge.IPAddress
		if ipAddress, ok := networkMap["IPAddress"].(string); ok && ipAddress != "" {
			info.NetworkSettings.InternalIP = ipAddress
		} else if networks, ok := networkMap["Networks"].(map[string]interface{}); ok {
			if bridge, ok := networks["bridge"].(map[string]interface{}); ok {
				if ipAddress, ok := bridge["IPAddress"].(string); ok {
					info.NetworkSettings.InternalIP = ipAddress
				}
			}
		}
		
		// Extract port mappings
		if ports, ok := networkMap["Ports"].(map[string]interface{}); ok {
			info.NetworkSettings.Ports = make(map[string][]backend.PortBinding)
			for portKey, portValue := range ports {
				if bindings, ok := portValue.([]interface{}); ok {
					var portBindings []backend.PortBinding
					for _, binding := range bindings {
						if bindingMap, ok := binding.(map[string]interface{}); ok {
							pb := backend.PortBinding{}
							if hostIP, ok := bindingMap["HostIp"].(string); ok {
								pb.HostIP = hostIP
							}
							if hostPort, ok := bindingMap["HostPort"].(string); ok {
								pb.HostPort = hostPort
							}
							portBindings = append(portBindings, pb)
						}
					}
					info.NetworkSettings.Ports[portKey] = portBindings
				}
			}
		}
	}
	
	// Config (for labels)
	if configMap, ok := dockerInfo["Config"].(map[string]interface{}); ok {
		if labels, ok := configMap["Labels"].(map[string]interface{}); ok {
			info.Config.Labels = make(map[string]string)
			for k, v := range labels {
				if strValue, ok := v.(string); ok {
					info.Config.Labels[k] = strValue
				}
			}
		}
	}

	return info, nil
}

// ExecInContainer executes a command in the container.
func (b *Backend) ExecInContainer(containerID string, cmd []string) (int, string, error) {
	runtime := b.getRuntime()
	args := append([]string{"exec", containerID}, cmd...)
	command := exec.Command(runtime, args...)

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

// GetContainerLogs retrieves container logs.
func (b *Backend) GetContainerLogs(containerID string) (string, error) {
	runtime := b.getRuntime()
	cmd := exec.Command(runtime, "logs", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a known error we should ignore
		outputStr := string(output)
		if strings.Contains(outputStr, "No such container") ||
			strings.Contains(outputStr, "dead or marked for removal") {
			return "", fmt.Errorf("container not found")
		}
	}
	return string(output), err
}

// WaitForLog waits for a specific log line with context support.
func (b *Backend) WaitForLog(containerID string, logLine string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	start := time.Now()
	attempts := 0
	var lastOutput string

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Check immediately before waiting
	logs, _ := b.GetContainerLogs(containerID)
	lastOutput = logs
	attempts++
	if strings.Contains(logs, logLine) {
		return nil
	}

	// Then check periodically
	for {
		select {
		case <-ctx.Done():
			// Get fresh logs one more time
			if finalLogs, _ := b.GetContainerLogs(containerID); finalLogs != "" {
				lastOutput = finalLogs
			}

			// Include last 20 lines of output in error message
			lines := strings.Split(lastOutput, "\n")
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
			logs, _ := b.GetContainerLogs(containerID)
			lastOutput = logs
			if strings.Contains(logs, logLine) {
				return nil
			}
		}
	}
}

// InternalIP returns the container's internal IP address.
func (b *Backend) InternalIP(containerID string) (string, error) {
	info, err := b.InspectContainer(containerID)
	if err != nil {
		return "", err
	}
	return info.NetworkSettings.InternalIP, nil
}

// Commit commits the container to an image.
func (b *Backend) Commit(containerID string, imageName string) error {
	runtime := b.getRuntime()
	cmd := exec.Command(runtime, "commit", containerID, imageName)
	return cmd.Run()
}

// getRuntime returns the cached runtime or discovers it.
func (b *Backend) getRuntime() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.runtime != "" {
		return b.runtime
	}
	
	b.runtime = discoverRuntime()
	return b.runtime
}

// discoverRuntime finds which container runtime to use.
func discoverRuntime() string {
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

	// Default to docker if nothing found
	return "docker"
}

// EnsureImage ensures the image exists locally, pulling if necessary.
func (b *Backend) EnsureImage(t testing.TB, image string) error {
	t.Helper()
	
	runtime := b.getRuntime()
	
	// Check if image exists locally
	cmd := exec.Command(runtime, "image", "inspect", image)
	if err := cmd.Run(); err == nil {
		// Image exists
		return nil
	}

	// Image doesn't exist, pull it
	t.Logf("testctr: Pulling image %s (this may take a moment)...", image)

	pullCmd := exec.Command(runtime, "pull", image)
	output, err := pullCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w\nOutput: %s", image, err, output)
	}

	t.Logf("testctr: Successfully pulled image %s", image)
	return nil
}

// StreamLogs streams container logs to the test logger.
func (b *Backend) StreamLogs(ctx context.Context, containerID string, logFunc func(string)) error {
	runtime := b.getRuntime()
	cmd := exec.CommandContext(ctx, runtime, "logs", "-f", containerID)
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		// Don't error if container is already stopped
		if !strings.Contains(err.Error(), "dead") && !strings.Contains(err.Error(), "removal") {
			return fmt.Errorf("failed to start log streaming: %w", err)
		}
		return nil
	}

	// Read both stdout and stderr
	var wg sync.WaitGroup
	wg.Add(2)
	
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			logFunc(scanner.Text())
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			logFunc("[stderr] " + scanner.Text())
		}
	}()
	
	// Start a goroutine to wait for completion
	go func() {
		wg.Wait()
		cmd.Wait()
	}()
	
	return nil
}

// CLIConfig holds CLI-specific container configuration.
type CLIConfig struct {
	Labels      map[string]string
	Env         map[string]string
	Ports       []string
	Cmd         []string
	Mounts      []string
	Network     string
	User        string
	WorkDir     string
	MemoryLimit string
	Privileged  bool
	Files       []FileEntry
}

// FileEntry represents a file to copy into the container.
type FileEntry struct {
	Source interface{} // string path or io.Reader
	Target string
	Mode   os.FileMode
}

// CopyFilesToContainer copies files into a running container.
func (b *Backend) CopyFilesToContainer(containerID string, files []FileEntry, t testing.TB) error {
	t.Helper()
	runtime := b.getRuntime()
	
	for _, file := range files {
		// Create parent directory first
		dir := strings.TrimSuffix(file.Target, "/"+strings.Split(file.Target, "/")[len(strings.Split(file.Target, "/"))-1])
		if dir != "" && dir != "/" {
			mkdirCmd := exec.Command(runtime, "exec", containerID, "mkdir", "-p", dir)
			if err := mkdirCmd.Run(); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
		
		// Copy the file
		var srcPath string
		var tempFile string
		
		switch src := file.Source.(type) {
		case string:
			// Copy from file path
			srcPath = src
		case io.Reader:
			// Create temp file from reader
			tmpFile, err := os.CreateTemp("", "testctr-cp-*")
			if err != nil {
				return fmt.Errorf("failed to create temp file for reader source: %w", err)
			}
			tempFile = tmpFile.Name()
			if _, err := io.Copy(tmpFile, src); err != nil {
				tmpFile.Close()
				os.Remove(tempFile)
				return fmt.Errorf("failed to write to temp file from reader: %w", err)
			}
			if err := tmpFile.Close(); err != nil {
				os.Remove(tempFile)
				return fmt.Errorf("failed to close temp file: %w", err)
			}
			srcPath = tempFile
		case []byte:
			// Create temp file from byte content
			tmpFile, err := os.CreateTemp("", "testctr-cp-*")
			if err != nil {
				return fmt.Errorf("failed to create temp file for byte source: %w", err)
			}
			tempFile = tmpFile.Name()
			if _, err := tmpFile.Write(src); err != nil {
				tmpFile.Close()
				os.Remove(tempFile)
				return fmt.Errorf("failed to write bytes to temp file: %w", err)
			}
			if err := tmpFile.Close(); err != nil {
				os.Remove(tempFile)
				return fmt.Errorf("failed to close temp file: %w", err)
			}
			srcPath = tempFile
		default:
			return fmt.Errorf("unsupported file source type: %T", src)
		}
		
		// Clean up temp file if created
		if tempFile != "" {
			defer os.Remove(tempFile)
		}
		
		// Copy the file
		cpCmd := exec.Command(runtime, "cp", srcPath, fmt.Sprintf("%s:%s", containerID, file.Target))
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("failed to copy file %s to %s: %w", srcPath, file.Target, err)
		}
		
		// Set file permissions if specified
		if file.Mode != 0 {
			chmodCmd := exec.Command(runtime, "exec", containerID, "chmod", fmt.Sprintf("%o", file.Mode), file.Target)
			if err := chmodCmd.Run(); err != nil {
				return fmt.Errorf("failed to set permissions on %s: %w", file.Target, err)
			}
		}
	}
	
	return nil
}

// extractTestConfig extracts test config using reflection to avoid import cycles
func extractTestConfig(config interface{}) *testConfig {
	if config == nil {
		return nil
	}
	
	// Use reflection to extract fields from the test config
	// This avoids importing the testctrbackendtest package
	v := reflect.ValueOf(config)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	
	if v.Kind() != reflect.Struct {
		return nil
	}
	
	tc := &testConfig{}
	
	// Extract cmd field
	if cmdField := v.FieldByName("cmd"); cmdField.IsValid() && cmdField.Kind() == reflect.Slice {
		if cmdField.Type().Elem().Kind() == reflect.String {
			tc.cmd = make([]string, cmdField.Len())
			for i := 0; i < cmdField.Len(); i++ {
				tc.cmd[i] = cmdField.Index(i).String()
			}
		}
	}
	
	// Extract env field
	if envField := v.FieldByName("env"); envField.IsValid() && envField.Kind() == reflect.Map {
		if envField.Type().Key().Kind() == reflect.String && envField.Type().Elem().Kind() == reflect.String {
			tc.env = make(map[string]string)
			for _, key := range envField.MapKeys() {
				tc.env[key.String()] = envField.MapIndex(key).String()
			}
		}
	}
	
	// Extract ports field
	if portsField := v.FieldByName("ports"); portsField.IsValid() && portsField.Kind() == reflect.Slice {
		if portsField.Type().Elem().Kind() == reflect.String {
			tc.ports = make([]string, portsField.Len())
			for i := 0; i < portsField.Len(); i++ {
				tc.ports[i] = portsField.Index(i).String()
			}
		}
	}
	
	return tc
}

// testConfig represents a simplified config for testing
type testConfig struct {
	cmd   []string
	env   map[string]string
	ports []string
}