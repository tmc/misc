// Socket.IO WebSocket testing with chrome-to-har
// This example demonstrates testing Socket.IO applications with WebSocket monitoring
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

// SocketIOEvent represents a Socket.IO event
type SocketIOEvent struct {
	Type      string        `json:"type"`
	Event     string        `json:"event"`
	Data      interface{}   `json:"data"`
	Timestamp time.Time     `json:"timestamp"`
	Namespace string        `json:"namespace,omitempty"`
	Ack       bool          `json:"ack,omitempty"`
}

// SocketIOTestCase represents a Socket.IO test case
type SocketIOTestCase struct {
	Name          string             `json:"name"`
	URL           string             `json:"url"`
	Namespace     string             `json:"namespace,omitempty"`
	Events        []SocketIOEvent    `json:"events"`
	Expected      SocketIOExpected   `json:"expected"`
	Duration      time.Duration      `json:"duration"`
	Timeout       time.Duration      `json:"timeout"`
	Reconnection  bool               `json:"reconnection"`
	Auth          map[string]string  `json:"auth,omitempty"`
}

// SocketIOExpected represents expected Socket.IO behavior
type SocketIOExpected struct {
	Connected       bool              `json:"connected"`
	Events          []string          `json:"events"`
	Acks            []string          `json:"acks"`
	Disconnected    bool              `json:"disconnected"`
	Reconnections   int               `json:"reconnections"`
	ResponseTime    time.Duration     `json:"response_time"`
	EventCounts     map[string]int    `json:"event_counts"`
}

// SocketIOTestResult represents Socket.IO test results
type SocketIOTestResult struct {
	TestCase         string                    `json:"test_case"`
	URL              string                    `json:"url"`
	Success          bool                      `json:"success"`
	Error            string                    `json:"error,omitempty"`
	Duration         time.Duration             `json:"duration"`
	Connected        bool                      `json:"connected"`
	Disconnected     bool                      `json:"disconnected"`
	Reconnections    int                       `json:"reconnections"`
	EventsSent       int                       `json:"events_sent"`
	EventsReceived   int                       `json:"events_received"`
	AcksReceived     int                       `json:"acks_received"`
	AverageLatency   time.Duration             `json:"average_latency"`
	Events           []SocketIOEvent           `json:"events"`
	Connections      []SocketIOConnection      `json:"connections"`
	Performance      SocketIOPerformance       `json:"performance"`
	Transport        string                    `json:"transport"`
	Protocol         string                    `json:"protocol"`
	Issues           []string                  `json:"issues"`
}

// SocketIOConnection represents a Socket.IO connection
type SocketIOConnection struct {
	ID               string        `json:"id"`
	Connected        bool          `json:"connected"`
	Transport        string        `json:"transport"`
	Namespace        string        `json:"namespace"`
	ConnectionTime   time.Duration `json:"connection_time"`
	LastPing         time.Time     `json:"last_ping"`
	LastPong         time.Time     `json:"last_pong"`
	Upgrades         []string      `json:"upgrades"`
}

// SocketIOPerformance represents Socket.IO performance metrics
type SocketIOPerformance struct {
	ConnectionLatency time.Duration `json:"connection_latency"`
	EventLatency      time.Duration `json:"event_latency"`
	PingLatency       time.Duration `json:"ping_latency"`
	Throughput        float64       `json:"throughput"`
	ErrorRate         float64       `json:"error_rate"`
	Uptime            time.Duration `json:"uptime"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run socketio-tester.go <socketio-url>")
		fmt.Println("Example: go run socketio-tester.go http://localhost:3000")
		fmt.Println("Example: go run socketio-tester.go https://socket.io-server.com")
		os.Exit(1)
	}

	socketIOURL := os.Args[1]

	// Create Socket.IO test cases
	testCases := []SocketIOTestCase{
		{
			Name:     "Basic Socket.IO Connection",
			URL:      socketIOURL,
			Duration: 10 * time.Second,
			Timeout:  15 * time.Second,
			Expected: SocketIOExpected{
				Connected:    true,
				Events:       []string{"connect"},
				ResponseTime: 2 * time.Second,
			},
		},
		{
			Name:     "Socket.IO Event Communication",
			URL:      socketIOURL,
			Duration: 20 * time.Second,
			Timeout:  30 * time.Second,
			Events: []SocketIOEvent{
				{Type: "emit", Event: "hello", Data: "world"},
				{Type: "emit", Event: "test", Data: map[string]interface{}{"message": "test data"}},
				{Type: "emit", Event: "ping", Data: time.Now().Unix()},
			},
			Expected: SocketIOExpected{
				Connected:    true,
				Events:       []string{"connect", "hello", "test", "ping"},
				ResponseTime: 1 * time.Second,
				EventCounts:  map[string]int{"hello": 1, "test": 1, "ping": 1},
			},
		},
		{
			Name:      "Socket.IO Namespaces",
			URL:       socketIOURL,
			Namespace: "/admin",
			Duration:  15 * time.Second,
			Timeout:   20 * time.Second,
			Events: []SocketIOEvent{
				{Type: "emit", Event: "admin-action", Data: "status"},
				{Type: "emit", Event: "admin-query", Data: map[string]interface{}{"query": "users"}},
			},
			Expected: SocketIOExpected{
				Connected:    true,
				Events:       []string{"connect", "admin-action", "admin-query"},
				ResponseTime: 1 * time.Second,
			},
		},
		{
			Name:         "Socket.IO Reconnection Test",
			URL:          socketIOURL,
			Duration:     30 * time.Second,
			Timeout:      45 * time.Second,
			Reconnection: true,
			Expected: SocketIOExpected{
				Connected:     true,
				Reconnections: 1,
				Events:        []string{"connect", "disconnect", "reconnect"},
				ResponseTime:  3 * time.Second,
			},
		},
		{
			Name:     "Socket.IO Performance Test",
			URL:      socketIOURL,
			Duration: 60 * time.Second,
			Timeout:  90 * time.Second,
			Events: []SocketIOEvent{
				{Type: "emit", Event: "performance", Data: "load test", Timestamp: time.Now()},
			},
			Expected: SocketIOExpected{
				Connected:    true,
				Events:       []string{"connect", "performance"},
				ResponseTime: 500 * time.Millisecond,
				EventCounts:  map[string]int{"performance": 100},
			},
		},
		{
			Name:     "Socket.IO Authentication",
			URL:      socketIOURL,
			Duration: 15 * time.Second,
			Timeout:  20 * time.Second,
			Auth: map[string]string{
				"token": "test-token",
				"user":  "test-user",
			},
			Events: []SocketIOEvent{
				{Type: "emit", Event: "authenticated", Data: true},
				{Type: "emit", Event: "private-message", Data: "secure data"},
			},
			Expected: SocketIOExpected{
				Connected:    true,
				Events:       []string{"connect", "authenticated", "private-message"},
				ResponseTime: 1 * time.Second,
			},
		},
	}

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create browser
	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		log.Fatalf("Failed to create profile manager: %v", err)
	}

	b, err := browser.New(ctx, pm)
	if err != nil {
		log.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		log.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	// Create WebSocket recorder
	wsRecorder, err := recorder.NewWebSocketRecorder(
		recorder.WithVerbose(true),
		recorder.WithStreaming(true),
	)
	if err != nil {
		log.Fatalf("Failed to create WebSocket recorder: %v", err)
	}

	// Run test cases
	var results []SocketIOTestResult
	fmt.Printf("Testing Socket.IO server: %s\n", socketIOURL)
	fmt.Printf("Running %d test cases\n\n", len(testCases))

	for i, testCase := range testCases {
		fmt.Printf("[%d/%d] Testing: %s\n", i+1, len(testCases), testCase.Name)
		
		result := runSocketIOTest(ctx, b, wsRecorder, testCase)
		results = append(results, result)
		
		if result.Success {
			fmt.Printf("  ✓ PASSED - %d events, avg latency: %v\n", 
				result.EventsReceived, result.AverageLatency)
		} else {
			fmt.Printf("  ✗ FAILED - %s\n", result.Error)
		}
		
		// Brief pause between tests
		time.Sleep(2 * time.Second)
	}

	// Generate reports
	generateSocketIOReport(results, wsRecorder)
	printSocketIOSummary(results)
}

// runSocketIOTest runs a single Socket.IO test
func runSocketIOTest(ctx context.Context, b *browser.Browser, wsRecorder *recorder.WebSocketRecorder, testCase SocketIOTestCase) SocketIOTestResult {
	testCtx, cancel := context.WithTimeout(ctx, testCase.Timeout)
	defer cancel()

	result := SocketIOTestResult{
		TestCase:    testCase.Name,
		URL:         testCase.URL,
		Events:      []SocketIOEvent{},
		Connections: []SocketIOConnection{},
		Issues:      []string{},
	}

	startTime := time.Now()

	// Create page for test
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
			result.Connected = true
			result.Connections = append(result.Connections, SocketIOConnection{
				ID:             conn.ID,
				Connected:      true,
				Transport:      detectTransport(conn.URL),
				ConnectionTime: conn.ConnectionLatency,
				LastPing:       time.Now(),
			})
		},
		func(conn *browser.WebSocketConnection) {
			result.Disconnected = true
			// Update connection info
			for i, connection := range result.Connections {
				if connection.ID == conn.ID {
					result.Connections[i].Connected = false
					break
				}
			}
		},
		func(conn *browser.WebSocketConnection, err error) {
			result.Issues = append(result.Issues, fmt.Sprintf("Connection error: %v", err))
		},
	)

	page.SetWebSocketFrameHandler(
		func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
			result.EventsReceived++
			
			// Parse Socket.IO event
			if event, err := parseSocketIOEvent(frame.Data, "received"); err == nil {
				result.Events = append(result.Events, event)
			}
		},
		func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
			result.EventsSent++
			
			// Parse Socket.IO event
			if event, err := parseSocketIOEvent(frame.Data, "sent"); err == nil {
				result.Events = append(result.Events, event)
			}
		},
	)

	// Navigate to Socket.IO test page
	socketIOHTML := generateSocketIOHTML(testCase)
	if err := page.Navigate(fmt.Sprintf("data:text/html,%s", socketIOHTML)); err != nil {
		result.Error = fmt.Sprintf("Failed to navigate to test page: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Wait for test completion
	time.Sleep(testCase.Duration)

	// Collect final metrics
	connections := page.GetWebSocketConnections()
	result.Performance = calculateSocketIOPerformance(result.Events, connections, startTime)

	// Validate results
	result.Success = validateSocketIOResults(testCase, result)

	result.Duration = time.Since(startTime)
	return result
}

// generateSocketIOHTML generates HTML for Socket.IO testing
func generateSocketIOHTML(testCase SocketIOTestCase) string {
	var eventsJS strings.Builder
	for _, event := range testCase.Events {
		eventData, _ := json.Marshal(event.Data)
		eventsJS.WriteString(fmt.Sprintf(`
			setTimeout(() => {
				socket.emit('%s', %s);
				console.log('Emitted event: %s');
			}, %d);
		`, event.Event, eventData, event.Event, int(event.Timestamp.Sub(time.Now()).Milliseconds())))
	}

	authJS := ""
	if len(testCase.Auth) > 0 {
		authData, _ := json.Marshal(testCase.Auth)
		authJS = fmt.Sprintf(`auth: %s,`, authData)
	}

	namespaceURL := testCase.URL
	if testCase.Namespace != "" {
		namespaceURL += testCase.Namespace
	}

	return fmt.Sprintf(`
		<html>
		<head>
			<title>Socket.IO Test: %s</title>
			<script src="https://cdn.socket.io/4.0.0/socket.io.min.js"></script>
		</head>
		<body>
		<h1>Socket.IO Test: %s</h1>
		<div id="status">Connecting...</div>
		<div id="events"></div>
		<div id="transport">Transport: <span id="transport-type">unknown</span></div>
		
		<script>
			let socket;
			let eventCount = 0;
			let connected = false;
			
			try {
				socket = io('%s', {
					%s
					transports: ['websocket', 'polling'],
					upgrade: true,
					reconnection: %t,
					timeout: 5000
				});
				
				socket.on('connect', () => {
					connected = true;
					document.getElementById('status').textContent = 'Connected (ID: ' + socket.id + ')';
					document.getElementById('transport-type').textContent = socket.io.engine.transport.name;
					console.log('Connected to Socket.IO server');
				});
				
				socket.on('disconnect', (reason) => {
					connected = false;
					document.getElementById('status').textContent = 'Disconnected: ' + reason;
					console.log('Disconnected from Socket.IO server:', reason);
				});
				
				socket.on('reconnect', (attemptNumber) => {
					console.log('Reconnected after', attemptNumber, 'attempts');
				});
				
				socket.on('error', (error) => {
					console.error('Socket.IO error:', error);
					document.getElementById('status').textContent = 'Error: ' + error;
				});
				
				// Listen for any event
				socket.onAny((event, ...args) => {
					eventCount++;
					const eventsDiv = document.getElementById('events');
					const eventElement = document.createElement('div');
					eventElement.textContent = 'Event ' + eventCount + ': ' + event + ' - ' + JSON.stringify(args);
					eventsDiv.appendChild(eventElement);
					console.log('Received event:', event, args);
				});
				
				// Send test events
				socket.on('connect', () => {
					%s
					
					// Send periodic ping for performance testing
					setInterval(() => {
						if (connected) {
							socket.emit('ping', Date.now());
						}
					}, 1000);
				});
				
				// Handle pong for latency calculation
				socket.on('pong', (timestamp) => {
					const latency = Date.now() - timestamp;
					console.log('Ping latency:', latency + 'ms');
				});
				
				// Transport upgrade handling
				socket.io.on('upgrade', (transport) => {
					console.log('Transport upgraded to:', transport.name);
					document.getElementById('transport-type').textContent = transport.name;
				});
				
			} catch (error) {
				console.error('Failed to initialize Socket.IO:', error);
				document.getElementById('status').textContent = 'Failed to initialize';
			}
		</script>
		</body>
		</html>
	`, testCase.Name, testCase.Name, namespaceURL, authJS, testCase.Reconnection, eventsJS.String())
}

// parseSocketIOEvent parses Socket.IO event from WebSocket frame data
func parseSocketIOEvent(data interface{}, direction string) (SocketIOEvent, error) {
	event := SocketIOEvent{
		Type:      direction,
		Timestamp: time.Now(),
	}

	dataStr := fmt.Sprintf("%v", data)
	
	// Simple Socket.IO protocol parsing
	if strings.HasPrefix(dataStr, "42") {
		// Engine.IO packet type 4 (message) + Socket.IO packet type 2 (event)
		payload := dataStr[2:]
		var eventData []interface{}
		if err := json.Unmarshal([]byte(payload), &eventData); err == nil {
			if len(eventData) > 0 {
				if eventName, ok := eventData[0].(string); ok {
					event.Event = eventName
					if len(eventData) > 1 {
						event.Data = eventData[1:]
					}
				}
			}
		}
	} else if strings.HasPrefix(dataStr, "40") {
		// Engine.IO packet type 4 + Socket.IO packet type 0 (connect)
		event.Event = "connect"
	} else if strings.HasPrefix(dataStr, "41") {
		// Engine.IO packet type 4 + Socket.IO packet type 1 (disconnect)
		event.Event = "disconnect"
	} else if strings.HasPrefix(dataStr, "2") {
		// Engine.IO ping
		event.Event = "ping"
	} else if strings.HasPrefix(dataStr, "3") {
		// Engine.IO pong
		event.Event = "pong"
	}

	return event, nil
}

// detectTransport detects the transport type from WebSocket URL
func detectTransport(url string) string {
	if strings.Contains(url, "websocket") {
		return "websocket"
	} else if strings.Contains(url, "polling") {
		return "polling"
	}
	return "unknown"
}

// calculateSocketIOPerformance calculates Socket.IO performance metrics
func calculateSocketIOPerformance(events []SocketIOEvent, connections map[string]*browser.WebSocketConnection, startTime time.Time) SocketIOPerformance {
	performance := SocketIOPerformance{}

	if len(connections) > 0 {
		for _, conn := range connections {
			performance.ConnectionLatency = conn.ConnectionLatency
			performance.Uptime = time.Since(conn.ConnectedAt)
			break
		}
	}

	// Calculate event latency
	var latencies []time.Duration
	for i := 1; i < len(events); i++ {
		if events[i-1].Event == "ping" && events[i].Event == "pong" {
			latency := events[i].Timestamp.Sub(events[i-1].Timestamp)
			latencies = append(latencies, latency)
		}
	}

	if len(latencies) > 0 {
		var totalLatency time.Duration
		for _, latency := range latencies {
			totalLatency += latency
		}
		performance.EventLatency = totalLatency / time.Duration(len(latencies))
		performance.PingLatency = performance.EventLatency
	}

	// Calculate throughput
	duration := time.Since(startTime)
	if duration > 0 {
		performance.Throughput = float64(len(events)) / duration.Seconds()
	}

	return performance
}

// validateSocketIOResults validates Socket.IO test results
func validateSocketIOResults(testCase SocketIOTestCase, result SocketIOTestResult) bool {
	expected := testCase.Expected

	// Check connection
	if expected.Connected && !result.Connected {
		return false
	}

	// Check events
	eventCounts := make(map[string]int)
	for _, event := range result.Events {
		eventCounts[event.Event]++
	}

	for _, expectedEvent := range expected.Events {
		if eventCounts[expectedEvent] == 0 {
			return false
		}
	}

	// Check event counts
	for event, expectedCount := range expected.EventCounts {
		if eventCounts[event] < expectedCount {
			return false
		}
	}

	// Check response time
	if expected.ResponseTime > 0 && result.AverageLatency > expected.ResponseTime {
		return false
	}

	// Check reconnections
	if expected.Reconnections > 0 && result.Reconnections < expected.Reconnections {
		return false
	}

	return true
}

// generateSocketIOReport generates Socket.IO test report
func generateSocketIOReport(results []SocketIOTestResult, wsRecorder *recorder.WebSocketRecorder) {
	// Generate JSON report
	jsonReport, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("Failed to generate JSON report: %v", err)
	} else {
		if err := os.WriteFile("socketio-test-report.json", jsonReport, 0644); err != nil {
			log.Printf("Failed to write JSON report: %v", err)
		}
	}

	// Generate HAR report
	harData, err := wsRecorder.HARWithWebSocketData()
	if err != nil {
		log.Printf("Failed to generate HAR report: %v", err)
	} else {
		if err := os.WriteFile("socketio-test-report.har", harData, 0644); err != nil {
			log.Printf("Failed to write HAR report: %v", err)
		}
	}

	fmt.Printf("\nReports generated:\n")
	fmt.Printf("- socketio-test-report.json (detailed results)\n")
	fmt.Printf("- socketio-test-report.har (HAR with WebSocket data)\n")
}

// printSocketIOSummary prints Socket.IO test summary
func printSocketIOSummary(results []SocketIOTestResult) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("Socket.IO Test Summary\n")
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")

	passed := 0
	failed := 0
	totalEvents := 0
	var totalLatency time.Duration
	latencyCount := 0

	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
		
		totalEvents += result.EventsReceived
		if result.AverageLatency > 0 {
			totalLatency += result.AverageLatency
			latencyCount++
		}
	}

	fmt.Printf("Total Test Cases: %d\n", len(results))
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Success Rate: %.1f%%\n", float64(passed)/float64(len(results))*100)
	fmt.Printf("Total Events: %d\n", totalEvents)
	
	if latencyCount > 0 {
		avgLatency := totalLatency / time.Duration(latencyCount)
		fmt.Printf("Average Event Latency: %v\n", avgLatency)
	}

	fmt.Printf("\nTest Case Results:\n")
	fmt.Printf(strings.Repeat("-", 40) + "\n")
	
	for i, result := range results {
		status := "✓ PASS"
		if !result.Success {
			status = "✗ FAIL"
		}
		
		fmt.Printf("%d. %s - %s\n", i+1, result.TestCase, status)
		fmt.Printf("   Duration: %v\n", result.Duration)
		fmt.Printf("   Connected: %t\n", result.Connected)
		fmt.Printf("   Events: %d sent, %d received\n", result.EventsSent, result.EventsReceived)
		fmt.Printf("   Transport: %s\n", result.Transport)
		fmt.Printf("   Latency: %v\n", result.AverageLatency)
		
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.Error)
		}
		
		if len(result.Issues) > 0 {
			fmt.Printf("   Issues: %d\n", len(result.Issues))
		}
		
		fmt.Printf("\n")
	}
}