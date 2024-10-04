package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func NewRealtimeClient(apiKey string, state *AppState) *RealtimeClient {
	return &RealtimeClient{
		URL:        "wss://api.openai.com/v1/realtime",
		APIKey:     apiKey,
		handlers:   make(map[string][]func(Event)),
		send:       make(chan []byte, 256),
		debug:      state.DebugLevel > 0,
		dumpFrames: state.DebugLevel > 1,
		state:      state,
	}
}

func (c *RealtimeClient) Connect(model string) error {
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

	c.logf("Connecting to %s", u.String())
	c.logf("Headers: %v", headers)

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}

	conn, resp, err := dialer.Dial(u.String(), headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("websocket handshake failed with status %d: %s\nResponse body: %s", resp.StatusCode, err, string(body))
		}
		return fmt.Errorf("error connecting to websocket: %v", err)
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
