package browser

import (
	"encoding/json"
	"time"

	"github.com/chromedp/cdproto/har"
)

// WebSocketHAREntry represents a WebSocket connection in HAR format
type WebSocketHAREntry struct {
	*har.Entry
	WebSocket *WebSocketHARData `json:"_webSocket,omitempty"`
}

// WebSocketHARData contains WebSocket-specific data for HAR format
type WebSocketHARData struct {
	URL               string                `json:"url"`
	Protocol          string                `json:"protocol,omitempty"`
	Extensions        []string              `json:"extensions,omitempty"`
	State             string                `json:"state"`
	ConnectedAt       string                `json:"connectedAt"`
	DisconnectedAt    string                `json:"disconnectedAt,omitempty"`
	ConnectionLatency float64               `json:"connectionLatency"` // in milliseconds
	CloseCode         int                   `json:"closeCode,omitempty"`
	CloseReason       string                `json:"closeReason,omitempty"`
	Messages          []WebSocketHARMessage `json:"messages"`
	Statistics        WebSocketHARStats     `json:"statistics"`
}

// WebSocketHARMessage represents a WebSocket message in HAR format
type WebSocketHARMessage struct {
	Type      string      `json:"type"`      // "text", "binary", "close", "ping", "pong"
	Direction string      `json:"direction"` // "sent", "received"
	Data      interface{} `json:"data"`
	Time      string      `json:"time"`   // ISO 8601 timestamp
	Size      int64       `json:"size"`   // in bytes
	Opcode    int         `json:"opcode"` // WebSocket opcode
}

// WebSocketHARStats contains WebSocket connection statistics
type WebSocketHARStats struct {
	BytesSent        int64 `json:"bytesSent"`
	BytesReceived    int64 `json:"bytesReceived"`
	MessagesSent     int   `json:"messagesSent"`
	MessagesReceived int   `json:"messagesReceived"`
	ConnectionTime   int64 `json:"connectionTime"`   // in milliseconds
	ActiveTime       int64 `json:"activeTime"`       // in milliseconds
	AverageLatency   int64 `json:"averageLatency"`   // in milliseconds
	MessageFrequency int64 `json:"messageFrequency"` // messages per second
	DataThroughput   int64 `json:"dataThroughput"`   // bytes per second
}

// WebSocketHARConverter converts WebSocket connections to HAR format
type WebSocketHARConverter struct {
	connections map[string]*WebSocketConnection
}

// NewWebSocketHARConverter creates a new WebSocket HAR converter
func NewWebSocketHARConverter() *WebSocketHARConverter {
	return &WebSocketHARConverter{
		connections: make(map[string]*WebSocketConnection),
	}
}

// AddConnection adds a WebSocket connection to be converted
func (c *WebSocketHARConverter) AddConnection(conn *WebSocketConnection) {
	c.connections[conn.ID] = conn
}

// ConvertToHAR converts all WebSocket connections to HAR format
func (c *WebSocketHARConverter) ConvertToHAR() []*WebSocketHAREntry {
	var entries []*WebSocketHAREntry

	for _, conn := range c.connections {
		entry := c.convertConnectionToHAR(conn)
		entries = append(entries, entry)
	}

	return entries
}

// convertConnectionToHAR converts a single WebSocket connection to HAR format
func (c *WebSocketHARConverter) convertConnectionToHAR(conn *WebSocketConnection) *WebSocketHAREntry {
	conn.mu.RLock()
	defer conn.mu.RUnlock()

	// Create basic HAR entry
	entry := &har.Entry{
		StartedDateTime: conn.ConnectedAt.Format(time.RFC3339),
		Time:            c.calculateTotalTime(conn),
		Request:         c.createHARRequest(conn),
		Response:        c.createHARResponse(conn),
	}

	// Create WebSocket-specific data
	wsData := &WebSocketHARData{
		URL:               conn.URL,
		Protocol:          conn.Protocol,
		Extensions:        conn.Extensions,
		State:             conn.State,
		ConnectedAt:       conn.ConnectedAt.Format(time.RFC3339),
		ConnectionLatency: float64(conn.ConnectionLatency.Nanoseconds()) / 1e6,
		CloseCode:         conn.CloseCode,
		CloseReason:       conn.CloseReason,
		Messages:          c.convertMessages(conn.Frames),
		Statistics:        c.calculateStatistics(conn),
	}

	if conn.DisconnectedAt != nil {
		wsData.DisconnectedAt = conn.DisconnectedAt.Format(time.RFC3339)
	}

	return &WebSocketHAREntry{
		Entry:     entry,
		WebSocket: wsData,
	}
}

// createHARRequest creates a HAR request for WebSocket handshake
func (c *WebSocketHARConverter) createHARRequest(conn *WebSocketConnection) *har.Request {
	headers := make([]*har.NameValuePair, 0, len(conn.Headers))
	for name, value := range conn.Headers {
		headers = append(headers, &har.NameValuePair{
			Name:  name,
			Value: value,
		})
	}

	return &har.Request{
		Method:      "GET",
		URL:         conn.URL,
		HTTPVersion: "HTTP/1.1",
		Headers:     headers,
		HeadersSize: -1,
		BodySize:    0,
	}
}

// createHARResponse creates a HAR response for WebSocket handshake
func (c *WebSocketHARConverter) createHARResponse(conn *WebSocketConnection) *har.Response {
	status := 101 // Switching Protocols
	if conn.State == "closed" && conn.CloseCode != 0 {
		status = 400 // Bad Request (if connection failed)
	}

	return &har.Response{
		Status:      int64(status),
		StatusText:  c.getStatusText(status),
		HTTPVersion: "HTTP/1.1",
		Headers:     []*har.NameValuePair{},
		HeadersSize: -1,
		BodySize:    0,
		Content: &har.Content{
			Size:     0,
			MimeType: "application/octet-stream",
		},
	}
}

// convertMessages converts WebSocket frames to HAR messages
func (c *WebSocketHARConverter) convertMessages(frames []WebSocketFrame) []WebSocketHARMessage {
	messages := make([]WebSocketHARMessage, len(frames))

	for i, frame := range frames {
		messages[i] = WebSocketHARMessage{
			Type:      frame.Type,
			Direction: frame.Direction,
			Data:      frame.Data,
			Time:      frame.Timestamp.Format(time.RFC3339),
			Size:      frame.Size,
			Opcode:    frame.Opcode,
		}
	}

	return messages
}

// calculateStatistics calculates WebSocket connection statistics
func (c *WebSocketHARConverter) calculateStatistics(conn *WebSocketConnection) WebSocketHARStats {
	stats := WebSocketHARStats{
		BytesSent:        conn.BytesSent,
		BytesReceived:    conn.BytesReceived,
		MessagesSent:     conn.MessagesSent,
		MessagesReceived: conn.MessagesReceived,
		ConnectionTime:   int64(conn.ConnectionLatency.Nanoseconds() / 1e6),
	}

	// Calculate active time
	if conn.DisconnectedAt != nil {
		stats.ActiveTime = conn.DisconnectedAt.Sub(conn.ConnectedAt).Nanoseconds() / 1e6
	} else {
		stats.ActiveTime = time.Since(conn.ConnectedAt).Nanoseconds() / 1e6
	}

	// Calculate averages and rates
	if stats.ActiveTime > 0 {
		totalMessages := conn.MessagesSent + conn.MessagesReceived
		totalBytes := conn.BytesSent + conn.BytesReceived

		if totalMessages > 0 {
			stats.MessageFrequency = int64(float64(totalMessages) / (float64(stats.ActiveTime) / 1000.0))
		}

		if totalBytes > 0 {
			stats.DataThroughput = int64(float64(totalBytes) / (float64(stats.ActiveTime) / 1000.0))
		}

		// Calculate average latency (simplified estimation)
		if len(conn.Frames) > 0 {
			stats.AverageLatency = c.calculateAverageLatency(conn.Frames)
		}
	}

	return stats
}

// calculateAverageLatency calculates average message latency
func (c *WebSocketHARConverter) calculateAverageLatency(frames []WebSocketFrame) int64 {
	if len(frames) < 2 {
		return 0
	}

	var totalLatency int64
	var pairs int

	// Simple approach: measure time between consecutive sent/received pairs
	for i := 1; i < len(frames); i++ {
		if frames[i-1].Direction == "sent" && frames[i].Direction == "received" {
			latency := frames[i].Timestamp.Sub(frames[i-1].Timestamp).Nanoseconds() / 1e6
			totalLatency += latency
			pairs++
		}
	}

	if pairs > 0 {
		return totalLatency / int64(pairs)
	}

	return 0
}

// calculateTotalTime calculates total time for HAR entry
func (c *WebSocketHARConverter) calculateTotalTime(conn *WebSocketConnection) float64 {
	if conn.DisconnectedAt != nil {
		return float64(conn.DisconnectedAt.Sub(conn.ConnectedAt).Nanoseconds()) / 1e6
	}
	return float64(time.Since(conn.ConnectedAt).Nanoseconds()) / 1e6
}

// getStatusText returns HTTP status text for status code
func (c *WebSocketHARConverter) getStatusText(status int) string {
	switch status {
	case 101:
		return "Switching Protocols"
	case 400:
		return "Bad Request"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	default:
		return "Unknown"
	}
}

// MarshalJSON implements json.Marshaler for custom JSON serialization
func (w *WebSocketHAREntry) MarshalJSON() ([]byte, error) {
	type Alias WebSocketHAREntry
	return json.Marshal(&struct {
		*Alias
		WebSocket *WebSocketHARData `json:"_webSocket,omitempty"`
	}{
		Alias:     (*Alias)(w),
		WebSocket: w.WebSocket,
	})
}

// ExtendHARWithWebSockets extends a standard HAR with WebSocket data
func ExtendHARWithWebSockets(harData *har.HAR, connections map[string]*WebSocketConnection) *har.HAR {
	if harData == nil {
		harData = &har.HAR{
			Log: &har.Log{
				Version: "1.2",
				Creator: &har.Creator{
					Name:    "chrome-to-har",
					Version: "1.0.0",
				},
				Pages:   []*har.Page{},
				Entries: []*har.Entry{},
			},
		}
	}

	// Convert WebSocket connections to HAR entries
	converter := NewWebSocketHARConverter()
	for _, conn := range connections {
		converter.AddConnection(conn)
	}

	wsEntries := converter.ConvertToHAR()

	// Add WebSocket entries to HAR
	for _, wsEntry := range wsEntries {
		harData.Log.Entries = append(harData.Log.Entries, wsEntry.Entry)
	}

	return harData
}

// WebSocketHARFilter filters WebSocket HAR entries based on criteria
type WebSocketHARFilter struct {
	URLPattern  string
	MessageType string
	Direction   string
	MinSize     int64
	MaxSize     int64
	TimeRange   *TimeRange
	StateFilter []string
}

// TimeRange represents a time range filter
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// ApplyFilter applies filters to WebSocket HAR entries
func (f *WebSocketHARFilter) ApplyFilter(entries []*WebSocketHAREntry) []*WebSocketHAREntry {
	var filtered []*WebSocketHAREntry

	for _, entry := range entries {
		if f.matchesFilter(entry) {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

// matchesFilter checks if an entry matches the filter criteria
func (f *WebSocketHARFilter) matchesFilter(entry *WebSocketHAREntry) bool {
	if entry.WebSocket == nil {
		return false
	}

	ws := entry.WebSocket

	// URL pattern filter
	if f.URLPattern != "" && !matchesPattern(ws.URL, f.URLPattern) {
		return false
	}

	// State filter
	if len(f.StateFilter) > 0 && !contains(f.StateFilter, ws.State) {
		return false
	}

	// Time range filter
	if f.TimeRange != nil {
		connectedAt, err := time.Parse(time.RFC3339, ws.ConnectedAt)
		if err != nil {
			return false
		}

		if connectedAt.Before(f.TimeRange.Start) || connectedAt.After(f.TimeRange.End) {
			return false
		}
	}

	// Message filters
	if f.MessageType != "" || f.Direction != "" || f.MinSize > 0 || f.MaxSize > 0 {
		hasMatchingMessage := false
		for _, msg := range ws.Messages {
			if f.matchesMessageFilter(msg) {
				hasMatchingMessage = true
				break
			}
		}
		if !hasMatchingMessage {
			return false
		}
	}

	return true
}

// matchesMessageFilter checks if a message matches the filter criteria
func (f *WebSocketHARFilter) matchesMessageFilter(msg WebSocketHARMessage) bool {
	if f.MessageType != "" && msg.Type != f.MessageType {
		return false
	}

	if f.Direction != "" && msg.Direction != f.Direction {
		return false
	}

	if f.MinSize > 0 && msg.Size < f.MinSize {
		return false
	}

	if f.MaxSize > 0 && msg.Size > f.MaxSize {
		return false
	}

	return true
}

// Helper functions
func matchesPattern(text, pattern string) bool {
	// Simple pattern matching - can be enhanced with regex
	return text == pattern || pattern == "*"
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// WebSocketHARExporter exports WebSocket HAR data
type WebSocketHARExporter struct {
	filter *WebSocketHARFilter
}

// NewWebSocketHARExporter creates a new WebSocket HAR exporter
func NewWebSocketHARExporter(filter *WebSocketHARFilter) *WebSocketHARExporter {
	return &WebSocketHARExporter{
		filter: filter,
	}
}

// Export exports WebSocket connections to HAR format
func (e *WebSocketHARExporter) Export(connections map[string]*WebSocketConnection) ([]byte, error) {
	converter := NewWebSocketHARConverter()
	for _, conn := range connections {
		converter.AddConnection(conn)
	}

	entries := converter.ConvertToHAR()

	// Apply filters if provided
	if e.filter != nil {
		entries = e.filter.ApplyFilter(entries)
	}

	// Create HAR structure
	harData := &har.HAR{
		Log: &har.Log{
			Version: "1.2",
			Creator: &har.Creator{
				Name:    "chrome-to-har-websocket",
				Version: "1.0.0",
			},
			Pages:   []*har.Page{},
			Entries: make([]*har.Entry, len(entries)),
		},
	}

	// Convert to standard HAR entries
	for i, entry := range entries {
		harData.Log.Entries[i] = entry.Entry
	}

	return json.MarshalIndent(harData, "", "  ")
}

// ExportWithWebSocketData exports with WebSocket-specific data included
func (e *WebSocketHARExporter) ExportWithWebSocketData(connections map[string]*WebSocketConnection) ([]byte, error) {
	converter := NewWebSocketHARConverter()
	for _, conn := range connections {
		converter.AddConnection(conn)
	}

	entries := converter.ConvertToHAR()

	// Apply filters if provided
	if e.filter != nil {
		entries = e.filter.ApplyFilter(entries)
	}

	// Create extended HAR structure with WebSocket data
	result := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"creator": map[string]interface{}{
				"name":    "chrome-to-har-websocket",
				"version": "1.0.0",
			},
			"pages":   []interface{}{},
			"entries": entries,
		},
	}

	return json.MarshalIndent(result, "", "  ")
}
