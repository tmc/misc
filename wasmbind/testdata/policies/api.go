// Package policies exercises explicit browser contracts.
//
//wasmbind:naming camelCase
package policies

import "context"

// Kind is a closed event kind.
//
//wasmbind:enum Kind
//wasmbind:validate
type Kind uint8

const (
	First Kind = 1
	Alias Kind = 1
	Last  Kind = 255
)

// OpenKind accepts future event kinds.
//
//wasmbind:enum OpenKind
type OpenKind uint8

const Known OpenKind = 1

// Boundary demonstrates bigint enum precision.
//
//wasmbind:enum Boundary
//wasmbind:validate
type Boundary int64

const (
	Min Boundary = -9223372036854775808
	Max Boundary = 9223372036854775807
)

// Record is a copied record.
//
//wasmbind:record
type Record struct {
	// HTTPStatus is the status code.
	HTTPStatus int
	URL        string
	Exact      string `wasmbind:"Exact"`
	Kind       Kind
}

// HTTPResult returns a populated record or deliberately violates its contract.
//
//wasmbind:export
//wasmbind:nonnull
func HTTPResult(nilResult bool) *Record {
	if nilResult {
		return nil
	}
	return &Record{HTTPStatus: 200, URL: "https://example.com", Exact: "override", Kind: First}
}

//wasmbind:export kind
func EchoKind(kind Kind) Kind { return kind }

//wasmbind:export invalidKind
func InvalidKind() Kind { return 2 }

//wasmbind:export openKind
func EchoOpenKind(kind OpenKind) OpenKind { return kind }

//wasmbind:export boundary
func EchoBoundary(boundary Boundary) Boundary { return boundary }

// Resource demonstrates a non-null resource result.
//
//wasmbind:resource Resource
type Resource struct{ token byte }

//wasmbind:export resource
//wasmbind:nonnull
func NewResource(nilResult bool) *Resource {
	if nilResult {
		return nil
	}
	return &Resource{}
}

//wasmbind:close
func (r *Resource) Close() {}

// Concurrent permits a blocked receive and a simultaneous send.
//
//wasmbind:resource Concurrent
//wasmbind:concurrent
type Concurrent struct{ values chan string }

//wasmbind:export concurrent
//wasmbind:nonnull
func NewConcurrent() *Concurrent { return &Concurrent{values: make(chan string, 1)} }

//wasmbind:export receive
func (c *Concurrent) Receive(ctx context.Context) (string, error) {
	select {
	case v := <-c.values:
		return v, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

//wasmbind:export send
func (c *Concurrent) Send(v string) { c.values <- v }

//wasmbind:close
func (c *Concurrent) Close() {}
