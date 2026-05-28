package oairt

import "errors"

// Sentinel errors returned by Client. Callers may match these with
// errors.Is to distinguish recoverable conditions from transport errors.
var (
	// ErrNotConnected is returned by Send and SendAudio when the client
	// has not yet successfully completed Connect, or after Close.
	ErrNotConnected = errors.New("oairt: not connected")

	// ErrSendQueueFull is returned by Send and SendAudio when the outbound
	// queue is saturated. Callers may retry after a short delay.
	ErrSendQueueFull = errors.New("oairt: send queue full")

	// ErrHandshake wraps a websocket handshake failure from Connect. The
	// underlying error chain carries the gorilla/websocket diagnostic and,
	// when available, the HTTP response status and body.
	ErrHandshake = errors.New("oairt: websocket handshake failed")

	// ErrClosed is returned by Send and SendAudio after Close has run.
	ErrClosed = errors.New("oairt: client closed")
)
