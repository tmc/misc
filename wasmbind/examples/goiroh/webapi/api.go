// Package webapi binds the relay-only endpoint, connection, and framed stream API.
// Dispose streams before connections and connections before endpoints. Stream
// cancellation aborts both directions; retry with a new stream.
package webapi

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/tmc/go-iroh/blobs"
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
	return &Endpoint{ep, url, cancel}, nil
}

// Connect opens a connection to a peer through the relay.
//
//wasmbind:export connect
func (e *Endpoint) Connect(ctx context.Context, peer, alpn string) (*Connection, error) {
	id, err := key.ParseEndpointID(peer)
	if err != nil {
		return nil, fmt.Errorf("parse peer: %w", err)
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
func (e *Endpoint) Close() error { e.cancel(); return e.endpoint.Shutdown(context.Background()) }

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

// Stream exchanges explicitly framed messages of at most one MiB.
//
//wasmbind:resource Stream
type Stream struct{ stream *iroh.Stream }

// Exchange writes one length-prefixed message and reads one response.
//
//wasmbind:export exchange
func (s *Stream) Exchange(ctx context.Context, data []byte) ([]byte, error) {
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("frame exceeds one MiB")
	}
	stop := context.AfterFunc(ctx, func() { s.stream.CancelRead(1); s.stream.CancelWrite(1) })
	defer stop()
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))
	if _, err := s.stream.Write(header[:]); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}
	if _, err := s.stream.Write(data); err != nil {
		return nil, fmt.Errorf("write frame: %w", err)
	}
	if _, err := io.ReadFull(s.stream, header[:]); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	n := binary.BigEndian.Uint32(header[:])
	if n > 1<<20 {
		return nil, fmt.Errorf("response exceeds one MiB")
	}
	out := make([]byte, n)
	if _, err := io.ReadFull(s.stream, out); err != nil {
		return nil, fmt.Errorf("read frame: %w", err)
	}
	return out, nil
}

// Close aborts both halves of the stream.
//
//wasmbind:close
func (s *Stream) Close() error { s.stream.CancelRead(0); s.stream.CancelWrite(0); return nil }
