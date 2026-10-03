// Package api exercises the opt-in browser extensions.
package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

//wasmbind:imports Host
type Host interface {
	Lookup(context.Context, string) ([]byte, error)
}

var host Host

//wasmbind:init
func Initialize(ctx context.Context, h Host) error {
	host = h
	_, err := h.Lookup(ctx, "init")
	return err
}

//wasmbind:export lookup
func Lookup(ctx context.Context, key string) ([]byte, error) { return host.Lookup(ctx, key) }

//wasmbind:error NotFound
type Missing struct {
	Key     string
	Private string `wasmbind:"-"`
}

func (e *Missing) Error() string { return "missing " + e.Key }

type cycle struct{}

func (*cycle) Error() string   { return "cycle" }
func (e *cycle) Unwrap() error { return e }

//wasmbind:export fail
func Fail(kind string) error {
	switch kind {
	case "wrapped":
		return fmt.Errorf("lookup: %w", &Missing{Key: "key", Private: "secret"})
	case "joined":
		return errors.Join(errors.New("first"), &Missing{Key: "second"})
	case "invalid":
		return &Missing{Key: string([]byte{255})}
	case "cycle":
		return &cycle{}
	default:
		return errors.New("plain")
	}
}

//wasmbind:resource Stream
type Stream struct {
	mode    string
	reads   int
	stopped chan struct{}
	once    sync.Once
	written []byte
}

//wasmbind:export stream
func NewStream(mode string) *Stream { return &Stream{mode: mode, stopped: make(chan struct{})} }

//wasmbind:read
func (s *Stream) Read(ctx context.Context, p []byte) (int, error) {
	s.reads++
	switch s.mode {
	case "blocked":
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-s.stopped:
			return 0, io.EOF
		}
	case "idle":
		return 0, nil
	case "invalid":
		return len(p) + 1, nil
	case "typed":
		return copy(p, "typed"), &Missing{Key: "read"}
	case "error":
		return copy(p, "last"), errors.New("read failed")
	default:
		return copy(p, "final"), io.EOF
	}
}

//wasmbind:cancelread
func (s *Stream) CancelRead() error { s.once.Do(func() { close(s.stopped) }); return nil }

//wasmbind:write
func (s *Stream) Write(ctx context.Context, p []byte) (int, error) {
	if s.mode == "blocked" {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	if s.mode == "zero" {
		return 0, nil
	}
	n := len(p)
	if n > 2 {
		n = 2
	}
	s.written = append(s.written, p[:n]...)
	return n, nil
}

//wasmbind:cancelwrite
func (s *Stream) CancelWrite() error { return nil }

//wasmbind:finish
func (s *Stream) Finish(context.Context) error {
	if s.mode == "finish-error" {
		return errors.New("finish failed")
	}
	return nil
}

//wasmbind:close
func (s *Stream) Close() error { return nil }

//wasmbind:export streamStats
func StreamStats(s *Stream) []int { return []int{s.reads, len(s.written)} }

//wasmbind:resource Subscription
type Subscription struct {
	mode  string
	count int
}

//wasmbind:export subscribe
func Subscribe(mode string) *Subscription { return &Subscription{mode: mode} }

//wasmbind:next
func (s *Subscription) Next(ctx context.Context) (string, bool, error) {
	if s.mode == "blocked" {
		<-ctx.Done()
		return "", false, ctx.Err()
	}
	if s.mode == "error" {
		return "", false, errors.New("event failed")
	}
	s.count++
	return "event", s.count < 3, nil
}

//wasmbind:interrupt
func (s *Subscription) Interrupt() error { return nil }

//wasmbind:close
func (s *Subscription) Close() error { return nil }
