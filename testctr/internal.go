package testctr

import (
	"context"
	"fmt"
	"time"
)

// SetWaitForLogOpt is called by ctropts.WithWaitForLog
func (cc *containerConfig) SetWaitForLogOpt(logLine string, timeout time.Duration) {
	waitCond := func(ctx context.Context, c *Container) error {
		// Use backend's WaitForLog method with the timeout
		return c.backend.WaitForLog(c.id, logLine, timeout)
	}
	cc.waitConditions = append(cc.waitConditions, waitCond)
}

// SetWaitForExecOpt is called by ctropts.WithWaitForExec
func (cc *containerConfig) SetWaitForExecOpt(cmd []string, timeout time.Duration) {
	// Ensure startup timeout is at least as long as this wait condition
	if timeout > cc.startupTimeout {
		cc.startupTimeout = timeout + 2*time.Second // Add buffer
	}
	
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
				// Use the container's Exec method
				exitCode, output, execErr := c.Exec(waitCtx, cmd)
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

// SetLogs enables log streaming
func (cc *containerConfig) SetLogs() {
	cc.logStreaming = true
}

// SetPrivileged enables privileged mode
func (cc *containerConfig) SetPrivileged() {
	cc.privileged = true
}

// SetDSNProvider sets the DSN provider
func (cc *containerConfig) SetDSNProvider(provider DSNProvider) {
	cc.dsnProvider = provider
}

// SetLogFilter sets the log filter
func (cc *containerConfig) SetLogFilter(filter func(string) bool) {
	cc.logFilter = filter
}

// SetRuntime sets the container runtime - for CLI backend only
func (cc *containerConfig) SetRuntime(runtime string) {
	// This is now handled by the backend selection
	// For CLI backend, we could pass this as part of backend config
	// but for now, CLI backend auto-discovers the runtime
}

// SetStartupTimeout sets the startup timeout
func (cc *containerConfig) SetStartupTimeout(timeout time.Duration) {
	cc.startupTimeout = timeout
}

// SetStartupDelay sets the startup delay
func (cc *containerConfig) SetStartupDelay(delay time.Duration) {
	cc.startupDelay = delay
}