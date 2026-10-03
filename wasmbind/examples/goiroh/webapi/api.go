// Package webapi binds the relay-only endpoint, connection, and framed stream API.
// Dispose subscriptions and streams before connections and endpoints.
// Stream views have independent read and write cancellation.
package webapi

import (
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/tmc/go-iroh/blobs"
	"github.com/tmc/go-iroh/gossip"
	"github.com/tmc/go-iroh/iroh"
	"github.com/tmc/go-iroh/key"
	"github.com/tmc/go-iroh/netaddr"
	"github.com/tmc/go-iroh/relay"
)

// Endpoint owns a relay-only transport.
//
//wasmbind:resource Endpoint
type Endpoint struct {
	endpoint *iroh.Endpoint
	gossip   *gossip.Gossip
	router   *iroh.Router
	relay    netaddr.RelayURL
	cancel   context.CancelFunc
}

// Bind opens an endpoint on the given relay and waits for it to be online.
//
//wasmbind:export bind
func Bind(ctx context.Context, relayAddress string) (*Endpoint, error) {
	url, err := netaddr.ParseRelayURL(relayAddress)
	if err != nil {
		return nil, fmt.Errorf("parse relay: %w", err)
	}
	lifetime, cancel := context.WithCancel(context.Background())
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	ep, err := iroh.Bind(lifetime, iroh.WithRelayMode(relay.ModeCustom(relay.MapFromURLs(url))), iroh.WithoutIPTransports())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("bind: %w", err)
	}
	if err := ep.Online(ctx); err != nil {
		cancel()
		ep.Shutdown(context.Background())
		return nil, fmt.Errorf("online: %w", err)
	}
	g := gossip.NewGossip(ep)
	router, err := iroh.NewRouter(ep, map[string]iroh.ProtocolHandler{gossip.ALPN: g}, nil)
	if err != nil {
		cancel()
		ep.Shutdown(context.Background())
		return nil, fmt.Errorf("router: %w", err)
	}
	return &Endpoint{endpoint: ep, relay: url, cancel: cancel, gossip: g, router: router}, nil
}

// Connect opens a connection to a peer through the relay.
//
//wasmbind:export connect
func (e *Endpoint) Connect(ctx context.Context, peer, alpn string) (*Connection, error) {
	id, err := key.ParseEndpointID(peer)
	if err != nil {
		return nil, &WrapperError{Operation: "connect", Message: err.Error()}
	}
	c, err := e.endpoint.Connect(ctx, netaddr.NewEndpointAddr(id).WithRelayURL(e.relay), alpn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &Connection{c}, nil
}

// FetchBlob fetches and verifies a blob identified by its BLAKE3 hash.
//
//wasmbind:export fetchBlob
func (e *Endpoint) FetchBlob(ctx context.Context, peer, hash string) ([]byte, error) {
	h, err := blobs.ParseHash(hash)
	if err != nil {
		return nil, fmt.Errorf("parse hash: %w", err)
	}
	c, err := e.Connect(ctx, peer, blobs.ALPN)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("open blob stream: %w", err)
	}
	defer stream.Close()
	stop := context.AfterFunc(ctx, func() { stream.CancelRead(1); stream.CancelWrite(1) })
	defer stop()
	return blobs.GetBlobBytes(ctx, stream, h)
}

// Close shuts down the transport.
//
//wasmbind:close
func (e *Endpoint) Close() error { e.cancel(); return e.router.Shutdown(context.Background()) }

// Connection owns one peer connection.
//
//wasmbind:resource Connection
type Connection struct{ conn *iroh.Conn }

// Open opens a bidirectional stream.
//
//wasmbind:export open
func (c *Connection) Open(ctx context.Context) (*Stream, error) {
	s, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}
	return &Stream{s}, nil
}

// Close shuts down the connection.
//
//wasmbind:close
func (c *Connection) Close() error { return c.conn.Close() }

// Stream owns independent readable and writable views.
//
//wasmbind:resource Stream
type Stream struct{ stream *iroh.Stream }

// Read receives bytes; cancellation interrupts only reading.
//
//wasmbind:read
func (s *Stream) Read(ctx context.Context, p []byte) (int, error) {
	stop := context.AfterFunc(ctx, func() { s.stream.CancelRead(1) })
	defer stop()
	return s.stream.Read(p)
}

// Write sends bytes; cancellation interrupts only writing.
//
//wasmbind:write
func (s *Stream) Write(ctx context.Context, p []byte) (int, error) {
	stop := context.AfterFunc(ctx, func() { s.stream.CancelWrite(1) })
	defer stop()
	return s.stream.Write(p)
}

// Finish gracefully closes writing.
//
//wasmbind:finish
func (s *Stream) Finish(context.Context) error { return s.stream.CloseWrite() }

// CancelRead interrupts an active read.
//
//wasmbind:cancelread
func (s *Stream) CancelRead() error { s.stream.CancelRead(1); return nil }

// CancelWrite interrupts an active write.
//
//wasmbind:cancelwrite
func (s *Stream) CancelWrite() error { s.stream.CancelWrite(1); return nil }

// Close aborts both halves of the stream.
//
//wasmbind:close
func (s *Stream) Close() error { s.stream.CancelRead(0); s.stream.CancelWrite(0); return nil }

// WrapperError reports public wrapper validation failures.
//
//wasmbind:error WrapperError
type WrapperError struct {
	Operation string
	Message   string
}

func (e *WrapperError) Error() string { return e.Operation + ": " + e.Message }

// Host supplies browser HTTP functionality.
//
//wasmbind:imports Host
type Host interface {
	Fetch(context.Context, string) ([]byte, error)
}

var host Host

// Initialize captures the generated host implementation.
//
//wasmbind:init
func Initialize(ctx context.Context, h Host) error { host = h; return ctx.Err() }

// FetchFixture invokes the supplied browser fetch callback.
//
//wasmbind:export fetchFixture
func FetchFixture(ctx context.Context, url string) ([]byte, error) { return host.Fetch(ctx, url) }

// Event copies a gossip event. The upstream subscription has a bounded
// 128-event queue; Lagged events report loss when its consumer falls behind.
//
//wasmbind:record
type Event struct {
	Kind    uint8
	Content []byte
	Dropped uint64
}

// Subscription owns one pull iterator over a gossip topic.
//
//wasmbind:resource Subscription
type Subscription struct {
	topic *gossip.Topic
	next  func() (gossip.Event, error, bool)
	stop  func()
	once  sync.Once
	err   error
}

// Subscribe joins a topic using the supplied relay peer. The subscription
// outlives this constructor's context and is closed by return or disposal.
//
//wasmbind:export subscribe
func (e *Endpoint) Subscribe(ctx context.Context, peer string, id []byte) (*Subscription, error) {
	if len(id) != 32 {
		return nil, &WrapperError{Operation: "subscribe", Message: "topic must be 32 bytes"}
	}
	keyID, err := key.ParseEndpointID(peer)
	if err != nil {
		return nil, &WrapperError{Operation: "subscribe", Message: err.Error()}
	}
	var topicID gossip.TopicID
	copy(topicID[:], id)
	topic, err := e.gossip.SubscribeWithOpts(ctx, topicID, gossip.JoinOptions{Bootstrap: []netaddr.EndpointAddr{netaddr.NewEndpointAddr(keyID).WithRelayURL(e.relay)}, SubscriptionCapacity: 128})
	if err != nil {
		return nil, err
	}
	next, stop := iter.Pull2(topic.Events())
	return &Subscription{topic: topic, next: next, stop: stop}, nil
}

// Next pulls one event without an additional wrapper queue.
//
//wasmbind:next
func (s *Subscription) Next(ctx context.Context) (Event, bool, error) {
	stop := context.AfterFunc(ctx, func() { s.Interrupt() })
	defer stop()
	v, err, more := s.next()
	if ctx.Err() != nil {
		return Event{}, false, ctx.Err()
	}
	return Event{uint8(v.Kind), append([]byte(nil), v.Content...), v.Dropped}, more, err
}

// Interrupt closes the topic and wakes an active pull.
//
//wasmbind:interrupt
func (s *Subscription) Interrupt() error { s.once.Do(func() { s.err = s.topic.Close() }); return s.err }

// Close releases the pull iterator after active Next calls have returned.
//
//wasmbind:close
func (s *Subscription) Close() error { err := s.Interrupt(); s.stop(); return err }
