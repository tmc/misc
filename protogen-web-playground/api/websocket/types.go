package websocket

// Session represents a collaborative editing session
type Session struct {
	// Unique session ID
	ID string

	// Clients connected to this session
	Clients map[*Client]bool

	// Session data (proto files, templates, etc.)
	Data map[string]interface{}
}

// Message represents a WebSocket message
type Message struct {
	// Type of message (update, state, etc.)
	Type string `json:"type"`

	// Session ID
	SessionID string `json:"sessionId"`

	// Message data
	Data interface{} `json:"data"`
}

// StateMessage represents a message with the current state
type StateMessage struct {
	// Type of message ("state")
	Type string `json:"type"`

	// Session data
	Data map[string]interface{} `json:"data"`
}