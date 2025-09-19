package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/testutil"
)

// skipIfNoChromish skips the test if no Chromium-based browser is available
func skipIfNoChromish(t testing.TB) {
	t.Helper()

	// Skip browser tests in CI only
	if os.Getenv("CI") != "" {
		t.Skip("Skipping browser test in CI environment")
	}

	if os.Getenv("CI") == "true" && runtime.GOOS != "linux" {
		t.Skip("Skipping browser test in CI on non-Linux platform")
	}

	chromePath := testutil.FindChrome()
	if chromePath == "" {
		t.Skip("No Chromium-based browser found (Chrome, Brave, Chromium, etc.), skipping browser tests")
	}
}

// TestWebSocketMonitoring tests basic WebSocket monitoring functionality
func TestWebSocketMonitoring(t *testing.T) {
	// Create test WebSocket server
	server := createTestWebSocketServer(t)
	defer server.Close()

	// Create browser and page
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		t.Fatalf("Failed to create profile manager: %v", err)
	}

	// Get the Chrome path from discovery
	chromePath := testutil.FindChrome()

	b, err := New(ctx, pm, WithChromePath(chromePath), WithSecurityProfile("permissive"))
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Enable WebSocket monitoring
	if err := page.EnableWebSocketMonitoring(); err != nil {
		t.Fatalf("Failed to enable WebSocket monitoring: %v", err)
	}

	// Track events
	var connections []*WebSocketConnection
	var frames []WebSocketFrame
	var mu sync.Mutex

	page.SetWebSocketConnectionHandler(
		func(conn *WebSocketConnection) {
			mu.Lock()
			connections = append(connections, conn)
			mu.Unlock()
		},
		func(conn *WebSocketConnection) {
			mu.Lock()
			// Update connection state
			for i, c := range connections {
				if c.ID == conn.ID {
					connections[i] = conn
					break
				}
			}
			mu.Unlock()
		},
		func(conn *WebSocketConnection, err error) {
			t.Logf("WebSocket error: %v", err)
		},
	)

	page.SetWebSocketFrameHandler(
		func(conn *WebSocketConnection, frame *WebSocketFrame) {
			mu.Lock()
			frames = append(frames, *frame)
			mu.Unlock()
		},
		func(conn *WebSocketConnection, frame *WebSocketFrame) {
			mu.Lock()
			frames = append(frames, *frame)
			mu.Unlock()
		},
	)

	// Navigate to test page
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	testHTML := fmt.Sprintf(`
		<html>
		<head><title>WebSocket Test</title></head>
		<body>
		<script>
			const ws = new WebSocket('%s');
			ws.onopen = function() {
				console.log('WebSocket connected');
				ws.send('Hello Server');
			};
			ws.onmessage = function(event) {
				console.log('Received:', event.data);
			};
			ws.onclose = function() {
				console.log('WebSocket disconnected');
			};
		</script>
		</body>
		</html>
	`, wsURL)

	if err := page.Navigate(fmt.Sprintf("data:text/html,%s", testHTML)); err != nil {
		t.Fatalf("Failed to navigate to test page: %v", err)
	}

	// Wait for WebSocket connection
	if err := waitForCondition(5*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(connections) > 0
	}); err != nil {
		t.Fatalf("WebSocket connection not established: %v", err)
	}

	// Wait for frames
	if err := waitForCondition(5*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(frames) > 0
	}); err != nil {
		t.Fatalf("WebSocket frames not received: %v", err)
	}

	// Verify connection
	mu.Lock()
	if len(connections) == 0 {
		t.Fatal("No WebSocket connections recorded")
	}
	if len(frames) == 0 {
		t.Fatal("No WebSocket frames recorded")
	}
	mu.Unlock()

	// Test WebSocket wait conditions
	conn, err := page.WaitForWebSocketConnection("*", 10*time.Second)
	if err != nil {
		t.Fatalf("Failed to wait for WebSocket connection: %v", err)
	}

	// Normalize URLs by removing trailing slashes for comparison
	expectedURL := strings.TrimSuffix(wsURL, "/")
	actualURL := strings.TrimSuffix(conn.URL, "/")
	if actualURL != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, actualURL)
	}

	// Test WebSocket statistics
	stats := page.GetWebSocketStats()
	if stats["active_connections"].(int) == 0 {
		t.Error("Expected active connections > 0")
	}
}

// TestWebSocketWaitConditions tests WebSocket wait conditions
func TestWebSocketWaitConditions(t *testing.T) {
	t.Parallel()
	skipIfNoChromish(t)

	// Create test WebSocket server
	server := createTestWebSocketServer(t)
	defer server.Close()

	// Create browser and page
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		t.Fatalf("Failed to create profile manager: %v", err)
	}

	// Get the Chrome path from discovery
	chromePath := testutil.FindChrome()

	b, err := New(ctx, pm, WithChromePath(chromePath), WithSecurityProfile("permissive"))
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Enable WebSocket monitoring
	if err := page.EnableWebSocketMonitoring(); err != nil {
		t.Fatalf("Failed to enable WebSocket monitoring: %v", err)
	}

	// Navigate to test page
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	testHTML := fmt.Sprintf(`
		<html>
		<head><title>WebSocket Test</title></head>
		<body>
		<script>
			setTimeout(() => {
				const ws = new WebSocket('%s');
				ws.onopen = function() {
					console.log('WebSocket connected');
					setTimeout(() => {
						ws.send('test message');
					}, 1000);
				};
				ws.onmessage = function(event) {
					console.log('Received:', event.data);
				};
			}, 2000);
		</script>
		</body>
		</html>
	`, wsURL)

	if err := page.Navigate(fmt.Sprintf("data:text/html,%s", testHTML)); err != nil {
		t.Fatalf("Failed to navigate to test page: %v", err)
	}

	// Test waiting for connection
	conn, err := page.WaitForWebSocket(WebSocketOpen, WithWebSocketWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Failed to wait for WebSocket connection: %v", err)
	}

	if conn.State != "open" {
		t.Errorf("Expected connection state 'open', got '%s'", conn.State)
	}

	// Test waiting for message
	_, err = page.WaitForWebSocket(WebSocketMessage, WithWebSocketWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Failed to wait for WebSocket message: %v", err)
	}

	// Test waiting for messages with count
	frames, err := page.WaitForWebSocketMessages(1, WithWebSocketWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("Failed to wait for WebSocket messages: %v", err)
	}

	if len(frames) == 0 {
		t.Error("Expected at least 1 frame")
	}
}

// TestWebSocketHARExport tests WebSocket HAR export functionality
func TestWebSocketHARExport(t *testing.T) {
	t.Parallel()
	skipIfNoChromish(t)

	// Create test WebSocket server
	server := createTestWebSocketServer(t)
	defer server.Close()

	// Create browser and page
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		t.Fatalf("Failed to create profile manager: %v", err)
	}

	// Get the Chrome path from discovery
	chromePath := testutil.FindChrome()

	b, err := New(ctx, pm, WithChromePath(chromePath), WithSecurityProfile("permissive"))
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Enable WebSocket monitoring
	if err := page.EnableWebSocketMonitoring(); err != nil {
		t.Fatalf("Failed to enable WebSocket monitoring: %v", err)
	}

	// Navigate to test page
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	testHTML := fmt.Sprintf(`
		<html>
		<head><title>WebSocket Test</title></head>
		<body>
		<script>
			const ws = new WebSocket('%s');
			ws.onopen = function() {
				ws.send('test message');
			};
			ws.onmessage = function(event) {
				console.log('Received:', event.data);
			};
		</script>
		</body>
		</html>
	`, wsURL)

	if err := page.Navigate(fmt.Sprintf("data:text/html,%s", testHTML)); err != nil {
		t.Fatalf("Failed to navigate to test page: %v", err)
	}

	// Wait for WebSocket activity
	time.Sleep(3 * time.Second)

	// Test HAR export
	connections := page.GetWebSocketConnections()
	if len(connections) == 0 {
		t.Fatal("No WebSocket connections found")
	}

	converter := NewWebSocketHARConverter()
	for _, conn := range connections {
		converter.AddConnection(conn)
	}

	entries := converter.ConvertToHAR()
	if len(entries) == 0 {
		t.Fatal("No HAR entries generated")
	}

	entry := entries[0]
	if entry.WebSocket == nil {
		t.Fatal("WebSocket data not included in HAR entry")
	}

	// Normalize URLs by removing trailing slashes for comparison
	expectedURL := strings.TrimSuffix(wsURL, "/")
	actualURL := strings.TrimSuffix(entry.WebSocket.URL, "/")
	if actualURL != expectedURL {
		t.Errorf("Expected URL %s, got %s", expectedURL, actualURL)
	}

	if len(entry.WebSocket.Messages) == 0 {
		t.Error("No messages in HAR WebSocket data")
	}
}

// TestWebSocketPerformanceMonitoring tests WebSocket performance monitoring
func TestWebSocketPerformanceMonitoring(t *testing.T) {
	t.Parallel()
	skipIfNoChromish(t)

	// Create test WebSocket server
	server := createTestWebSocketServer(t)
	defer server.Close()

	// Create browser and page
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		t.Fatalf("Failed to create profile manager: %v", err)
	}

	// Get the Chrome path from discovery
	chromePath := testutil.FindChrome()

	b, err := New(ctx, pm, WithChromePath(chromePath), WithSecurityProfile("permissive"))
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Enable WebSocket monitoring
	if err := page.EnableWebSocketMonitoring(); err != nil {
		t.Fatalf("Failed to enable WebSocket monitoring: %v", err)
	}

	// Start performance monitoring
	monitor, err := page.StartPerformanceMonitoring()
	if err != nil {
		t.Fatalf("Failed to start performance monitoring: %v", err)
	}

	// Navigate to test page
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	testHTML := fmt.Sprintf(`
		<html>
		<head><title>WebSocket Test</title></head>
		<body>
		<script>
			const ws = new WebSocket('%s');
			ws.onopen = function() {
				for (let i = 0; i < 10; i++) {
					setTimeout(() => {
						ws.send('message ' + i);
					}, i * 100);
				}
			};
			ws.onmessage = function(event) {
				console.log('Received:', event.data);
			};
		</script>
		</body>
		</html>
	`, wsURL)

	if err := page.Navigate(fmt.Sprintf("data:text/html,%s", testHTML)); err != nil {
		t.Fatalf("Failed to navigate to test page: %v", err)
	}

	// Wait for WebSocket activity
	time.Sleep(5 * time.Second)

	// Stop monitoring
	if err := monitor.Stop(); err != nil {
		t.Fatalf("Failed to stop performance monitoring: %v", err)
	}

	// Check metrics
	metrics := monitor.GetMetrics()
	if metrics.TotalConnections == 0 {
		t.Error("No connections recorded in performance metrics")
	}

	if metrics.MessagesSent == 0 && metrics.MessagesReceived == 0 {
		t.Error("No messages recorded in performance metrics")
	}

	// Check samples
	samples := monitor.GetSamples()
	if len(samples) == 0 {
		t.Error("No performance samples collected")
	}

	// Generate report
	report := monitor.GenerateReport()
	if report.Summary.OverallRating == "" {
		t.Error("Performance report summary not generated")
	}
}

// TestWebSocketFiltering tests WebSocket filtering functionality
func TestWebSocketFiltering(t *testing.T) {
	t.Parallel()
	skipIfNoChromish(t)

	// Create test WebSocket server
	server := createTestWebSocketServer(t)
	defer server.Close()

	// Create browser and page
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		t.Fatalf("Failed to create profile manager: %v", err)
	}

	// Get the Chrome path from discovery
	chromePath := testutil.FindChrome()

	b, err := New(ctx, pm, WithChromePath(chromePath), WithSecurityProfile("permissive"))
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Enable WebSocket monitoring
	if err := page.EnableWebSocketMonitoring(); err != nil {
		t.Fatalf("Failed to enable WebSocket monitoring: %v", err)
	}

	// Navigate to test page
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	testHTML := fmt.Sprintf(`
		<html>
		<head><title>WebSocket Test</title></head>
		<body>
		<script>
			const ws = new WebSocket('%s');
			ws.onopen = function() {
				ws.send('text message');
				ws.send('another message');
			};
			ws.onmessage = function(event) {
				console.log('Received:', event.data);
			};
		</script>
		</body>
		</html>
	`, wsURL)

	if err := page.Navigate(fmt.Sprintf("data:text/html,%s", testHTML)); err != nil {
		t.Fatalf("Failed to navigate to test page: %v", err)
	}

	// Wait for WebSocket activity
	time.Sleep(3 * time.Second)

	// Test filtering
	connections := page.GetWebSocketConnections()
	if len(connections) == 0 {
		t.Fatal("No WebSocket connections found")
	}

	// Create filter
	filter := &WebSocketHARFilter{
		URLPattern:  wsURL,
		MessageType: "text",
		Direction:   "sent",
	}

	// Convert to HAR entries
	converter := NewWebSocketHARConverter()
	for _, conn := range connections {
		converter.AddConnection(conn)
	}

	entries := converter.ConvertToHAR()
	filteredEntries := filter.ApplyFilter(entries)

	if len(filteredEntries) == 0 {
		t.Error("Filter removed all entries")
	}

	// Verify filtering
	for _, entry := range filteredEntries {
		if entry.WebSocket.URL != wsURL {
			t.Errorf("Filter did not work correctly: expected URL %s, got %s", wsURL, entry.WebSocket.URL)
		}
	}
}

// TestWebSocketMultipleConnections tests multiple WebSocket connections
func TestWebSocketMultipleConnections(t *testing.T) {
	t.Parallel()
	skipIfNoChromish(t)

	// Create multiple test WebSocket servers
	server1 := createTestWebSocketServer(t)
	defer server1.Close()

	server2 := createTestWebSocketServer(t)
	defer server2.Close()

	// Create browser and page
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		t.Fatalf("Failed to create profile manager: %v", err)
	}

	// Get the Chrome path from discovery
	chromePath := testutil.FindChrome()

	b, err := New(ctx, pm, WithChromePath(chromePath), WithSecurityProfile("permissive"))
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}

	if err := b.Launch(ctx); err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}

	// Enable WebSocket monitoring
	if err := page.EnableWebSocketMonitoring(); err != nil {
		t.Fatalf("Failed to enable WebSocket monitoring: %v", err)
	}

	// Navigate to test page with multiple connections
	wsURL1 := strings.Replace(server1.URL, "http://", "ws://", 1)
	wsURL2 := strings.Replace(server2.URL, "http://", "ws://", 1)
	testHTML := fmt.Sprintf(`
		<html>
		<head><title>WebSocket Test</title></head>
		<body>
		<script>
			const ws1 = new WebSocket('%s');
			const ws2 = new WebSocket('%s');
			
			ws1.onopen = function() {
				ws1.send('message from connection 1');
			};
			
			ws2.onopen = function() {
				ws2.send('message from connection 2');
			};
			
			ws1.onmessage = function(event) {
				console.log('Connection 1 received:', event.data);
			};
			
			ws2.onmessage = function(event) {
				console.log('Connection 2 received:', event.data);
			};
		</script>
		</body>
		</html>
	`, wsURL1, wsURL2)

	if err := page.Navigate(fmt.Sprintf("data:text/html,%s", testHTML)); err != nil {
		t.Fatalf("Failed to navigate to test page: %v", err)
	}

	// Wait for WebSocket connections
	if err := waitForCondition(10*time.Second, func() bool {
		connections := page.GetWebSocketConnections()
		return len(connections) == 2
	}); err != nil {
		t.Fatalf("Expected 2 WebSocket connections: %v", err)
	}

	// Verify connections
	connections := page.GetWebSocketConnections()
	if len(connections) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(connections))
	}

	// Verify different URLs
	urls := make(map[string]bool)
	for _, conn := range connections {
		urls[conn.URL] = true
	}

	if !urls[wsURL1] {
		t.Errorf("Connection to %s not found", wsURL1)
	}
	if !urls[wsURL2] {
		t.Errorf("Connection to %s not found", wsURL2)
	}
}

// createTestWebSocketServer creates a test WebSocket server
func createTestWebSocketServer(t *testing.T) *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("WebSocket upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Echo server
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			// Echo the message back
			if err := conn.WriteMessage(messageType, message); err != nil {
				break
			}
		}
	}))

	return server
}

// waitForCondition waits for a condition to be true with timeout
func waitForCondition(timeout time.Duration, condition func() bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if condition() {
				return nil
			}
		}
	}
}

// BenchmarkWebSocketMonitoring benchmarks WebSocket monitoring performance
func BenchmarkWebSocketMonitoring(b *testing.B) {
	// Create test WebSocket server
	server := createTestWebSocketServerForBenchmark(b)
	defer server.Close()

	// Create browser and page
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pm, err := chromeprofiles.NewProfileManager()
	if err != nil {
		b.Fatalf("Failed to create profile manager: %v", err)
	}

	browser, err := New(ctx, pm)
	if err != nil {
		b.Fatalf("Failed to create browser: %v", err)
	}

	if err := browser.Launch(ctx); err != nil {
		b.Fatalf("Failed to launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		b.Fatalf("Failed to create page: %v", err)
	}

	// Enable WebSocket monitoring
	if err := page.EnableWebSocketMonitoring(); err != nil {
		b.Fatalf("Failed to enable WebSocket monitoring: %v", err)
	}

	// Navigate to test page
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	testHTML := fmt.Sprintf(`
		<html>
		<head><title>WebSocket Test</title></head>
		<body>
		<script>
			const ws = new WebSocket('%s');
			ws.onopen = function() {
				window.sendMessage = function(msg) {
					ws.send(msg);
				};
			};
			ws.onmessage = function(event) {
				console.log('Received:', event.data);
			};
		</script>
		</body>
		</html>
	`, wsURL)

	if err := page.Navigate(fmt.Sprintf("data:text/html,%s", testHTML)); err != nil {
		b.Fatalf("Failed to navigate to test page: %v", err)
	}

	// Wait for connection
	time.Sleep(2 * time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Send message via WebSocket
			connections := page.GetWebSocketConnections()
			if len(connections) > 0 {
				for _, conn := range connections {
					page.SendWebSocketMessage(conn.ID, fmt.Sprintf("benchmark message %d", b.N))
					break
				}
			}
		}
	})
}

// createTestWebSocketServerForBenchmark creates a WebSocket server for benchmarking
func createTestWebSocketServerForBenchmark(b *testing.B) *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			b.Fatalf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		// Send a welcome message
		if err := conn.WriteMessage(websocket.TextMessage, []byte("Hello from server")); err != nil {
			b.Fatalf("Failed to send message: %v", err)
			return
		}

		// Read messages from client
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			// Echo back the message
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				break
			}
		}
	}))
}
