package oairt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// Option configures a Client.
type Option func(*Client)

// WithDebug enables verbose debug logging via the configured Logger.
func WithDebug(debug bool) Option {
	return func(c *Client) { c.debug = debug }
}

// WithDumpFrames enables logging of raw WebSocket frames via the configured
// Logger. Frames may contain user data; do not enable in production.
func WithDumpFrames(dumpFrames bool) Option {
	return func(c *Client) { c.dumpFrames = dumpFrames }
}

// NewClient returns a Client configured for the given API key.
//
// Call Connect to dial the Realtime endpoint.
func NewClient(apiKey string, options ...Option) *Client {
	c := &Client{
		URL:      "wss://api.openai.com/v1/realtime",
		APIKey:   apiKey,
		handlers: make(map[string][]func(Event)),
		ordered:  make(map[string][]*orderedHandler),
		send:     make(chan []byte, 256),
		logger:   nopLogger{},
		closed:   make(chan struct{}),
	}
	for _, option := range options {
		option(c)
	}
	return c
}

// Connect dials the Realtime endpoint and starts the read/write pumps.
func (c *Client) Connect(ctx context.Context, model string) error {
	u, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if model != "" {
		q := u.Query()
		q.Set("model", model)
		u.RawQuery = q.Encode()
	}

	ua := c.userAgent
	if ua == "" {
		ua = "OpenAI-Realtime-Client/1.0"
	}
	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+c.APIKey)
	headers.Add("User-Agent", ua)

	if c.debug {
		c.logger.Debugf("connecting to %s", u.String())
	}

	dialer := c.resolveDialer()

	conn, resp, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("%w (status %d): %w: %s", ErrHandshake, resp.StatusCode, err, string(body))
		}
		return fmt.Errorf("%w: %w", ErrHandshake, err)
	}
	c.conn = conn

	if c.debug && resp != nil {
		c.logger.Debugf("connected: %s", resp.Status)
	}

	go c.readPump()
	go c.writePump()

	// Tie pump teardown to the caller's context: when ctx ends (or Close
	// runs first), shut the connection down. closeOnce makes either path
	// idempotent.
	go func() {
		select {
		case <-ctx.Done():
			c.Close()
		case <-c.closed:
		}
	}()

	return nil
}

// Close terminates the WebSocket connection and waits for in-flight
// handler goroutines to finish. It is safe to call concurrently and
// repeatedly.
//
// Close does not close the c.send channel; writePump observes c.closed
// and returns. That ordering avoids a data race between concurrent
// Send calls and Close on the channel itself.
func (c *Client) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		// Mark the client closed under the mutex so dispatch sees the
		// state transition before we Wait, ruling out Add-after-Wait
		// on dispatchWG.
		c.mu.Lock()
		close(c.closed)
		for _, handlers := range c.ordered {
			for _, h := range handlers {
				close(h.ch)
			}
		}
		c.mu.Unlock()

		if c.conn != nil {
			closeErr = c.conn.Close()
		}
		c.dispatchWG.Wait()
		c.orderedWG.Wait()
	})
	return closeErr
}

// Send queues an event for delivery to the server.
func (c *Client) Send(event Event) error {
	if c.conn == nil {
		return fmt.Errorf("send: %w", ErrNotConnected)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	// Single select: c.closed wins over c.send if both are ready, so a
	// post-Close caller deterministically sees ErrClosed instead of
	// pushing onto a channel that writePump is no longer draining.
	select {
	case <-c.closed:
		return fmt.Errorf("send: %w", ErrClosed)
	default:
	}
	select {
	case c.send <- data:
		return nil
	case <-c.closed:
		return fmt.Errorf("send: %w", ErrClosed)
	default:
		return fmt.Errorf("send: %w", ErrSendQueueFull)
	}
}

// SendAudio base64-encodes data and queues it as an
// input_audio_buffer.append event.
func (c *Client) SendAudio(data []byte) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	return c.Send(Event{
		Type:  "input_audio_buffer.append",
		Audio: encoded,
	})
}

// On registers a handler for the given event type. Use "*" to receive every
// event. Handlers run in their own goroutine.
func (c *Client) On(eventType string, handler func(Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[eventType] = append(c.handlers[eventType], handler)
}

// OnOrdered registers a handler that receives matching events in websocket
// read order. Use it for stream assembly, such as text or transcript deltas.
//
// Ordered handlers run on a per-handler worker goroutine. A slow ordered
// handler can backpressure reads for its own event stream, but it does not
// serialize ordinary [Client.On] handlers.
func (c *Client) OnOrdered(eventType string, handler func(Event)) {
	h := &orderedHandler{
		fn: handler,
		ch: make(chan Event, 1024),
	}
	c.mu.Lock()
	select {
	case <-c.closed:
		c.mu.Unlock()
		return
	default:
	}
	c.ordered[eventType] = append(c.ordered[eventType], h)
	c.orderedWG.Add(1)
	c.mu.Unlock()

	go func() {
		defer c.orderedWG.Done()
		for event := range h.ch {
			c.safeInvoke(h.fn, event)
		}
	}()
}

func (c *Client) readPump() {
	defer c.conn.Close()
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Errorf("read: %v", err)
			}
			return
		}
		if c.dumpFrames {
			c.logger.Debugf("recv frame: %s", string(message))
		}
		var event Event
		if err := json.Unmarshal(message, &event); err != nil {
			c.logger.Errorf("unmarshal: %v", err)
			continue
		}
		c.dispatch(event)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case <-c.closed:
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		case message := <-c.send:
			if c.dumpFrames {
				c.logger.Debugf("send frame: %s", string(message))
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.logger.Errorf("next writer: %v", err)
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				c.logger.Errorf("close writer: %v", err)
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Errorf("ping: %v", err)
				return
			}
		}
	}
}

func (c *Client) dispatch(event Event) {
	// Snapshot handlers and reserve waitgroup slots under the mutex.
	// Holding c.mu across dispatchWG.Add serializes Add against Close,
	// which closes c.closed under the same mutex — once Close passes
	// that point, no further Adds happen and dispatchWG.Wait is safe.
	c.mu.Lock()
	select {
	case <-c.closed:
		c.mu.Unlock()
		return
	default:
	}
	handlers := append([]func(Event){}, c.handlers[event.Type]...)
	allHandlers := append([]func(Event){}, c.handlers["*"]...)
	ordered := append([]*orderedHandler{}, c.ordered[event.Type]...)
	allOrdered := append([]*orderedHandler{}, c.ordered["*"]...)
	c.dispatchWG.Add(len(handlers) + len(allHandlers))
	for _, h := range ordered {
		h.ch <- event
	}
	for _, h := range allOrdered {
		h.ch <- event
	}
	c.mu.Unlock()

	for _, h := range handlers {
		go func(fn func(Event)) {
			defer c.dispatchWG.Done()
			c.safeInvoke(fn, event)
		}(h)
	}
	for _, h := range allHandlers {
		go func(fn func(Event)) {
			defer c.dispatchWG.Done()
			c.safeInvoke(fn, event)
		}(h)
	}
}

// resolveDialer returns the websocket.Dialer Connect should use,
// honoring WithDialer first and otherwise composing a default that
// picks up Proxy/Jar from any WithHTTPClient hint.
func (c *Client) resolveDialer() *websocket.Dialer {
	if c.dialer != nil {
		return c.dialer
	}
	d := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}
	if c.httpClient != nil {
		if t, ok := c.httpClient.Transport.(*http.Transport); ok && t != nil {
			if t.Proxy != nil {
				d.Proxy = t.Proxy
			}
			if t.TLSClientConfig != nil {
				d.TLSClientConfig = t.TLSClientConfig
			}
		}
		if c.httpClient.Jar != nil {
			d.Jar = c.httpClient.Jar
		}
	}
	return d
}

// safeInvoke runs a registered handler, recovering from panics so a single
// faulty handler cannot strand the dispatch waitgroup or take down the read
// pump. The recovered value is reported via the configured Logger.
func (c *Client) safeInvoke(fn func(Event), event Event) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("handler panic on %q: %v", event.Type, r)
		}
	}()
	fn(event)
}
