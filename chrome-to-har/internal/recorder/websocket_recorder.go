package recorder

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/chromedp/cdproto/har"
	"github.com/chromedp/cdproto/network"
	"github.com/tmc/misc/chrome-to-har/internal/browser"
)

// WebSocketRecorder extends the standard recorder with WebSocket support
type WebSocketRecorder struct {
	*Recorder
	webSocketConnections map[string]*browser.WebSocketConnection
	wsConverter          *browser.WebSocketHARConverter
	wsFilter             *browser.WebSocketHARFilter
	wsEnabled            bool
	wsLock               sync.RWMutex
}

// WebSocketRecorderOption configures WebSocket recording options
type WebSocketRecorderOption func(*WebSocketRecorder) error

// WithWebSocketEnabled enables WebSocket recording
func WithWebSocketEnabled(enabled bool) WebSocketRecorderOption {
	return func(r *WebSocketRecorder) error {
		r.wsEnabled = enabled
		return nil
	}
}

// WithWebSocketFilter sets a filter for WebSocket connections
func WithWebSocketFilter(filter *browser.WebSocketHARFilter) WebSocketRecorderOption {
	return func(r *WebSocketRecorder) error {
		r.wsFilter = filter
		return nil
	}
}

// NewWebSocketRecorder creates a new WebSocket-enabled recorder
func NewWebSocketRecorder(opts ...Option) (*WebSocketRecorder, error) {
	baseRecorder, err := New(opts...)
	if err != nil {
		return nil, err
	}

	return &WebSocketRecorder{
		Recorder:             baseRecorder,
		webSocketConnections: make(map[string]*browser.WebSocketConnection),
		wsConverter:          browser.NewWebSocketHARConverter(),
		wsEnabled:            true,
	}, nil
}

// NewWebSocketRecorderWithOptions creates a new WebSocket recorder with WebSocket-specific options
func NewWebSocketRecorderWithOptions(opts []Option, wsOpts []WebSocketRecorderOption) (*WebSocketRecorder, error) {
	recorder, err := NewWebSocketRecorder(opts...)
	if err != nil {
		return nil, err
	}

	for _, opt := range wsOpts {
		if err := opt(recorder); err != nil {
			return nil, err
		}
	}

	return recorder, nil
}

// HandleNetworkEvent handles both standard network events and WebSocket events
func (r *WebSocketRecorder) HandleNetworkEvent(ctx context.Context) func(interface{}) {
	baseHandler := r.Recorder.HandleNetworkEvent(ctx)

	return func(ev interface{}) {
		// Handle standard network events
		baseHandler(ev)

		// Handle WebSocket events if enabled
		if r.wsEnabled {
			r.handleWebSocketEvent(ev)
		}
	}
}

// handleWebSocketEvent processes WebSocket-specific events
func (r *WebSocketRecorder) handleWebSocketEvent(ev interface{}) {
	r.wsLock.Lock()
	defer r.wsLock.Unlock()

	switch e := ev.(type) {
	case *network.EventWebSocketCreated:
		r.handleWebSocketCreated(e)
	case *network.EventWebSocketFrameReceived:
		r.handleWebSocketFrameReceived(e)
	case *network.EventWebSocketFrameSent:
		r.handleWebSocketFrameSent(e)
	case *network.EventWebSocketClosed:
		r.handleWebSocketClosed(e)
	case *network.EventWebSocketFrameError:
		r.handleWebSocketFrameError(e)
	case *network.EventWebSocketWillSendHandshakeRequest:
		r.handleWebSocketHandshakeRequest(e)
	case *network.EventWebSocketHandshakeResponseReceived:
		r.handleWebSocketHandshakeResponse(e)
	}
}

// handleWebSocketCreated handles WebSocket creation events
func (r *WebSocketRecorder) handleWebSocketCreated(e *network.EventWebSocketCreated) {
	if r.verbose {
		log.Printf("WebSocket Created: %s", e.URL)
	}

	conn := &browser.WebSocketConnection{
		ID:          string(e.RequestID),
		URL:         e.URL,
		State:       "connecting",
		ConnectedAt: time.Now(),
		Frames:      make([]browser.WebSocketFrame, 0),
		Headers:     make(map[string]string),
	}

	r.webSocketConnections[conn.ID] = conn
	r.wsConverter.AddConnection(conn)
}

// handleWebSocketFrameReceived handles received WebSocket frames
func (r *WebSocketRecorder) handleWebSocketFrameReceived(e *network.EventWebSocketFrameReceived) {
	connID := string(e.RequestID)
	conn, exists := r.webSocketConnections[connID]
	if !exists {
		return
	}

	frame := browser.WebSocketFrame{
		Type:      r.getFrameType(e.Response.Opcode),
		Direction: "received",
		Data:      e.Response.PayloadData,
		Timestamp: time.Now(),
		Size:      int64(len(e.Response.PayloadData)),
		Opcode:    int64(e.Response.Opcode),
	}

	conn.Frames = append(conn.Frames, frame)
	conn.BytesReceived += frame.Size
	conn.MessagesReceived++

	if r.verbose {
		log.Printf("WebSocket Frame Received: %s [%s] %d bytes", 
			conn.URL, frame.Type, frame.Size)
	}

	if r.streaming {
		r.streamWebSocketFrame(conn, &frame)
	}
}

// handleWebSocketFrameSent handles sent WebSocket frames
func (r *WebSocketRecorder) handleWebSocketFrameSent(e *network.EventWebSocketFrameSent) {
	connID := string(e.RequestID)
	conn, exists := r.webSocketConnections[connID]
	if !exists {
		return
	}

	frame := browser.WebSocketFrame{
		Type:      r.getFrameType(e.Response.Opcode),
		Direction: "sent",
		Data:      e.Response.PayloadData,
		Timestamp: time.Now(),
		Size:      int64(len(e.Response.PayloadData)),
		Opcode:    int64(e.Response.Opcode),
	}

	conn.Frames = append(conn.Frames, frame)
	conn.BytesSent += frame.Size
	conn.MessagesSent++

	if r.verbose {
		log.Printf("WebSocket Frame Sent: %s [%s] %d bytes", 
			conn.URL, frame.Type, frame.Size)
	}

	if r.streaming {
		r.streamWebSocketFrame(conn, &frame)
	}
}

// handleWebSocketClosed handles WebSocket closure events
func (r *WebSocketRecorder) handleWebSocketClosed(e *network.EventWebSocketClosed) {
	connID := string(e.RequestID)
	conn, exists := r.webSocketConnections[connID]
	if !exists {
		return
	}

	now := time.Now()
	conn.DisconnectedAt = &now
	conn.State = "closed"

	if r.verbose {
		log.Printf("WebSocket Closed: %s", conn.URL)
	}

	if r.streaming {
		r.streamWebSocketClosure(conn)
	}
}

// handleWebSocketFrameError handles WebSocket frame errors
func (r *WebSocketRecorder) handleWebSocketFrameError(e *network.EventWebSocketFrameError) {
	connID := string(e.RequestID)
	conn, exists := r.webSocketConnections[connID]
	if !exists {
		return
	}

	if r.verbose {
		log.Printf("WebSocket Frame Error: %s - %s", conn.URL, e.ErrorMessage)
	}

	if r.streaming {
		r.streamWebSocketError(conn, e.ErrorMessage)
	}
}

// handleWebSocketHandshakeRequest handles WebSocket handshake requests
func (r *WebSocketRecorder) handleWebSocketHandshakeRequest(e *network.EventWebSocketWillSendHandshakeRequest) {
	connID := string(e.RequestID)
	conn, exists := r.webSocketConnections[connID]
	if !exists {
		return
	}

	// Store handshake request headers
	if e.Request.Headers != nil {
		for key, value := range e.Request.Headers {
			if str, ok := value.(string); ok {
				conn.Headers[key] = str
			}
		}
	}

	if r.verbose {
		log.Printf("WebSocket Handshake Request: %s", conn.URL)
	}
}

// handleWebSocketHandshakeResponse handles WebSocket handshake responses
func (r *WebSocketRecorder) handleWebSocketHandshakeResponse(e *network.EventWebSocketHandshakeResponseReceived) {
	connID := string(e.RequestID)
	conn, exists := r.webSocketConnections[connID]
	if !exists {
		return
	}

	conn.State = "open"
	conn.ConnectionLatency = time.Since(conn.ConnectedAt)

	// Extract protocol and extensions from response
	if e.Response.Headers != nil {
		if protocol, ok := e.Response.Headers["sec-websocket-protocol"]; ok {
			if str, ok := protocol.(string); ok {
				conn.Protocol = str
			}
		}
		if extensions, ok := e.Response.Headers["sec-websocket-extensions"]; ok {
			if str, ok := extensions.(string); ok {
				conn.Extensions = []string{str}
			}
		}
	}

	if r.verbose {
		log.Printf("WebSocket Handshake Response: %s - Connected in %v", 
			conn.URL, conn.ConnectionLatency)
	}

	if r.streaming {
		r.streamWebSocketConnection(conn)
	}
}

// getFrameType converts opcode to frame type
func (r *WebSocketRecorder) getFrameType(opcode int64) string {
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

// streamWebSocketFrame streams WebSocket frame events
func (r *WebSocketRecorder) streamWebSocketFrame(conn *browser.WebSocketConnection, frame *browser.WebSocketFrame) {
	event := map[string]interface{}{
		"type":      "websocket_frame",
		"timestamp": frame.Timestamp.Format(time.RFC3339),
		"websocket": map[string]interface{}{
			"id":        conn.ID,
			"url":       conn.URL,
			"state":     conn.State,
		},
		"frame": map[string]interface{}{
			"type":      frame.Type,
			"direction": frame.Direction,
			"size":      frame.Size,
			"opcode":    frame.Opcode,
			"data":      frame.Data,
		},
	}

	if r.template != "" {
		r.streamTemplatedEvent(event)
	} else {
		json.NewEncoder(os.Stdout).Encode(event)
	}
}

// streamWebSocketConnection streams WebSocket connection events
func (r *WebSocketRecorder) streamWebSocketConnection(conn *browser.WebSocketConnection) {
	event := map[string]interface{}{
		"type":      "websocket_connect",
		"timestamp": time.Now().Format(time.RFC3339),
		"websocket": map[string]interface{}{
			"id":                conn.ID,
			"url":               conn.URL,
			"state":             conn.State,
			"protocol":          conn.Protocol,
			"extensions":        conn.Extensions,
			"connection_latency": conn.ConnectionLatency.Nanoseconds() / 1e6,
		},
	}

	if r.template != "" {
		r.streamTemplatedEvent(event)
	} else {
		json.NewEncoder(os.Stdout).Encode(event)
	}
}

// streamWebSocketClosure streams WebSocket closure events
func (r *WebSocketRecorder) streamWebSocketClosure(conn *browser.WebSocketConnection) {
	event := map[string]interface{}{
		"type":      "websocket_close",
		"timestamp": time.Now().Format(time.RFC3339),
		"websocket": map[string]interface{}{
			"id":           conn.ID,
			"url":          conn.URL,
			"state":        conn.State,
			"close_code":   conn.CloseCode,
			"close_reason": conn.CloseReason,
			"stats": map[string]interface{}{
				"bytes_sent":         conn.BytesSent,
				"bytes_received":     conn.BytesReceived,
				"messages_sent":      conn.MessagesSent,
				"messages_received":  conn.MessagesReceived,
				"connection_time":    conn.ConnectionLatency.Nanoseconds() / 1e6,
			},
		},
	}

	if r.template != "" {
		r.streamTemplatedEvent(event)
	} else {
		json.NewEncoder(os.Stdout).Encode(event)
	}
}

// streamWebSocketError streams WebSocket error events
func (r *WebSocketRecorder) streamWebSocketError(conn *browser.WebSocketConnection, errorMessage string) {
	event := map[string]interface{}{
		"type":      "websocket_error",
		"timestamp": time.Now().Format(time.RFC3339),
		"websocket": map[string]interface{}{
			"id":    conn.ID,
			"url":   conn.URL,
			"state": conn.State,
		},
		"error": errorMessage,
	}

	if r.template != "" {
		r.streamTemplatedEvent(event)
	} else {
		json.NewEncoder(os.Stdout).Encode(event)
	}
}

// streamTemplatedEvent streams events using a template
func (r *WebSocketRecorder) streamTemplatedEvent(event map[string]interface{}) {
	// This would use the same template mechanism as the base recorder
	// For now, fallback to JSON
	json.NewEncoder(os.Stdout).Encode(event)
}

// GetWebSocketConnections returns all recorded WebSocket connections
func (r *WebSocketRecorder) GetWebSocketConnections() map[string]*browser.WebSocketConnection {
	r.wsLock.RLock()
	defer r.wsLock.RUnlock()

	result := make(map[string]*browser.WebSocketConnection)
	for k, v := range r.webSocketConnections {
		result[k] = v
	}
	return result
}

// HAR returns the HAR data including WebSocket connections
func (r *WebSocketRecorder) HAR() (*har.HAR, error) {
	// Get base HAR data
	baseHAR, err := r.Recorder.HAR()
	if err != nil {
		return nil, err
	}

	// Extend with WebSocket data if enabled
	if r.wsEnabled {
		return browser.ExtendHARWithWebSockets(baseHAR, r.GetWebSocketConnections()), nil
	}

	return baseHAR, nil
}

// HARWithWebSocketData returns HAR data with WebSocket-specific extensions
func (r *WebSocketRecorder) HARWithWebSocketData() ([]byte, error) {
	exporter := browser.NewWebSocketHARExporter(r.wsFilter)
	return exporter.ExportWithWebSocketData(r.GetWebSocketConnections())
}

// GetWebSocketStatistics returns statistics for all WebSocket connections
func (r *WebSocketRecorder) GetWebSocketStatistics() map[string]interface{} {
	r.wsLock.RLock()
	defer r.wsLock.RUnlock()

	stats := map[string]interface{}{
		"total_connections":      len(r.webSocketConnections),
		"active_connections":     0,
		"total_bytes_sent":       int64(0),
		"total_bytes_received":   int64(0),
		"total_messages_sent":    0,
		"total_messages_received": 0,
		"connections":            make(map[string]interface{}),
	}

	for id, conn := range r.webSocketConnections {
		if conn.State == "open" {
			stats["active_connections"] = stats["active_connections"].(int) + 1
		}

		stats["total_bytes_sent"] = stats["total_bytes_sent"].(int64) + conn.BytesSent
		stats["total_bytes_received"] = stats["total_bytes_received"].(int64) + conn.BytesReceived
		stats["total_messages_sent"] = stats["total_messages_sent"].(int) + conn.MessagesSent
		stats["total_messages_received"] = stats["total_messages_received"].(int) + conn.MessagesReceived

		connStats := map[string]interface{}{
			"url":               conn.URL,
			"state":             conn.State,
			"protocol":          conn.Protocol,
			"bytes_sent":        conn.BytesSent,
			"bytes_received":    conn.BytesReceived,
			"messages_sent":     conn.MessagesSent,
			"messages_received": conn.MessagesReceived,
			"connection_time":   conn.ConnectionLatency.Nanoseconds() / 1e6,
			"frame_count":       len(conn.Frames),
		}

		if conn.DisconnectedAt != nil {
			connStats["disconnected_at"] = conn.DisconnectedAt.Format(time.RFC3339)
			connStats["active_time"] = conn.DisconnectedAt.Sub(conn.ConnectedAt).Nanoseconds() / 1e6
		} else {
			connStats["active_time"] = time.Since(conn.ConnectedAt).Nanoseconds() / 1e6
		}

		stats["connections"].(map[string]interface{})[id] = connStats
	}

	return stats
}

// FilterWebSocketConnections filters WebSocket connections based on criteria
func (r *WebSocketRecorder) FilterWebSocketConnections(filter *browser.WebSocketHARFilter) []*browser.WebSocketConnection {
	r.wsLock.RLock()
	defer r.wsLock.RUnlock()

	var filtered []*browser.WebSocketConnection

	// Convert connections to HAR entries for filtering
	converter := browser.NewWebSocketHARConverter()
	for _, conn := range r.webSocketConnections {
		converter.AddConnection(conn)
	}

	wsEntries := converter.ConvertToHAR()
	filteredEntries := filter.ApplyFilter(wsEntries)

	// Convert back to connections
	for _, entry := range filteredEntries {
		if entry.WebSocket != nil {
			if conn, exists := r.webSocketConnections[entry.WebSocket.URL]; exists {
				filtered = append(filtered, conn)
			}
		}
	}

	return filtered
}

// ClearWebSocketConnections clears all recorded WebSocket connections
func (r *WebSocketRecorder) ClearWebSocketConnections() {
	r.wsLock.Lock()
	defer r.wsLock.Unlock()

	r.webSocketConnections = make(map[string]*browser.WebSocketConnection)
	r.wsConverter = browser.NewWebSocketHARConverter()
}

// EnableWebSocketRecording enables WebSocket recording
func (r *WebSocketRecorder) EnableWebSocketRecording() {
	r.wsLock.Lock()
	defer r.wsLock.Unlock()
	r.wsEnabled = true
}

// DisableWebSocketRecording disables WebSocket recording
func (r *WebSocketRecorder) DisableWebSocketRecording() {
	r.wsLock.Lock()
	defer r.wsLock.Unlock()
	r.wsEnabled = false
}