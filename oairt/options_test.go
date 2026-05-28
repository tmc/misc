package oairt

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

func TestWithLogger_NilFallsBackToNop(t *testing.T) {
	c := NewClient("k", WithLogger(nil))
	if _, ok := c.logger.(nopLogger); !ok {
		t.Fatalf("WithLogger(nil) should yield nopLogger, got %T", c.logger)
	}
	c.logger.Debugf("safe to call")
}

type capLogger struct {
	mu   sync.Mutex
	msgs []string
}

func (l *capLogger) Debugf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, "D:"+format)
}
func (l *capLogger) Errorf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, "E:"+format)
}

func TestWithLogger_RoutesPanicReports(t *testing.T) {
	l := &capLogger{}
	c := NewClient("k", WithLogger(l))
	c.On("evt", func(Event) { panic("x") })
	c.dispatch(Event{Type: "evt"})
	c.dispatchWG.Wait()

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.msgs) == 0 {
		t.Fatal("panic was not routed to Logger")
	}
}

func TestWithURL(t *testing.T) {
	c := NewClient("k", WithURL("wss://example.test/rt"))
	if c.URL != "wss://example.test/rt" {
		t.Fatalf("WithURL not applied: %q", c.URL)
	}
	// Empty string is a no-op.
	c2 := NewClient("k", WithURL(""))
	if c2.URL == "" {
		t.Fatal("WithURL(\"\") should not blank the default URL")
	}
}

func TestWithUserAgent(t *testing.T) {
	c := NewClient("k", WithUserAgent("my-app/2.0"))
	if c.userAgent != "my-app/2.0" {
		t.Fatalf("WithUserAgent not applied: %q", c.userAgent)
	}
}

func TestWithDialer_TakesPrecedence(t *testing.T) {
	custom := &websocket.Dialer{HandshakeTimeout: 1}
	hc := &http.Client{Transport: &http.Transport{}}
	c := NewClient("k", WithHTTPClient(hc), WithDialer(custom))
	if got := c.resolveDialer(); got != custom {
		t.Fatalf("WithDialer should win over WithHTTPClient")
	}
}

func TestWithHTTPClient_PropagatesProxyAndTLS(t *testing.T) {
	tlsCfg := &tls.Config{InsecureSkipVerify: true}
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsCfg,
		},
		Jar: jar,
	}
	c := NewClient("k", WithHTTPClient(hc))
	d := c.resolveDialer()
	if d.TLSClientConfig != tlsCfg {
		t.Fatal("TLSClientConfig not propagated from http.Client")
	}
	if d.Jar != jar {
		t.Fatal("Jar not propagated from http.Client")
	}
	if d.Proxy == nil {
		t.Fatal("Proxy not propagated from http.Client.Transport")
	}
}

func TestResolveDialer_Default(t *testing.T) {
	c := NewClient("k")
	d := c.resolveDialer()
	if d.HandshakeTimeout == 0 {
		t.Fatal("default dialer should set HandshakeTimeout")
	}
	if d.Proxy == nil {
		t.Fatal("default dialer should set Proxy=http.ProxyFromEnvironment")
	}
}
