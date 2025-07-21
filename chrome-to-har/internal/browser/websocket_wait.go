package browser

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/pkg/errors"
)

// WebSocketWaitCondition represents a condition to wait for
type WebSocketWaitCondition string

const (
	// WebSocket connection states
	WebSocketConnecting WebSocketWaitCondition = "connecting"
	WebSocketOpen       WebSocketWaitCondition = "open"
	WebSocketClosing    WebSocketWaitCondition = "closing"
	WebSocketClosed     WebSocketWaitCondition = "closed"
	
	// WebSocket frame types
	WebSocketTextFrame   WebSocketWaitCondition = "text_frame"
	WebSocketBinaryFrame WebSocketWaitCondition = "binary_frame"
	WebSocketCloseFrame  WebSocketWaitCondition = "close_frame"
	WebSocketPingFrame   WebSocketWaitCondition = "ping_frame"
	WebSocketPongFrame   WebSocketWaitCondition = "pong_frame"
	
	// WebSocket events
	WebSocketMessage     WebSocketWaitCondition = "message"
	WebSocketError       WebSocketWaitCondition = "error"
	WebSocketFirstMessage WebSocketWaitCondition = "first_message"
	WebSocketLastMessage  WebSocketWaitCondition = "last_message"
)

// WebSocketWaitOptions configures WebSocket waiting behavior
type WebSocketWaitOptions struct {
	URLPattern     string
	MessagePattern string
	MessageCount   int
	DataPattern    string
	Direction      string // "sent", "received", or "" for both
	Timeout        time.Duration
	PollInterval   time.Duration
	CaseSensitive  bool
}

// DefaultWebSocketWaitOptions returns default wait options
func DefaultWebSocketWaitOptions() *WebSocketWaitOptions {
	return &WebSocketWaitOptions{
		URLPattern:     "*",
		MessagePattern: "",
		MessageCount:   1,
		DataPattern:    "",
		Direction:      "",
		Timeout:        30 * time.Second,
		PollInterval:   100 * time.Millisecond,
		CaseSensitive:  true,
	}
}

// WebSocketWaitOption configures WebSocket wait options
type WebSocketWaitOption func(*WebSocketWaitOptions)

// WithURLPattern sets the URL pattern to match
func WithURLPattern(pattern string) WebSocketWaitOption {
	return func(opts *WebSocketWaitOptions) {
		opts.URLPattern = pattern
	}
}

// WithMessagePattern sets the message pattern to match
func WithMessagePattern(pattern string) WebSocketWaitOption {
	return func(opts *WebSocketWaitOptions) {
		opts.MessagePattern = pattern
	}
}

// WithMessageCount sets the number of messages to wait for
func WithMessageCount(count int) WebSocketWaitOption {
	return func(opts *WebSocketWaitOptions) {
		opts.MessageCount = count
	}
}

// WithDataPattern sets the data pattern to match in messages
func WithDataPattern(pattern string) WebSocketWaitOption {
	return func(opts *WebSocketWaitOptions) {
		opts.DataPattern = pattern
	}
}

// WithDirection sets the direction filter
func WithDirection(direction string) WebSocketWaitOption {
	return func(opts *WebSocketWaitOptions) {
		opts.Direction = direction
	}
}

// WithWebSocketWaitTimeout sets the WebSocket wait timeout
func WithWebSocketWaitTimeout(timeout time.Duration) WebSocketWaitOption {
	return func(opts *WebSocketWaitOptions) {
		opts.Timeout = timeout
	}
}

// WithPollInterval sets the polling interval
func WithPollInterval(interval time.Duration) WebSocketWaitOption {
	return func(opts *WebSocketWaitOptions) {
		opts.PollInterval = interval
	}
}

// WithCaseSensitive sets case sensitivity for pattern matching
func WithCaseSensitive(sensitive bool) WebSocketWaitOption {
	return func(opts *WebSocketWaitOptions) {
		opts.CaseSensitive = sensitive
	}
}

// WaitForWebSocket waits for a WebSocket connection matching the condition
func (p *Page) WaitForWebSocket(condition WebSocketWaitCondition, opts ...WebSocketWaitOption) (*WebSocketConnection, error) {
	options := DefaultWebSocketWaitOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Enable WebSocket monitoring if not already enabled
	if p.webSocketMonitor == nil {
		if err := p.EnableWebSocketMonitoring(); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout waiting for WebSocket condition")
		case <-ticker.C:
			conn, found := p.checkWebSocketCondition(condition, options)
			if found {
				return conn, nil
			}
		}
	}
}

// checkWebSocketCondition checks if a WebSocket condition is met
func (p *Page) checkWebSocketCondition(condition WebSocketWaitCondition, options *WebSocketWaitOptions) (*WebSocketConnection, bool) {
	connections := p.GetWebSocketConnections()

	for _, conn := range connections {
		if p.matchesURLPattern(conn.URL, options.URLPattern) {
			switch condition {
			case WebSocketConnecting:
				if conn.State == "connecting" {
					return conn, true
				}
			case WebSocketOpen:
				if conn.State == "open" {
					return conn, true
				}
			case WebSocketClosing:
				if conn.State == "closing" {
					return conn, true
				}
			case WebSocketClosed:
				if conn.State == "closed" {
					return conn, true
				}
			case WebSocketMessage:
				if p.hasMatchingMessage(conn, options) {
					return conn, true
				}
			case WebSocketTextFrame:
				if p.hasMatchingFrame(conn, "text", options) {
					return conn, true
				}
			case WebSocketBinaryFrame:
				if p.hasMatchingFrame(conn, "binary", options) {
					return conn, true
				}
			case WebSocketCloseFrame:
				if p.hasMatchingFrame(conn, "close", options) {
					return conn, true
				}
			case WebSocketPingFrame:
				if p.hasMatchingFrame(conn, "ping", options) {
					return conn, true
				}
			case WebSocketPongFrame:
				if p.hasMatchingFrame(conn, "pong", options) {
					return conn, true
				}
			case WebSocketFirstMessage:
				if p.hasFirstMessage(conn, options) {
					return conn, true
				}
			case WebSocketLastMessage:
				if p.hasLastMessage(conn, options) {
					return conn, true
				}
			case WebSocketError:
				// This would require additional error tracking
				return conn, false
			}
		}
	}

	return nil, false
}

// matchesURLPattern checks if a URL matches the given pattern
func (p *Page) matchesURLPattern(url, pattern string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}

	// Simple pattern matching - can be enhanced with regex
	if pattern == url {
		return true
	}

	// Check if pattern is a regex
	if regexp.MustCompile(pattern).MatchString(url) {
		return true
	}

	return false
}

// hasMatchingMessage checks if a connection has a matching message
func (p *Page) hasMatchingMessage(conn *WebSocketConnection, options *WebSocketWaitOptions) bool {
	conn.mu.RLock()
	defer conn.mu.RUnlock()

	matchCount := 0
	for _, frame := range conn.Frames {
		if p.matchesMessageCriteria(frame, options) {
			matchCount++
			if matchCount >= options.MessageCount {
				return true
			}
		}
	}

	return false
}

// hasMatchingFrame checks if a connection has a matching frame of specific type
func (p *Page) hasMatchingFrame(conn *WebSocketConnection, frameType string, options *WebSocketWaitOptions) bool {
	conn.mu.RLock()
	defer conn.mu.RUnlock()

	matchCount := 0
	for _, frame := range conn.Frames {
		if frame.Type == frameType && p.matchesMessageCriteria(frame, options) {
			matchCount++
			if matchCount >= options.MessageCount {
				return true
			}
		}
	}

	return false
}

// hasFirstMessage checks if a connection has received its first message
func (p *Page) hasFirstMessage(conn *WebSocketConnection, options *WebSocketWaitOptions) bool {
	conn.mu.RLock()
	defer conn.mu.RUnlock()

	for _, frame := range conn.Frames {
		if frame.Direction == "received" && p.matchesMessageCriteria(frame, options) {
			return true
		}
	}

	return false
}

// hasLastMessage checks if a connection has received its last message (connection is closed)
func (p *Page) hasLastMessage(conn *WebSocketConnection, options *WebSocketWaitOptions) bool {
	conn.mu.RLock()
	defer conn.mu.RUnlock()

	if conn.State != "closed" {
		return false
	}

	// Find the last received message
	for i := len(conn.Frames) - 1; i >= 0; i-- {
		frame := conn.Frames[i]
		if frame.Direction == "received" && p.matchesMessageCriteria(frame, options) {
			return true
		}
	}

	return false
}

// matchesMessageCriteria checks if a frame matches the message criteria
func (p *Page) matchesMessageCriteria(frame WebSocketFrame, options *WebSocketWaitOptions) bool {
	// Check direction
	if options.Direction != "" && frame.Direction != options.Direction {
		return false
	}

	// Check data pattern
	if options.DataPattern != "" {
		dataStr := fmt.Sprintf("%v", frame.Data)
		matched, err := p.matchesPattern(dataStr, options.DataPattern, options.CaseSensitive)
		if err != nil || !matched {
			return false
		}
	}

	// Check message pattern (alias for data pattern)
	if options.MessagePattern != "" {
		dataStr := fmt.Sprintf("%v", frame.Data)
		matched, err := p.matchesPattern(dataStr, options.MessagePattern, options.CaseSensitive)
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// matchesPattern checks if text matches a pattern
func (p *Page) matchesPattern(text, pattern string, caseSensitive bool) (bool, error) {
	if !caseSensitive {
		text = regexp.MustCompile(`(?i)`).ReplaceAllString(text, "${1}")
		pattern = regexp.MustCompile(`(?i)`).ReplaceAllString(pattern, "${1}")
	}

	// Try exact match first
	if text == pattern {
		return true, nil
	}

	// Try regex match
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}

	return re.MatchString(text), nil
}

// WaitForWebSocketMessages waits for a specific number of messages
func (p *Page) WaitForWebSocketMessages(count int, opts ...WebSocketWaitOption) ([]*WebSocketFrame, error) {
	options := DefaultWebSocketWaitOptions()
	options.MessageCount = count
	for _, opt := range opts {
		opt(options)
	}

	// Enable WebSocket monitoring if not already enabled
	if p.webSocketMonitor == nil {
		if err := p.EnableWebSocketMonitoring(); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout waiting for WebSocket messages")
		case <-ticker.C:
			frames := p.getMatchingFrames(options)
			if len(frames) >= count {
				return frames[:count], nil
			}
		}
	}
}

// getMatchingFrames gets all frames matching the criteria
func (p *Page) getMatchingFrames(options *WebSocketWaitOptions) []*WebSocketFrame {
	var frames []*WebSocketFrame

	connections := p.GetWebSocketConnections()
	for _, conn := range connections {
		if p.matchesURLPattern(conn.URL, options.URLPattern) {
			conn.mu.RLock()
			for _, frame := range conn.Frames {
				if p.matchesMessageCriteria(frame, options) {
					frameCopy := frame
					frames = append(frames, &frameCopy)
				}
			}
			conn.mu.RUnlock()
		}
	}

	return frames
}

// WaitForWebSocketData waits for specific data in WebSocket messages
func (p *Page) WaitForWebSocketData(dataPattern string, opts ...WebSocketWaitOption) (*WebSocketFrame, error) {
	options := DefaultWebSocketWaitOptions()
	options.DataPattern = dataPattern
	for _, opt := range opts {
		opt(options)
	}

	// Enable WebSocket monitoring if not already enabled
	if p.webSocketMonitor == nil {
		if err := p.EnableWebSocketMonitoring(); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("timeout waiting for WebSocket data")
		case <-ticker.C:
			frames := p.getMatchingFrames(options)
			if len(frames) > 0 {
				return frames[0], nil
			}
		}
	}
}

// WaitForWebSocketIdle waits for WebSocket connections to be idle
func (p *Page) WaitForWebSocketIdle(idleDuration time.Duration, opts ...WebSocketWaitOption) error {
	options := DefaultWebSocketWaitOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Enable WebSocket monitoring if not already enabled
	if p.webSocketMonitor == nil {
		if err := p.EnableWebSocketMonitoring(); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	ticker := time.NewTicker(options.PollInterval)
	defer ticker.Stop()

	var lastActivityTime time.Time
	var initialCheck bool

	for {
		select {
		case <-ctx.Done():
			return errors.New("timeout waiting for WebSocket idle")
		case <-ticker.C:
			currentTime := time.Now()
			recentActivity := p.hasRecentWebSocketActivity(currentTime.Add(-idleDuration), options)

			if !initialCheck {
				initialCheck = true
				lastActivityTime = currentTime
			}

			if recentActivity {
				lastActivityTime = currentTime
			} else if currentTime.Sub(lastActivityTime) >= idleDuration {
				return nil
			}
		}
	}
}

// hasRecentWebSocketActivity checks if there's been recent WebSocket activity
func (p *Page) hasRecentWebSocketActivity(since time.Time, options *WebSocketWaitOptions) bool {
	connections := p.GetWebSocketConnections()

	for _, conn := range connections {
		if p.matchesURLPattern(conn.URL, options.URLPattern) {
			conn.mu.RLock()
			for _, frame := range conn.Frames {
				if frame.Timestamp.After(since) {
					conn.mu.RUnlock()
					return true
				}
			}
			conn.mu.RUnlock()
		}
	}

	return false
}

// WebSocketEventWaiter provides advanced WebSocket event waiting
type WebSocketEventWaiter struct {
	page    *Page
	options *WebSocketWaitOptions
}

// NewWebSocketEventWaiter creates a new WebSocket event waiter
func (p *Page) NewWebSocketEventWaiter(opts ...WebSocketWaitOption) *WebSocketEventWaiter {
	options := DefaultWebSocketWaitOptions()
	for _, opt := range opts {
		opt(options)
	}

	return &WebSocketEventWaiter{
		page:    p,
		options: options,
	}
}

// WaitForEvent waits for a specific WebSocket event
func (w *WebSocketEventWaiter) WaitForEvent(condition WebSocketWaitCondition) (*WebSocketConnection, error) {
	return w.page.WaitForWebSocket(condition, func(opts *WebSocketWaitOptions) {
		*opts = *w.options
	})
}

// WaitForMultipleEvents waits for multiple WebSocket events
func (w *WebSocketEventWaiter) WaitForMultipleEvents(conditions []WebSocketWaitCondition) ([]*WebSocketConnection, error) {
	var results []*WebSocketConnection
	
	for _, condition := range conditions {
		conn, err := w.WaitForEvent(condition)
		if err != nil {
			return nil, err
		}
		results = append(results, conn)
	}
	
	return results, nil
}

// WaitForSequence waits for a sequence of WebSocket events in order
func (w *WebSocketEventWaiter) WaitForSequence(conditions []WebSocketWaitCondition) ([]*WebSocketConnection, error) {
	var results []*WebSocketConnection
	
	for _, condition := range conditions {
		conn, err := w.WaitForEvent(condition)
		if err != nil {
			return nil, fmt.Errorf("failed waiting for condition %s: %w", condition, err)
		}
		results = append(results, conn)
	}
	
	return results, nil
}

// WaitWithCallback waits for WebSocket events with a callback
func (w *WebSocketEventWaiter) WaitWithCallback(
	condition WebSocketWaitCondition,
	callback func(*WebSocketConnection, *WebSocketFrame),
) error {
	// Enable WebSocket monitoring if not already enabled
	if w.page.webSocketMonitor == nil {
		if err := w.page.EnableWebSocketMonitoring(); err != nil {
			return err
		}
	}

	// Set up frame handlers
	w.page.SetWebSocketFrameHandler(
		func(conn *WebSocketConnection, frame *WebSocketFrame) {
			if callback != nil {
				callback(conn, frame)
			}
		},
		func(conn *WebSocketConnection, frame *WebSocketFrame) {
			if callback != nil {
				callback(conn, frame)
			}
		},
	)

	// Wait for the condition
	_, err := w.WaitForEvent(condition)
	return err
}