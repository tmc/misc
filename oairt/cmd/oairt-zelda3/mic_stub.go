//go:build !darwin

package main

import (
	"context"
	"errors"
)

type micRecorder struct{}

func newMicRecorder(_ int, _ func([]byte)) *micRecorder {
	return &micRecorder{}
}

func (r *micRecorder) Start(context.Context) error {
	return errors.New("microphone recording is only implemented on darwin")
}

func (r *micRecorder) Stop() error {
	return nil
}
