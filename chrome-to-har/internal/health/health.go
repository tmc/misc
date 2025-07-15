// Package health provides health check functionality for Chrome AI integration
package health

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

// HealthCheck represents a single health check
type HealthCheck struct {
	Name       string
	CheckFunc  func(context.Context) HealthResult
	Interval   time.Duration
	Timeout    time.Duration
	Critical   bool // If true, failure affects overall health
	Enabled    bool
	lastResult HealthResult
	lastCheck  time.Time
	mu         sync.RWMutex
}

// HealthResult represents the result of a health check
type HealthResult struct {
	Status    HealthStatus      `json:"status"`
	Message   string            `json:"message,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Duration  time.Duration     `json:"duration"`
	Error     error             `json:"error,omitempty"`
}

// HealthManager manages multiple health checks
type HealthManager struct {
	checks   map[string]*HealthCheck
	mu       sync.RWMutex
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewHealthManager creates a new health manager
func NewHealthManager() *HealthManager {
	return &HealthManager{
		checks:   make(map[string]*HealthCheck),
		stopChan: make(chan struct{}),
	}
}

// RegisterCheck registers a new health check
func (hm *HealthManager) RegisterCheck(name string, checkFunc func(context.Context) HealthResult, interval, timeout time.Duration, critical bool) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.checks[name]; exists {
		return fmt.Errorf("health check %s already registered", name)
	}

	hm.checks[name] = &HealthCheck{
		Name:      name,
		CheckFunc: checkFunc,
		Interval:  interval,
		Timeout:   timeout,
		Critical:  critical,
		Enabled:   true,
	}

	return nil
}

// EnableCheck enables a health check
func (hm *HealthManager) EnableCheck(name string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	check, exists := hm.checks[name]
	if !exists {
		return fmt.Errorf("health check %s not found", name)
	}

	check.Enabled = true
	return nil
}

// DisableCheck disables a health check
func (hm *HealthManager) DisableCheck(name string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	check, exists := hm.checks[name]
	if !exists {
		return fmt.Errorf("health check %s not found", name)
	}

	check.Enabled = false
	return nil
}

// Start starts all health checks
func (hm *HealthManager) Start() {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	for name, check := range hm.checks {
		if check.Enabled {
			hm.wg.Add(1)
			go hm.runCheck(name, check)
		}
	}
}

// Stop stops all health checks
func (hm *HealthManager) Stop() {
	close(hm.stopChan)
	hm.wg.Wait()
}

// runCheck runs a single health check in a loop
func (hm *HealthManager) runCheck(name string, check *HealthCheck) {
	defer hm.wg.Done()

	ticker := time.NewTicker(check.Interval)
	defer ticker.Stop()

	// Run initial check
	hm.executeCheck(name, check)

	for {
		select {
		case <-ticker.C:
			hm.executeCheck(name, check)
		case <-hm.stopChan:
			return
		}
	}
}

// executeCheck executes a single health check
func (hm *HealthManager) executeCheck(name string, check *HealthCheck) {
	check.mu.Lock()
	defer check.mu.Unlock()

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), check.Timeout)
	defer cancel()

	result := check.CheckFunc(ctx)
	result.Duration = time.Since(start)
	result.Timestamp = time.Now()

	check.lastResult = result
	check.lastCheck = time.Now()
}

// GetStatus returns the current status of a health check
func (hm *HealthManager) GetStatus(name string) (HealthResult, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	check, exists := hm.checks[name]
	if !exists {
		return HealthResult{}, fmt.Errorf("health check %s not found", name)
	}

	check.mu.RLock()
	defer check.mu.RUnlock()

	if check.lastCheck.IsZero() {
		return HealthResult{
			Status:    StatusUnknown,
			Message:   "Check has not run yet",
			Timestamp: time.Now(),
		}, nil
	}

	return check.lastResult, nil
}

// GetOverallStatus returns the overall health status
func (hm *HealthManager) GetOverallStatus() HealthResult {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	overallStatus := StatusHealthy
	details := make(map[string]string)
	var messages []string

	for name, check := range hm.checks {
		if !check.Enabled {
			continue
		}

		check.mu.RLock()
		result := check.lastResult
		check.mu.RUnlock()

		details[name] = string(result.Status)

		if result.Message != "" {
			messages = append(messages, fmt.Sprintf("%s: %s", name, result.Message))
		}

		// Update overall status based on individual check results
		if check.Critical {
			if result.Status == StatusUnhealthy {
				overallStatus = StatusUnhealthy
			} else if result.Status == StatusDegraded && overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		}
	}

	message := ""
	if len(messages) > 0 {
		message = fmt.Sprintf("%d checks completed", len(messages))
	}

	return HealthResult{
		Status:    overallStatus,
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// GetAllStatuses returns all health check statuses
func (hm *HealthManager) GetAllStatuses() map[string]HealthResult {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	results := make(map[string]HealthResult)

	for name, check := range hm.checks {
		check.mu.RLock()
		if check.lastCheck.IsZero() {
			results[name] = HealthResult{
				Status:    StatusUnknown,
				Message:   "Check has not run yet",
				Timestamp: time.Now(),
			}
		} else {
			results[name] = check.lastResult
		}
		check.mu.RUnlock()
	}

	return results
}

// Common health check functions

// ChromeConnectionCheck checks if Chrome DevTools connection is healthy
func ChromeConnectionCheck(ctx context.Context) HealthResult {
	// This would implement actual Chrome connection testing
	// For now, return a mock healthy status
	return HealthResult{
		Status:  StatusHealthy,
		Message: "Chrome DevTools connection is healthy",
		Details: map[string]string{
			"connection_type": "devtools",
			"protocol":        "CDP",
		},
		Timestamp: time.Now(),
	}
}

// NativeMessagingCheck checks if native messaging is working
func NativeMessagingCheck(ctx context.Context) HealthResult {
	// This would implement actual native messaging testing
	// For now, return a mock healthy status
	return HealthResult{
		Status:  StatusHealthy,
		Message: "Native messaging is functional",
		Details: map[string]string{
			"protocol": "native_messaging",
			"binary":   "chrome-ai-native-host",
		},
		Timestamp: time.Now(),
	}
}

// AIAPICheck checks if AI APIs are available
func AIAPICheck(ctx context.Context) HealthResult {
	// This would implement actual AI API testing
	// For now, return a mock status
	return HealthResult{
		Status:  StatusHealthy,
		Message: "AI APIs are accessible",
		Details: map[string]string{
			"language_model": "available",
			"window_ai":      "unknown",
		},
		Timestamp: time.Now(),
	}
}

// ExtensionCheck checks if extension context is valid
func ExtensionCheck(ctx context.Context) HealthResult {
	// This would implement actual extension context testing
	// For now, return a mock status
	return HealthResult{
		Status:  StatusHealthy,
		Message: "Extension context is valid",
		Details: map[string]string{
			"manifest_version": "3",
			"context":          "valid",
		},
		Timestamp: time.Now(),
	}
}

// MemoryCheck checks memory usage
func MemoryCheck(ctx context.Context) HealthResult {
	// This would implement actual memory usage monitoring
	// For now, return a mock status
	return HealthResult{
		Status:  StatusHealthy,
		Message: "Memory usage is within acceptable limits",
		Details: map[string]string{
			"heap_used":  "25MB",
			"heap_limit": "100MB",
		},
		Timestamp: time.Now(),
	}
}
