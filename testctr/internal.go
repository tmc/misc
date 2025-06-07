// Package testctr contains internal types and helpers not meant for direct public use.
// However, some options in ctropts might need to type-assert to interfaces
// implemented by containerConfig to set specific fields.
package testctr

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// This file contains setter methods for containerConfig that are called by OptionFunc
// in the ctropts package. These allow ctropts to modify internal configuration
// without directly exposing containerConfig.

// --- Setters for options from ctropts/options.go ---

func (cc *containerConfig) SetBindMount(hostPath, containerPath string) {
	if cc.dockerRun != nil { // Relevant for default CLI backend
		mount := fmt.Sprintf("%s:%s", hostPath, containerPath)
		cc.dockerRun.mounts = append(cc.dockerRun.mounts, mount)
	}
}

func (cc *containerConfig) SetNetwork(network string) {
	if cc.dockerRun != nil {
		cc.dockerRun.network = network
	}
}

func (cc *containerConfig) SetUser(user string) {
	if cc.dockerRun != nil {
		cc.dockerRun.user = user
	}
}

func (cc *containerConfig) SetWorkingDir(dir string) {
	if cc.dockerRun != nil {
		cc.dockerRun.workdir = dir
	}
}

func (cc *containerConfig) SetPrivileged() { // This is for ctropts.WithPrivileged
	cc.privileged = true     // Set the general privileged flag on containerConfig
	if cc.dockerRun != nil { // If using CLI backend, ensure its config reflects this
		cc.dockerRun.privileged = true
	}
}

func (cc *containerConfig) SetLogs() {
	cc.logStreaming = true
}

func (cc *containerConfig) SetRuntime(runtime string) { // For ctropts.WithRuntime
	cc.localRuntime = runtime // This specifies which container runtime to use
}

func (cc *containerConfig) SetStartupTimeout(timeout time.Duration) {
	cc.startupTimeout = timeout
}

func (cc *containerConfig) SetDSNProvider(provider DSNProvider) {
	cc.dsnProvider = provider
}

func (cc *containerConfig) SetMemoryLimit(limit string) {
	if cc.dockerRun != nil {
		cc.dockerRun.memoryLimit = limit
	}
}

func (cc *containerConfig) SetStartupDelay(delay time.Duration) {
	cc.startupDelay = delay
}

func (cc *containerConfig) SetLogFilter(filter func(string) bool) {
	cc.logFilter = filter
}

// SetWaitForLogOpt is called by ctropts.WithWaitForLog
func (cc *containerConfig) SetWaitForLogOpt(logLine string, timeout time.Duration) {
	waitCond := func(ctx context.Context, c *Container) error {
		// Apply specific timeout for this wait condition
		waitCtx, cancel := context.WithTimeoutCause(ctx, timeout,
			fmt.Errorf("timeout (%v) waiting for log line %q in container %s", timeout, logLine, c.id[:12]))
		defer cancel()

		// Dispatch to the overridden backend
		if c.be != nil {
			// TODO: Update backend interface to accept context
			return c.be.WaitForLog(c.id, logLine, timeout)
		}
		// Fallback to CLI-based wait
		return waitForLogWithContext(waitCtx, c.id, logLine)
	}
	cc.waitConditions = append(cc.waitConditions, waitCond)
}

// SetWaitForExecOpt is called by ctropts.WithWaitForExec
func (cc *containerConfig) SetWaitForExecOpt(cmd []string, timeout time.Duration) {
	waitCond := func(ctx context.Context, c *Container) error {
		// Apply specific timeout for this wait condition
		waitCtx, cancel := context.WithTimeoutCause(ctx, timeout,
			fmt.Errorf("timeout (%v) waiting for exec command %v in container %s", timeout, cmd, c.id[:12]))
		defer cancel()

		startTime := time.Now()
		var lastErr error
		attempts := 0
		for {
			select {
			case <-waitCtx.Done():
				return fmt.Errorf("failed after %v waiting for exec to succeed (tried %d times, last error: %w): %w",
					time.Since(startTime), attempts, lastErr, context.Cause(waitCtx))
			default:
				attempts++
				// Use the active backend's ExecInContainer
				var exitCode int
				var output string
				var execErr error
				if c.be != nil {
					exitCode, output, execErr = c.be.ExecInContainer(c.id, cmd)
				} else {
					// Fallback to CLI exec with context
					exitCode, output, execErr = c.Exec(waitCtx, cmd)
				}
				if execErr == nil && exitCode == 0 {
					if *verbose {
						c.t.Logf("Exec wait succeeded for %v in container %s after %d attempts", cmd, c.id[:12], attempts)
					}
					return nil // Success
				}

				errMsg := "exec failed"
				if execErr != nil {
					errMsg = fmt.Sprintf("%s: %v", errMsg, execErr)
				} else {
					errMsg = fmt.Sprintf("%s with exit code %d", errMsg, exitCode)
				}
				lastErr = fmt.Errorf("%s. Output: %s", errMsg, output)
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
	cc.waitConditions = append(cc.waitConditions, waitCond)
}

// SetWaitForHTTPOpt is called by ctropts.WithWaitForHTTP
func (cc *containerConfig) SetWaitForHTTPOpt(path, internalPort string, expectedStatus int, timeout time.Duration) {
	waitCond := func(ctx context.Context, c *Container) error {
		// Apply specific timeout for this wait condition
		waitCtx, cancel := context.WithTimeoutCause(ctx, timeout,
			fmt.Errorf("timeout (%v) waiting for HTTP %s:%s to return status %d in container %s",
				timeout, internalPort, path, expectedStatus, c.id[:12]))
		defer cancel()

		startTime := time.Now()
		var lastErr error
		checkCmd := []string{
			"sh", "-c",
			fmt.Sprintf("if command -v curl >/dev/null 2>&1; then "+
				"curl -s -o /dev/null -w '%%{http_code}' http://127.0.0.1:%s%s; "+
				"elif command -v wget >/dev/null 2>&1; then "+
				"wget --spider -S --timeout=5 http://127.0.0.1:%s%s 2>&1 | grep 'HTTP/' | awk '{print $2}' | tail -n1; "+
				"else echo 0; fi",
				internalPort, path, internalPort, path),
		}

		for {
			select {
			case <-waitCtx.Done():
				return fmt.Errorf("failed after %v waiting for HTTP status %d (last error: %w): %w",
					time.Since(startTime), expectedStatus, lastErr, context.Cause(waitCtx))
			default:
				// Use a shorter timeout for each individual exec attempt
				execCtx, execCancel := context.WithTimeout(waitCtx, 5*time.Second)
				var exitCode int
				var output string
				var execErr error
				if c.be != nil {
					exitCode, output, execErr = c.be.ExecInContainer(c.id, checkCmd)
				} else {
					// Fallback to CLI exec
					exitCode, output, execErr = c.Exec(execCtx, checkCmd)
				}
				execCancel()

				if execErr != nil {
					lastErr = fmt.Errorf("exec error for HTTP check: %w, output: %s", execErr, output)
					time.Sleep(500 * time.Millisecond)
					continue
				}
				if exitCode != 0 && output == "" { // Script might have failed (e.g. no curl/wget)
					lastErr = fmt.Errorf("HTTP check script in %s exited with code %d without output", c.id[:min(12, len(c.id))], exitCode)
					time.Sleep(500 * time.Millisecond)
					continue
				}

				httpCodeStr := strings.TrimSpace(output)
				httpCode, convErr := strconv.Atoi(httpCodeStr)

				if convErr != nil {
					lastErr = fmt.Errorf("could not parse HTTP code '%s' from output (container %s): %w. Full output: %s", httpCodeStr, c.id[:min(12, len(c.id))], convErr, output)
					time.Sleep(500 * time.Millisecond)
					continue
				}

				if httpCode == expectedStatus {
					return nil // Success
				}
				lastErr = fmt.Errorf("unexpected HTTP status for 127.0.0.1:%s%s in %s: got %d, want %d. Output: %s", internalPort, path, c.id[:min(12, len(c.id))], httpCode, expectedStatus, output)
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
	cc.waitConditions = append(cc.waitConditions, waitCond)
}

// --- Setter for backend selection (from testctr/option.go) ---

func (cc *containerConfig) SetBackend(name string) {
	cc.localRuntime = name
}

// --- Setters for testcontainers-specific options (from ctropts/testcontainers.go) ---
// These store the customizers on containerConfig. The testcontainers adapter will use them.

func (cc *containerConfig) AddTestcontainersCustomizer(customizer interface{}) {
	cc.testcontainersCustomizers = append(cc.testcontainersCustomizers, customizer)
}
