//go:build !darwin

package main

import (
	"context"
	"errors"
	"time"
)

type nativePCMPlayer struct{}

func newNativePCMPlayer(sampleRate int) (pcmPlayer, error) {
	return nil, errors.New("native audio playback is only available on darwin")
}

func (p *nativePCMPlayer) Play(_ context.Context, _ []byte) error {
	return nil
}

func (p *nativePCMPlayer) Cleanup() error {
	return nil
}

func (p *nativePCMPlayer) EstimatedLatency() time.Duration {
	return 0
}
