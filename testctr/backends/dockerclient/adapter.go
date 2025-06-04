package dockerclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"archive/tar"

	"github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/tmc/misc/testctr/backend"
)

// DockerClientBackend implements the Backend interface using the Docker Go client
type DockerClientBackend struct {
	client     *client.Client
	clientOnce sync.Once
	clientErr  error
	containers map[string]containerInfo
	mu         sync.Mutex
}

type containerInfo struct {
	id      string
	started bool
}

// init registers the dockerclient backend
func init() {
	backend.Register("dockerclient", &DockerClientBackend{
		containers: make(map[string]containerInfo),
	})
}

// ensureClient creates the Docker client if not already created
func (d *DockerClientBackend) ensureClient() error {
	d.clientOnce.Do(func() {
		d.client, d.clientErr = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
	})
	return d.clientErr
}

// Helper types to match testctr's internal structures
type containerConfig struct {
	dockerRun      *dockerRun
	startupTimeout time.Duration
	startupDelay   time.Duration
	logStreaming   bool
	localRuntime   string
	dsnProvider    interface{}
	files          []fileEntry
	privileged     bool
	waitConditions []interface{}
	logFilter      func(string) bool
	backend        string
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
	privileged  bool
}

type fileEntry struct {
	Source interface{} // string (file path) or io.Reader
	Target string
	Mode   os.FileMode
}

// extractConfig uses reflection to extract configuration from various config types
func extractConfig(src interface{}, dst *containerConfig) error {
	srcVal := reflect.ValueOf(src)
	if srcVal.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
	}

	if srcVal.Kind() != reflect.Struct {
		return fmt.Errorf("config must be a struct, got %v", srcVal.Kind())
	}

	// Try to extract dockerRun field
	if dockerRunField := srcVal.FieldByName("dockerRun"); dockerRunField.IsValid() && dockerRunField.CanInterface() && !dockerRunField.IsNil() {
		if dr, ok := dockerRunField.Interface().(*dockerRun); ok {
			dst.dockerRun = dr
		}
	}

	// Extract individual fields
	if envField := srcVal.FieldByName("env"); envField.IsValid() && envField.CanInterface() {
		if env, ok := envField.Interface().(map[string]string); ok && env != nil {
			for k, v := range env {
				dst.dockerRun.env[k] = v
			}
		}
	}

	if portsField := srcVal.FieldByName("ports"); portsField.IsValid() && portsField.CanInterface() {
		if ports, ok := portsField.Interface().([]string); ok {
			dst.dockerRun.ports = ports
		}
	}

	if cmdField := srcVal.FieldByName("cmd"); cmdField.IsValid() && cmdField.CanInterface() {
		if cmd, ok := cmdField.Interface().([]string); ok {
			dst.dockerRun.cmd = cmd
		}
	}

	if filesField := srcVal.FieldByName("files"); filesField.IsValid() && filesField.CanInterface() {
		if files, ok := filesField.Interface().([]fileEntry); ok {
			dst.files = files
		}
	}

	if labelsField := srcVal.FieldByName("labels"); labelsField.IsValid() && labelsField.CanInterface() {
		if labels, ok := labelsField.Interface().(map[string]string); ok && labels != nil {
			if dst.dockerRun.labels == nil {
				dst.dockerRun.labels = make(map[string]string)
			}
			for k, v := range labels {
				dst.dockerRun.labels[k] = v
			}
		}
	}

	return nil
}

// CreateContainer creates a new container with the given image and configuration
func (d *DockerClientBackend) CreateContainer(t testing.TB, img string, config interface{}) (string, error) {
	if err := d.ensureClient(); err != nil {
		return "", err
	}

	ctx := context.Background()

	// Initialize default config
	cfg := &containerConfig{
		dockerRun: &dockerRun{
			env:    make(map[string]string),
			labels: make(map[string]string),
		},
	}

	// Try to extract config information from the provided config
	if config != nil {
		// Use reflection to handle different config types
		if err := extractConfig(config, cfg); err != nil {
			// If we can't extract, just log and continue with defaults
			// This allows the backend to work with various config types
			t.Logf("Note: Unable to extract full config from %T, using defaults", config)
		}
	}

	// Create container configuration
	containerCfg := &container.Config{
		Image:        img,
		Env:          []string{},
		Labels:       map[string]string{},
		ExposedPorts: nat.PortSet{},
	}

	hostCfg := &container.HostConfig{
		AutoRemove:   false,
		PortBindings: nat.PortMap{},
		Mounts:       []mount.Mount{},
	}

	networkCfg := &network.NetworkingConfig{}

	// Apply configuration from dockerRun
	if cfg.dockerRun != nil {
		dr := cfg.dockerRun

		// Environment variables
		for k, v := range dr.env {
			containerCfg.Env = append(containerCfg.Env, fmt.Sprintf("%s=%s", k, v))
		}

		// Labels
		containerCfg.Labels = dr.labels

		// Command
		if len(dr.cmd) > 0 {
			containerCfg.Cmd = dr.cmd
		}

		// User
		if dr.user != "" {
			containerCfg.User = dr.user
		}

		// Working directory
		if dr.workdir != "" {
			containerCfg.WorkingDir = dr.workdir
		}

		// Ports
		for _, port := range dr.ports {
			containerPort, err := nat.NewPort("tcp", port)
			if err != nil {
				return "", fmt.Errorf("invalid port %s: %w", port, err)
			}
			containerCfg.ExposedPorts[containerPort] = struct{}{}
			hostCfg.PortBindings[containerPort] = []nat.PortBinding{
				{
					HostIP:   "127.0.0.1",
					HostPort: "0", // Random port
				},
			}
		}

		// Mounts
		for _, mountStr := range dr.mounts {
			parts := strings.Split(mountStr, ":")
			if len(parts) >= 2 {
				hostCfg.Binds = append(hostCfg.Binds, mountStr)
			}
		}

		// Network
		if dr.network != "" {
			hostCfg.NetworkMode = container.NetworkMode(dr.network)
		}

		// Memory limit
		if dr.memoryLimit != "" {
			// Parse memory limit - simple implementation
			var memory int64
			if strings.HasSuffix(dr.memoryLimit, "m") {
				fmt.Sscanf(dr.memoryLimit, "%dm", &memory)
				memory = memory * 1024 * 1024
			} else if strings.HasSuffix(dr.memoryLimit, "g") {
				fmt.Sscanf(dr.memoryLimit, "%dg", &memory)
				memory = memory * 1024 * 1024 * 1024
			}
			if memory > 0 {
				hostCfg.Resources = container.Resources{
					Memory: memory,
				}
			}
		}

		// Privileged mode
		if dr.privileged || cfg.privileged {
			hostCfg.Privileged = true
		}
	}

	// Pull image if needed
	reader, err := d.client.ImagePull(ctx, img, imagetypes.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image %s: %w", img, err)
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	// Create container
	resp, err := d.client.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Handle file copying
	if len(cfg.files) > 0 {
		for _, file := range cfg.files {
			if err := d.copyFileToContainer(ctx, resp.ID, file); err != nil {
				// Clean up container on error
				d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
				return "", fmt.Errorf("failed to copy file to container: %w", err)
			}
		}
	}

	// Store container info
	d.mu.Lock()
	d.containers[resp.ID] = containerInfo{id: resp.ID, started: false}
	d.mu.Unlock()

	// Handle startup delay
	if cfg.startupDelay > 0 {
		time.Sleep(cfg.startupDelay)
	}

	return resp.ID, nil
}

// copyFileToContainer copies a file into the container
func (d *DockerClientBackend) copyFileToContainer(ctx context.Context, containerID string, file fileEntry) error {
	var content []byte
	var err error

	// Get file content
	switch src := file.Source.(type) {
	case string:
		content, err = os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", src, err)
		}
	case io.Reader:
		content, err = io.ReadAll(src)
		if err != nil {
			return fmt.Errorf("failed to read from io.Reader: %w", err)
		}
	default:
		return fmt.Errorf("unsupported file source type: %T", file.Source)
	}

	// Create tar archive
	tarContent := new(bytes.Buffer)
	tarWriter := tar.NewWriter(tarContent)

	// Add file to tar
	header := &tar.Header{
		Name: filepath.Base(file.Target),
		Mode: int64(file.Mode),
		Size: int64(len(content)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Copy to container
	return d.client.CopyToContainer(ctx, containerID, filepath.Dir(file.Target), tarContent, container.CopyToContainerOptions{})
}

// StartContainer starts a previously created container
func (d *DockerClientBackend) StartContainer(containerID string) error {
	if err := d.ensureClient(); err != nil {
		return err
	}

	ctx := context.Background()
	if err := d.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return err
	}

	d.mu.Lock()
	if info, ok := d.containers[containerID]; ok {
		info.started = true
		d.containers[containerID] = info
	}
	d.mu.Unlock()

	return nil
}

// StopContainer stops a running container
func (d *DockerClientBackend) StopContainer(containerID string) error {
	if err := d.ensureClient(); err != nil {
		return err
	}

	ctx := context.Background()
	timeout := 10 // seconds
	return d.client.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
}

// RemoveContainer removes a container
func (d *DockerClientBackend) RemoveContainer(containerID string) error {
	if err := d.ensureClient(); err != nil {
		return err
	}

	ctx := context.Background()
	err := d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

	d.mu.Lock()
	delete(d.containers, containerID)
	d.mu.Unlock()

	return err
}

// InspectContainer returns information about a container
func (d *DockerClientBackend) InspectContainer(containerID string) (*backend.ContainerInfo, error) {
	if err := d.ensureClient(); err != nil {
		return nil, err
	}

	ctx := context.Background()
	data, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	info := &backend.ContainerInfo{
		ID:      data.ID,
		Name:    data.Name,
		Created: data.Created,
	}

	// Set state
	info.State.Running = data.State.Running
	info.State.Status = data.State.Status
	info.State.ExitCode = data.State.ExitCode

	// Set network settings
	info.NetworkSettings.Ports = make(map[string][]backend.PortBinding)
	for port, bindings := range data.NetworkSettings.Ports {
		portStr := string(port)
		for _, binding := range bindings {
			info.NetworkSettings.Ports[portStr] = append(info.NetworkSettings.Ports[portStr], backend.PortBinding{
				HostIP:   binding.HostIP,
				HostPort: binding.HostPort,
			})
		}
	}

	// Set internal IP
	if data.NetworkSettings.IPAddress != "" {
		info.NetworkSettings.InternalIP = data.NetworkSettings.IPAddress
	} else if data.NetworkSettings.Networks != nil {
		// Try to get IP from the first network
		for _, net := range data.NetworkSettings.Networks {
			if net.IPAddress != "" {
				info.NetworkSettings.InternalIP = net.IPAddress
				break
			}
		}
	}

	// Set labels
	info.Config.Labels = data.Config.Labels

	return info, nil
}

// ExecInContainer executes a command inside a running container
func (d *DockerClientBackend) ExecInContainer(containerID string, cmd []string) (int, string, error) {
	if err := d.ensureClient(); err != nil {
		return -1, "", err
	}

	ctx := context.Background()

	// Create exec instance
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	createResp, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return -1, "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Start exec
	attachResp, err := d.client.ContainerExecAttach(ctx, createResp.ID, container.ExecStartOptions{})
	if err != nil {
		return -1, "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// Read output
	var output bytes.Buffer
	_, err = stdcopy.StdCopy(&output, &output, attachResp.Reader)
	if err != nil && err != io.EOF {
		return -1, "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Get exit code
	inspectResp, err := d.client.ContainerExecInspect(ctx, createResp.ID)
	if err != nil {
		return -1, output.String(), fmt.Errorf("failed to inspect exec: %w", err)
	}

	return inspectResp.ExitCode, output.String(), nil
}

// GetContainerLogs retrieves the logs of a container
func (d *DockerClientBackend) GetContainerLogs(containerID string) (string, error) {
	if err := d.ensureClient(); err != nil {
		return "", err
	}

	ctx := context.Background()
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Follow:     false,
	}

	reader, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}

	// Combine stdout and stderr
	var combined bytes.Buffer
	combined.Write(stdout.Bytes())
	if stderr.Len() > 0 {
		combined.Write(stderr.Bytes())
	}

	return combined.String(), nil
}

// WaitForLog waits for a specific log line to appear in the container's logs
func (d *DockerClientBackend) WaitForLog(containerID string, logLine string, timeout time.Duration) error {
	if err := d.ensureClient(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var lastLogs string
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout after %v waiting for log line %q (checked %d times). Last logs:\n%s",
				timeout, logLine, attempts, lastLogs)
		case <-ticker.C:
			attempts++
			logs, err := d.GetContainerLogs(containerID)
			if err != nil {
				return err
			}
			lastLogs = logs
			if strings.Contains(logs, logLine) {
				return nil
			}
		}
	}
}

// InternalIP returns the IP address of the container within its primary Docker network
func (d *DockerClientBackend) InternalIP(containerID string) (string, error) {
	info, err := d.InspectContainer(containerID)
	if err != nil {
		return "", err
	}

	if info.NetworkSettings.InternalIP == "" {
		return "", fmt.Errorf("container has no internal IP address")
	}

	return info.NetworkSettings.InternalIP, nil
}

// Commit commits the current state of the container to a new image
func (d *DockerClientBackend) Commit(containerID string, imageName string) error {
	if err := d.ensureClient(); err != nil {
		return err
	}

	ctx := context.Background()
	options := container.CommitOptions{
		Reference: imageName,
	}

	_, err := d.client.ContainerCommit(ctx, containerID, options)
	return err
}

// Cleanup cleans up all containers created by this backend
func (d *DockerClientBackend) Cleanup() error {
	if d.client == nil {
		return nil
	}

	d.mu.Lock()
	containerIDs := make([]string, 0, len(d.containers))
	for id := range d.containers {
		containerIDs = append(containerIDs, id)
	}
	d.mu.Unlock()

	var errs []error

	for _, id := range containerIDs {
		if err := d.RemoveContainer(id); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove container %s: %w", id, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}
