// Package mockrt is a test-only fake of the OpenAI Realtime WebSocket
// endpoint.
//
// A Server replays a scripted sequence of frames against a single connecting
// client, asserts inbound frames if the script asks for it, and records every
// frame it receives so tests can inspect them after the fact.
//
// The package is internal: it is intended for oairt's own tests only.
//
// Implementation note: the WebSocket transport itself is isolated to
// transport.go behind the conn interface. The rest of the package uses only
// net/http and the standard library, so swapping WebSocket libraries (planned
// for Workstream A5) requires editing one file.
package mockrt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Step is one entry in a server Script. Exactly one of Send, Expect,
// CloseCode, or Sleep should be set; zero-value Steps are a no-op.
type Step struct {
	// Send is a JSON frame the server writes to the client.
	Send json.RawMessage

	// Expect, if non-nil, is invoked with the next frame received from the
	// client. Returning a non-nil error fails the test.
	Expect func([]byte) error

	// CloseCode, if non-zero, closes the connection with the given WebSocket
	// close code (e.g. 1000 normal, 1006 abnormal).
	CloseCode int

	// Sleep pauses script execution for the given duration. Useful for
	// reproducing slow servers.
	Sleep time.Duration
}

// Script is an ordered sequence of Steps the Server executes against the
// connected client.
type Script []Step

// Then concatenates two scripts.
func (s Script) Then(next Script) Script {
	out := make(Script, 0, len(s)+len(next))
	out = append(out, s...)
	out = append(out, next...)
	return out
}

// Options configures Server behaviour beyond the script.
type Options struct {
	// ExpectAuth, if non-empty, makes the upgrade handler reject any request
	// whose Authorization header is not "Bearer <ExpectAuth>" with a 401.
	ExpectAuth string

	// RequireBetaHeader, if true, rejects requests missing the
	// "OpenAI-Beta: realtime=v1" header.
	RequireBetaHeader bool
}

// Server is a fake Realtime endpoint backed by httptest.Server.
type Server struct {
	URL     string // ws://host:port/v1/realtime
	HTTPURL string // http://host:port/v1/realtime (the underlying httptest URL)

	t      testing.TB
	opts   Options
	script Script

	srv *httptest.Server

	mu          sync.Mutex
	received    [][]byte
	connOnce    sync.Once
	connStarted chan struct{} // closed when a connection has been accepted
	done        chan struct{}
	scriptErr   error
	runCtx      context.Context
	runCancel   context.CancelFunc
}

// New starts a Server listening on a loopback port. Cleanup is registered on
// t; the caller does not need to close the server explicitly.
func New(t testing.TB, script Script) *Server {
	t.Helper()
	return NewWithOptions(t, script, Options{})
}

// NewWithOptions is like New but accepts non-default Options.
func NewWithOptions(t testing.TB, script Script, opts Options) *Server {
	t.Helper()
	runCtx, runCancel := context.WithCancel(context.Background())
	s := &Server{
		t:           t,
		opts:        opts,
		script:      script,
		connStarted: make(chan struct{}),
		done:        make(chan struct{}),
		runCtx:      runCtx,
		runCancel:   runCancel,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/realtime", s.handle)
	s.srv = httptest.NewServer(mux)
	s.HTTPURL = s.srv.URL + "/v1/realtime"
	s.URL = "ws" + strings.TrimPrefix(s.HTTPURL, "http")
	t.Cleanup(func() {
		// Cancel the run context first so any Sleep/Expect step bails out
		// promptly, then close the listener and any in-flight conns to
		// unblock the read goroutine inside run().
		s.runCancel()
		s.srv.Close()
		// If a connection ever arrived, wait for run() to finish. This
		// guarantees the read goroutine spawned in run() exits before the
		// test goroutine reaches goleak.VerifyNone.
		select {
		case <-s.connStarted:
			select {
			case <-s.done:
			case <-time.After(2 * time.Second):
				// Don't fail the test — we've done what we can; goleak will
				// surface any actual leak.
			}
		default:
			// No connection ever arrived; nothing to wait on.
		}
	})
	return s
}

// Received returns a snapshot of every frame the server has read from the
// client so far.
func (s *Server) Received() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, len(s.received))
	for i, f := range s.received {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

// WaitDone blocks until the script finishes or the timeout elapses. Returns
// any error captured during script execution.
func (s *Server) WaitDone(timeout time.Duration) error {
	select {
	case <-s.done:
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.scriptErr
	case <-time.After(timeout):
		return fmt.Errorf("mockrt: script did not complete within %s", timeout)
	}
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	first := false
	s.connOnce.Do(func() {
		first = true
		close(s.connStarted)
	})
	if !first {
		s.t.Errorf("mockrt: unexpected second connection")
		http.Error(w, "single connection only", http.StatusConflict)
		return
	}
	if s.opts.ExpectAuth != "" {
		got := r.Header.Get("Authorization")
		want := "Bearer " + s.opts.ExpectAuth
		if got != want {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			close(s.done)
			return
		}
	}
	if s.opts.RequireBetaHeader && r.Header.Get("OpenAI-Beta") != "realtime=v1" {
		http.Error(w, "missing beta header", http.StatusBadRequest)
		close(s.done)
		return
	}

	c, err := upgrade(w, r)
	if err != nil {
		s.fail(fmt.Errorf("upgrade: %w", err))
		return
	}
	defer c.Close()
	s.run(c)
}

func (s *Server) run(c conn) {
	defer close(s.done)
	ctx, cancel := context.WithCancel(s.runCtx)
	defer cancel()

	// Spin a goroutine to drain client frames; Expect steps consume from this
	// channel.
	frames := make(chan []byte, 16)
	readErr := make(chan error, 1)
	go func() {
		for {
			data, err := c.ReadText(ctx)
			if err != nil {
				readErr <- err
				close(frames)
				return
			}
			s.mu.Lock()
			s.received = append(s.received, append([]byte(nil), data...))
			s.mu.Unlock()
			select {
			case frames <- data:
			case <-ctx.Done():
				return
			}
		}
	}()

	for i, step := range s.script {
		switch {
		case len(step.Send) > 0:
			if err := c.WriteText(ctx, step.Send); err != nil {
				s.fail(fmt.Errorf("step %d send: %w", i, err))
				return
			}
		case step.Expect != nil:
			select {
			case data, ok := <-frames:
				if !ok {
					s.fail(fmt.Errorf("step %d expect: connection closed", i))
					return
				}
				if err := step.Expect(data); err != nil {
					s.fail(fmt.Errorf("step %d expect: %w", i, err))
					return
				}
			case <-time.After(5 * time.Second):
				s.fail(fmt.Errorf("step %d expect: timed out waiting for frame", i))
				return
			}
		case step.CloseCode != 0:
			_ = c.Close()
			return
		case step.Sleep > 0:
			select {
			case <-time.After(step.Sleep):
			case <-ctx.Done():
				goto teardown
			}
		}
	}

teardown:
	// Script complete; let the client see a clean close. Closing the conn
	// unblocks any pending ReadText; cancel the read context as a belt and
	// braces so the read goroutine exits even if the underlying transport
	// silently swallows the close.
	_ = c.Close()
	cancel()
	// Drain the read goroutine so it does not outlive the test. The
	// underlying gorilla ReadMessage call returns once the conn closes; if
	// for any reason it doesn't, give up gracefully — the t.Cleanup-side
	// httptest.Server.Close will eventually force termination.
	select {
	case <-readErr:
	case <-time.After(2 * time.Second):
	}
}

func (s *Server) fail(err error) {
	s.mu.Lock()
	if s.scriptErr == nil {
		s.scriptErr = err
	}
	s.mu.Unlock()
	s.t.Errorf("mockrt: %v", err)
}

// SendJSON marshals v and returns it as a Send step.
func SendJSON(t testing.TB, v any) Step {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mockrt: marshal: %v", err)
	}
	return Step{Send: b}
}
