package oairt_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmc/misc/oairt"
	"github.com/tmc/misc/oairt/internal/mockrt"
)

// connect wires a Client to a mockrt.Server and returns the client. Cleanup
// (Close) is registered on t.
func connect(t *testing.T, srv *mockrt.Server, opts ...oairt.Option) *oairt.Client {
	t.Helper()
	allOpts := append([]oairt.Option{oairt.WithURL(srv.URL)}, opts...)
	c := oairt.NewClient("test-key", allOpts...)
	// Ctx outlives the test. The ctx-watcher in Connect runs Close on ctx
	// cancel, so we cancel only after the test (and Cleanup'd Close) finishes.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := c.Connect(ctx, ""); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestClient_CloseIsIdempotent verifies sync.Once semantics on Close: many
// concurrent calls produce no panic and no race-detector hit.
func TestClient_CloseIsIdempotent(t *testing.T) {
	t.Parallel()
	srv := mockrt.New(t, mockrt.ScriptSessionCreated(t, "s1", "gpt-test"))
	c := connect(t, srv)

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() { defer wg.Done(); _ = c.Close() }()
	}
	wg.Wait()
	// One more, sequentially, must still be a no-op.
	if err := c.Close(); err != nil {
		t.Fatalf("Close after concurrent close: %v", err)
	}
}

// TestClient_ConcurrentSend stresses Send with many goroutines while the
// dispatcher is also delivering inbound frames.
func TestClient_ConcurrentSend(t *testing.T) {
	t.Parallel()
	srv := mockrt.New(t, mockrt.ScriptAudioDelta(t, 50))
	c := connect(t, srv)

	const goroutines = 32
	const perG = 16
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perG {
				err := c.Send(oairt.Event{Type: "input_audio_buffer.commit"})
				// Acceptable: nil OR ErrSendQueueFull (buffer is 256, but we
				// sometimes overrun under -race) OR ErrClosed if the test is
				// tearing down.
				if err != nil &&
					!errors.Is(err, oairt.ErrSendQueueFull) &&
					!errors.Is(err, oairt.ErrClosed) {
					t.Errorf("Send: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestClient_HandlerPanicIsContained ensures a panicking handler does not
// strand dispatchWG or kill the read pump (subsequent events still arrive).
func TestClient_HandlerPanicIsContained(t *testing.T) {
	t.Parallel()
	srv := mockrt.New(t, mockrt.ScriptAudioDelta(t, 3))

	// Register handlers BEFORE Connect so dispatch never sees an empty
	// handler slice when the first frame arrives.
	c := oairt.NewClient("test-key", oairt.WithURL(srv.URL))
	var saw atomic.Int32
	c.On("response.audio.delta", func(oairt.Event) {
		saw.Add(1)
		panic("boom")
	})
	var done atomic.Bool
	c.On("response.audio.done", func(oairt.Event) { done.Store(true) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := c.Connect(ctx, ""); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !done.Load() {
		time.Sleep(10 * time.Millisecond)
	}
	if !done.Load() {
		t.Fatalf("response.audio.done never delivered (saw=%d) — pump likely stranded", saw.Load())
	}
	if got := saw.Load(); got != 3 {
		t.Errorf("delta count = %d, want 3", got)
	}
}

// TestClient_SendAfterClose returns ErrClosed (or ErrNotConnected if Close
// raced ahead of any dial), never deadlocks or panics.
func TestClient_SendAfterClose(t *testing.T) {
	t.Parallel()
	srv := mockrt.New(t, mockrt.ScriptSessionCreated(t, "s2", "gpt-test"))
	c := connect(t, srv)
	// Close may surface an "already closed" error from the underlying gorilla
	// conn when the server-side script has already torn the connection down;
	// that's benign here — the relevant invariant is that Send afterwards
	// reports ErrClosed.
	_ = c.Close()

	got := c.Send(oairt.Event{Type: "input_audio_buffer.commit"})
	if got == nil {
		t.Fatalf("Send after Close: got nil error, want ErrClosed/ErrNotConnected")
	}
	if !errors.Is(got, oairt.ErrClosed) && !errors.Is(got, oairt.ErrNotConnected) {
		t.Fatalf("Send after Close: got %v, want ErrClosed or ErrNotConnected", got)
	}
}

// TestClient_ContextCancelTriggersClose verifies the ctx-watcher: cancelling
// the Connect context shuts the client down without an explicit Close.
func TestClient_ContextCancelTriggersClose(t *testing.T) {
	t.Parallel()
	srv := mockrt.New(t, mockrt.ScriptSessionCreated(t, "s3", "gpt-test"))
	c := oairt.NewClient("test-key", oairt.WithURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	if err := c.Connect(ctx, ""); err != nil {
		t.Fatalf("connect: %v", err)
	}
	cancel()

	// Send should eventually report ErrClosed; poll briefly to dodge the race
	// between cancel() and the closeOnce body running.
	deadline := time.Now().Add(2 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		last = c.Send(oairt.Event{Type: "input_audio_buffer.commit"})
		if errors.Is(last, oairt.ErrClosed) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Send did not see ErrClosed after ctx cancel; last=%v", last)
}

// TestClient_ConcurrentOnAndDispatch races handler registration against
// inbound dispatch to surface map-access races on c.handlers and
// Add-after-Wait panics on dispatchWG.
func TestClient_ConcurrentOnAndDispatch(t *testing.T) {
	t.Parallel()
	srv := mockrt.New(t, mockrt.ScriptAudioDelta(t, 20))
	c := connect(t, srv)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					c.On("response.audio.delta", func(oairt.Event) {})
					_ = i
				}
			}
		}(i)
	}
	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}
