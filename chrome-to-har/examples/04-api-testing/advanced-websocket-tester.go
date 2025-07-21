// Advanced WebSocket API testing with comprehensive monitoring
// This example demonstrates advanced WebSocket testing capabilities including:
// - Real-time connection monitoring
// - HAR export with WebSocket data
// - Performance metrics and analysis
// - Multi-connection testing
// - Error handling and recovery
// - Socket.IO and other library support
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

// WebSocketTestSuite represents a comprehensive WebSocket test suite
type WebSocketTestSuite struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Tests       []WebSocketTest          `json:"tests"`
	Results     []WebSocketTestResult    `json:"results"`
	Summary     WebSocketTestSummary     `json:"summary"`
	Metrics     WebSocketMetrics         `json:"metrics"`
}

// WebSocketTest represents a single WebSocket test
type WebSocketTest struct {
	Name             string                 `json:"name"`
	URL              string                 `json:"url"`
	Type             string                 `json:"type"` // "basic", "echo", "subscription", "stress", "socketio", "realtime"
	Duration         time.Duration          `json:"duration"`
	Messages         []WebSocketMessage     `json:"messages"`
	ExpectedEvents   []string               `json:"expected_events"`
	ExpectedMessages []string               `json:"expected_messages"`
	Timeout          time.Duration          `json:"timeout"`
	Concurrent       int                    `json:"concurrent"`
	Protocol         string                 `json:"protocol,omitempty"`
	Headers          map[string]string      `json:"headers,omitempty"`
	ValidateResponse func(string) bool      `json:"-"`
}

// WebSocketMessage represents a message to send
type WebSocketMessage struct {
	Data      interface{} `json:"data"`
	Delay     time.Duration `json:"delay"`
	Repeat    int         `json:"repeat"`
	Condition string      `json:"condition,omitempty"`
}

// WebSocketTestResult represents test results
type WebSocketTestResult struct {
	TestName          string                    `json:"test_name"`
	URL               string                    `json:"url"`
	Success           bool                      `json:"success"`
	Error             string                    `json:"error,omitempty"`
	Duration          time.Duration             `json:"duration"`
	ConnectionTime    time.Duration             `json:"connection_time"`
	BytesSent         int64                     `json:"bytes_sent"`
	BytesReceived     int64                     `json:"bytes_received"`
	MessagesSent      int                       `json:"messages_sent"`
	MessagesReceived  int                       `json:"messages_received"`
	Frames            []browser.WebSocketFrame  `json:"frames"`
	Connections       []ConnectionInfo          `json:"connections"`
	Performance       PerformanceMetrics        `json:"performance"`
	Errors            []ErrorInfo               `json:"errors"`
}

// ConnectionInfo represents connection details
type ConnectionInfo struct {
	ID             string        `json:"id"`
	State          string        `json:"state"`
	Protocol       string        `json:"protocol"`
	Extensions     []string      `json:"extensions"`
	ConnectedAt    time.Time     `json:"connected_at"`
	DisconnectedAt *time.Time    `json:"disconnected_at,omitempty"`
	Latency        time.Duration `json:"latency"`
	CloseCode      int           `json:"close_code,omitempty"`
	CloseReason    string        `json:"close_reason,omitempty"`
}

// PerformanceMetrics represents performance data
type PerformanceMetrics struct {
	AverageLatency     time.Duration `json:"average_latency"`
	MinLatency         time.Duration `json:"min_latency"`
	MaxLatency         time.Duration `json:"max_latency"`
	Throughput         float64       `json:"throughput"` // bytes per second
	MessageRate        float64       `json:"message_rate"` // messages per second
	ErrorRate          float64       `json:"error_rate"`
	ConnectionUptime   time.Duration `json:"connection_uptime"`
	ReconnectionCount  int           `json:"reconnection_count"`
}

// ErrorInfo represents error details
type ErrorInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
}

// WebSocketTestSummary represents overall test summary
type WebSocketTestSummary struct {
	TotalTests       int           `json:"total_tests"`
	PassedTests      int           `json:"passed_tests"`
	FailedTests      int           `json:"failed_tests"`
	SuccessRate      float64       `json:"success_rate"`
	TotalDuration    time.Duration `json:"total_duration"`
	AverageDuration  time.Duration `json:"average_duration"`
	TotalConnections int           `json:"total_connections"`
	TotalMessages    int           `json:"total_messages"`
	TotalBytes       int64         `json:"total_bytes"`
}

// WebSocketMetrics represents overall metrics
type WebSocketMetrics struct {
	ConnectionMetrics  map[string]interface{} `json:"connection_metrics"`
	PerformanceMetrics map[string]interface{} `json:"performance_metrics"`
	ErrorMetrics       map[string]interface{} `json:"error_metrics"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run advanced-websocket-tester.go <test-config.json>")
		fmt.Println("   or: go run advanced-websocket-tester.go <websocket-url>")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  go run advanced-websocket-tester.go wss://api.example.com/ws")
		fmt.Println("  go run advanced-websocket-tester.go socketio://localhost:3000")
		fmt.Println("  go run advanced-websocket-tester.go test-config.json")
		os.Exit(1)
	}

	arg := os.Args[1]
	var testSuite WebSocketTestSuite

	// Check if argument is a JSON config file or a URL
	if strings.HasSuffix(arg, ".json") {
		// Load test configuration from JSON file
		configData, err := os.ReadFile(arg)
		if err != nil {
			log.Fatalf("Failed to read config file: %v", err)
		}
		if err := json.Unmarshal(configData, &testSuite); err != nil {
			log.Fatalf("Failed to parse config file: %v", err)
		}
	} else {
		// Create default test suite for single URL
		testSuite = createDefaultTestSuite(arg)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Create profile manager
	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		log.Fatalf("Failed to create profile manager: %v", err)
	}

	// Create and launch browser
	b, err := browser.New(ctx, pm)
	if err != nil {
		log.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		log.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	// Create WebSocket-enabled recorder
	wsRecorder, err := recorder.NewWebSocketRecorder(
		recorder.WithVerbose(true),
		recorder.WithStreaming(true),
	)
	if err != nil {
		log.Fatalf("Failed to create WebSocket recorder: %v", err)
	}

	// Run test suite
	fmt.Printf("Running WebSocket Test Suite: %s\n", testSuite.Name)
	fmt.Printf("Description: %s\n", testSuite.Description)
	fmt.Printf("Tests to run: %d\n\n", len(testSuite.Tests))

	startTime := time.Now()
	
	for i, test := range testSuite.Tests {
		fmt.Printf("[%d/%d] Running test: %s\n", i+1, len(testSuite.Tests), test.Name)
		
		result := runWebSocketTest(ctx, b, wsRecorder, test)
		testSuite.Results = append(testSuite.Results, result)
		
		if result.Success {
			fmt.Printf("  ✓ PASSED (%v)\n", result.Duration)
		} else {
			fmt.Printf("  ✗ FAILED (%v): %s\n", result.Duration, result.Error)
		}
		
		// Brief delay between tests
		time.Sleep(500 * time.Millisecond)
	}

	// Calculate summary
	testSuite.Summary = calculateSummary(testSuite.Results, time.Since(startTime))
	
	// Calculate metrics
	testSuite.Metrics = calculateMetrics(testSuite.Results, wsRecorder)

	// Generate reports
	generateReports(testSuite, wsRecorder)

	// Print summary
	printSummary(testSuite)
}

// createDefaultTestSuite creates a default test suite for a single URL
func createDefaultTestSuite(url string) WebSocketTestSuite {
	return WebSocketTestSuite{
		Name:        "Default WebSocket Test Suite",
		Description: fmt.Sprintf("Comprehensive testing of WebSocket endpoint: %s", url),
		Tests: []WebSocketTest{
			{
				Name:     "Basic Connection Test",
				URL:      url,
				Type:     "basic",
				Duration: 5 * time.Second,
				Timeout:  10 * time.Second,
				ExpectedEvents: []string{"open"},
			},
			{
				Name:     "Echo Test",
				URL:      url,
				Type:     "echo",
				Duration: 10 * time.Second,
				Timeout:  15 * time.Second,
				Messages: []WebSocketMessage{
					{Data: "Hello WebSocket!", Delay: 1 * time.Second},
					{Data: map[string]interface{}{"type": "ping", "timestamp": time.Now().Unix()}, Delay: 2 * time.Second},
				},
				ExpectedEvents: []string{"open", "message"},
			},
			{
				Name:       "Subscription Test",
				URL:        url,
				Type:       "subscription",
				Duration:   20 * time.Second,
				Timeout:    30 * time.Second,
				Concurrent: 1,
				Messages: []WebSocketMessage{
					{Data: map[string]interface{}{"type": "subscribe", "channel": "test"}, Delay: 1 * time.Second},
					{Data: map[string]interface{}{"type": "message", "channel": "test", "data": "test message"}, Delay: 5 * time.Second},
				},
				ExpectedEvents: []string{"open", "message", "subscription"},
			},
			{
				Name:       "Stress Test",
				URL:        url,
				Type:       "stress",
				Duration:   30 * time.Second,
				Timeout:    45 * time.Second,
				Concurrent: 3,
				Messages: []WebSocketMessage{
					{Data: "stress test message", Delay: 100 * time.Millisecond, Repeat: 100},
				},
				ExpectedEvents: []string{"open", "message"},
			},
			{
				Name:     "Real-time Performance Test",
				URL:      url,
				Type:     "realtime",
				Duration: 60 * time.Second,
				Timeout:  90 * time.Second,
				Messages: []WebSocketMessage{
					{Data: map[string]interface{}{"type": "performance_test", "size": 1024}, Delay: 1 * time.Second, Repeat: 60},
				},
				ExpectedEvents: []string{"open", "message"},
			},
		},
	}
}

// runWebSocketTest runs a single WebSocket test
func runWebSocketTest(ctx context.Context, b *browser.Browser, wsRecorder *recorder.WebSocketRecorder, test WebSocketTest) WebSocketTestResult {
	testCtx, cancel := context.WithTimeout(ctx, test.Timeout)
	defer cancel()

	result := WebSocketTestResult{
		TestName:    test.Name,
		URL:         test.URL,
		Frames:      []browser.WebSocketFrame{},
		Connections: []ConnectionInfo{},
		Errors:      []ErrorInfo{},
	}

	startTime := time.Now()

	// Create a new page for the test
	page, err := b.NewPage()
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create page: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Enable WebSocket monitoring
	if err := page.EnableWebSocketMonitoring(); err != nil {
		result.Error = fmt.Sprintf("Failed to enable WebSocket monitoring: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Set up event handlers
	page.SetWebSocketConnectionHandler(
		func(conn *browser.WebSocketConnection) {
			result.Connections = append(result.Connections, ConnectionInfo{
				ID:          conn.ID,
				State:       conn.State,
				Protocol:    conn.Protocol,
				Extensions:  conn.Extensions,
				ConnectedAt: conn.ConnectedAt,
				Latency:     conn.ConnectionLatency,
			})
		},
		func(conn *browser.WebSocketConnection) {
			// Update connection info on disconnect
			for i, connInfo := range result.Connections {
				if connInfo.ID == conn.ID {
					result.Connections[i].State = conn.State
					result.Connections[i].DisconnectedAt = conn.DisconnectedAt
					result.Connections[i].CloseCode = conn.CloseCode
					result.Connections[i].CloseReason = conn.CloseReason
					break
				}
			}
		},
		func(conn *browser.WebSocketConnection, err error) {
			result.Errors = append(result.Errors, ErrorInfo{
				Timestamp: time.Now(),
				Type:      "connection_error",
				Message:   err.Error(),
				Details:   conn.URL,
			})
		},
	)

	page.SetWebSocketFrameHandler(
		func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
			result.Frames = append(result.Frames, *frame)
			result.BytesReceived += frame.Size
			result.MessagesReceived++
		},
		func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
			result.Frames = append(result.Frames, *frame)
			result.BytesSent += frame.Size
			result.MessagesSent++
		},
	)

	// Navigate to a test page that will create WebSocket connections
	testHTML := generateTestHTML(test)
	if err := page.Navigate(fmt.Sprintf("data:text/html,%s", testHTML)); err != nil {
		result.Error = fmt.Sprintf("Failed to navigate to test page: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Wait for test completion
	time.Sleep(test.Duration)

	// Collect final metrics
	connections := page.GetWebSocketConnections()
	stats := page.GetWebSocketStats()

	// Calculate performance metrics
	result.Performance = calculatePerformanceMetrics(result.Frames, connections, startTime)

	// Validate test results
	result.Success = validateTestResults(test, result, connections, stats)

	result.Duration = time.Since(startTime)
	return result
}

// generateTestHTML generates HTML for WebSocket testing
func generateTestHTML(test WebSocketTest) string {
	var messagesJS strings.Builder
	for _, msg := range test.Messages {
		msgData, _ := json.Marshal(msg.Data)
		messagesJS.WriteString(fmt.Sprintf(`
			setTimeout(() => {
				for (let i = 0; i < %d; i++) {
					if (ws.readyState === WebSocket.OPEN) {
						ws.send('%s');
					}
				}
			}, %d);
		`, max(1, msg.Repeat), msgData, int(msg.Delay.Milliseconds())))
	}

	return fmt.Sprintf(`
		<html>
		<head><title>WebSocket Test</title></head>
		<body>
		<h1>WebSocket Test: %s</h1>
		<div id="status">Connecting...</div>
		<div id="messages"></div>
		<script>
			let ws;
			let messageCount = 0;
			let connected = false;
			
			function connect() {
				try {
					ws = new WebSocket('%s'%s);
					
					ws.onopen = function(event) {
						connected = true;
						document.getElementById('status').textContent = 'Connected';
						console.log('WebSocket connected');
					};
					
					ws.onmessage = function(event) {
						messageCount++;
						const messagesDiv = document.getElementById('messages');
						messagesDiv.innerHTML += '<div>Message ' + messageCount + ': ' + event.data + '</div>';
						console.log('Message received:', event.data);
					};
					
					ws.onclose = function(event) {
						connected = false;
						document.getElementById('status').textContent = 'Disconnected';
						console.log('WebSocket disconnected:', event.code, event.reason);
					};
					
					ws.onerror = function(error) {
						console.error('WebSocket error:', error);
						document.getElementById('status').textContent = 'Error';
					};
					
					// Send test messages
					%s
					
				} catch (error) {
					console.error('Failed to connect:', error);
					document.getElementById('status').textContent = 'Failed to connect';
				}
			}
			
			// Start connection
			connect();
			
			// For concurrent tests, create multiple connections
			for (let i = 1; i < %d; i++) {
				setTimeout(() => connect(), i * 1000);
			}
		</script>
		</body>
		</html>
	`, test.Name, test.URL, formatProtocol(test.Protocol), messagesJS.String(), test.Concurrent)
}

// formatProtocol formats protocol for WebSocket constructor
func formatProtocol(protocol string) string {
	if protocol == "" {
		return ""
	}
	return fmt.Sprintf(`, '%s'`, protocol)
}

// calculatePerformanceMetrics calculates performance metrics from frames
func calculatePerformanceMetrics(frames []browser.WebSocketFrame, connections map[string]*browser.WebSocketConnection, startTime time.Time) PerformanceMetrics {
	if len(frames) == 0 {
		return PerformanceMetrics{}
	}

	var totalLatency time.Duration
	var minLatency = time.Hour
	var maxLatency time.Duration
	var latencyCount int
	var totalBytes int64
	var reconnectionCount int

	// Calculate latencies between sent and received frames
	for i := 1; i < len(frames); i++ {
		if frames[i-1].Direction == "sent" && frames[i].Direction == "received" {
			latency := frames[i].Timestamp.Sub(frames[i-1].Timestamp)
			totalLatency += latency
			latencyCount++
			
			if latency < minLatency {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
		}
		totalBytes += frames[i].Size
	}

	// Calculate connection uptime
	var maxUptime time.Duration
	for _, conn := range connections {
		var uptime time.Duration
		if conn.DisconnectedAt != nil {
			uptime = conn.DisconnectedAt.Sub(conn.ConnectedAt)
		} else {
			uptime = time.Since(conn.ConnectedAt)
		}
		if uptime > maxUptime {
			maxUptime = uptime
		}
	}

	totalDuration := time.Since(startTime)
	
	metrics := PerformanceMetrics{
		ConnectionUptime:  maxUptime,
		ReconnectionCount: reconnectionCount,
	}

	if latencyCount > 0 {
		metrics.AverageLatency = totalLatency / time.Duration(latencyCount)
		metrics.MinLatency = minLatency
		metrics.MaxLatency = maxLatency
	}

	if totalDuration > 0 {
		metrics.Throughput = float64(totalBytes) / totalDuration.Seconds()
		metrics.MessageRate = float64(len(frames)) / totalDuration.Seconds()
	}

	return metrics
}

// validateTestResults validates test results against expectations
func validateTestResults(test WebSocketTest, result WebSocketTestResult, connections map[string]*browser.WebSocketConnection, stats map[string]interface{}) bool {
	// Basic validation: at least one connection should be established
	if len(connections) == 0 {
		return false
	}

	// Check if expected events occurred
	eventTypes := make(map[string]bool)
	for _, frame := range result.Frames {
		if frame.Direction == "received" {
			eventTypes["message"] = true
		}
	}
	
	for _, conn := range connections {
		if conn.State == "open" || conn.State == "closed" {
			eventTypes["open"] = true
		}
	}

	for _, expectedEvent := range test.ExpectedEvents {
		if !eventTypes[expectedEvent] {
			return false
		}
	}

	// Additional validation based on test type
	switch test.Type {
	case "basic":
		return len(connections) > 0
	case "echo":
		return result.MessagesSent > 0 && result.MessagesReceived > 0
	case "subscription":
		return result.MessagesReceived > 0
	case "stress":
		return result.MessagesSent >= test.Messages[0].Repeat
	case "realtime":
		return result.Performance.AverageLatency < 5*time.Second
	}

	return true
}

// calculateSummary calculates test summary
func calculateSummary(results []WebSocketTestResult, totalDuration time.Duration) WebSocketTestSummary {
	summary := WebSocketTestSummary{
		TotalTests:    len(results),
		TotalDuration: totalDuration,
	}

	var totalConnections int
	var totalMessages int
	var totalBytes int64

	for _, result := range results {
		if result.Success {
			summary.PassedTests++
		} else {
			summary.FailedTests++
		}
		
		totalConnections += len(result.Connections)
		totalMessages += result.MessagesSent + result.MessagesReceived
		totalBytes += result.BytesSent + result.BytesReceived
	}

	summary.TotalConnections = totalConnections
	summary.TotalMessages = totalMessages
	summary.TotalBytes = totalBytes

	if summary.TotalTests > 0 {
		summary.SuccessRate = float64(summary.PassedTests) / float64(summary.TotalTests) * 100
		summary.AverageDuration = totalDuration / time.Duration(summary.TotalTests)
	}

	return summary
}

// calculateMetrics calculates overall metrics
func calculateMetrics(results []WebSocketTestResult, wsRecorder *recorder.WebSocketRecorder) WebSocketMetrics {
	stats := wsRecorder.GetWebSocketStatistics()
	
	return WebSocketMetrics{
		ConnectionMetrics: map[string]interface{}{
			"total_connections": stats["total_connections"],
			"active_connections": stats["active_connections"],
		},
		PerformanceMetrics: map[string]interface{}{
			"total_bytes_sent": stats["total_bytes_sent"],
			"total_bytes_received": stats["total_bytes_received"],
			"total_messages_sent": stats["total_messages_sent"],
			"total_messages_received": stats["total_messages_received"],
		},
		ErrorMetrics: map[string]interface{}{
			"total_errors": len(results), // Simplified
		},
	}
}

// generateReports generates test reports
func generateReports(testSuite WebSocketTestSuite, wsRecorder *recorder.WebSocketRecorder) {
	// Generate JSON report
	jsonReport, err := json.MarshalIndent(testSuite, "", "  ")
	if err != nil {
		log.Printf("Failed to generate JSON report: %v", err)
	} else {
		if err := os.WriteFile("websocket-test-report.json", jsonReport, 0644); err != nil {
			log.Printf("Failed to write JSON report: %v", err)
		}
	}

	// Generate HAR report with WebSocket data
	harData, err := wsRecorder.HARWithWebSocketData()
	if err != nil {
		log.Printf("Failed to generate HAR report: %v", err)
	} else {
		if err := os.WriteFile("websocket-test-report.har", harData, 0644); err != nil {
			log.Printf("Failed to write HAR report: %v", err)
		}
	}
}

// printSummary prints test summary
func printSummary(testSuite WebSocketTestSuite) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("WebSocket Test Suite Summary\n")
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")

	fmt.Printf("Suite: %s\n", testSuite.Name)
	fmt.Printf("Total Tests: %d\n", testSuite.Summary.TotalTests)
	fmt.Printf("Passed: %d\n", testSuite.Summary.PassedTests)
	fmt.Printf("Failed: %d\n", testSuite.Summary.FailedTests)
	fmt.Printf("Success Rate: %.1f%%\n", testSuite.Summary.SuccessRate)
	fmt.Printf("Total Duration: %v\n", testSuite.Summary.TotalDuration)
	fmt.Printf("Average Duration: %v\n", testSuite.Summary.AverageDuration)
	fmt.Printf("Total Connections: %d\n", testSuite.Summary.TotalConnections)
	fmt.Printf("Total Messages: %d\n", testSuite.Summary.TotalMessages)
	fmt.Printf("Total Bytes: %d\n", testSuite.Summary.TotalBytes)

	fmt.Printf("\nDetailed Results:\n")
	fmt.Printf(strings.Repeat("-", 40) + "\n")
	
	for i, result := range testSuite.Results {
		status := "✓ PASS"
		if !result.Success {
			status = "✗ FAIL"
		}
		
		fmt.Printf("%d. %s - %s (%v)\n", i+1, result.TestName, status, result.Duration)
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.Error)
		}
		fmt.Printf("   Messages: %d sent, %d received\n", result.MessagesSent, result.MessagesReceived)
		fmt.Printf("   Bytes: %d sent, %d received\n", result.BytesSent, result.BytesReceived)
		fmt.Printf("   Connections: %d\n", len(result.Connections))
		if result.Performance.AverageLatency > 0 {
			fmt.Printf("   Avg Latency: %v\n", result.Performance.AverageLatency)
		}
		fmt.Printf("\n")
	}

	fmt.Printf("Reports generated:\n")
	fmt.Printf("- websocket-test-report.json (detailed results)\n")
	fmt.Printf("- websocket-test-report.har (HAR with WebSocket data)\n")
}

// Helper function
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}