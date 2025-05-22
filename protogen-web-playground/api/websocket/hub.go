package websocket

import (
	"encoding/json"
	"sync"
)

// Hub maintains active WebSocket connections and handles messages
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from clients
	broadcast chan []byte

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for concurrent access to the sessions map
	mu sync.Mutex

	// Map of session IDs to session data
	sessions map[string]*Session
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		sessions:   make(map[string]*Session),
	}
}

// Run starts the WebSocket hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			h.handleClientRegistration(client)

		case client := <-h.unregister:
			h.handleClientUnregistration(client)

		case message := <-h.broadcast:
			h.handleBroadcastMessage(message)
		}
	}
}

// handleClientRegistration processes a new client connection
func (h *Hub) handleClientRegistration(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if the session exists, create it if it doesn't
	session, exists := h.sessions[client.sessionID]
	if !exists {
		session = &Session{
			ID:      client.sessionID,
			Clients: make(map[*Client]bool),
			Data:    make(map[string]interface{}),
		}
		h.sessions[client.sessionID] = session
	}

	// Add client to the session
	session.Clients[client] = true

	// Send current session state to the client
	stateMsg := StateMessage{
		Type: "state",
		Data: session.Data,
	}
	data, _ := json.Marshal(stateMsg)
	client.send <- data
}

// handleClientUnregistration processes a client disconnection
func (h *Hub) handleClientUnregistration(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)

		// Remove client from session
		if session, exists := h.sessions[client.sessionID]; exists {
			delete(session.Clients, client)

			// Clean up empty sessions
			if len(session.Clients) == 0 {
				delete(h.sessions, client.sessionID)
			}
		}
	}
}

// handleBroadcastMessage processes a message from a client
func (h *Hub) handleBroadcastMessage(message []byte) {
	// Parse message to determine the session ID
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Get the session
	session, exists := h.sessions[msg.SessionID]
	if !exists {
		return
	}

	// Process the message
	switch msg.Type {
	case "update":
		// Update session data
		if msg.Data != nil {
			var updateData map[string]interface{}
			if updateBytes, err := json.Marshal(msg.Data); err == nil {
				if err := json.Unmarshal(updateBytes, &updateData); err == nil {
					for k, v := range updateData {
						session.Data[k] = v
					}
				}
			}
		}

		// Broadcast the update to all clients in the session
		for client := range session.Clients {
			select {
			case client.send <- message:
			default:
				delete(session.Clients, client)
				delete(h.clients, client)
				close(client.send)
			}
		}
	}
}