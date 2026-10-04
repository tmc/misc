package lifetime

import (
	"context"
	"errors"
	"example.com/lifetime/api/bindadapter"
	"io"
	"sync"
)

//wasmbind:resource Events
type Events struct {
	events  *bindadapter.Events[int]
	once    sync.Once
	started chan struct{}
}

//wasmbind:export events
//wasmbind:owned
//wasmbind:nonnull
func (n *Node) Events() *Events {
	source := make(chan int)
	events, _ := bindadapter.FromChannel(source, nil)
	return &Events{events: events, started: make(chan struct{})}
}

//wasmbind:next
func (e *Events) Next(ctx context.Context) (int, bool, error) {
	e.once.Do(func() { close(e.started) })
	return e.events.Next(ctx)
}

//wasmbind:interrupt
func (e *Events) Interrupt() error { return e.events.Interrupt() }

//wasmbind:close
func (e *Events) Close() error { sequence += "e"; return e.events.Close() }

//wasmbind:export started
func (e *Events) Started() bool {
	select {
	case <-e.started:
		return true
	default:
		return false
	}
}

//wasmbind:resource Stream
type Stream struct {
	reader *bindadapter.Reader
	writer *bindadapter.Writer
}

//wasmbind:export stream
//wasmbind:owned
//wasmbind:nonnull
func (n *Node) Stream() *Stream {
	r, w := io.Pipe()
	reader, _ := bindadapter.ReadCloser(r)
	writer, _ := bindadapter.WriteCloser(w)
	return &Stream{reader, writer}
}

//wasmbind:read
func (s *Stream) Read(ctx context.Context, p []byte) (int, error) { return s.reader.Read(ctx, p) }

//wasmbind:write
func (s *Stream) Write(ctx context.Context, p []byte) (int, error) { return s.writer.Write(ctx, p) }

//wasmbind:finish
func (s *Stream) Finish(ctx context.Context) error { return s.writer.Finish(ctx) }

//wasmbind:cancelread
func (s *Stream) CancelRead() error { return s.reader.Interrupt() }

//wasmbind:cancelwrite
func (s *Stream) CancelWrite() error { return s.writer.Interrupt() }

//wasmbind:close
func (s *Stream) Close() error {
	sequence += "s"
	return errors.Join(s.reader.Close(), s.writer.Close())
}
