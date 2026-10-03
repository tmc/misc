// Package api is the browser integration fixture.
package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// Payload is copied data.
//
//wasmbind:record
type Payload struct {
	Name   string `wasmbind:"name"`
	Values []int32
	Bytes  []byte
	Skip   string `wasmbind:"-"`
}

// Scalars exercises all scalar representations.
//
//wasmbind:record
type Scalars struct {
	Bool bool
	Int  int
	Uint uint
	I8   int8
	U8   uint8
	I16  int16
	U16  uint16
	I32  int32
	U32  uint32
	I64  int64
	U64  uint64
	F32  float32
	F64  float64
	Text string
}

//wasmbind:export scalars
func RoundtripScalars(v Scalars) Scalars { return v }

//wasmbind:export payload
func Roundtrip(v *Payload) *Payload { return v }

//wasmbind:export bytes
func Bytes(v []byte) []byte { return v }

//wasmbind:export array
func Array(v [2]byte) [2]byte { return v }

//wasmbind:export nested
func Nested(v [][]string) [][]string { return v }

//wasmbind:export fail
func Fail() error { return errors.New("exact Go failure") }

//wasmbind:export panic
func Panic() { panic("private panic message") }

//wasmbind:export wait
func Wait(ctx context.Context, ms int) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(time.Duration(ms) * time.Millisecond):
		return "done", nil
	}
}

//wasmbind:export fetch
func Fetch(ctx context.Context, url string) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

//wasmbind:resource Client
type Client struct {
	value      int
	closeError bool
	closed     bool
}

var active, closed atomic.Int32

//wasmbind:export newClient
func NewClient(fail bool) *Client { active.Add(1); return &Client{closeError: fail} }

//wasmbind:export same
func (c *Client) Same() *Client { return c }

//wasmbind:export add
func (c *Client) Add(ctx context.Context, n int) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(10 * time.Millisecond):
	}
	c.value += n
	return c.value, nil
}

//wasmbind:export echoInt[int]
//wasmbind:export echoString[string]
func (c *Client) Echo[T ~int | ~string](v T) T { return v }

//wasmbind:close
func (c *Client) Close() error {
	if c.closed {
		panic("closed twice")
	}
	c.closed = true
	active.Add(-1)
	closed.Add(1)
	if c.closeError {
		return errors.New("close failed")
	}
	return nil
}

//wasmbind:export use
func Use(ctx context.Context, c *Client) (int, error) {
	if c == nil {
		return -1, nil
	}
	<-ctx.Done()
	return c.value, ctx.Err()
}

//wasmbind:export late
func Late(ctx context.Context, ms int) *Client {
	time.Sleep(time.Duration(ms) * time.Millisecond)
	return NewClient(false)
}

//wasmbind:export counts
func Counts() [2]int32 { return [2]int32{active.Load(), closed.Load()} }

//wasmbind:export identityInt[int]
func Identity[T ~int](v T) T { return v }

//wasmbind:export clients
func Clients(n int) []*Client {
	var out []*Client
	for i := 0; i < n; i++ {
		out = append(out, NewClient(false))
	}
	return out
}

//wasmbind:export duplicateClients
func DuplicateClients() []*Client { c := NewClient(false); return []*Client{c, c, c} }

//wasmbind:export exit
func Exit() { os.Exit(0) }

var saved *Client

//wasmbind:export saved
func Saved() *Client {
	if saved == nil {
		saved = NewClient(false)
	}
	return saved
}

var ignoreGate = make(chan struct{})
var ignoreStarted bool

func IgnoreStarted() bool { return ignoreStarted }
func ReleaseIgnore()      { close(ignoreGate) }

//wasmbind:export ignore
func Ignore(ctx context.Context, ms int) string {
	if ms < 0 {
		ignoreStarted = true
		<-ignoreGate
		return "done"
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
	return "finished"
}

//wasmbind:export huge
func Huge(n int) []byte { return make([]byte, n) }
