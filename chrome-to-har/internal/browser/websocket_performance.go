package browser

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// WebSocketPerformanceMonitor monitors WebSocket performance metrics
type WebSocketPerformanceMonitor struct {
	page           *Page
	enabled        bool
	metrics        *WebSocketPerformanceMetrics
	samples        []WebSocketPerformanceSample
	mu             sync.RWMutex
	sampleInterval time.Duration
	maxSamples     int
	startTime      time.Time
	stopChan       chan struct{}
}

// WebSocketPerformanceMetrics contains comprehensive performance metrics
type WebSocketPerformanceMetrics struct {
	// Connection metrics
	ConnectionLatency time.Duration `json:"connection_latency"`
	ConnectionUptime  time.Duration `json:"connection_uptime"`
	ReconnectionCount int           `json:"reconnection_count"`
	ConnectionSuccess float64       `json:"connection_success_rate"`

	// Message metrics
	MessageLatency    LatencyMetrics `json:"message_latency"`
	MessageThroughput float64        `json:"message_throughput"` // messages per second
	DataThroughput    float64        `json:"data_throughput"`    // bytes per second
	MessageLossRate   float64        `json:"message_loss_rate"`

	// Frame metrics
	FrameLatency   LatencyMetrics `json:"frame_latency"`
	FrameSize      SizeMetrics    `json:"frame_size"`
	FrameErrorRate float64        `json:"frame_error_rate"`

	// Network metrics
	BytesSent        int64 `json:"bytes_sent"`
	BytesReceived    int64 `json:"bytes_received"`
	MessagesSent     int   `json:"messages_sent"`
	MessagesReceived int   `json:"messages_received"`

	// Performance indicators
	CPU    float64 `json:"cpu_usage"`
	Memory int64   `json:"memory_usage"`

	// Quality metrics
	Jitter     time.Duration `json:"jitter"`
	PacketLoss float64       `json:"packet_loss"`

	// Timestamps
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`

	// Connection details
	ActiveConnections int `json:"active_connections"`
	TotalConnections  int `json:"total_connections"`
	FailedConnections int `json:"failed_connections"`

	// Additional metrics
	AverageRTT time.Duration `json:"average_rtt"`
	MinRTT     time.Duration `json:"min_rtt"`
	MaxRTT     time.Duration `json:"max_rtt"`
	StdDevRTT  time.Duration `json:"stddev_rtt"`
}

// LatencyMetrics contains latency statistics
type LatencyMetrics struct {
	Min    time.Duration `json:"min"`
	Max    time.Duration `json:"max"`
	Mean   time.Duration `json:"mean"`
	Median time.Duration `json:"median"`
	P95    time.Duration `json:"p95"`
	P99    time.Duration `json:"p99"`
	StdDev time.Duration `json:"stddev"`
	Count  int           `json:"count"`
}

// SizeMetrics contains size statistics
type SizeMetrics struct {
	Min    int64   `json:"min"`
	Max    int64   `json:"max"`
	Mean   float64 `json:"mean"`
	Median int64   `json:"median"`
	Total  int64   `json:"total"`
	Count  int     `json:"count"`
}

// WebSocketPerformanceSample represents a single performance sample
type WebSocketPerformanceSample struct {
	Timestamp         time.Time     `json:"timestamp"`
	ConnectionCount   int           `json:"connection_count"`
	MessageLatency    time.Duration `json:"message_latency"`
	BytesPerSecond    float64       `json:"bytes_per_second"`
	MessagesPerSecond float64       `json:"messages_per_second"`
	ErrorRate         float64       `json:"error_rate"`
	CPU               float64       `json:"cpu"`
	Memory            int64         `json:"memory"`
}

// WebSocketPerformanceAlert represents a performance alert
type WebSocketPerformanceAlert struct {
	Type       string      `json:"type"`
	Severity   string      `json:"severity"` // "low", "medium", "high", "critical"
	Message    string      `json:"message"`
	Timestamp  time.Time   `json:"timestamp"`
	Value      interface{} `json:"value"`
	Threshold  interface{} `json:"threshold"`
	Connection string      `json:"connection,omitempty"`
}

// WebSocketPerformanceThresholds defines performance thresholds
type WebSocketPerformanceThresholds struct {
	MaxConnectionLatency time.Duration `json:"max_connection_latency"`
	MaxMessageLatency    time.Duration `json:"max_message_latency"`
	MinThroughput        float64       `json:"min_throughput"`
	MaxErrorRate         float64       `json:"max_error_rate"`
	MaxCPUUsage          float64       `json:"max_cpu_usage"`
	MaxMemoryUsage       int64         `json:"max_memory_usage"`
	MaxJitter            time.Duration `json:"max_jitter"`
	MaxPacketLoss        float64       `json:"max_packet_loss"`
}

// NewWebSocketPerformanceMonitor creates a new performance monitor
func NewWebSocketPerformanceMonitor(page *Page) *WebSocketPerformanceMonitor {
	return &WebSocketPerformanceMonitor{
		page:           page,
		metrics:        &WebSocketPerformanceMetrics{},
		samples:        make([]WebSocketPerformanceSample, 0),
		sampleInterval: 1 * time.Second,
		maxSamples:     1000,
		stopChan:       make(chan struct{}),
	}
}

// Start starts performance monitoring
func (wpm *WebSocketPerformanceMonitor) Start() error {
	wpm.mu.Lock()
	defer wpm.mu.Unlock()

	if wpm.enabled {
		return nil
	}

	wpm.enabled = true
	wpm.startTime = time.Now()
	wpm.metrics.StartTime = wpm.startTime

	// Start sampling goroutine
	go wpm.sampleLoop()

	return nil
}

// Stop stops performance monitoring
func (wpm *WebSocketPerformanceMonitor) Stop() error {
	wpm.mu.Lock()
	defer wpm.mu.Unlock()

	if !wpm.enabled {
		return nil
	}

	wpm.enabled = false
	close(wpm.stopChan)

	// Calculate final metrics
	wpm.calculateFinalMetrics()

	return nil
}

// sampleLoop runs the performance sampling loop
func (wpm *WebSocketPerformanceMonitor) sampleLoop() {
	ticker := time.NewTicker(wpm.sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-wpm.stopChan:
			return
		case <-ticker.C:
			wpm.takeSample()
		}
	}
}

// takeSample takes a performance sample
func (wpm *WebSocketPerformanceMonitor) takeSample() {
	wpm.mu.Lock()
	defer wpm.mu.Unlock()

	if !wpm.enabled {
		return
	}

	connections := wpm.page.GetWebSocketConnections()
	_ = wpm.page.GetWebSocketStats() // stats may be used in future versions

	sample := WebSocketPerformanceSample{
		Timestamp:       time.Now(),
		ConnectionCount: len(connections),
	}

	// Calculate throughput
	if len(wpm.samples) > 0 {
		lastSample := wpm.samples[len(wpm.samples)-1]
		timeDiff := sample.Timestamp.Sub(lastSample.Timestamp).Seconds()

		if timeDiff > 0 {
			// Calculate bytes per second
			totalBytes := int64(0)
			totalMessages := 0

			for _, conn := range connections {
				totalBytes += conn.BytesSent + conn.BytesReceived
				totalMessages += conn.MessagesSent + conn.MessagesReceived
			}

			sample.BytesPerSecond = float64(totalBytes) / timeDiff
			sample.MessagesPerSecond = float64(totalMessages) / timeDiff
		}
	}

	// Calculate message latency (simplified)
	sample.MessageLatency = wpm.calculateCurrentLatency(connections)

	// Get system metrics (simplified)
	sample.CPU = wpm.getCPUUsage()
	sample.Memory = wpm.getMemoryUsage()

	// Calculate error rate
	sample.ErrorRate = wpm.calculateErrorRate(connections)

	// Add sample
	wpm.samples = append(wpm.samples, sample)

	// Limit sample count
	if len(wpm.samples) > wpm.maxSamples {
		wpm.samples = wpm.samples[1:]
	}
}

// calculateCurrentLatency calculates current message latency
func (wpm *WebSocketPerformanceMonitor) calculateCurrentLatency(connections map[string]*WebSocketConnection) time.Duration {
	var latencies []time.Duration

	for _, conn := range connections {
		conn.mu.RLock()
		frames := conn.Frames
		conn.mu.RUnlock()

		// Find recent ping-pong pairs
		for i := len(frames) - 1; i > 0; i-- {
			if frames[i].Type == "pong" && frames[i-1].Type == "ping" {
				latency := frames[i].Timestamp.Sub(frames[i-1].Timestamp)
				if latency > 0 && latency < 10*time.Second {
					latencies = append(latencies, latency)
				}
				break
			}
		}
	}

	if len(latencies) == 0 {
		return 0
	}

	// Return average latency
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	return total / time.Duration(len(latencies))
}

// getCPUUsage gets CPU usage (simplified implementation)
func (wpm *WebSocketPerformanceMonitor) getCPUUsage() float64 {
	// In a real implementation, this would measure actual CPU usage
	// For now, return a placeholder value
	return 0.0
}

// getMemoryUsage gets memory usage (simplified implementation)
func (wpm *WebSocketPerformanceMonitor) getMemoryUsage() int64 {
	// In a real implementation, this would measure actual memory usage
	// For now, return a placeholder value
	return 0
}

// calculateErrorRate calculates current error rate
func (wpm *WebSocketPerformanceMonitor) calculateErrorRate(connections map[string]*WebSocketConnection) float64 {
	totalFrames := 0
	errorFrames := 0

	for _, conn := range connections {
		conn.mu.RLock()
		frames := conn.Frames
		conn.mu.RUnlock()

		for _, frame := range frames {
			totalFrames++
			if frame.Type == "error" {
				errorFrames++
			}
		}
	}

	if totalFrames == 0 {
		return 0
	}

	return float64(errorFrames) / float64(totalFrames) * 100
}

// calculateFinalMetrics calculates final performance metrics
func (wpm *WebSocketPerformanceMonitor) calculateFinalMetrics() {
	wpm.metrics.EndTime = time.Now()
	wpm.metrics.Duration = wpm.metrics.EndTime.Sub(wpm.metrics.StartTime)

	connections := wpm.page.GetWebSocketConnections()

	// Calculate connection metrics
	wpm.metrics.TotalConnections = len(connections)
	wpm.metrics.ActiveConnections = 0
	wpm.metrics.FailedConnections = 0

	var connectionLatencies []time.Duration
	var uptimes []time.Duration

	for _, conn := range connections {
		connectionLatencies = append(connectionLatencies, conn.ConnectionLatency)

		if conn.State == "open" {
			wpm.metrics.ActiveConnections++
			uptimes = append(uptimes, time.Since(conn.ConnectedAt))
		} else if conn.State == "closed" && conn.CloseCode != 0 {
			wpm.metrics.FailedConnections++
		}

		if conn.DisconnectedAt != nil {
			uptimes = append(uptimes, conn.DisconnectedAt.Sub(conn.ConnectedAt))
		}

		wpm.metrics.BytesSent += conn.BytesSent
		wpm.metrics.BytesReceived += conn.BytesReceived
		wpm.metrics.MessagesSent += conn.MessagesSent
		wpm.metrics.MessagesReceived += conn.MessagesReceived
	}

	// Calculate connection success rate
	if wpm.metrics.TotalConnections > 0 {
		successfulConnections := wpm.metrics.TotalConnections - wpm.metrics.FailedConnections
		wpm.metrics.ConnectionSuccess = float64(successfulConnections) / float64(wpm.metrics.TotalConnections) * 100
	}

	// Calculate connection latency
	if len(connectionLatencies) > 0 {
		wpm.metrics.ConnectionLatency = wpm.calculateLatencyMetrics(connectionLatencies).Mean
	}

	// Calculate connection uptime
	if len(uptimes) > 0 {
		var totalUptime time.Duration
		for _, uptime := range uptimes {
			totalUptime += uptime
		}
		wpm.metrics.ConnectionUptime = totalUptime / time.Duration(len(uptimes))
	}

	// Calculate throughput
	if wpm.metrics.Duration > 0 {
		wpm.metrics.MessageThroughput = float64(wpm.metrics.MessagesSent+wpm.metrics.MessagesReceived) / wpm.metrics.Duration.Seconds()
		wpm.metrics.DataThroughput = float64(wpm.metrics.BytesSent+wpm.metrics.BytesReceived) / wpm.metrics.Duration.Seconds()
	}

	// Calculate latency metrics from samples
	wpm.calculateLatencyMetricsFromSamples()

	// Calculate RTT metrics
	wpm.calculateRTTMetrics(connections)

	// Calculate jitter
	wpm.calculateJitter()
}

// calculateLatencyMetrics calculates latency statistics
func (wpm *WebSocketPerformanceMonitor) calculateLatencyMetrics(latencies []time.Duration) LatencyMetrics {
	if len(latencies) == 0 {
		return LatencyMetrics{}
	}

	// Sort latencies
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	metrics := LatencyMetrics{
		Min:   latencies[0],
		Max:   latencies[len(latencies)-1],
		Count: len(latencies),
	}

	// Calculate mean
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	metrics.Mean = total / time.Duration(len(latencies))

	// Calculate median
	if len(latencies)%2 == 0 {
		metrics.Median = (latencies[len(latencies)/2-1] + latencies[len(latencies)/2]) / 2
	} else {
		metrics.Median = latencies[len(latencies)/2]
	}

	// Calculate percentiles
	if len(latencies) > 0 {
		p95Index := int(0.95 * float64(len(latencies)))
		if p95Index >= len(latencies) {
			p95Index = len(latencies) - 1
		}
		metrics.P95 = latencies[p95Index]

		p99Index := int(0.99 * float64(len(latencies)))
		if p99Index >= len(latencies) {
			p99Index = len(latencies) - 1
		}
		metrics.P99 = latencies[p99Index]
	}

	// Calculate standard deviation
	var variance time.Duration
	for _, latency := range latencies {
		diff := latency - metrics.Mean
		variance += diff * diff / time.Duration(len(latencies))
	}
	metrics.StdDev = time.Duration(float64(variance) * 0.5) // Simplified sqrt

	return metrics
}

// calculateLatencyMetricsFromSamples calculates latency metrics from samples
func (wpm *WebSocketPerformanceMonitor) calculateLatencyMetricsFromSamples() {
	var messageLatencies []time.Duration

	for _, sample := range wpm.samples {
		if sample.MessageLatency > 0 {
			messageLatencies = append(messageLatencies, sample.MessageLatency)
		}
	}

	wpm.metrics.MessageLatency = wpm.calculateLatencyMetrics(messageLatencies)
}

// calculateRTTMetrics calculates RTT metrics
func (wpm *WebSocketPerformanceMonitor) calculateRTTMetrics(connections map[string]*WebSocketConnection) {
	var rtts []time.Duration

	for _, conn := range connections {
		conn.mu.RLock()
		frames := conn.Frames
		conn.mu.RUnlock()

		// Find ping-pong pairs
		for i := 1; i < len(frames); i++ {
			if frames[i-1].Type == "ping" && frames[i].Type == "pong" {
				rtt := frames[i].Timestamp.Sub(frames[i-1].Timestamp)
				if rtt > 0 && rtt < 10*time.Second {
					rtts = append(rtts, rtt)
				}
			}
		}
	}

	if len(rtts) > 0 {
		sort.Slice(rtts, func(i, j int) bool {
			return rtts[i] < rtts[j]
		})

		wpm.metrics.MinRTT = rtts[0]
		wpm.metrics.MaxRTT = rtts[len(rtts)-1]

		var total time.Duration
		for _, rtt := range rtts {
			total += rtt
		}
		wpm.metrics.AverageRTT = total / time.Duration(len(rtts))

		// Calculate standard deviation
		var variance time.Duration
		for _, rtt := range rtts {
			diff := rtt - wpm.metrics.AverageRTT
			variance += diff * diff / time.Duration(len(rtts))
		}
		wpm.metrics.StdDevRTT = time.Duration(float64(variance) * 0.5) // Simplified sqrt
	}
}

// calculateJitter calculates jitter from RTT measurements
func (wpm *WebSocketPerformanceMonitor) calculateJitter() {
	var rtts []time.Duration

	for _, sample := range wpm.samples {
		if sample.MessageLatency > 0 {
			rtts = append(rtts, sample.MessageLatency)
		}
	}

	if len(rtts) < 2 {
		return
	}

	// Calculate jitter as average of absolute differences
	var jitterSum time.Duration
	for i := 1; i < len(rtts); i++ {
		diff := rtts[i] - rtts[i-1]
		if diff < 0 {
			diff = -diff
		}
		jitterSum += diff
	}

	wpm.metrics.Jitter = jitterSum / time.Duration(len(rtts)-1)
}

// GetMetrics returns current performance metrics
func (wpm *WebSocketPerformanceMonitor) GetMetrics() *WebSocketPerformanceMetrics {
	wpm.mu.RLock()
	defer wpm.mu.RUnlock()

	// Create a copy of metrics
	metrics := *wpm.metrics
	return &metrics
}

// GetSamples returns performance samples
func (wpm *WebSocketPerformanceMonitor) GetSamples() []WebSocketPerformanceSample {
	wpm.mu.RLock()
	defer wpm.mu.RUnlock()

	// Create a copy of samples
	samples := make([]WebSocketPerformanceSample, len(wpm.samples))
	copy(samples, wpm.samples)
	return samples
}

// CheckThresholds checks performance against thresholds
func (wpm *WebSocketPerformanceMonitor) CheckThresholds(thresholds *WebSocketPerformanceThresholds) []WebSocketPerformanceAlert {
	wpm.mu.RLock()
	defer wpm.mu.RUnlock()

	var alerts []WebSocketPerformanceAlert

	if len(wpm.samples) == 0 {
		return alerts
	}

	currentSample := wpm.samples[len(wpm.samples)-1]

	// Check connection latency
	if thresholds.MaxConnectionLatency > 0 && wpm.metrics.ConnectionLatency > thresholds.MaxConnectionLatency {
		alerts = append(alerts, WebSocketPerformanceAlert{
			Type:      "connection_latency",
			Severity:  "high",
			Message:   "Connection latency exceeds threshold",
			Timestamp: time.Now(),
			Value:     wpm.metrics.ConnectionLatency,
			Threshold: thresholds.MaxConnectionLatency,
		})
	}

	// Check message latency
	if thresholds.MaxMessageLatency > 0 && currentSample.MessageLatency > thresholds.MaxMessageLatency {
		alerts = append(alerts, WebSocketPerformanceAlert{
			Type:      "message_latency",
			Severity:  "medium",
			Message:   "Message latency exceeds threshold",
			Timestamp: time.Now(),
			Value:     currentSample.MessageLatency,
			Threshold: thresholds.MaxMessageLatency,
		})
	}

	// Check throughput
	if thresholds.MinThroughput > 0 && currentSample.MessagesPerSecond < thresholds.MinThroughput {
		alerts = append(alerts, WebSocketPerformanceAlert{
			Type:      "throughput",
			Severity:  "medium",
			Message:   "Throughput below threshold",
			Timestamp: time.Now(),
			Value:     currentSample.MessagesPerSecond,
			Threshold: thresholds.MinThroughput,
		})
	}

	// Check error rate
	if thresholds.MaxErrorRate > 0 && currentSample.ErrorRate > thresholds.MaxErrorRate {
		alerts = append(alerts, WebSocketPerformanceAlert{
			Type:      "error_rate",
			Severity:  "high",
			Message:   "Error rate exceeds threshold",
			Timestamp: time.Now(),
			Value:     currentSample.ErrorRate,
			Threshold: thresholds.MaxErrorRate,
		})
	}

	// Check CPU usage
	if thresholds.MaxCPUUsage > 0 && currentSample.CPU > thresholds.MaxCPUUsage {
		alerts = append(alerts, WebSocketPerformanceAlert{
			Type:      "cpu_usage",
			Severity:  "medium",
			Message:   "CPU usage exceeds threshold",
			Timestamp: time.Now(),
			Value:     currentSample.CPU,
			Threshold: thresholds.MaxCPUUsage,
		})
	}

	// Check memory usage
	if thresholds.MaxMemoryUsage > 0 && currentSample.Memory > thresholds.MaxMemoryUsage {
		alerts = append(alerts, WebSocketPerformanceAlert{
			Type:      "memory_usage",
			Severity:  "medium",
			Message:   "Memory usage exceeds threshold",
			Timestamp: time.Now(),
			Value:     currentSample.Memory,
			Threshold: thresholds.MaxMemoryUsage,
		})
	}

	// Check jitter
	if thresholds.MaxJitter > 0 && wpm.metrics.Jitter > thresholds.MaxJitter {
		alerts = append(alerts, WebSocketPerformanceAlert{
			Type:      "jitter",
			Severity:  "medium",
			Message:   "Jitter exceeds threshold",
			Timestamp: time.Now(),
			Value:     wpm.metrics.Jitter,
			Threshold: thresholds.MaxJitter,
		})
	}

	return alerts
}

// GenerateReport generates a performance report
func (wpm *WebSocketPerformanceMonitor) GenerateReport() *WebSocketPerformanceReport {
	wpm.mu.RLock()
	defer wpm.mu.RUnlock()

	return &WebSocketPerformanceReport{
		Metrics: *wpm.metrics,
		Samples: wpm.samples,
		Summary: wpm.generateSummary(),
	}
}

// WebSocketPerformanceReport represents a comprehensive performance report
type WebSocketPerformanceReport struct {
	Metrics WebSocketPerformanceMetrics  `json:"metrics"`
	Samples []WebSocketPerformanceSample `json:"samples"`
	Summary WebSocketPerformanceSummary  `json:"summary"`
}

// WebSocketPerformanceSummary represents a performance summary
type WebSocketPerformanceSummary struct {
	OverallRating   string   `json:"overall_rating"` // "excellent", "good", "fair", "poor"
	KeyFindings     []string `json:"key_findings"`
	Recommendations []string `json:"recommendations"`
	Issues          []string `json:"issues"`
	Strengths       []string `json:"strengths"`
}

// generateSummary generates a performance summary
func (wpm *WebSocketPerformanceMonitor) generateSummary() WebSocketPerformanceSummary {
	summary := WebSocketPerformanceSummary{
		KeyFindings:     []string{},
		Recommendations: []string{},
		Issues:          []string{},
		Strengths:       []string{},
	}

	// Analyze connection metrics
	if wpm.metrics.ConnectionSuccess >= 95 {
		summary.Strengths = append(summary.Strengths, "High connection success rate")
	} else if wpm.metrics.ConnectionSuccess < 80 {
		summary.Issues = append(summary.Issues, "Low connection success rate")
		summary.Recommendations = append(summary.Recommendations, "Investigate connection stability")
	}

	// Analyze latency
	if wpm.metrics.MessageLatency.Mean < 100*time.Millisecond {
		summary.Strengths = append(summary.Strengths, "Low message latency")
	} else if wpm.metrics.MessageLatency.Mean > 500*time.Millisecond {
		summary.Issues = append(summary.Issues, "High message latency")
		summary.Recommendations = append(summary.Recommendations, "Optimize message processing")
	}

	// Analyze throughput
	if wpm.metrics.MessageThroughput > 100 {
		summary.Strengths = append(summary.Strengths, "High message throughput")
	} else if wpm.metrics.MessageThroughput < 10 {
		summary.Issues = append(summary.Issues, "Low message throughput")
		summary.Recommendations = append(summary.Recommendations, "Investigate throughput bottlenecks")
	}

	// Determine overall rating
	issueCount := len(summary.Issues)
	strengthCount := len(summary.Strengths)

	if issueCount == 0 && strengthCount > 2 {
		summary.OverallRating = "excellent"
	} else if issueCount <= 1 && strengthCount > 1 {
		summary.OverallRating = "good"
	} else if issueCount <= 2 {
		summary.OverallRating = "fair"
	} else {
		summary.OverallRating = "poor"
	}

	// Add key findings
	summary.KeyFindings = append(summary.KeyFindings,
		fmt.Sprintf("Average connection latency: %v", wpm.metrics.ConnectionLatency))
	summary.KeyFindings = append(summary.KeyFindings,
		fmt.Sprintf("Message throughput: %.1f messages/sec", wpm.metrics.MessageThroughput))
	summary.KeyFindings = append(summary.KeyFindings,
		fmt.Sprintf("Connection success rate: %.1f%%", wpm.metrics.ConnectionSuccess))

	return summary
}

// StartPerformanceMonitoring starts performance monitoring on a page
func (p *Page) StartPerformanceMonitoring() (*WebSocketPerformanceMonitor, error) {
	monitor := NewWebSocketPerformanceMonitor(p)
	if err := monitor.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start performance monitoring")
	}
	return monitor, nil
}

// MonitorPerformanceWithThresholds monitors performance with threshold checking
func (p *Page) MonitorPerformanceWithThresholds(
	ctx context.Context,
	thresholds *WebSocketPerformanceThresholds,
	alertHandler func([]WebSocketPerformanceAlert),
) (*WebSocketPerformanceMonitor, error) {
	monitor, err := p.StartPerformanceMonitoring()
	if err != nil {
		return nil, err
	}

	// Start threshold monitoring
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				alerts := monitor.CheckThresholds(thresholds)
				if len(alerts) > 0 && alertHandler != nil {
					alertHandler(alerts)
				}
			}
		}
	}()

	return monitor, nil
}
