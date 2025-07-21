# WebSocket Support in Chrome-to-HAR Toolkit

This document provides comprehensive examples of how to use the WebSocket monitoring and testing capabilities in the chrome-to-har toolkit.

## Table of Contents

1. [Basic WebSocket Monitoring](#basic-websocket-monitoring)
2. [Using churl with WebSocket Support](#using-churl-with-websocket-support)
3. [WebSocket Performance Monitoring](#websocket-performance-monitoring)
4. [HAR Export with WebSocket Data](#har-export-with-websocket-data)
5. [Advanced WebSocket Testing](#advanced-websocket-testing)
6. [Real-time Chat Application Testing](#real-time-chat-application-testing)
7. [Socket.IO Application Testing](#socketio-application-testing)
8. [WebSocket Load Testing](#websocket-load-testing)
9. [Troubleshooting and Debugging](#troubleshooting-and-debugging)

## Basic WebSocket Monitoring

### Simple WebSocket Connection Monitoring

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/tmc/misc/chrome-to-har/internal/browser"
    "github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

func main() {
    ctx := context.Background()
    
    // Create profile manager and browser
    pm, err := chromeprofiles.NewProfileManager()
    if err != nil {
        log.Fatal(err)
    }
    
    b, err := browser.New(ctx, pm)
    if err != nil {
        log.Fatal(err)
    }
    
    if err := b.Launch(ctx); err != nil {
        log.Fatal(err)
    }
    defer b.Close()
    
    // Create page and enable WebSocket monitoring
    page, err := b.NewPage()
    if err != nil {
        log.Fatal(err)
    }
    
    if err := page.EnableWebSocketMonitoring(); err != nil {
        log.Fatal(err)
    }
    
    // Set up event handlers
    page.SetWebSocketConnectionHandler(
        func(conn *browser.WebSocketConnection) {
            fmt.Printf("WebSocket connected: %s\n", conn.URL)
        },
        func(conn *browser.WebSocketConnection) {
            fmt.Printf("WebSocket disconnected: %s\n", conn.URL)
        },
        func(conn *browser.WebSocketConnection, err error) {
            fmt.Printf("WebSocket error: %v\n", err)
        },
    )
    
    page.SetWebSocketFrameHandler(
        func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
            fmt.Printf("Frame received: %s (%d bytes)\n", frame.Type, frame.Size)
        },
        func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
            fmt.Printf("Frame sent: %s (%d bytes)\n", frame.Type, frame.Size)
        },
    )
    
    // Navigate to page with WebSocket
    if err := page.Navigate("https://websocket-echo-server.com"); err != nil {
        log.Fatal(err)
    }
    
    // Wait for WebSocket connection
    conn, err := page.WaitForWebSocketConnection("*", 10*time.Second)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Connected to: %s\n", conn.URL)
    
    // Send a message
    if err := page.SendWebSocketMessage(conn.ID, "Hello WebSocket!"); err != nil {
        log.Fatal(err)
    }
    
    // Wait for response
    time.Sleep(2 * time.Second)
    
    // Get statistics
    stats := page.GetWebSocketStats()
    fmt.Printf("Statistics: %+v\n", stats)
}
```

## Using churl with WebSocket Support

### Basic WebSocket Testing with churl

```bash
# Test WebSocket connection
churl --ws-enabled --ws-wait-for=open --ws-stats wss://echo.websocket.org

# Send messages to WebSocket
churl --ws-enabled --ws-send="Hello World" --ws-send="Test Message" wss://echo.websocket.org

# Wait for specific WebSocket events
churl --ws-enabled --ws-wait-for=message --ws-timeout=30 wss://api.example.com/ws

# Filter WebSocket data
churl --ws-enabled --ws-url-pattern="wss://api.example.com/*" --ws-direction=received wss://api.example.com/ws

# Export WebSocket data to file
churl --ws-enabled --ws-output=websocket-data.json wss://api.example.com/ws

# Comprehensive WebSocket testing
churl --ws-enabled \
      --ws-send="ping" \
      --ws-wait-for=message \
      --ws-data-pattern="pong" \
      --ws-timeout=30 \
      --ws-stats \
      --ws-output=test-results.har \
      wss://api.example.com/ws
```

### Advanced churl WebSocket Options

```bash
# Test with specific URL pattern
churl --ws-enabled --ws-url-pattern="wss://api.example.com/chat/*" https://chat.example.com

# Test with direction filtering
churl --ws-enabled --ws-direction=sent --ws-data-pattern="message" https://app.example.com

# Test with custom timeout
churl --ws-enabled --ws-timeout=60 --ws-wait-for=first_message https://realtime.example.com

# Verbose WebSocket monitoring
churl --verbose --ws-enabled --ws-stats https://websocket-app.example.com
```

## WebSocket Performance Monitoring

### Basic Performance Monitoring

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/tmc/misc/chrome-to-har/internal/browser"
    "github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

func main() {
    ctx := context.Background()
    
    // Create browser and page
    pm, err := chromeprofiles.NewProfileManager()
    if err != nil {
        log.Fatal(err)
    }
    
    b, err := browser.New(ctx, pm)
    if err != nil {
        log.Fatal(err)
    }
    
    if err := b.Launch(ctx); err != nil {
        log.Fatal(err)
    }
    defer b.Close()
    
    page, err := b.NewPage()
    if err != nil {
        log.Fatal(err)
    }
    
    // Enable WebSocket monitoring
    if err := page.EnableWebSocketMonitoring(); err != nil {
        log.Fatal(err)
    }
    
    // Start performance monitoring
    monitor, err := page.StartPerformanceMonitoring()
    if err != nil {
        log.Fatal(err)
    }
    
    // Navigate to application
    if err := page.Navigate("https://realtime-app.example.com"); err != nil {
        log.Fatal(err)
    }
    
    // Wait for WebSocket activity
    time.Sleep(30 * time.Second)
    
    // Stop monitoring
    if err := monitor.Stop(); err != nil {
        log.Fatal(err)
    }
    
    // Get performance metrics
    metrics := monitor.GetMetrics()
    fmt.Printf("Connection Latency: %v\n", metrics.ConnectionLatency)
    fmt.Printf("Message Throughput: %.2f msg/sec\n", metrics.MessageThroughput)
    fmt.Printf("Data Throughput: %.2f bytes/sec\n", metrics.DataThroughput)
    fmt.Printf("Connection Success Rate: %.1f%%\n", metrics.ConnectionSuccess)
    
    // Generate performance report
    report := monitor.GenerateReport()
    fmt.Printf("Overall Rating: %s\n", report.Summary.OverallRating)
    
    for _, finding := range report.Summary.KeyFindings {
        fmt.Printf("Finding: %s\n", finding)
    }
    
    for _, recommendation := range report.Summary.Recommendations {
        fmt.Printf("Recommendation: %s\n", recommendation)
    }
}
```

### Performance Monitoring with Thresholds

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/tmc/misc/chrome-to-har/internal/browser"
    "github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

func main() {
    ctx := context.Background()
    
    // Create browser and page
    pm, err := chromeprofiles.NewProfileManager()
    if err != nil {
        log.Fatal(err)
    }
    
    b, err := browser.New(ctx, pm)
    if err != nil {
        log.Fatal(err)
    }
    
    if err := b.Launch(ctx); err != nil {
        log.Fatal(err)
    }
    defer b.Close()
    
    page, err := b.NewPage()
    if err != nil {
        log.Fatal(err)
    }
    
    // Enable WebSocket monitoring
    if err := page.EnableWebSocketMonitoring(); err != nil {
        log.Fatal(err)
    }
    
    // Define performance thresholds
    thresholds := &browser.WebSocketPerformanceThresholds{
        MaxConnectionLatency: 2 * time.Second,
        MaxMessageLatency:    500 * time.Millisecond,
        MinThroughput:        10, // messages per second
        MaxErrorRate:         5,  // 5% error rate
        MaxJitter:            100 * time.Millisecond,
    }
    
    // Start monitoring with thresholds
    monitor, err := page.MonitorPerformanceWithThresholds(ctx, thresholds, 
        func(alerts []browser.WebSocketPerformanceAlert) {
            for _, alert := range alerts {
                fmt.Printf("ALERT [%s]: %s (Value: %v, Threshold: %v)\n", 
                    alert.Severity, alert.Message, alert.Value, alert.Threshold)
            }
        })
    if err != nil {
        log.Fatal(err)
    }
    
    // Navigate to application
    if err := page.Navigate("https://realtime-app.example.com"); err != nil {
        log.Fatal(err)
    }
    
    // Monitor for 60 seconds
    time.Sleep(60 * time.Second)
    
    // Stop monitoring
    if err := monitor.Stop(); err != nil {
        log.Fatal(err)
    }
}
```

## HAR Export with WebSocket Data

### Basic HAR Export

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/tmc/misc/chrome-to-har/internal/browser"
    "github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
    "github.com/tmc/misc/chrome-to-har/internal/recorder"
)

func main() {
    ctx := context.Background()
    
    // Create browser and page
    pm, err := chromeprofiles.NewProfileManager()
    if err != nil {
        log.Fatal(err)
    }
    
    b, err := browser.New(ctx, pm)
    if err != nil {
        log.Fatal(err)
    }
    
    if err := b.Launch(ctx); err != nil {
        log.Fatal(err)
    }
    defer b.Close()
    
    page, err := b.NewPage()
    if err != nil {
        log.Fatal(err)
    }
    
    // Enable WebSocket monitoring
    if err := page.EnableWebSocketMonitoring(); err != nil {
        log.Fatal(err)
    }
    
    // Create WebSocket recorder
    wsRecorder, err := recorder.NewWebSocketRecorder(
        recorder.WithVerbose(true),
        recorder.WithStreaming(false),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Navigate to application
    if err := page.Navigate("https://websocket-app.example.com"); err != nil {
        log.Fatal(err)
    }
    
    // Wait for WebSocket activity
    time.Sleep(10 * time.Second)
    
    // Export HAR with WebSocket data
    harData, err := wsRecorder.HARWithWebSocketData()
    if err != nil {
        log.Fatal(err)
    }
    
    // Write to file
    if err := os.WriteFile("websocket-capture.har", harData, 0644); err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("HAR file with WebSocket data saved to websocket-capture.har")
    
    // Get WebSocket statistics
    stats := wsRecorder.GetWebSocketStatistics()
    fmt.Printf("WebSocket Statistics: %+v\n", stats)
}
```

### Filtered HAR Export

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/tmc/misc/chrome-to-har/internal/browser"
    "github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

func main() {
    ctx := context.Background()
    
    // Create browser and page
    pm, err := chromeprofiles.NewProfileManager()
    if err != nil {
        log.Fatal(err)
    }
    
    b, err := browser.New(ctx, pm)
    if err != nil {
        log.Fatal(err)
    }
    
    if err := b.Launch(ctx); err != nil {
        log.Fatal(err)
    }
    defer b.Close()
    
    page, err := b.NewPage()
    if err != nil {
        log.Fatal(err)
    }
    
    // Enable WebSocket monitoring
    if err := page.EnableWebSocketMonitoring(); err != nil {
        log.Fatal(err)
    }
    
    // Navigate to application
    if err := page.Navigate("https://chat-app.example.com"); err != nil {
        log.Fatal(err)
    }
    
    // Wait for WebSocket activity
    time.Sleep(10 * time.Second)
    
    // Get WebSocket connections
    connections := page.GetWebSocketConnections()
    
    // Create filter for chat messages only
    filter := &browser.WebSocketHARFilter{
        URLPattern:    "wss://chat-app.example.com/*",
        MessageType:   "text",
        Direction:     "received",
        DataPattern:   "message",
        MinSize:       10,
        MaxSize:       1000,
    }
    
    // Export filtered HAR
    exporter := browser.NewWebSocketHARExporter(filter)
    harData, err := exporter.ExportWithWebSocketData(connections)
    if err != nil {
        log.Fatal(err)
    }
    
    // Write to file
    if err := os.WriteFile("chat-messages.har", harData, 0644); err != nil {
        log.Fatal(err)
    }
    
    log.Println("Filtered HAR file saved to chat-messages.har")
}
```

## Advanced WebSocket Testing

### WebSocket Wait Conditions

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/tmc/misc/chrome-to-har/internal/browser"
    "github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

func main() {
    ctx := context.Background()
    
    // Create browser and page
    pm, err := chromeprofiles.NewProfileManager()
    if err != nil {
        log.Fatal(err)
    }
    
    b, err := browser.New(ctx, pm)
    if err != nil {
        log.Fatal(err)
    }
    
    if err := b.Launch(ctx); err != nil {
        log.Fatal(err)
    }
    defer b.Close()
    
    page, err := b.NewPage()
    if err != nil {
        log.Fatal(err)
    }
    
    // Enable WebSocket monitoring
    if err := page.EnableWebSocketMonitoring(); err != nil {
        log.Fatal(err)
    }
    
    // Navigate to application
    if err := page.Navigate("https://realtime-app.example.com"); err != nil {
        log.Fatal(err)
    }
    
    // Wait for WebSocket connection
    conn, err := page.WaitForWebSocket(browser.WebSocketOpen, 
        browser.WithTimeout(10*time.Second),
        browser.WithURLPattern("wss://realtime-app.example.com/*"))
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("WebSocket connected: %s\n", conn.URL)
    
    // Wait for first message
    _, err = page.WaitForWebSocket(browser.WebSocketFirstMessage,
        browser.WithTimeout(15*time.Second),
        browser.WithDataPattern("welcome"))
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Received welcome message")
    
    // Wait for specific number of messages
    frames, err := page.WaitForWebSocketMessages(5,
        browser.WithTimeout(30*time.Second),
        browser.WithDirection("received"))
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Received %d messages\n", len(frames))
    
    // Wait for WebSocket to be idle
    err = page.WaitForWebSocketIdle(5*time.Second,
        browser.WithTimeout(60*time.Second))
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("WebSocket is idle")
    
    // Wait for specific data pattern
    frame, err := page.WaitForWebSocketData(`{"type":"notification"}`,
        browser.WithTimeout(30*time.Second))
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Received notification: %v\n", frame.Data)
}
```

### WebSocket Event Waiter

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/tmc/misc/chrome-to-har/internal/browser"
    "github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

func main() {
    ctx := context.Background()
    
    // Create browser and page
    pm, err := chromeprofiles.NewProfileManager()
    if err != nil {
        log.Fatal(err)
    }
    
    b, err := browser.New(ctx, pm)
    if err != nil {
        log.Fatal(err)
    }
    
    if err := b.Launch(ctx); err != nil {
        log.Fatal(err)
    }
    defer b.Close()
    
    page, err := b.NewPage()
    if err != nil {
        log.Fatal(err)
    }
    
    // Enable WebSocket monitoring
    if err := page.EnableWebSocketMonitoring(); err != nil {
        log.Fatal(err)
    }
    
    // Create event waiter
    waiter := page.NewWebSocketEventWaiter(
        browser.WithTimeout(30*time.Second),
        browser.WithURLPattern("wss://api.example.com/*"),
        browser.WithDataPattern("message"),
    )
    
    // Navigate to application
    if err := page.Navigate("https://app.example.com"); err != nil {
        log.Fatal(err)
    }
    
    // Wait for sequence of events
    events := []browser.WebSocketWaitCondition{
        browser.WebSocketOpen,
        browser.WebSocketFirstMessage,
        browser.WebSocketMessage,
    }
    
    connections, err := waiter.WaitForSequence(events)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Completed sequence with %d connections\n", len(connections))
    
    // Wait with callback
    err = waiter.WaitWithCallback(browser.WebSocketMessage,
        func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
            fmt.Printf("Callback: %s frame from %s\n", frame.Type, conn.URL)
        })
    if err != nil {
        log.Fatal(err)
    }
}
```

## Real-time Chat Application Testing

Run the comprehensive chat tester:

```bash
# Test basic chat functionality
go run examples/04-api-testing/realtime-chat-tester.go wss://chat.example.com/ws

# Test with custom configuration
go run examples/04-api-testing/realtime-chat-tester.go chat-config.json
```

Example chat configuration file (`chat-config.json`):

```json
{
  "scenarios": [
    {
      "name": "Multi-User Chat Test",
      "chat_url": "wss://chat.example.com/ws",
      "duration": "2m",
      "users": [
        {"name": "Alice", "join_delay": "0s", "active": true},
        {"name": "Bob", "join_delay": "5s", "active": true},
        {"name": "Charlie", "join_delay": "10s", "active": true}
      ],
      "expected": {
        "min_messages": 10,
        "max_latency": "2s",
        "expected_users": ["Alice", "Bob", "Charlie"],
        "message_delivery": true,
        "user_presence": true
      }
    }
  ]
}
```

## Socket.IO Application Testing

Run the Socket.IO tester:

```bash
# Test Socket.IO application
go run examples/04-api-testing/socketio-tester.go http://localhost:3000

# Test production Socket.IO server
go run examples/04-api-testing/socketio-tester.go https://socket.io-server.com
```

## WebSocket Load Testing

### Basic Load Testing

```go
package main

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/tmc/misc/chrome-to-har/internal/browser"
    "github.com/tmc/misc/chrome-to-har/internal/chromeprofiles"
)

func main() {
    ctx := context.Background()
    
    // Create browser
    pm, err := chromeprofiles.NewProfileManager()
    if err != nil {
        log.Fatal(err)
    }
    
    b, err := browser.New(ctx, pm)
    if err != nil {
        log.Fatal(err)
    }
    
    if err := b.Launch(ctx); err != nil {
        log.Fatal(err)
    }
    defer b.Close()
    
    // Test configuration
    numConnections := 10
    testDuration := 30 * time.Second
    messageRate := 1 // messages per second per connection
    
    var wg sync.WaitGroup
    results := make(chan browser.WebSocketConnection, numConnections)
    
    // Create multiple connections
    for i := 0; i < numConnections; i++ {
        wg.Add(1)
        go func(connID int) {
            defer wg.Done()
            
            // Create page
            page, err := b.NewPage()
            if err != nil {
                log.Printf("Connection %d: Failed to create page: %v", connID, err)
                return
            }
            
            // Enable WebSocket monitoring
            if err := page.EnableWebSocketMonitoring(); err != nil {
                log.Printf("Connection %d: Failed to enable monitoring: %v", connID, err)
                return
            }
            
            // Navigate to test page
            testHTML := fmt.Sprintf(`
                <html>
                <head><title>Load Test %d</title></head>
                <body>
                <script>
                    const ws = new WebSocket('wss://echo.websocket.org');
                    ws.onopen = function() {
                        console.log('Connection %d opened');
                        
                        // Send messages at specified rate
                        const interval = setInterval(() => {
                            if (ws.readyState === WebSocket.OPEN) {
                                ws.send('Load test message from connection %d');
                            }
                        }, %d);
                        
                        // Stop after test duration
                        setTimeout(() => {
                            clearInterval(interval);
                            ws.close();
                        }, %d);
                    };
                    
                    ws.onmessage = function(event) {
                        console.log('Connection %d received:', event.data);
                    };
                    
                    ws.onclose = function() {
                        console.log('Connection %d closed');
                    };
                </script>
                </body>
                </html>
            `, connID, connID, connID, 1000/messageRate, int(testDuration.Milliseconds()))
            
            if err := page.Navigate(fmt.Sprintf("data:text/html,%s", testHTML)); err != nil {
                log.Printf("Connection %d: Failed to navigate: %v", connID, err)
                return
            }
            
            // Wait for test completion
            time.Sleep(testDuration + 5*time.Second)
            
            // Get connection info
            connections := page.GetWebSocketConnections()
            for _, conn := range connections {
                results <- *conn
                break
            }
        }(i)
    }
    
    // Wait for all connections to complete
    go func() {
        wg.Wait()
        close(results)
    }()
    
    // Collect results
    var totalMessages int
    var totalBytes int64
    var successfulConnections int
    
    for conn := range results {
        successfulConnections++
        totalMessages += conn.MessagesSent + conn.MessagesReceived
        totalBytes += conn.BytesSent + conn.BytesReceived
    }
    
    // Print results
    fmt.Printf("Load Test Results:\n")
    fmt.Printf("Successful Connections: %d/%d\n", successfulConnections, numConnections)
    fmt.Printf("Total Messages: %d\n", totalMessages)
    fmt.Printf("Total Bytes: %d\n", totalBytes)
    fmt.Printf("Message Rate: %.2f messages/second\n", float64(totalMessages)/testDuration.Seconds())
    fmt.Printf("Data Rate: %.2f bytes/second\n", float64(totalBytes)/testDuration.Seconds())
    
    if successfulConnections > 0 {
        fmt.Printf("Average Messages per Connection: %.2f\n", float64(totalMessages)/float64(successfulConnections))
        fmt.Printf("Average Bytes per Connection: %.2f\n", float64(totalBytes)/float64(successfulConnections))
    }
}
```

## Troubleshooting and Debugging

### Common Issues and Solutions

1. **WebSocket Connection Fails**
   ```go
   // Check for connection errors
   page.SetWebSocketConnectionHandler(
       func(conn *browser.WebSocketConnection) {
           fmt.Printf("Connected to: %s\n", conn.URL)
       },
       func(conn *browser.WebSocketConnection) {
           fmt.Printf("Disconnected from: %s\n", conn.URL)
       },
       func(conn *browser.WebSocketConnection, err error) {
           fmt.Printf("Connection error: %v\n", err)
           // Log additional debugging info
           fmt.Printf("Connection state: %s\n", conn.State)
           fmt.Printf("Close code: %d\n", conn.CloseCode)
           fmt.Printf("Close reason: %s\n", conn.CloseReason)
       },
   )
   ```

2. **WebSocket Frames Not Captured**
   ```go
   // Enable verbose logging
   page.SetWebSocketFrameHandler(
       func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
           fmt.Printf("RX: %s - %s (%d bytes) at %v\n", 
               conn.URL, frame.Type, frame.Size, frame.Timestamp)
           fmt.Printf("Data: %v\n", frame.Data)
       },
       func(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
           fmt.Printf("TX: %s - %s (%d bytes) at %v\n", 
               conn.URL, frame.Type, frame.Size, frame.Timestamp)
           fmt.Printf("Data: %v\n", frame.Data)
       },
   )
   ```

3. **Performance Issues**
   ```go
   // Monitor performance and identify bottlenecks
   monitor, err := page.StartPerformanceMonitoring()
   if err != nil {
       log.Fatal(err)
   }
   
   // Check samples periodically
   go func() {
       ticker := time.NewTicker(5 * time.Second)
       defer ticker.Stop()
       
       for range ticker.C {
           samples := monitor.GetSamples()
           if len(samples) > 0 {
               latest := samples[len(samples)-1]
               fmt.Printf("Latest sample: %+v\n", latest)
               
               if latest.ErrorRate > 5.0 {
                   fmt.Printf("WARNING: High error rate detected\n")
               }
               
               if latest.MessageLatency > 1*time.Second {
                   fmt.Printf("WARNING: High message latency detected\n")
               }
           }
       }
   }()
   ```

4. **Debugging HAR Export**
   ```go
   // Export HAR and verify WebSocket data
   connections := page.GetWebSocketConnections()
   
   converter := browser.NewWebSocketHARConverter()
   for _, conn := range connections {
       converter.AddConnection(conn)
       
       // Debug connection info
       fmt.Printf("Connection: %s\n", conn.URL)
       fmt.Printf("State: %s\n", conn.State)
       fmt.Printf("Frames: %d\n", len(conn.Frames))
       
       for i, frame := range conn.Frames {
           fmt.Printf("Frame %d: %s %s %d bytes\n", 
               i, frame.Direction, frame.Type, frame.Size)
       }
   }
   
   entries := converter.ConvertToHAR()
   fmt.Printf("Generated %d HAR entries\n", len(entries))
   ```

5. **Testing with Different WebSocket Libraries**
   ```html
   <!-- Socket.IO -->
   <script src="https://cdn.socket.io/4.0.0/socket.io.min.js"></script>
   <script>
       const socket = io('http://localhost:3000');
       socket.on('connect', () => console.log('Socket.IO connected'));
   </script>
   
   <!-- Standard WebSocket -->
   <script>
       const ws = new WebSocket('wss://echo.websocket.org');
       ws.onopen = () => console.log('WebSocket connected');
   </script>
   
   <!-- SockJS -->
   <script src="https://cdn.jsdelivr.net/npm/sockjs-client@1/dist/sockjs.min.js"></script>
   <script>
       const sock = new SockJS('http://localhost:3000/sockjs');
       sock.onopen = () => console.log('SockJS connected');
   </script>
   ```

## Best Practices

1. **Always enable WebSocket monitoring before navigation**
2. **Use appropriate timeouts for wait conditions**
3. **Handle connection errors gracefully**
4. **Monitor performance for production testing**
5. **Filter WebSocket data appropriately for analysis**
6. **Use verbose logging for debugging**
7. **Test with realistic load patterns**
8. **Validate WebSocket protocol compliance**
9. **Monitor for memory leaks during long tests**
10. **Export HAR data for post-analysis**

This comprehensive guide covers all aspects of WebSocket testing with the chrome-to-har toolkit. The toolkit provides full visibility into WebSocket behavior, enabling thorough testing of real-time applications.