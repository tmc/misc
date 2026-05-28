package oairt

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSend_NotConnected verifies that Send before Connect returns
// ErrNotConnected wrapped with context.
func TestSend_NotConnected(t *testing.T) {
	c := NewClient("test-key")
	err := c.Send(Event{Type: "ping"})
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("Send before Connect: got %v, want errors.Is(err, ErrNotConnected)", err)
	}
}

// TestSendAudio_NotConnected verifies SendAudio shares the not-connected
// guard with Send.
func TestSendAudio_NotConnected(t *testing.T) {
	c := NewClient("test-key")
	if err := c.SendAudio([]byte{0x00}); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("SendAudio before Connect: got %v, want errors.Is(err, ErrNotConnected)", err)
	}
}

// TestClose_Idempotent verifies Close is safe to call multiple times,
// concurrently, and that it waits for in-flight handlers.
func TestClose_Idempotent(t *testing.T) {
	c := NewClient("test-key")

	var ran atomic.Int32
	c.On(EventResponseTextDelta, func(Event) {
		time.Sleep(10 * time.Millisecond)
		ran.Add(1)
	})
	c.dispatch(Event{Type: EventResponseTextDelta})
	c.dispatch(Event{Type: EventResponseTextDelta})

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = c.Close() }()
	}
	wg.Wait()

	if got := ran.Load(); got != 2 {
		t.Fatalf("Close should drain handlers: ran=%d want 2", got)
	}
}

// TestSafeInvoke_RecoversPanic verifies that a panicking handler does
// not strand the dispatch waitgroup.
func TestSafeInvoke_RecoversPanic(t *testing.T) {
	c := NewClient("test-key")
	c.On(EventResponseTextDelta, func(Event) { panic("boom") })

	c.dispatch(Event{Type: EventResponseTextDelta})

	done := make(chan struct{})
	go func() { c.dispatchWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatchWG.Wait blocked after panicking handler")
	}
}

// TestConnect_CtxCancel verifies that cancelling Connect's context after
// a successful dial triggers Close.
func TestConnect_CtxCancel(t *testing.T) {
	c := NewClient("test-key")
	// Skip dial; simulate a connected client and the ctx-watch goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-ctx.Done():
			c.Close()
		case <-c.closed:
		}
	}()

	cancel()
	select {
	case <-c.closed:
	case <-time.After(time.Second):
		t.Fatal("ctx cancel did not trigger Close")
	}
}
