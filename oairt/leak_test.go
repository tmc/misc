package oairt

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/tmc/misc/oairt/internal/mockrt"
)

// TestMain runs every test in this package under goleak's TestMain wrapper.
// Any goroutine still running at process exit fails the run; in practice this
// is what catches stranded readPump/writePump/dispatch goroutines after Close.
//
// httptest.Server's accept loop and the runtime poller are exempted: the
// httptest server is closed by t.Cleanup, but the netFD.accept call may still
// be parked in the runtime poller for one scheduler tick after Close returns.
// This is not an oairt-side leak.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// httptest server accept loop and runtime poller may not have
		// drained by the time httptest.Server.Close returns.
		goleak.IgnoreTopFunction("net/http/httptest.(*Server).goServe.func1"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		// TestLeak_SendQueueFull intentionally parks mockrt's handler in a
		// time.Sleep so the writer pump never drains. httptest.Server.Close
		// does not interrupt time.Sleep, so these top-frames may persist past
		// the per-test goleak.VerifyNone. They are not oairt-side leaks.
		goleak.IgnoreAnyFunction("net/http.(*conn).serve"),
		goleak.IgnoreAnyFunction("github.com/tmc/misc/oairt/internal/mockrt.(*Server).run"),
		goleak.IgnoreAnyFunction("github.com/tmc/misc/oairt/internal/mockrt.(*Server).run.func1"),
	)
}

// TestLeak_ConnectClose covers the happy path: dial, exchange one frame, then
// Close. After Close returns, readPump, writePump, the ctx-watcher, and any
// dispatched handler goroutines must all be gone.
func TestLeak_ConnectClose(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("net/http/httptest.(*Server).goServe.func1"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	)

	srv := mockrt.New(t, mockrt.ScriptSessionCreated(t, "sess_x", "gpt-test"))

	c := NewClient("test-key", WithURL(srv.URL))

	got := make(chan Event, 1)
	c.On(EventSessionCreated, func(e Event) { got <- e })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Connect(ctx, ""); err != nil {
		t.Fatalf("connect: %v", err)
	}

	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatalf("did not receive session.created within 2s")
	}

	if err := c.Close(); err != nil && !isBenignCloseErr(err) {
		t.Fatalf("close: %v", err)
	}
	// Idempotent: second Close must not deadlock or panic.
	if err := c.Close(); err != nil {
		t.Fatalf("close (second call): %v", err)
	}
}

// TestLeak_ConnectCtxCancel covers ctx-driven teardown: cancelling the
// Connect ctx must trigger Close via the ctx-watcher goroutine spawned in
// Connect. After cancel + Close, no oairt goroutines remain.
func TestLeak_ConnectCtxCancel(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("net/http/httptest.(*Server).goServe.func1"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	)

	srv := mockrt.New(t, mockrt.ScriptSessionCreated(t, "sess_x", "gpt-test"))

	c := NewClient("test-key", WithURL(srv.URL))

	ctx, cancel := context.WithCancel(context.Background())
	if err := c.Connect(ctx, ""); err != nil {
		t.Fatalf("connect: %v", err)
	}

	cancel()

	// Close is idempotent: the ctx-watcher will call it on cancel; we call
	// it again here to wait deterministically on the dispatchWG.
	if err := c.Close(); err != nil && !isBenignCloseErr(err) {
		t.Fatalf("close: %v", err)
	}
}

// TestLeak_ConnBroken covers the server-initiated close path: the mock sends
// one event then hangs up. readPump must observe the close, exit, and let a
// caller-issued Close drain cleanly with no goroutine leak.
func TestLeak_ConnBroken(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("net/http/httptest.(*Server).goServe.func1"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	)

	script := mockrt.Script{
		mockrt.SendJSON(t, map[string]any{
			"type":    EventSessionCreated,
			"session": map[string]any{"id": "sess_x"},
		}),
		{CloseCode: 1006}, // abnormal close
	}
	srv := mockrt.New(t, script)

	c := NewClient("test-key", WithURL(srv.URL))

	got := make(chan struct{}, 1)
	c.On(EventSessionCreated, func(Event) { got <- struct{}{} })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Connect(ctx, ""); err != nil {
		t.Fatalf("connect: %v", err)
	}
	<-got

	// Give readPump time to observe the server close.
	time.Sleep(50 * time.Millisecond)

	if err := c.Close(); err != nil && !isBenignCloseErr(err) {
		t.Fatalf("close: %v", err)
	}
}

// TestLeak_SendQueueFull covers the saturation path: fill the send buffer,
// observe ErrSendQueueFull, then Close. Even with the writer pump unable to
// drain (because the mock script never reads), all goroutines tear down.
func TestLeak_SendQueueFull(t *testing.T) {
	// This test deliberately leaves the mockrt server with parked goroutines
	// (the script Sleep keeps Server.run alive; its read pump blocks on
	// c.ReadText). Those are mockrt-side, not oairt-side, so we exempt them
	// from the leak check. The thing under test is that *Client*'s readPump,
	// writePump, ctx-watcher, and dispatch goroutines all exit on Close.
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("net/http/httptest.(*Server).goServe.func1"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreAnyFunction("net/http.(*conn).serve"),
		goleak.IgnoreAnyFunction("github.com/tmc/misc/oairt/internal/mockrt.(*Server).run"),
		goleak.IgnoreAnyFunction("github.com/tmc/misc/oairt/internal/mockrt.(*Server).run.func1"),
	)

	// Server sleeps for the duration of the test, never reading, so the
	// Client's send buffer (256) saturates.
	srv := mockrt.New(t, mockrt.Script{{Sleep: 5 * time.Second}})

	c := NewClient("test-key", WithURL(srv.URL))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := c.Connect(ctx, ""); err != nil {
		t.Fatalf("connect: %v", err)
	}

	var sawFull bool
	for i := 0; i < 4096; i++ {
		err := c.Send(Event{Type: EventInputAudioBufferAppend, Audio: "AAAA"})
		if err == nil {
			continue
		}
		if errors.Is(err, ErrSendQueueFull) {
			sawFull = true
			break
		}
		t.Fatalf("unexpected Send error: %v", err)
	}
	if !sawFull {
		t.Fatalf("never observed ErrSendQueueFull after 4096 sends")
	}

	if err := c.Close(); err != nil && !isBenignCloseErr(err) {
		t.Fatalf("close: %v", err)
	}
}

// isBenignCloseErr reports whether err is the "already closed" error from
// net.Conn.Close after the peer (or readPump's defer) has torn the socket
// down. The goroutine has exited; only the bookkeeping is noisy.
func isBenignCloseErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "use of closed network connection")
}
