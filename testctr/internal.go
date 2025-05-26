package testctr

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Internal helper functions that are not part of the public API

// setWaitForLog sets up waiting for a specific log line
func setWaitForLog(cfg *containerConfig, logLine string, timeout time.Duration) {
	prevWait := cfg.waitFunc
	cfg.waitFunc = func(containerID, runtime string) error {
		// Chain with previous wait if any
		if prevWait != nil {
			if err := prevWait(containerID, runtime); err != nil {
				return err
			}
		}
		return waitForLog(containerID, logLine, timeout)
	}
}

// setWaitForExec sets up waiting for a command to succeed
func setWaitForExec(cfg *containerConfig, cmd []string, timeout time.Duration) {
	waitFn := func(containerID, runtime string) error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		start := time.Now()
		attempts := 0
		var lastErr error

		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout after %v waiting for exec %v to succeed (tried %d times, last error: %v)", time.Since(start), cmd, attempts, lastErr)
			default:
				attempts++
				command := append([]string{"exec", containerID}, cmd...)
				if err := exec.Command(runtime, command...).Run(); err == nil {
					return nil
				} else {
					lastErr = err
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	prevWait := cfg.waitFunc
	cfg.waitFunc = func(containerID, runtime string) error {
		// Chain with previous wait if any
		if prevWait != nil {
			if err := prevWait(containerID, runtime); err != nil {
				return err
			}
		}
		return waitFn(containerID, runtime)
	}
}

// setBindMount adds a bind mount
func setBindMount(cfg *containerConfig, hostPath, containerPath string) {
	if cfg.dockerRun != nil {
		mount := fmt.Sprintf("%s:%s", hostPath, containerPath)
		cfg.dockerRun.mounts = append(cfg.dockerRun.mounts, mount)
	}
}

// setNetwork sets the network
func setNetwork(cfg *containerConfig, network string) {
	if cfg.dockerRun != nil {
		cfg.dockerRun.network = network
	}
}

// setUser sets the user
func setUser(cfg *containerConfig, user string) {
	if cfg.dockerRun != nil {
		cfg.dockerRun.user = user
	}
}

// setWorkingDir sets the working directory
func setWorkingDir(cfg *containerConfig, dir string) {
	if cfg.dockerRun != nil {
		cfg.dockerRun.workdir = dir
	}
}

// setPrivileged sets privileged mode
func setPrivileged(cfg *containerConfig) {
	// Would add --privileged flag in buildDockerRunArgs
	// This is a placeholder for now
}

// setLogs enables log streaming
func setLogs(cfg *containerConfig) {
	cfg.logStreaming = true
}

// setRuntime sets the container runtime
func setRuntime(cfg *containerConfig, runtime string) {
	cfg.forceRuntime = runtime
}

// setStartupTimeout sets the startup timeout
func setStartupTimeout(cfg *containerConfig, timeout time.Duration) {
	cfg.startupTimeout = timeout
}

// setWaitForHTTP sets up HTTP endpoint waiting
func setWaitForHTTP(cfg *containerConfig, path string) {
	// For direct implementation, we could implement HTTP checking
	// This is a no-op but kept for API compatibility
}

// setDSNProvider sets the DSN provider
func setDSNProvider(cfg *containerConfig, provider DSNProvider) {
	cfg.dsnProvider = provider
}

// setMemoryLimit sets the memory limit
func setMemoryLimit(cfg *containerConfig, limit string) {
	if cfg.dockerRun != nil {
		cfg.dockerRun.memoryLimit = limit
	}
}

// setStartupDelay sets the startup delay
func setStartupDelay(cfg *containerConfig, delay time.Duration) {
	cfg.startupDelay = delay
}

// setBackend sets the backend name
func setBackend(cfg *containerConfig, backend string) {
	cfg.backend = backend
}

// Setter methods for containerConfig to implement the interfaces expected by ctropts

func (c *containerConfig) SetBindMount(hostPath, containerPath string) {
	setBindMount(c, hostPath, containerPath)
}

func (c *containerConfig) SetNetwork(network string) {
	setNetwork(c, network)
}

func (c *containerConfig) SetUser(user string) {
	setUser(c, user)
}

func (c *containerConfig) SetWorkingDir(dir string) {
	setWorkingDir(c, dir)
}

func (c *containerConfig) SetPrivileged() {
	setPrivileged(c)
}

func (c *containerConfig) SetLogs() {
	setLogs(c)
}

func (c *containerConfig) SetRuntime(runtime string) {
	setRuntime(c, runtime)
}

func (c *containerConfig) SetStartupTimeout(timeout time.Duration) {
	setStartupTimeout(c, timeout)
}

func (c *containerConfig) SetWaitForHTTP(path string) {
	setWaitForHTTP(c, path)
}

func (c *containerConfig) SetWaitForExec(cmd []string, timeout time.Duration) {
	setWaitForExec(c, cmd, timeout)
}

func (c *containerConfig) SetWaitForLog(logLine string, timeout time.Duration) {
	setWaitForLog(c, logLine, timeout)
}

func (c *containerConfig) SetDSNProvider(provider DSNProvider) {
	setDSNProvider(c, provider)
}

func (c *containerConfig) SetMemoryLimit(limit string) {
	setMemoryLimit(c, limit)
}

func (c *containerConfig) SetStartupDelay(delay time.Duration) {
	setStartupDelay(c, delay)
}

func (c *containerConfig) SetBackend(backend string) {
	setBackend(c, backend)
}

// Testcontainers-specific setters (used by ctropts/testcontainers.go)
func (c *containerConfig) SetTestcontainersCustomizer(customizer interface{}) {
	// This is a no-op in the main package, only used by testcontainers backend
}

func (c *containerConfig) SetTestcontainersPrivileged(privileged bool) {
	// This is a no-op in the main package, only used by testcontainers backend
}

func (c *containerConfig) SetAutoRemove(autoRemove bool) {
	// This is a no-op in the main package, only used by testcontainers backend
}

func (c *containerConfig) SetWaitStrategy(strategy interface{}) {
	// This is a no-op in the main package, only used by testcontainers backend
}

func (c *containerConfig) SetHostConfigModifier(modifier func(interface{})) {
	// This is a no-op in the main package, only used by testcontainers backend
}

func (c *containerConfig) SetSkipReaper(skip bool) {
	// This is a no-op in the main package, only used by testcontainers backend
}
