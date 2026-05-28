package mockrt

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestServerScript exercises the mock end-to-end with a real WebSocket
// client. We use gorilla/websocket here directly to keep the test independent
// of the production oairt.Client.
func TestServerScript(t *testing.T) {
	srv := New(t, ScriptSessionCreated(t, "sess_x", "gpt-test"))

	dialer := websocket.DefaultDialer
	c, _, err := dialer.DialContext(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	_, msg, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Contains(msg, []byte(`"session.created"`)) {
		t.Fatalf("got frame %q, want session.created", msg)
	}

	if err := srv.WaitDone(2 * time.Second); err != nil {
		t.Fatalf("WaitDone: %v", err)
	}
}

func TestServerExpect(t *testing.T) {
	script := Script{
		{Expect: func(b []byte) error {
			if !bytes.Contains(b, []byte("hello")) {
				t.Errorf("expected hello, got %q", b)
			}
			return nil
		}},
		SendJSON(t, map[string]any{"type": "session.created"}),
	}
	srv := New(t, script)

	c, _, err := websocket.DefaultDialer.DialContext(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	if err := c.WriteMessage(websocket.TextMessage, []byte(`{"type":"hello"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := c.ReadMessage(); err != nil {
		t.Fatalf("read: %v", err)
	}

	if err := srv.WaitDone(2 * time.Second); err != nil {
		t.Fatalf("WaitDone: %v", err)
	}
	if got := len(srv.Received()); got != 1 {
		t.Fatalf("Received len = %d, want 1", got)
	}
}

func TestServerAuthRejection(t *testing.T) {
	srv := NewWithOptions(t, nil, Options{ExpectAuth: "secret-token"})
	_, resp, err := websocket.DefaultDialer.DialContext(context.Background(), srv.URL, nil)
	if err == nil {
		t.Fatalf("expected handshake failure")
	}
	if resp == nil || resp.StatusCode != 401 {
		t.Fatalf("got resp %+v, want 401", resp)
	}
}
