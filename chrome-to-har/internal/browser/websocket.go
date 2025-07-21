package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

// WebSocketFrame represents a WebSocket frame
type WebSocketFrame struct {
	Type      string      `json:"type"`      // "text", "binary", "close", "ping", "pong"
	Direction string      `json:"direction"` // "sent", "received"
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	Size      int64       `json:"size"`
	Opcode    int         `json:"opcode"`
}

// WebSocketConnection represents a WebSocket connection
type WebSocketConnection struct {
	ID                string            `json:"id"`
	URL               string            `json:"url"`
	State             string            `json:"state"`       // "connecting", "open", "closing", "closed"
	Protocol          string            `json:"protocol"`
	Extensions        []string          `json:"extensions"`
	ConnectedAt       time.Time         `json:"connected_at"`
	DisconnectedAt    *time.Time        `json:"disconnected_at,omitempty"`
	Frames            []WebSocketFrame  `json:"frames"`
	Headers           map[string]string `json:"headers"`
	CloseCode         int               `json:"close_code,omitempty"`
	CloseReason       string            `json:"close_reason,omitempty"`
	BytesSent         int64             `json:"bytes_sent"`
	BytesReceived     int64             `json:"bytes_received"`
	MessagesSent      int               `json:"messages_sent"`
	MessagesReceived  int               `json:"messages_received"`
	ConnectionLatency time.Duration     `json:"connection_latency"`
	mu                sync.RWMutex
}

// WebSocketMonitor manages WebSocket connections monitoring
type WebSocketMonitor struct {
	page        *Page
	connections map[string]*WebSocketConnection
	enabled     bool
	mu          sync.RWMutex
	
	// Event handlers
	onFrameReceived func(*WebSocketConnection, *WebSocketFrame)
	onFrameSent     func(*WebSocketConnection, *WebSocketFrame)
	onConnect       func(*WebSocketConnection)
	onDisconnect    func(*WebSocketConnection)
	onError         func(*WebSocketConnection, error)
}

// NewWebSocketMonitor creates a new WebSocket monitor
func NewWebSocketMonitor(page *Page) *WebSocketMonitor {
	return &WebSocketMonitor{
		page:        page,
		connections: make(map[string]*WebSocketConnection),
	}
}

// Enable enables WebSocket monitoring
func (wsm *WebSocketMonitor) Enable() error {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	if wsm.enabled {
		return nil
	}

	// Enable network domain for WebSocket protocol handshake
	if err := chromedp.Run(wsm.page.ctx, network.Enable()); err != nil {
		return errors.Wrap(err, "enabling network domain")
	}

	// Set up event handlers
	chromedp.ListenTarget(wsm.page.ctx, wsm.handleNetworkEvent)

	// Inject WebSocket monitoring script
	if err := wsm.injectWebSocketMonitoring(); err != nil {
		return errors.Wrap(err, "injecting WebSocket monitoring script")
	}

	wsm.enabled = true
	return nil
}

// Disable disables WebSocket monitoring
func (wsm *WebSocketMonitor) Disable() error {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	if !wsm.enabled {
		return nil
	}

	wsm.enabled = false
	return nil
}

// injectWebSocketMonitoring injects JavaScript to monitor WebSocket connections
func (wsm *WebSocketMonitor) injectWebSocketMonitoring() error {
	script := `
		(function() {
			if (window.__websocketMonitorInjected) return;
			window.__websocketMonitorInjected = true;
			
			const originalWebSocket = window.WebSocket;
			const connections = new Map();
			
			function generateConnectionId() {
				return 'ws_' + Math.random().toString(36).substr(2, 9) + '_' + Date.now();
			}
			
			function sendToMonitor(type, data) {
				window.postMessage({
					type: '__websocket_monitor',
					subtype: type,
					data: data
				}, '*');
			}
			
			window.WebSocket = function(url, protocols) {
				const connectionId = generateConnectionId();
				const startTime = performance.now();
				
				const ws = new originalWebSocket(url, protocols);
				
				const connection = {
					id: connectionId,
					url: url,
					protocols: protocols,
					state: 'connecting',
					startTime: startTime,
					frames: [],
					bytesSent: 0,
					bytesReceived: 0,
					messagesSent: 0,
					messagesReceived: 0
				};
				
				connections.set(connectionId, connection);
				
				// Override send method
				const originalSend = ws.send;
				ws.send = function(data) {
					const frame = {
						type: typeof data === 'string' ? 'text' : 'binary',
						direction: 'sent',
						data: data,
						timestamp: Date.now(),
						size: data.length || data.byteLength || 0
					};
					
					connection.frames.push(frame);
					connection.bytesSent += frame.size;
					connection.messagesSent++;
					
					sendToMonitor('frame', {
						connectionId: connectionId,
						frame: frame
					});
					
					return originalSend.call(this, data);
				};
				
				// Set up event listeners
				ws.addEventListener('open', function(event) {
					connection.state = 'open';
					connection.connectedAt = Date.now();
					connection.connectionLatency = performance.now() - startTime;
					
					sendToMonitor('connect', {
						connectionId: connectionId,
						connection: connection
					});
				});
				
				ws.addEventListener('message', function(event) {
					const frame = {
						type: typeof event.data === 'string' ? 'text' : 'binary',
						direction: 'received',
						data: event.data,
						timestamp: Date.now(),
						size: event.data.length || event.data.byteLength || 0
					};
					
					connection.frames.push(frame);
					connection.bytesReceived += frame.size;
					connection.messagesReceived++;
					
					sendToMonitor('frame', {
						connectionId: connectionId,
						frame: frame
					});
				});
				
				ws.addEventListener('close', function(event) {
					connection.state = 'closed';
					connection.disconnectedAt = Date.now();
					connection.closeCode = event.code;
					connection.closeReason = event.reason;
					
					sendToMonitor('disconnect', {
						connectionId: connectionId,
						connection: connection,
						closeCode: event.code,
						closeReason: event.reason
					});
					
					connections.delete(connectionId);
				});
				
				ws.addEventListener('error', function(event) {
					sendToMonitor('error', {
						connectionId: connectionId,
						error: event.error || 'WebSocket error'
					});
				});
				
				return ws;
			};
			
			// Copy static properties
			Object.setPrototypeOf(window.WebSocket, originalWebSocket);
			Object.defineProperty(window.WebSocket, 'prototype', {
				value: originalWebSocket.prototype,
				writable: false
			});
			
			// Add constants
			window.WebSocket.CONNECTING = originalWebSocket.CONNECTING;
			window.WebSocket.OPEN = originalWebSocket.OPEN;
			window.WebSocket.CLOSING = originalWebSocket.CLOSING;
			window.WebSocket.CLOSED = originalWebSocket.CLOSED;
			
			// Listen for messages from the monitor
			window.addEventListener('message', function(event) {
				if (event.data && event.data.type === '__websocket_monitor') {
					// Handle monitor messages if needed
				}
			});
			
			// Export for external access
			window.__websocketConnections = connections;
		})();
	`

	return chromedp.Run(wsm.page.ctx, chromedp.Evaluate(script, nil))
}

// handleNetworkEvent processes network events for WebSocket handshake
func (wsm *WebSocketMonitor) handleNetworkEvent(ev interface{}) {
	switch ev := ev.(type) {
	case *network.EventWebSocketCreated:
		wsm.handleWebSocketCreated(ev)
	case *network.EventWebSocketFrameReceived:
		wsm.handleWebSocketFrameReceived(ev)
	case *network.EventWebSocketFrameSent:
		wsm.handleWebSocketFrameSent(ev)
	case *network.EventWebSocketClosed:
		wsm.handleWebSocketClosed(ev)
	case *network.EventWebSocketFrameError:
		wsm.handleWebSocketFrameError(ev)
	}
}

// handleWebSocketCreated handles WebSocket connection creation
func (wsm *WebSocketMonitor) handleWebSocketCreated(ev *network.EventWebSocketCreated) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	connection := &WebSocketConnection{
		ID:          string(ev.RequestID),
		URL:         ev.URL,
		State:       "connecting",
		ConnectedAt: time.Now(),
		Frames:      make([]WebSocketFrame, 0),
		Headers:     make(map[string]string),
	}

	wsm.connections[connection.ID] = connection

	if wsm.onConnect != nil {
		wsm.onConnect(connection)
	}
}

// handleWebSocketFrameReceived handles received WebSocket frames
func (wsm *WebSocketMonitor) handleWebSocketFrameReceived(ev *network.EventWebSocketFrameReceived) {
	wsm.mu.RLock()
	connection, exists := wsm.connections[string(ev.RequestID)]
	wsm.mu.RUnlock()

	if !exists {
		return
	}

	connection.mu.Lock()
	defer connection.mu.Unlock()

	frame := WebSocketFrame{
		Type:      wsm.getFrameType(int64(ev.Response.Opcode)),
		Direction: "received",
		Data:      ev.Response.PayloadData,
		Timestamp: time.Now(),
		Size:      int64(len(ev.Response.PayloadData)),
		Opcode:    int(ev.Response.Opcode),
	}

	connection.Frames = append(connection.Frames, frame)
	connection.BytesReceived += frame.Size
	connection.MessagesReceived++

	if wsm.onFrameReceived != nil {
		wsm.onFrameReceived(connection, &frame)
	}
}

// handleWebSocketFrameSent handles sent WebSocket frames
func (wsm *WebSocketMonitor) handleWebSocketFrameSent(ev *network.EventWebSocketFrameSent) {
	wsm.mu.RLock()
	connection, exists := wsm.connections[string(ev.RequestID)]
	wsm.mu.RUnlock()

	if !exists {
		return
	}

	connection.mu.Lock()
	defer connection.mu.Unlock()

	frame := WebSocketFrame{
		Type:      wsm.getFrameType(int64(ev.Response.Opcode)),
		Direction: "sent",
		Data:      ev.Response.PayloadData,
		Timestamp: time.Now(),
		Size:      int64(len(ev.Response.PayloadData)),
		Opcode:    int(ev.Response.Opcode),
	}

	connection.Frames = append(connection.Frames, frame)
	connection.BytesSent += frame.Size
	connection.MessagesSent++

	if wsm.onFrameSent != nil {
		wsm.onFrameSent(connection, &frame)
	}
}

// handleWebSocketClosed handles WebSocket connection closure
func (wsm *WebSocketMonitor) handleWebSocketClosed(ev *network.EventWebSocketClosed) {
	wsm.mu.Lock()
	connection, exists := wsm.connections[string(ev.RequestID)]
	if exists {
		delete(wsm.connections, string(ev.RequestID))
	}
	wsm.mu.Unlock()

	if !exists {
		return
	}

	connection.mu.Lock()
	now := time.Now()
	connection.DisconnectedAt = &now
	connection.State = "closed"
	connection.mu.Unlock()

	if wsm.onDisconnect != nil {
		wsm.onDisconnect(connection)
	}
}

// handleWebSocketFrameError handles WebSocket frame errors
func (wsm *WebSocketMonitor) handleWebSocketFrameError(ev *network.EventWebSocketFrameError) {
	wsm.mu.RLock()
	connection, exists := wsm.connections[string(ev.RequestID)]
	wsm.mu.RUnlock()

	if !exists {
		return
	}

	if wsm.onError != nil {
		wsm.onError(connection, errors.New(ev.ErrorMessage))
	}
}

// getFrameType converts opcode to frame type
func (wsm *WebSocketMonitor) getFrameType(opcode int64) string {
	switch opcode {
	case 0x1:
		return "text"
	case 0x2:
		return "binary"
	case 0x8:
		return "close"
	case 0x9:
		return "ping"
	case 0xA:
		return "pong"
	default:
		return "unknown"
	}
}

// GetConnections returns all active WebSocket connections
func (wsm *WebSocketMonitor) GetConnections() map[string]*WebSocketConnection {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	result := make(map[string]*WebSocketConnection)
	for k, v := range wsm.connections {
		result[k] = v
	}
	return result
}

// GetConnection returns a specific WebSocket connection by ID
func (wsm *WebSocketMonitor) GetConnection(id string) (*WebSocketConnection, bool) {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	conn, exists := wsm.connections[id]
	return conn, exists
}

// WaitForConnection waits for a WebSocket connection to a specific URL
func (wsm *WebSocketMonitor) WaitForConnection(urlPattern string, timeout time.Duration) (*WebSocketConnection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout waiting for WebSocket connection")
		case <-ticker.C:
			wsm.mu.RLock()
			for _, conn := range wsm.connections {
				if conn.URL == urlPattern || (urlPattern == "*") {
					wsm.mu.RUnlock()
					return conn, nil
				}
			}
			wsm.mu.RUnlock()
		}
	}
}

// SendMessage sends a message through a WebSocket connection
func (wsm *WebSocketMonitor) SendMessage(connectionID string, message interface{}) error {
	var messageData string
	switch msg := message.(type) {
	case string:
		messageData = msg
	case []byte:
		messageData = string(msg)
	default:
		data, err := json.Marshal(message)
		if err != nil {
			return errors.Wrap(err, "marshaling message")
		}
		messageData = string(data)
	}

	script := fmt.Sprintf(`
		(function() {
			const connections = window.__websocketConnections;
			if (!connections) return false;
			
			for (const [id, conn] of connections) {
				if (id === '%s') {
					// Find the WebSocket instance
					if (conn.ws && conn.ws.readyState === WebSocket.OPEN) {
						conn.ws.send('%s');
						return true;
					}
				}
			}
			return false;
		})()
	`, connectionID, messageData)

	var result bool
	if err := chromedp.Run(wsm.page.ctx, chromedp.Evaluate(script, &result)); err != nil {
		return errors.Wrap(err, "sending WebSocket message")
	}

	if !result {
		return errors.New("WebSocket connection not found")
	}

	return nil
}

// SetOnFrameReceived sets the callback for received frames
func (wsm *WebSocketMonitor) SetOnFrameReceived(callback func(*WebSocketConnection, *WebSocketFrame)) {
	wsm.onFrameReceived = callback
}

// SetOnFrameSent sets the callback for sent frames
func (wsm *WebSocketMonitor) SetOnFrameSent(callback func(*WebSocketConnection, *WebSocketFrame)) {
	wsm.onFrameSent = callback
}

// SetOnConnect sets the callback for connection events
func (wsm *WebSocketMonitor) SetOnConnect(callback func(*WebSocketConnection)) {
	wsm.onConnect = callback
}

// SetOnDisconnect sets the callback for disconnection events
func (wsm *WebSocketMonitor) SetOnDisconnect(callback func(*WebSocketConnection)) {
	wsm.onDisconnect = callback
}

// SetOnError sets the callback for error events
func (wsm *WebSocketMonitor) SetOnError(callback func(*WebSocketConnection, error)) {
	wsm.onError = callback
}

// GetStats returns statistics for all WebSocket connections
func (wsm *WebSocketMonitor) GetStats() map[string]interface{} {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	stats := map[string]interface{}{
		"active_connections": len(wsm.connections),
		"total_bytes_sent":   int64(0),
		"total_bytes_received": int64(0),
		"total_messages_sent": 0,
		"total_messages_received": 0,
	}

	for _, conn := range wsm.connections {
		conn.mu.RLock()
		stats["total_bytes_sent"] = stats["total_bytes_sent"].(int64) + conn.BytesSent
		stats["total_bytes_received"] = stats["total_bytes_received"].(int64) + conn.BytesReceived
		stats["total_messages_sent"] = stats["total_messages_sent"].(int) + conn.MessagesSent
		stats["total_messages_received"] = stats["total_messages_received"].(int) + conn.MessagesReceived
		conn.mu.RUnlock()
	}

	return stats
}