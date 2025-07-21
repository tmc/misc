// WebSocket API testing with comprehensive browser context
// This example shows how to test WebSocket connections and real-time APIs
// with full monitoring, HAR export, and performance metrics
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
	"github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
	"github.com/tmc/misc/chrome-to-har/internal/recorder"
)

type WebSocketTestResult struct {
	URL              string                 `json:"url"`
	Connected        bool                   `json:"connected"`
	ConnectionTime   float64                `json:"connection_time_ms"`
	MessagesSent     int                    `json:"messages_sent"`
	MessagesReceived int                    `json:"messages_received"`
	Success          bool                   `json:"success"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
	Duration         float64                `json:"duration_ms"`
	Messages         []WebSocketMessage     `json:"messages"`
	NetworkStats     NetworkStats           `json:"network_stats"`
}

type WebSocketMessage struct {
	Type      string      `json:"type"`      // "sent" or "received"
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

type NetworkStats struct {
	BytesSent     int64 `json:"bytes_sent"`
	BytesReceived int64 `json:"bytes_received"`
	MessageCount  int   `json:"message_count"`
}

type WebSocketTestCase struct {
	Name           string                 `json:"name"`
	URL            string                 `json:"url"`
	Protocol       string                 `json:"protocol,omitempty"`
	TestDuration   time.Duration          `json:"test_duration"`
	TestMessages   []interface{}          `json:"test_messages"`
	ExpectedEvents []string               `json:"expected_events"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run websocket-tester.go <websocket-url>")
		fmt.Println("Example: go run websocket-tester.go wss://api.example.com/ws")
		os.Exit(1)
	}

	wsURL := os.Args[1]
	
	// Define test cases
	testCases := []WebSocketTestCase{
		{
			Name:         "Basic Connection Test",
			URL:          wsURL,
			TestDuration: 5 * time.Second,
			TestMessages: []interface{}{
				map[string]interface{}{
					"type": "ping",
					"data": "hello",
				},
			},
			ExpectedEvents: []string{"open", "message"},
		},
		{
			Name:         "Echo Test",
			URL:          wsURL,
			TestDuration: 10 * time.Second,
			TestMessages: []interface{}{
				"test message 1",
				"test message 2",
				map[string]interface{}{
					"type":    "echo",
					"message": "hello world",
				},
			},
			ExpectedEvents: []string{"open", "message"},
		},
		{
			Name:         "Subscription Test",
			URL:          wsURL,
			TestDuration: 15 * time.Second,
			TestMessages: []interface{}{
				map[string]interface{}{
					"type": "subscribe",
					"channel": "updates",
				},
				map[string]interface{}{
					"type": "subscribe",
					"channel": "notifications",
				},
			},
			ExpectedEvents: []string{"open", "message", "subscription_confirmed"},
		},
		{
			Name:         "Stress Test",
			URL:          wsURL,
			TestDuration: 30 * time.Second,
			TestMessages: generateStressTestMessages(100),
			ExpectedEvents: []string{"open", "message"},
		},
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create Chrome browser
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (compatible; WebSocketTester/1.0)"),
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("enable-automation", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var results []WebSocketTestResult

	// Test each WebSocket scenario
	for _, testCase := range testCases {
		result := testWebSocket(chromeCtx, testCase)
		results = append(results, result)
		
		fmt.Printf("Testing %s: %s\n", 
			testCase.Name,
			map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		
		if !result.Success {
			fmt.Printf("  Error: %s\n", result.ErrorMessage)
		}
		
		// Small delay between tests
		time.Sleep(2 * time.Second)
	}

	// Generate report
	generateWebSocketReport(results, wsURL)
}

func testWebSocket(chromeCtx context.Context, testCase WebSocketTestCase) WebSocketTestResult {
	startTime := time.Now()
	
	result := WebSocketTestResult{
		URL:      testCase.URL,
		Success:  false,
		Messages: []WebSocketMessage{},
	}

	// Create recorder for this test
	rec := recorder.New()

	// Test timeout context
	ctx, cancel := context.WithTimeout(chromeCtx, testCase.TestDuration+10*time.Second)
	defer cancel()

	// Navigate to a page to establish browser context
	err := chromedp.Run(ctx,
		rec.Start(),
		chromedp.Navigate("about:blank"),
		
		// Execute WebSocket test
		chromedp.Evaluate(fmt.Sprintf(`
			(async function() {
				const results = {
					connected: false,
					connectionTime: 0,
					messagesSent: 0,
					messagesReceived: 0,
					messages: [],
					error: null
				};
				
				const startTime = performance.now();
				
				try {
					const ws = new WebSocket('%s'%s);
					
					// Connection promise
					const connectionPromise = new Promise((resolve, reject) => {
						ws.onopen = (event) => {
							results.connected = true;
							results.connectionTime = performance.now() - startTime;
							results.messages.push({
								type: 'event',
								data: 'connected',
								timestamp: Date.now()
							});
							resolve();
						};
						
						ws.onerror = (event) => {
							results.error = 'Connection failed';
							reject(new Error('Connection failed'));
						};
						
						ws.onclose = (event) => {
							results.messages.push({
								type: 'event',
								data: 'disconnected',
								timestamp: Date.now()
							});
						};
					});
					
					// Wait for connection
					await connectionPromise;
					
					// Message handler
					ws.onmessage = (event) => {
						results.messagesReceived++;
						results.messages.push({
							type: 'received',
							data: event.data,
							timestamp: Date.now()
						});
					};
					
					// Send test messages
					const testMessages = %s;
					for (const message of testMessages) {
						if (ws.readyState === WebSocket.OPEN) {
							const messageData = typeof message === 'string' ? message : JSON.stringify(message);
							ws.send(messageData);
							results.messagesSent++;
							results.messages.push({
								type: 'sent',
								data: messageData,
								timestamp: Date.now()
							});
							
							// Wait between messages
							await new Promise(resolve => setTimeout(resolve, 100));
						}
					}
					
					// Wait for the test duration
					await new Promise(resolve => setTimeout(resolve, %d));
					
					// Close connection
					ws.close();
					
				} catch (error) {
					results.error = error.message;
				}
				
				return results;
			})()
		`, 
			testCase.URL, 
			buildProtocolString(testCase.Protocol),
			jsonString(testCase.TestMessages),
			int(testCase.TestDuration.Milliseconds()),
		), &result),
		
		rec.Stop(),
	)

	duration := time.Since(startTime)
	result.Duration = float64(duration.Nanoseconds()) / 1e6

	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	// Extract results from JavaScript response
	if resultMap, ok := result.Messages.(map[string]interface{}); ok {
		if connected, ok := resultMap["connected"].(bool); ok {
			result.Connected = connected
		}
		if connectionTime, ok := resultMap["connectionTime"].(float64); ok {
			result.ConnectionTime = connectionTime
		}
		if messagesSent, ok := resultMap["messagesSent"].(float64); ok {
			result.MessagesSent = int(messagesSent)
		}
		if messagesReceived, ok := resultMap["messagesReceived"].(float64); ok {
			result.MessagesReceived = int(messagesReceived)
		}
		if messages, ok := resultMap["messages"].([]interface{}); ok {
			result.Messages = parseWebSocketMessages(messages)
		}
		if errorMsg, ok := resultMap["error"].(string); ok && errorMsg != "" {
			result.ErrorMessage = errorMsg
		}
	}

	// Extract network statistics from HAR
	harData, err := rec.HAR()
	if err == nil {
		result.NetworkStats = extractWebSocketNetworkStats(harData, testCase.URL)
	}

	// Validate results
	if result.ErrorMessage != "" {
		return result
	}

	if !result.Connected {
		result.ErrorMessage = "Failed to connect to WebSocket"
		return result
	}

	// Check if we received expected events
	eventTypes := make(map[string]bool)
	for _, msg := range result.Messages {
		if msg.Type == "event" {
			if eventStr, ok := msg.Data.(string); ok {
				eventTypes[eventStr] = true
			}
		}
	}

	for _, expectedEvent := range testCase.ExpectedEvents {
		if !eventTypes[expectedEvent] {
			result.ErrorMessage = fmt.Sprintf("Expected event '%s' not received", expectedEvent)
			return result
		}
	}

	result.Success = true
	return result
}

func buildProtocolString(protocol string) string {
	if protocol == "" {
		return ""
	}
	return fmt.Sprintf(", '%s'", protocol)
}

func parseWebSocketMessages(messages []interface{}) []WebSocketMessage {
	var result []WebSocketMessage
	for _, msg := range messages {
		if msgMap, ok := msg.(map[string]interface{}); ok {
			wsMsg := WebSocketMessage{}
			if msgType, ok := msgMap["type"].(string); ok {
				wsMsg.Type = msgType
			}
			if data, ok := msgMap["data"]; ok {
				wsMsg.Data = data
			}
			if timestamp, ok := msgMap["timestamp"].(float64); ok {
				wsMsg.Timestamp = int64(timestamp)
			}
			result = append(result, wsMsg)
		}
	}
	return result
}

func generateStressTestMessages(count int) []interface{} {
	var messages []interface{}
	for i := 0; i < count; i++ {
		messages = append(messages, map[string]interface{}{
			"type":    "stress_test",
			"index":   i,
			"message": fmt.Sprintf("Stress test message %d", i),
		})
	}
	return messages
}

func extractWebSocketNetworkStats(harData, url string) NetworkStats {
	// Simple HAR parsing for WebSocket connections
	stats := NetworkStats{}
	
	// This is a simplified implementation
	// In practice, you'd parse the HAR JSON and find the WebSocket upgrade request
	if strings.Contains(harData, url) {
		stats.MessageCount = 1 // Default assumption
	}
	
	return stats
}

func jsonString(v interface{}) string {
	if v == nil {
		return "null"
	}
	data, _ := json.Marshal(v)
	return string(data)
}

func generateWebSocketReport(results []WebSocketTestResult, wsURL string) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("WebSocket Test Report for %s\n", wsURL)
	fmt.Printf(strings.Repeat("=", 60) + "\n\n")

	passed := 0
	failed := 0
	totalDuration := 0.0
	totalMessages := 0
	totalConnectionTime := 0.0
	
	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
		totalDuration += result.Duration
		totalMessages += result.MessagesSent + result.MessagesReceived
		if result.Connected {
			totalConnectionTime += result.ConnectionTime
		}
	}

	fmt.Printf("Summary:\n")
	fmt.Printf("  Total Tests: %d\n", len(results))
	fmt.Printf("  Passed: %d\n", passed)
	fmt.Printf("  Failed: %d\n", failed)
	fmt.Printf("  Success Rate: %.1f%%\n", float64(passed)/float64(len(results))*100)
	fmt.Printf("  Average Duration: %.2f ms\n", totalDuration/float64(len(results)))
	fmt.Printf("  Total Messages: %d\n", totalMessages)
	if passed > 0 {
		fmt.Printf("  Average Connection Time: %.2f ms\n", totalConnectionTime/float64(passed))
	}

	fmt.Printf("\nDetailed Results:\n")
	for i, result := range results {
		fmt.Printf("\n%d. %s\n", i+1, result.URL)
		fmt.Printf("   Status: %s\n", map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Success])
		fmt.Printf("   Connected: %s\n", map[bool]string{true: "✓ Yes", false: "✗ No"}[result.Connected])
		
		if result.Connected {
			fmt.Printf("   Connection Time: %.2f ms\n", result.ConnectionTime)
		}
		
		fmt.Printf("   Duration: %.2f ms\n", result.Duration)
		fmt.Printf("   Messages Sent: %d\n", result.MessagesSent)
		fmt.Printf("   Messages Received: %d\n", result.MessagesReceived)
		
		if !result.Success {
			fmt.Printf("   Error: %s\n", result.ErrorMessage)
		}
		
		if len(result.Messages) > 0 {
			fmt.Printf("   Message Types: ")
			types := make(map[string]int)
			for _, msg := range result.Messages {
				types[msg.Type]++
			}
			for msgType, count := range types {
				fmt.Printf("%s(%d) ", msgType, count)
			}
			fmt.Printf("\n")
		}
	}

	// Save detailed report as JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Printf("Error marshaling results: %v", err)
		return
	}

	err = os.WriteFile("websocket-test-report.json", jsonData, 0644)
	if err != nil {
		log.Printf("Error writing report: %v", err)
		return
	}

	fmt.Printf("\nDetailed report saved to websocket-test-report.json\n")
}