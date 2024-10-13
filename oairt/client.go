package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type RealtimeClientOption func(*RealtimeClient)

func WithDebug(debug bool) RealtimeClientOption {
	return func(c *RealtimeClient) {
		c.debug = debug
	}
}

func WithDumpFrames(dumpFrames bool) RealtimeClientOption {
	return func(c *RealtimeClient) {
		c.dumpFrames = dumpFrames
	}
}

func NewRealtimeClient(apiKey string, state *AppState, options ...RealtimeClientOption) *RealtimeClient {
	c := &RealtimeClient{
		URL:      "wss://api.openai.com/v1/realtime",
		APIKey:   apiKey,
		handlers: make(map[string][]func(Event)),
		send:     make(chan []byte, 256),
		state:    state,
	}

	for _, option := range options {
		option(c)
	}

	return c
}

func (c *RealtimeClient) Connect(ctx context.Context, model string) error {
	u, err := url.Parse(c.URL)
	if err != nil {
		return NewAppError(err, "Error parsing URL", "URL_PARSE_ERROR")
	}

	if model != "" {
		q := u.Query()
		q.Set("model", model)
		u.RawQuery = q.Encode()
	}

	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+c.APIKey)
	headers.Add("OpenAI-Beta", "realtime=v1")
	headers.Add("User-Agent", "OpenAI-Realtime-Client/1.0")

	c.logf("Connecting to %s", u.String())
	c.logf("Headers: %v", headers)

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return NewAppError(err, fmt.Sprintf("WebSocket handshake failed with status %d: %s\nResponse body: %s", resp.StatusCode, err, string(body)), "WEBSOCKET_HANDSHAKE_ERROR")
		}
		return NewAppError(err, "Error connecting to websocket", "WEBSOCKET_CONNECTION_ERROR")
	}
	c.conn = conn

	if resp != nil {
		c.logf("Connected with status: %s", resp.Status)
		c.logf("Response headers: %v", resp.Header)
	}

	if c.dumpFrames {
		c.logf("WebSocket handshake request headers:")
		for k, v := range resp.Request.Header {
			c.logf("%s: %s", k, v)
		}
		c.logf("WebSocket handshake response status: %s", resp.Status)
		c.logf("WebSocket handshake response headers:")
		for k, v := range resp.Header {
			c.logf("%s: %s", k, v)
		}
	}

	go c.readPump()
	go c.writePump()

	return nil
}

func (c *RealtimeClient) Disconnect() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *RealtimeClient) Send(event Event) error {
	c.logf("Sending event: %s", mustMarshal(event))
	return c.conn.WriteJSON(event)
}

func (c *RealtimeClient) On(eventType string, handler func(Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[eventType] = append(c.handlers[eventType], handler)
}

func (c *RealtimeClient) readPump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logf("Error reading from websocket: %v", err)
			}
			break
		}

		if c.dumpFrames {
			c.logf("Received raw frame: %s", message)
		}

		var event Event
		if err := json.Unmarshal(message, &event); err != nil {
			c.logf("Error unmarshaling event: %v", err)
			continue
		}

		c.logf("Received event: %s", message)
		c.handleEvent(event)
	}
}

func (c *RealtimeClient) writePump() {
	ticker := time.NewTicker(time.Second * 30)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if c.dumpFrames {
				c.logf("Sending raw frame: %s", message)
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.logf("Error getting next writer: %v", err)
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				c.logf("Error closing writer: %v", err)
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logf("Error writing ping message: %v", err)
				return
			}
		}
	}
}

func (c *RealtimeClient) handleEvent(event Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	handlers := c.handlers[event.Type]
	for _, handler := range handlers {
		go handler(event)
	}

	allHandlers := c.handlers["*"]
	for _, handler := range allHandlers {
		go handler(event)
	}
}

func (c *RealtimeClient) logf(format string, v ...interface{}) {
	if c.debug {
		logDebug(c.state, format, v...)
	}
}
