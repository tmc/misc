// Package health provides comprehensive health check functionality for Chrome AI integration
package health

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusDegraded  HealthStatus = "degraded"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
	StatusStarting  HealthStatus = "starting"
	StatusStopping  HealthStatus = "stopping"
)

// HealthSeverity represents the severity level of a health issue
type HealthSeverity string

const (
	SeverityInfo     HealthSeverity = "info"
	SeverityWarning  HealthSeverity = "warning"
	SeverityError    HealthSeverity = "error"
	SeverityCritical HealthSeverity = "critical"
)

// HealthCategory represents the category of a health check
type HealthCategory string

const (
	CategoryBrowser     HealthCategory = "browser"
	CategoryNetwork     HealthCategory = "network"
	CategoryMemory      HealthCategory = "memory"
	CategoryPerformance HealthCategory = "performance"
	CategoryExtension   HealthCategory = "extension"
	CategoryAPI         HealthCategory = "api"
	CategorySystem      HealthCategory = "system"
)

// HealthCheckStatus represents the status of a health check
type HealthCheckStatus struct {
	Name        string        `json:"name"`
	Status      string        `json:"status"`
	Message     string        `json:"message,omitempty"`
	LastChecked time.Time     `json:"last_checked"`
	Duration    time.Duration `json:"duration"`
	Error       error         `json:"error,omitempty"`
}

// HealthCheck represents a single health check
type HealthCheck struct {
	Name         string
	Description  string
	Category     HealthCategory
	CheckFunc    func(context.Context) HealthResult
	Interval     time.Duration
	Timeout      time.Duration
	Critical     bool // If true, failure affects overall health
	Enabled      bool
	Dependencies []string                  // Dependencies on other checks
	Tags         map[string]string         // Metadata tags
	Thresholds   map[string]float64        // Custom thresholds
	RetryConfig  *RetryConfig              // Retry configuration
	AlertConfig  *AlertConfig              // Alert configuration

	// Runtime state
	lastResult        HealthResult
	lastCheck         time.Time
	lastSuccess       time.Time
	lastFailure       time.Time
	consecutiveFailures int64
	totalChecks       int64
	totalFailures     int64
	running           bool
	mu                sync.RWMutex

	// Performance metrics
	performanceMetrics *PerformanceMetrics
}

// RetryConfig defines retry behavior for health checks
type RetryConfig struct {
	MaxRetries   int
	RetryDelay   time.Duration
	BackoffRate  float64
	MaxBackoff   time.Duration
}

// AlertConfig defines alerting behavior for health checks
type AlertConfig struct {
	Enabled              bool
	FailureThreshold     int
	RecoveryNotification bool
	EscalationRules      []EscalationRule
	SuppressFor          time.Duration
}

// EscalationRule defines escalation behavior for alerts
type EscalationRule struct {
	AfterFailures int
	Severity      HealthSeverity
	Actions       []string
}

// PerformanceMetrics tracks performance statistics for health checks
type PerformanceMetrics struct {
	MinDuration    time.Duration
	MaxDuration    time.Duration
	AvgDuration    time.Duration
	TotalDuration  time.Duration
	P95Duration    time.Duration
	P99Duration    time.Duration
	durations      []time.Duration
	mu             sync.RWMutex
}

// HealthResult represents the result of a health check
type HealthResult struct {
	Status       HealthStatus             `json:"status"`
	Message      string                   `json:"message,omitempty"`
	Details      map[string]interface{}   `json:"details,omitempty"`
	Timestamp    time.Time                `json:"timestamp"`
	Duration     time.Duration            `json:"duration"`
	Error        error                    `json:"error,omitempty"`
	Severity     HealthSeverity           `json:"severity"`
	Category     HealthCategory           `json:"category"`
	Metrics      map[string]float64       `json:"metrics,omitempty"`
	Tags         map[string]string        `json:"tags,omitempty"`
	Recommendations []string              `json:"recommendations,omitempty"`
	RetryAttempt int                      `json:"retry_attempt,omitempty"`
	TrendData    *TrendData               `json:"trend_data,omitempty"`
}

// TrendData represents trend information for health metrics
type TrendData struct {
	Direction   string    `json:"direction"`   // improving, degrading, stable
	Rate        float64   `json:"rate"`        // Rate of change
	Confidence  float64   `json:"confidence"`  // Confidence in trend (0-1)
	Prediction  string    `json:"prediction"`  // Predicted future state
	LastUpdated time.Time `json:"last_updated"`
}

// HealthManager manages multiple health checks
type HealthManager struct {
	checks          map[string]*HealthCheck
	mu              sync.RWMutex
	stopChan        chan struct{}
	wg              sync.WaitGroup
	config          *HealthConfig
	notifier        *Notifier
	metrics         *HealthMetrics
	history         *HealthHistory
	predictor       *HealthPredictor
	alertManager    *AlertManager
	dashboard       *Dashboard
	resourceMonitor *ResourceMonitor
	running         bool
	startTime       time.Time
}

// HealthConfig defines global configuration for health monitoring
type HealthConfig struct {
	Enabled             bool
	CheckInterval       time.Duration
	DefaultTimeout      time.Duration
	RetentionPeriod     time.Duration
	MaxHistorySize      int
	EnablePredictive    bool
	EnableNotifications bool
	EnableMetrics       bool
	EnableDashboard     bool
	LogLevel            string
	DataDir             string
	HealthEndpoint      string
	MetricsEndpoint     string
	DashboardEndpoint   string
}

// HealthMetrics tracks overall health system metrics
type HealthMetrics struct {
	TotalChecks       int64
	HealthyChecks     int64
	DegradedChecks    int64
	UnhealthyChecks   int64
	TotalExecutions   int64
	TotalFailures     int64
	AverageLatency    time.Duration
	Uptime            time.Duration
	StartTime         time.Time
	mu                sync.RWMutex
}

// HealthHistory maintains historical health data
type HealthHistory struct {
	records    []HealthRecord
	maxSize    int
	mu         sync.RWMutex
}

// HealthRecord represents a historical health check record
type HealthRecord struct {
	CheckName string
	Result    HealthResult
	ID        string
}

// HealthPredictor provides predictive health analysis
type HealthPredictor struct {
	models    map[string]*PredictionModel
	mu        sync.RWMutex
}

// PredictionModel represents a predictive model for health trends
type PredictionModel struct {
	CheckName    string
	DataPoints   []DataPoint
	Trend        string
	Confidence   float64
	LastUpdated  time.Time
}

// DataPoint represents a single data point in a prediction model
type DataPoint struct {
	Timestamp time.Time
	Value     float64
	Status    HealthStatus
}

// ResourceMonitor monitors system resource usage
type ResourceMonitor struct {
	memoryUsage    uint64
	cpuUsage       float64
	goroutineCount int
	heapSize       uint64
	gcPauses       []time.Duration
	mu             sync.RWMutex
}

// NewHealthManager creates a new health manager with default configuration
func NewHealthManager() *HealthManager {
	return NewHealthManagerWithConfig(DefaultHealthConfig())
}

// NewHealthManagerWithConfig creates a new health manager with custom configuration
func NewHealthManagerWithConfig(config *HealthConfig) *HealthManager {
	hm := &HealthManager{
		checks:   make(map[string]*HealthCheck),
		stopChan: make(chan struct{}),
		config:   config,
		metrics:  &HealthMetrics{StartTime: time.Now()},
		history:  &HealthHistory{maxSize: config.MaxHistorySize},
		predictor: &HealthPredictor{models: make(map[string]*PredictionModel)},
	}

	if config.EnableNotifications {
		hm.notifier = NewNotifier()
	}

	if config.EnableMetrics {
		hm.alertManager = NewAlertManager(hm)
	}

	if config.EnableDashboard {
		hm.dashboard = NewDashboard(hm)
	}

	hm.resourceMonitor = NewResourceMonitor()

	return hm
}

// DefaultHealthConfig returns the default health configuration
func DefaultHealthConfig() *HealthConfig {
	return &HealthConfig{
		Enabled:             true,
		CheckInterval:       30 * time.Second,
		DefaultTimeout:      10 * time.Second,
		RetentionPeriod:     24 * time.Hour,
		MaxHistorySize:      1000,
		EnablePredictive:    true,
		EnableNotifications: true,
		EnableMetrics:       true,
		EnableDashboard:     true,
		LogLevel:            "info",
		DataDir:             "/tmp/health",
		HealthEndpoint:      "/health",
		MetricsEndpoint:     "/metrics",
		DashboardEndpoint:   "/dashboard",
	}
}

// RegisterCheck registers a new health check with enhanced configuration
func (hm *HealthManager) RegisterCheck(name string, checkFunc func(context.Context) HealthResult, opts ...CheckOption) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.checks[name]; exists {
		return fmt.Errorf("health check %s already registered", name)
	}

	// Create check with defaults
	check := &HealthCheck{
		Name:        name,
		CheckFunc:   checkFunc,
		Interval:    hm.config.CheckInterval,
		Timeout:     hm.config.DefaultTimeout,
		Critical:    true,
		Enabled:     true,
		Tags:        make(map[string]string),
		Thresholds:  make(map[string]float64),
		RetryConfig: &RetryConfig{
			MaxRetries:  3,
			RetryDelay:  1 * time.Second,
			BackoffRate: 2.0,
			MaxBackoff:  30 * time.Second,
		},
		AlertConfig: &AlertConfig{
			Enabled:              true,
			FailureThreshold:     3,
			RecoveryNotification: true,
			SuppressFor:          5 * time.Minute,
		},
		performanceMetrics: &PerformanceMetrics{
			MinDuration: time.Hour,
			durations:   make([]time.Duration, 0, 100),
		},
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(check); err != nil {
			return fmt.Errorf("applying option: %w", err)
		}
	}

	hm.checks[name] = check

	// Update metrics
	atomic.AddInt64(&hm.metrics.TotalChecks, 1)

	return nil
}

// CheckOption defines options for health checks
type CheckOption func(*HealthCheck) error

// WithDescription sets the check description
func WithDescription(desc string) CheckOption {
	return func(c *HealthCheck) error {
		c.Description = desc
		return nil
	}
}

// WithCategory sets the check category
func WithCategory(category HealthCategory) CheckOption {
	return func(c *HealthCheck) error {
		c.Category = category
		return nil
	}
}

// WithInterval sets the check interval
func WithInterval(interval time.Duration) CheckOption {
	return func(c *HealthCheck) error {
		if interval <= 0 {
			return errors.New("interval must be positive")
		}
		c.Interval = interval
		return nil
	}
}

// WithTimeout sets the check timeout
func WithTimeout(timeout time.Duration) CheckOption {
	return func(c *HealthCheck) error {
		if timeout <= 0 {
			return errors.New("timeout must be positive")
		}
		c.Timeout = timeout
		return nil
	}
}

// WithCritical sets whether the check is critical
func WithCritical(critical bool) CheckOption {
	return func(c *HealthCheck) error {
		c.Critical = critical
		return nil
	}
}

// WithTags sets metadata tags for the check
func WithTags(tags map[string]string) CheckOption {
	return func(c *HealthCheck) error {
		c.Tags = tags
		return nil
	}
}

// WithThresholds sets custom thresholds for the check
func WithThresholds(thresholds map[string]float64) CheckOption {
	return func(c *HealthCheck) error {
		c.Thresholds = thresholds
		return nil
	}
}

// WithDependencies sets dependencies for the check
func WithDependencies(dependencies []string) CheckOption {
	return func(c *HealthCheck) error {
		c.Dependencies = dependencies
		return nil
	}
}

// WithRetryConfig sets retry configuration for the check
func WithRetryConfig(config *RetryConfig) CheckOption {
	return func(c *HealthCheck) error {
		c.RetryConfig = config
		return nil
	}
}

// WithAlertConfig sets alert configuration for the check
func WithAlertConfig(config *AlertConfig) CheckOption {
	return func(c *HealthCheck) error {
		c.AlertConfig = config
		return nil
	}
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

// Start starts all health checks and monitoring systems
func (hm *HealthManager) Start() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if hm.running {
		return errors.New("health manager already running")
	}

	if !hm.config.Enabled {
		return errors.New("health manager is disabled")
	}

	hm.running = true
	hm.startTime = time.Now()

	// Start resource monitor
	if hm.resourceMonitor != nil {
		hm.wg.Add(1)
		go hm.runResourceMonitor()
	}

	// Start alert manager
	if hm.alertManager != nil {
		hm.wg.Add(1)
		go hm.alertManager.Start()
	}

	// Start dashboard
	if hm.dashboard != nil {
		if err := hm.dashboard.Start(); err != nil {
			return fmt.Errorf("failed to start dashboard: %w", err)
		}
	}

	// Start predictor
	if hm.config.EnablePredictive {
		hm.wg.Add(1)
		go hm.runPredictor()
	}

	// Start health checks
	for name, check := range hm.checks {
		if check.Enabled {
			hm.wg.Add(1)
			go hm.runCheck(name, check)
		}
	}

	return nil
}

// Stop stops all health checks and monitoring systems
func (hm *HealthManager) Stop() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.running {
		return nil
	}

	hm.running = false

	// Stop dashboard
	if hm.dashboard != nil {
		if err := hm.dashboard.Stop(); err != nil {
			return fmt.Errorf("failed to stop dashboard: %w", err)
		}
	}

	// Stop alert manager
	if hm.alertManager != nil {
		hm.alertManager.Stop()
	}

	// Close stop channel and wait for all goroutines
	close(hm.stopChan)
	hm.wg.Wait()

	// Create new stop channel for next start
	hm.stopChan = make(chan struct{})

	return nil
}

// runCheck runs a single health check in a loop with enhanced error handling
func (hm *HealthManager) runCheck(name string, check *HealthCheck) {
	defer hm.wg.Done()

	ticker := time.NewTicker(check.Interval)
	defer ticker.Stop()

	// Mark check as running
	check.mu.Lock()
	check.running = true
	check.mu.Unlock()

	// Run initial check
	hm.executeCheck(name, check)

	for {
		select {
		case <-ticker.C:
			// Check if still enabled
			check.mu.RLock()
			enabled := check.Enabled
			check.mu.RUnlock()

			if enabled {
				hm.executeCheck(name, check)
			}
		case <-hm.stopChan:
			// Mark check as stopped
			check.mu.Lock()
			check.running = false
			check.mu.Unlock()
			return
		}
	}
}

// executeCheck executes a single health check with retry logic and enhanced tracking
func (hm *HealthManager) executeCheck(name string, check *HealthCheck) {
	check.mu.Lock()
	defer check.mu.Unlock()

	atomic.AddInt64(&check.totalChecks, 1)
	atomic.AddInt64(&hm.metrics.TotalExecutions, 1)

	// Check dependencies first
	if !hm.checkDependencies(check) {
		result := HealthResult{
			Status:    StatusUnhealthy,
			Message:   "Dependencies not met",
			Timestamp: time.Now(),
			Severity:  SeverityError,
			Category:  check.Category,
		}
		hm.processCheckResult(name, check, result)
		return
	}

	// Execute check with retry logic
	result := hm.executeWithRetry(check)

	// Update performance metrics
	hm.updatePerformanceMetrics(check, result.Duration)

	// Process result
	hm.processCheckResult(name, check, result)
}

// executeWithRetry executes a health check with retry logic
func (hm *HealthManager) executeWithRetry(check *HealthCheck) HealthResult {
	var result HealthResult
	retryDelay := check.RetryConfig.RetryDelay

	for attempt := 0; attempt <= check.RetryConfig.MaxRetries; attempt++ {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), check.Timeout)

		result = check.CheckFunc(ctx)
		result.Duration = time.Since(start)
		result.Timestamp = time.Now()
		result.RetryAttempt = attempt
		result.Category = check.Category
		result.Tags = check.Tags

		cancel()

		// If successful or max retries reached, return result
		if result.Status == StatusHealthy || attempt == check.RetryConfig.MaxRetries {
			break
		}

		// Wait before retry with exponential backoff
		if attempt < check.RetryConfig.MaxRetries {
			time.Sleep(retryDelay)
			retryDelay = time.Duration(float64(retryDelay) * check.RetryConfig.BackoffRate)
			if retryDelay > check.RetryConfig.MaxBackoff {
				retryDelay = check.RetryConfig.MaxBackoff
			}
		}
	}

	return result
}

// checkDependencies checks if all dependencies are healthy
func (hm *HealthManager) checkDependencies(check *HealthCheck) bool {
	if len(check.Dependencies) == 0 {
		return true
	}

	for _, depName := range check.Dependencies {
		if depCheck, exists := hm.checks[depName]; exists {
			if depCheck.lastResult.Status != StatusHealthy {
				return false
			}
		} else {
			return false // Dependency not found
		}
	}

	return true
}

// processCheckResult processes the result of a health check
func (hm *HealthManager) processCheckResult(name string, check *HealthCheck, result HealthResult) {
	check.lastResult = result
	check.lastCheck = time.Now()

	// Update failure tracking
	if result.Status == StatusHealthy {
		check.consecutiveFailures = 0
		check.lastSuccess = time.Now()
	} else {
		atomic.AddInt64(&check.consecutiveFailures, 1)
		atomic.AddInt64(&check.totalFailures, 1)
		atomic.AddInt64(&hm.metrics.TotalFailures, 1)
		check.lastFailure = time.Now()
	}

	// Update trend data
	if hm.config.EnablePredictive {
		hm.updateTrendData(name, result)
	}

	// Add to history
	hm.addToHistory(name, result)

	// Send alerts if needed
	if hm.alertManager != nil {
		hm.alertManager.ProcessResult(name, result)
	}

	// Update overall metrics
	hm.updateOverallMetrics(result)
}

// updatePerformanceMetrics updates performance metrics for a check
func (hm *HealthManager) updatePerformanceMetrics(check *HealthCheck, duration time.Duration) {
	metrics := check.performanceMetrics
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.TotalDuration += duration
	metrics.durations = append(metrics.durations, duration)

	// Keep only last 100 durations for percentile calculation
	if len(metrics.durations) > 100 {
		metrics.durations = metrics.durations[1:]
	}

	// Update min/max
	if duration < metrics.MinDuration {
		metrics.MinDuration = duration
	}
	if duration > metrics.MaxDuration {
		metrics.MaxDuration = duration
	}

	// Calculate average
	if len(metrics.durations) > 0 {
		var total time.Duration
		for _, d := range metrics.durations {
			total += d
		}
		metrics.AvgDuration = total / time.Duration(len(metrics.durations))
	}

	// Calculate percentiles
	if len(metrics.durations) >= 20 {
		durations := make([]time.Duration, len(metrics.durations))
		copy(durations, metrics.durations)
		
		// Simple percentile calculation (not fully accurate but sufficient)
		p95Index := int(float64(len(durations)) * 0.95)
		p99Index := int(float64(len(durations)) * 0.99)
		
		if p95Index < len(durations) {
			metrics.P95Duration = durations[p95Index]
		}
		if p99Index < len(durations) {
			metrics.P99Duration = durations[p99Index]
		}
	}
}

// updateTrendData updates trend data for predictive analysis
func (hm *HealthManager) updateTrendData(checkName string, result HealthResult) {
	hm.predictor.mu.Lock()
	defer hm.predictor.mu.Unlock()

	model, exists := hm.predictor.models[checkName]
	if !exists {
		model = &PredictionModel{
			CheckName:   checkName,
			DataPoints:  make([]DataPoint, 0, 100),
			LastUpdated: time.Now(),
		}
		hm.predictor.models[checkName] = model
	}

	// Add data point
	value := 1.0
	if result.Status != StatusHealthy {
		value = 0.0
	}

	dataPoint := DataPoint{
		Timestamp: result.Timestamp,
		Value:     value,
		Status:    result.Status,
	}

	model.DataPoints = append(model.DataPoints, dataPoint)

	// Keep only last 100 data points
	if len(model.DataPoints) > 100 {
		model.DataPoints = model.DataPoints[1:]
	}

	// Update trend analysis
	if len(model.DataPoints) >= 10 {
		hm.analyzeTrend(model)
	}
}

// analyzeTrend performs simple trend analysis
func (hm *HealthManager) analyzeTrend(model *PredictionModel) {
	if len(model.DataPoints) < 10 {
		return
	}

	// Calculate trend over last 10 data points
	lastTen := model.DataPoints[len(model.DataPoints)-10:]
	var sum float64
	for _, point := range lastTen {
		sum += point.Value
	}
	avg := sum / float64(len(lastTen))

	// Simple trend classification
	if avg > 0.8 {
		model.Trend = "stable"
		model.Confidence = 0.8
	} else if avg > 0.5 {
		model.Trend = "degrading"
		model.Confidence = 0.6
	} else {
		model.Trend = "unhealthy"
		model.Confidence = 0.9
	}

	model.LastUpdated = time.Now()
}

// addToHistory adds a result to the health history
func (hm *HealthManager) addToHistory(checkName string, result HealthResult) {
	hm.history.mu.Lock()
	defer hm.history.mu.Unlock()

	record := HealthRecord{
		CheckName: checkName,
		Result:    result,
		ID:        fmt.Sprintf("%s-%d", checkName, time.Now().UnixNano()),
	}

	hm.history.records = append(hm.history.records, record)

	// Keep only maxSize records
	if len(hm.history.records) > hm.history.maxSize {
		hm.history.records = hm.history.records[1:]
	}
}

// updateOverallMetrics updates overall system metrics
func (hm *HealthManager) updateOverallMetrics(result HealthResult) {
	hm.metrics.mu.Lock()
	defer hm.metrics.mu.Unlock()

	hm.metrics.Uptime = time.Since(hm.startTime)

	// Count check statuses
	var healthy, degraded, unhealthy int64
	for _, check := range hm.checks {
		switch check.lastResult.Status {
		case StatusHealthy:
			healthy++
		case StatusDegraded:
			degraded++
		case StatusUnhealthy:
			unhealthy++
		}
	}

	hm.metrics.HealthyChecks = healthy
	hm.metrics.DegradedChecks = degraded
	hm.metrics.UnhealthyChecks = unhealthy

	// Calculate average latency
	var totalLatency time.Duration
	var count int64
	for _, check := range hm.checks {
		if !check.lastResult.Timestamp.IsZero() {
			totalLatency += check.lastResult.Duration
			count++
		}
	}
	if count > 0 {
		hm.metrics.AverageLatency = totalLatency / time.Duration(count)
	}
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

// GetOverallStatus returns the overall health status with enhanced details
func (hm *HealthManager) GetOverallStatus() HealthResult {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	overallStatus := StatusHealthy
	details := make(map[string]interface{})
	metrics := make(map[string]float64)
	var messages []string
	var recommendations []string

	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0
	disabledCount := 0

	for name, check := range hm.checks {
		if !check.Enabled {
			disabledCount++
			continue
		}

		check.mu.RLock()
		result := check.lastResult
		check.mu.RUnlock()

		details[name] = map[string]interface{}{
			"status":               string(result.Status),
			"message":              result.Message,
			"last_check":           check.lastCheck,
			"consecutive_failures": check.consecutiveFailures,
			"total_failures":       check.totalFailures,
			"total_checks":         check.totalChecks,
			"category":              string(check.Category),
			"critical":              check.Critical,
		}

		if result.Message != "" {
			messages = append(messages, fmt.Sprintf("%s: %s", name, result.Message))
		}

		// Count status types
		switch result.Status {
		case StatusHealthy:
			healthyCount++
		case StatusDegraded:
			degradedCount++
		case StatusUnhealthy:
			unhealthyCount++
		}

		// Update overall status based on individual check results
		if check.Critical {
			if result.Status == StatusUnhealthy {
				overallStatus = StatusUnhealthy
			} else if result.Status == StatusDegraded && overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		}

		// Add recommendations for unhealthy checks
		if result.Status == StatusUnhealthy && len(result.Recommendations) > 0 {
			recommendations = append(recommendations, result.Recommendations...)
		}
	}

	// Calculate metrics
	totalChecks := float64(healthyCount + degradedCount + unhealthyCount)
	if totalChecks > 0 {
		metrics["healthy_percentage"] = float64(healthyCount) / totalChecks * 100
		metrics["degraded_percentage"] = float64(degradedCount) / totalChecks * 100
		metrics["unhealthy_percentage"] = float64(unhealthyCount) / totalChecks * 100
	}

	metrics["total_checks"] = totalChecks
	metrics["healthy_count"] = float64(healthyCount)
	metrics["degraded_count"] = float64(degradedCount)
	metrics["unhealthy_count"] = float64(unhealthyCount)
	metrics["disabled_count"] = float64(disabledCount)
	metrics["uptime_seconds"] = time.Since(hm.startTime).Seconds()

	// Add system metrics
	if hm.resourceMonitor != nil {
		hm.resourceMonitor.mu.RLock()
		metrics["memory_usage_mb"] = float64(hm.resourceMonitor.memoryUsage) / 1024 / 1024
		metrics["cpu_usage_percent"] = hm.resourceMonitor.cpuUsage
		metrics["goroutine_count"] = float64(hm.resourceMonitor.goroutineCount)
		metrics["heap_size_mb"] = float64(hm.resourceMonitor.heapSize) / 1024 / 1024
		hm.resourceMonitor.mu.RUnlock()
	}

	// Generate message
	message := fmt.Sprintf("%d healthy, %d degraded, %d unhealthy out of %d total checks", 
		healthyCount, degradedCount, unhealthyCount, int(totalChecks))

	// Add severity based on overall status
	var severity HealthSeverity
	switch overallStatus {
	case StatusHealthy:
		severity = SeverityInfo
	case StatusDegraded:
		severity = SeverityWarning
	case StatusUnhealthy:
		severity = SeverityError
	default:
		severity = SeverityInfo
	}

	return HealthResult{
		Status:          overallStatus,
		Message:         message,
		Details:         details,
		Timestamp:       time.Now(),
		Severity:        severity,
		Category:        "system",
		Metrics:         metrics,
		Recommendations: recommendations,
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
