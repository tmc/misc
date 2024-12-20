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
	"go.uber.org/zap"
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
		return fmt.Errorf("error parsing URL: %v", err)
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

	logDebug("Connecting to WebSocket",
		zap.String("url", u.String()),
		zap.Any("headers", headers),
	)

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("websocket handshake failed with status %d: %s\nResponse body: %s", resp.StatusCode, err, string(body))
		}
		return fmt.Errorf("error connecting to websocket: %v", err)
	}
	c.conn = conn

	if resp != nil {
		logDebug("Connected to WebSocket",
			zap.String("status", resp.Status),
			zap.Any("headers", resp.Header),
		)
	}

	if c.dumpFrames {
		logDebug("WebSocket handshake details",
			zap.Any("requestHeaders", resp.Request.Header),
			zap.String("responseStatus", resp.Status),
			zap.Any("responseHeaders", resp.Header),
		)
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
	logDebug("Sending event", zap.Any("event", event))
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
				logError("Error reading from websocket", err)
			}
			break
		}

		if c.dumpFrames {
			logDebug("Received raw frame", zap.ByteString("message", message))
		}

		var event Event
		if err := json.Unmarshal(message, &event); err != nil {
			logError("Error unmarshaling event", err)
			continue
		}

		logDebug("Received event", zap.Any("event", event))
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
				logDebug("Sending raw frame", zap.ByteString("message", message))
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				logError("Error getting next writer", err)
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				logError("Error closing writer", err)
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logError("Error writing ping message", err)
				return
			}
		}
	}
}

func (c *RealtimeClient) handleEvent(event Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store session information when received
	if event.Type == "session.created" || event.Type == "session.update" {
		if event.Session != nil {
			c.state.Session = event.Session
			logDebug("Session updated", zap.Any("session", c.state.Session))
		}
	}

	handlers := c.handlers[event.Type]
	for _, handler := range handlers {
		go handler(event)
	}

	allHandlers := c.handlers["*"]
	for _, handler := range allHandlers {
		go handler(event)
	}
}
