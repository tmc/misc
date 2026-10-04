// Package lifetime exercises explicit ownership.
//
//wasmbind:naming camelCase
package lifetime

import (
	"context"
	"errors"
	"sync/atomic"
)

var sequence string
var closed atomic.Int32

//wasmbind:resource Node
type Node struct {
	Label string
	child *Node
	gate  chan struct{}
}

//wasmbind:export node
//wasmbind:nonnull
func NewNode(label string) *Node { return &Node{Label: label} }

//wasmbind:export child
//wasmbind:owned
//wasmbind:nonnull
func (n *Node) Child(label string) *Node {
	if n.child == nil {
		n.child = &Node{Label: label}
	}
	return n.child
}

//wasmbind:export adopt
//wasmbind:owned
//wasmbind:nonnull
func (n *Node) Adopt(c *Node) *Node { return c }

//wasmbind:export alias
//wasmbind:nonnull
func (n *Node) Alias() *Node { return n }

//wasmbind:export name
func (n *Node) Name() string { return n.Label }

//wasmbind:export children
//wasmbind:owned
func (n *Node) Children() []*Node { return []*Node{{Label: "a"}, {Label: "b"}} }

//wasmbind:close
func (n *Node) Close() error {
	sequence += n.Label
	closed.Add(1)
	if n.Label == "!" {
		return errors.New("close failed")
	}
	return nil
}

//wasmbind:export sequence
func Sequence() string { return sequence }

//wasmbind:export reset
func Reset() { sequence = ""; closed.Store(0) }

//wasmbind:export closed
func Closed() int { return int(closed.Load()) }

//wasmbind:export wait
func (n *Node) Wait(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }

var lateStarted chan struct{}
var lateRelease chan struct{}

//wasmbind:export prepareLate
func PrepareLate() { lateStarted = make(chan struct{}); lateRelease = make(chan struct{}) }

//wasmbind:export lateStarted
func LateStarted() bool {
	select {
	case <-lateStarted:
		return true
	default:
		return false
	}
}

//wasmbind:export releaseLate
func ReleaseLate() { close(lateRelease) }

//wasmbind:export late
//wasmbind:owned
func (n *Node) Late() *Node { close(lateStarted); <-lateRelease; return &Node{Label: "l"} }
