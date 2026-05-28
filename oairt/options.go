package oairt

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// WithLogger installs a Logger for diagnostic output. The default is a
// no-op logger, so the library is silent unless the caller opts in.
//
// A small adapter is enough to bridge log/slog, zap, zerolog, or the
// stdlib log package; see the package examples.
func WithLogger(l Logger) Option {
	return func(c *Client) {
		if l == nil {
			l = nopLogger{}
		}
		c.logger = l
	}
}

// WithURL overrides the default Realtime endpoint
// (wss://api.openai.com/v1/realtime). The model query parameter is still
// added by Connect.
func WithURL(rawURL string) Option {
	return func(c *Client) {
		if rawURL != "" {
			c.URL = rawURL
		}
	}
}

// WithUserAgent overrides the User-Agent header sent during the
// websocket handshake.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// WithHTTPClient supplies an *http.Client whose Transport.Proxy and Jar
// are used during the websocket handshake. The Timeout field is not
// honored because the websocket connection outlives a single HTTP round
// trip; use WithDialer for fine-grained control of HandshakeTimeout.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithDialer installs a fully-configured *websocket.Dialer, taking
// precedence over WithHTTPClient. Use this for custom TLS, NetDial, or
// HandshakeTimeout settings.
func WithDialer(d *websocket.Dialer) Option {
	return func(c *Client) { c.dialer = d }
}
