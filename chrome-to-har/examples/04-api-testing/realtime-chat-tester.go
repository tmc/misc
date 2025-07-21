// Real-time chat application WebSocket tester
// This example demonstrates testing real-time chat applications with WebSocket monitoring
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

// ChatMessage represents a chat message
type ChatMessage struct {
	ID        string    `json:"id"`
	User      string    `json:"user"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "message", "join", "leave", "typing"
}

// ChatTestScenario represents a chat testing scenario
type ChatTestScenario struct {
	Name        string          `json:"name"`
	ChatURL     string          `json:"chat_url"`
	Users       []ChatUser      `json:"users"`
	Duration    time.Duration   `json:"duration"`
	Messages    []ChatMessage   `json:"messages"`
	Expected    ChatExpectations `json:"expected"`
}

// ChatUser represents a chat user
type ChatUser struct {
	Name      string `json:"name"`
	JoinDelay time.Duration `json:"join_delay"`
	Active    bool   `json:"active"`
}

// ChatExpectations represents expected chat behavior
type ChatExpectations struct {
	MinMessages      int           `json:"min_messages"`
	MaxLatency       time.Duration `json:"max_latency"`
	ExpectedUsers    []string      `json:"expected_users"`
	MessageDelivery  bool          `json:"message_delivery"`
	UserPresence     bool          `json:"user_presence"`
	TypingIndicators bool          `json:"typing_indicators"`
}

// ChatTestResult represents chat test results
type ChatTestResult struct {
	Scenario        string                   `json:"scenario"`
	Success         bool                     `json:"success"`
	Error           string                   `json:"error,omitempty"`
	Duration        time.Duration            `json:"duration"`
	UsersJoined     []string                 `json:"users_joined"`
	MessagesExchanged int                    `json:"messages_exchanged"`
	AverageLatency  time.Duration            `json:"average_latency"`
	MaxLatency      time.Duration            `json:"max_latency"`
	Connections     []ChatConnection         `json:"connections"`
	Messages        []ChatMessage            `json:"messages"`
	Performance     ChatPerformanceMetrics   `json:"performance"`
	Issues          []string                 `json:"issues"`
}

// ChatConnection represents a chat connection
type ChatConnection struct {
	User           string        `json:"user"`
	Connected      bool          `json:"connected"`
	ConnectionTime time.Duration `json:"connection_time"`
	MessagesSent   int           `json:"messages_sent"`
	MessagesReceived int         `json:"messages_received"`
	LastActivity   time.Time     `json:"last_activity"`
}

// ChatPerformanceMetrics represents chat performance metrics
type ChatPerformanceMetrics struct {
	MessageDeliveryRate float64       `json:"message_delivery_rate"`
	AverageLatency      time.Duration `json:"average_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	ConnectionUptime    time.Duration `json:"connection_uptime"`
	ReconnectionCount   int           `json:"reconnection_count"`
	ErrorRate           float64       `json:"error_rate"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run realtime-chat-tester.go <chat-url>")
		fmt.Println("Example: go run realtime-chat-tester.go ws://localhost:3000/chat")
		os.Exit(1)
	}

	chatURL := os.Args[1]

	// Create test scenarios
	scenarios := []ChatTestScenario{
		{
			Name:     "Basic Chat Functionality",
			ChatURL:  chatURL,
			Duration: 30 * time.Second,
			Users: []ChatUser{
				{Name: "Alice", JoinDelay: 0, Active: true},
				{Name: "Bob", JoinDelay: 2 * time.Second, Active: true},
			},
			Expected: ChatExpectations{
				MinMessages:      5,
				MaxLatency:       2 * time.Second,
				ExpectedUsers:    []string{"Alice", "Bob"},
				MessageDelivery:  true,
				UserPresence:     true,
				TypingIndicators: false,
			},
		},
		{
			Name:     "Multi-User Chat",
			ChatURL:  chatURL,
			Duration: 60 * time.Second,
			Users: []ChatUser{
				{Name: "Alice", JoinDelay: 0, Active: true},
				{Name: "Bob", JoinDelay: 5 * time.Second, Active: true},
				{Name: "Charlie", JoinDelay: 10 * time.Second, Active: true},
				{Name: "David", JoinDelay: 15 * time.Second, Active: true},
			},
			Expected: ChatExpectations{
				MinMessages:      15,
				MaxLatency:       3 * time.Second,
				ExpectedUsers:    []string{"Alice", "Bob", "Charlie", "David"},
				MessageDelivery:  true,
				UserPresence:     true,
				TypingIndicators: true,
			},
		},
		{
			Name:     "High-Frequency Messaging",
			ChatURL:  chatURL,
			Duration: 45 * time.Second,
			Users: []ChatUser{
				{Name: "SpeedUser1", JoinDelay: 0, Active: true},
				{Name: "SpeedUser2", JoinDelay: 1 * time.Second, Active: true},
			},
			Expected: ChatExpectations{
				MinMessages:      50,
				MaxLatency:       1 * time.Second,
				ExpectedUsers:    []string{"SpeedUser1", "SpeedUser2"},
				MessageDelivery:  true,
				UserPresence:     true,
				TypingIndicators: false,
			},
		},
		{
			Name:     "User Join/Leave Behavior",
			ChatURL:  chatURL,
			Duration: 40 * time.Second,
			Users: []ChatUser{
				{Name: "Persistent", JoinDelay: 0, Active: true},
				{Name: "Joiner1", JoinDelay: 10 * time.Second, Active: true},
				{Name: "Joiner2", JoinDelay: 20 * time.Second, Active: true},
			},
			Expected: ChatExpectations{
				MinMessages:      8,
				MaxLatency:       2 * time.Second,
				ExpectedUsers:    []string{"Persistent", "Joiner1", "Joiner2"},
				MessageDelivery:  true,
				UserPresence:     true,
				TypingIndicators: false,
			},
		},
	}

	// Create context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

	// Run test scenarios
	var results []ChatTestResult
	fmt.Printf("Testing chat application: %s\n", chatURL)
	fmt.Printf("Running %d scenarios\n\n", len(scenarios))

	for i, scenario := range scenarios {
		fmt.Printf("[%d/%d] Testing scenario: %s\n", i+1, len(scenarios), scenario.Name)
		
		result := runChatTestScenario(ctx, b, wsRecorder, scenario)
		results = append(results, result)
		
		if result.Success {
			fmt.Printf("  ✓ PASSED - %d messages exchanged, avg latency: %v\n", 
				result.MessagesExchanged, result.AverageLatency)
		} else {
			fmt.Printf("  ✗ FAILED - %s\n", result.Error)
		}
		
		// Brief pause between scenarios
		time.Sleep(2 * time.Second)
	}

	// Generate reports
	generateChatTestReport(results, wsRecorder)
	printChatTestSummary(results)
}

// runChatTestScenario runs a single chat test scenario
func runChatTestScenario(ctx context.Context, b *browser.Browser, wsRecorder *recorder.WebSocketRecorder, scenario ChatTestScenario) ChatTestResult {
	testCtx, cancel := context.WithTimeout(ctx, scenario.Duration+30*time.Second)
	defer cancel()

	result := ChatTestResult{
		Scenario:    scenario.Name,
		Connections: []ChatConnection{},
		Messages:    []ChatMessage{},
		Issues:      []string{},
	}

	startTime := time.Now()

	// Create pages for each user
	var pages []*browser.Page
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, user := range scenario.Users {
		if !user.Active {
			continue
		}

		page, err := b.NewPage()
		if err != nil {
			result.Error = fmt.Sprintf("Failed to create page for user %s: %v", user.Name, err)
			result.Duration = time.Since(startTime)
			return result
		}
		pages = append(pages, page)

		// Enable WebSocket monitoring
		if err := page.EnableWebSocketMonitoring(); err != nil {
			result.Error = fmt.Sprintf("Failed to enable WebSocket monitoring for user %s: %v", user.Name, err)
			result.Duration = time.Since(startTime)
			return result
		}

		// Set up connection tracking
		connection := ChatConnection{
			User:      user.Name,
			Connected: false,
		}

		page.SetWebSocketConnectionHandler(
			func(conn *browser.WebSocketConnection) {
				mu.Lock()
				connection.Connected = true
				connection.ConnectionTime = conn.ConnectionLatency
				connection.LastActivity = time.Now()
				result.UsersJoined = append(result.UsersJoined, user.Name)
				mu.Unlock()
			},
			func(conn *browser.WebSocketConnection) {
				mu.Lock()
				connection.Connected = false
				mu.Unlock()
			},
			func(conn *browser.WebSocketConnection, err error) {
				mu.Lock()
				result.Issues = append(result.Issues, fmt.Sprintf("Connection error for %s: %v", user.Name, err))
				mu.Unlock()
			},
		)

		page.SetWebSocketFrameHandler(
			func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
				mu.Lock()
				connection.MessagesReceived++
				connection.LastActivity = time.Now()
				
				// Try to parse chat message
				if msg, err := parseChatMessage(frame.Data); err == nil {
					result.Messages = append(result.Messages, msg)
				}
				mu.Unlock()
			},
			func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
				mu.Lock()
				connection.MessagesSent++
				connection.LastActivity = time.Now()
				mu.Unlock()
			},
		)

		result.Connections = append(result.Connections, connection)

		// Start user session
		wg.Add(1)
		go func(page *browser.Page, user ChatUser, connIndex int) {
			defer wg.Done()
			
			// Wait for join delay
			time.Sleep(user.JoinDelay)
			
			// Navigate to chat application
			chatHTML := generateChatHTML(scenario.ChatURL, user.Name)
			if err := page.Navigate(fmt.Sprintf("data:text/html,%s", chatHTML)); err != nil {
				mu.Lock()
				result.Issues = append(result.Issues, fmt.Sprintf("Failed to navigate for user %s: %v", user.Name, err))
				mu.Unlock()
				return
			}

			// Wait for connection
			time.Sleep(2 * time.Second)

			// Send messages periodically
			messageCount := 0
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-testCtx.Done():
					return
				case <-ticker.C:
					messageCount++
					message := fmt.Sprintf("Message %d from %s", messageCount, user.Name)
					
					// Send message via WebSocket
					connections := page.GetWebSocketConnections()
					for _, conn := range connections {
						if conn.State == "open" {
							msgData := map[string]interface{}{
								"type":    "message",
								"user":    user.Name,
								"message": message,
								"timestamp": time.Now().Unix(),
							}
							
							if err := page.SendWebSocketMessage(conn.ID, msgData); err != nil {
								mu.Lock()
								result.Issues = append(result.Issues, fmt.Sprintf("Failed to send message for %s: %v", user.Name, err))
								mu.Unlock()
							}
							break
						}
					}
					
					// Send typing indicator occasionally
					if messageCount%3 == 0 {
						connections := page.GetWebSocketConnections()
						for _, conn := range connections {
							if conn.State == "open" {
								typingData := map[string]interface{}{
									"type": "typing",
									"user": user.Name,
								}
								page.SendWebSocketMessage(conn.ID, typingData)
								break
							}
						}
					}
				}
			}
		}(page, user, len(result.Connections)-1)
	}

	// Wait for scenario duration
	time.Sleep(scenario.Duration)
	cancel()
	wg.Wait()

	// Calculate metrics
	result.Duration = time.Since(startTime)
	result.MessagesExchanged = len(result.Messages)
	result.Performance = calculateChatPerformanceMetrics(result.Messages, result.Connections)

	// Calculate latencies
	if len(result.Messages) > 0 {
		var totalLatency time.Duration
		var latencies []time.Duration
		
		for i := 1; i < len(result.Messages); i++ {
			if result.Messages[i].Type == "message" {
				latency := result.Messages[i].Timestamp.Sub(result.Messages[i-1].Timestamp)
				if latency > 0 && latency < 10*time.Second {
					totalLatency += latency
					latencies = append(latencies, latency)
				}
			}
		}
		
		if len(latencies) > 0 {
			result.AverageLatency = totalLatency / time.Duration(len(latencies))
			
			// Find max latency
			for _, latency := range latencies {
				if latency > result.MaxLatency {
					result.MaxLatency = latency
				}
			}
		}
	}

	// Validate results
	result.Success = validateChatResults(scenario, result)

	return result
}

// generateChatHTML generates HTML for chat testing
func generateChatHTML(chatURL, userName string) string {
	return fmt.Sprintf(`
		<html>
		<head><title>Chat Test - %s</title></head>
		<body>
		<h1>Chat Test User: %s</h1>
		<div id="status">Connecting...</div>
		<div id="messages" style="height: 300px; overflow-y: scroll; border: 1px solid #ccc; padding: 10px;"></div>
		<div>
			<input type="text" id="messageInput" placeholder="Type a message..." style="width: 300px;">
			<button onclick="sendMessage()">Send</button>
		</div>
		
		<script>
			let ws;
			let connected = false;
			let userName = '%s';
			
			function connect() {
				try {
					ws = new WebSocket('%s');
					
					ws.onopen = function(event) {
						connected = true;
						document.getElementById('status').textContent = 'Connected as ' + userName;
						
						// Send join message
						ws.send(JSON.stringify({
							type: 'join',
							user: userName,
							timestamp: Date.now()
						}));
					};
					
					ws.onmessage = function(event) {
						const data = JSON.parse(event.data);
						addMessage(data);
					};
					
					ws.onclose = function(event) {
						connected = false;
						document.getElementById('status').textContent = 'Disconnected';
					};
					
					ws.onerror = function(error) {
						console.error('WebSocket error:', error);
						document.getElementById('status').textContent = 'Error';
					};
					
				} catch (error) {
					console.error('Failed to connect:', error);
					document.getElementById('status').textContent = 'Failed to connect';
				}
			}
			
			function sendMessage() {
				const input = document.getElementById('messageInput');
				const message = input.value.trim();
				
				if (message && connected) {
					ws.send(JSON.stringify({
						type: 'message',
						user: userName,
						message: message,
						timestamp: Date.now()
					}));
					
					input.value = '';
				}
			}
			
			function addMessage(data) {
				const messagesDiv = document.getElementById('messages');
				const messageElement = document.createElement('div');
				
				let messageText = '';
				switch (data.type) {
					case 'message':
						messageText = data.user + ': ' + data.message;
						break;
					case 'join':
						messageText = data.user + ' joined the chat';
						break;
					case 'leave':
						messageText = data.user + ' left the chat';
						break;
					case 'typing':
						messageText = data.user + ' is typing...';
						break;
				}
				
				messageElement.textContent = '[' + new Date().toLocaleTimeString() + '] ' + messageText;
				messagesDiv.appendChild(messageElement);
				messagesDiv.scrollTop = messagesDiv.scrollHeight;
			}
			
			// Handle enter key in input
			document.getElementById('messageInput').addEventListener('keypress', function(e) {
				if (e.key === 'Enter') {
					sendMessage();
				}
			});
			
			// Start connection
			connect();
		</script>
		</body>
		</html>
	`, userName, userName, userName, chatURL)
}

// parseChatMessage parses chat message from WebSocket frame data
func parseChatMessage(data interface{}) (ChatMessage, error) {
	var msg ChatMessage
	
	switch v := data.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &msg); err != nil {
			return msg, err
		}
	case []byte:
		if err := json.Unmarshal(v, &msg); err != nil {
			return msg, err
		}
	default:
		// Try to marshal and unmarshal
		jsonData, err := json.Marshal(v)
		if err != nil {
			return msg, err
		}
		if err := json.Unmarshal(jsonData, &msg); err != nil {
			return msg, err
		}
	}
	
	return msg, nil
}

// calculateChatPerformanceMetrics calculates chat performance metrics
func calculateChatPerformanceMetrics(messages []ChatMessage, connections []ChatConnection) ChatPerformanceMetrics {
	metrics := ChatPerformanceMetrics{}
	
	if len(messages) == 0 {
		return metrics
	}
	
	// Calculate message delivery rate
	totalMessages := 0
	deliveredMessages := 0
	
	for _, conn := range connections {
		totalMessages += conn.MessagesSent
		deliveredMessages += conn.MessagesReceived
	}
	
	if totalMessages > 0 {
		metrics.MessageDeliveryRate = float64(deliveredMessages) / float64(totalMessages) * 100
	}
	
	// Calculate latencies
	var latencies []time.Duration
	for i := 1; i < len(messages); i++ {
		if messages[i].Type == "message" && messages[i-1].Type == "message" {
			latency := messages[i].Timestamp.Sub(messages[i-1].Timestamp)
			if latency > 0 && latency < 10*time.Second {
				latencies = append(latencies, latency)
			}
		}
	}
	
	if len(latencies) > 0 {
		var totalLatency time.Duration
		for _, latency := range latencies {
			totalLatency += latency
		}
		metrics.AverageLatency = totalLatency / time.Duration(len(latencies))
		
		// Calculate P95 latency (simplified)
		if len(latencies) > 0 {
			p95Index := int(0.95 * float64(len(latencies)))
			if p95Index < len(latencies) {
				metrics.P95Latency = latencies[p95Index]
			}
		}
	}
	
	// Calculate connection uptime
	var maxUptime time.Duration
	for _, conn := range connections {
		if conn.Connected {
			uptime := time.Since(conn.LastActivity)
			if uptime > maxUptime {
				maxUptime = uptime
			}
		}
	}
	metrics.ConnectionUptime = maxUptime
	
	return metrics
}

// validateChatResults validates chat test results
func validateChatResults(scenario ChatTestScenario, result ChatTestResult) bool {
	// Check minimum message count
	if result.MessagesExchanged < scenario.Expected.MinMessages {
		return false
	}
	
	// Check maximum latency
	if result.MaxLatency > scenario.Expected.MaxLatency {
		return false
	}
	
	// Check expected users joined
	if len(result.UsersJoined) < len(scenario.Expected.ExpectedUsers) {
		return false
	}
	
	// Check message delivery
	if scenario.Expected.MessageDelivery && result.Performance.MessageDeliveryRate < 80 {
		return false
	}
	
	// Check for critical issues
	if len(result.Issues) > len(scenario.Users) {
		return false
	}
	
	return true
}

// generateChatTestReport generates test report
func generateChatTestReport(results []ChatTestResult, wsRecorder *recorder.WebSocketRecorder) {
	// Generate JSON report
	jsonReport, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("Failed to generate JSON report: %v", err)
	} else {
		if err := os.WriteFile("chat-test-report.json", jsonReport, 0644); err != nil {
			log.Printf("Failed to write JSON report: %v", err)
		}
	}

	// Generate HAR report
	harData, err := wsRecorder.HARWithWebSocketData()
	if err != nil {
		log.Printf("Failed to generate HAR report: %v", err)
	} else {
		if err := os.WriteFile("chat-test-report.har", harData, 0644); err != nil {
			log.Printf("Failed to write HAR report: %v", err)
		}
	}
	
	fmt.Printf("\nReports generated:\n")
	fmt.Printf("- chat-test-report.json (detailed results)\n")
	fmt.Printf("- chat-test-report.har (HAR with WebSocket data)\n")
}

// printChatTestSummary prints test summary
func printChatTestSummary(results []ChatTestResult) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("Chat Application Test Summary\n")
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")

	passed := 0
	failed := 0
	totalMessages := 0
	var totalLatency time.Duration
	latencyCount := 0

	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
		
		totalMessages += result.MessagesExchanged
		if result.AverageLatency > 0 {
			totalLatency += result.AverageLatency
			latencyCount++
		}
	}

	fmt.Printf("Total Scenarios: %d\n", len(results))
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Success Rate: %.1f%%\n", float64(passed)/float64(len(results))*100)
	fmt.Printf("Total Messages Exchanged: %d\n", totalMessages)
	
	if latencyCount > 0 {
		avgLatency := totalLatency / time.Duration(latencyCount)
		fmt.Printf("Average Message Latency: %v\n", avgLatency)
	}

	fmt.Printf("\nScenario Results:\n")
	fmt.Printf(strings.Repeat("-", 40) + "\n")
	
	for i, result := range results {
		status := "✓ PASS"
		if !result.Success {
			status = "✗ FAIL"
		}
		
		fmt.Printf("%d. %s - %s\n", i+1, result.Scenario, status)
		fmt.Printf("   Duration: %v\n", result.Duration)
		fmt.Printf("   Users: %d joined\n", len(result.UsersJoined))
		fmt.Printf("   Messages: %d exchanged\n", result.MessagesExchanged)
		fmt.Printf("   Avg Latency: %v\n", result.AverageLatency)
		fmt.Printf("   Delivery Rate: %.1f%%\n", result.Performance.MessageDeliveryRate)
		
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.Error)
		}
		
		if len(result.Issues) > 0 {
			fmt.Printf("   Issues: %d\n", len(result.Issues))
		}
		
		fmt.Printf("\n")
	}
}