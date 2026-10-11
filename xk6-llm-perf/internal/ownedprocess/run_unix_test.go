//go:build unix

package ownedprocess

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

type delayedErrorWriter struct{}

func (delayedErrorWriter) Write([]byte) (int, error) {
	// The process exits before the output copy finishes. Its cleanup timer
	// expires first; this later copy error must not replace ErrCleanup.
	time.Sleep(2 * time.Second)
	return 0, errors.New("late copy error")
}

func TestCleanupErrorPriority(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "echo", "hello")
	cmd.Stdout = delayedErrorWriter{}
	if err := Run(cmd); !errors.Is(err, ErrCleanup) {
		t.Fatalf("cleanup error: %v, want %v", err, ErrCleanup)
	}
}
