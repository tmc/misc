package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	audioChannels      = 1
	audioBitsPerSample = 16

	audioTargetDuration = 180 * time.Millisecond
	audioIdleFlush      = 45 * time.Millisecond
)

type pcmPlayer interface {
	Play(context.Context, []byte) error
	Cleanup() error
	EstimatedLatency() time.Duration
}

type audioSink struct {
	ctx        context.Context
	cancel     context.CancelFunc
	sampleRate int

	once sync.Once
	ch   chan []byte
	done chan struct{}

	mu      sync.Mutex
	err     error
	started bool
	closed  bool
}

func newAudioSink(ctx context.Context, sampleRate int) *audioSink {
	ctx, cancel := context.WithCancel(ctx)
	return &audioSink{
		ctx:        ctx,
		cancel:     cancel,
		sampleRate: sampleRate,
	}
}

func (s *audioSink) Write(audio []byte) {
	if s == nil || len(audio) == 0 {
		return
	}
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return
	}
	s.once.Do(s.start)
	s.mu.Lock()
	if s.closed || s.ch == nil || s.done == nil {
		s.mu.Unlock()
		return
	}
	ch := s.ch
	done := s.done
	s.mu.Unlock()

	buf := make([]byte, len(audio))
	copy(buf, audio)
	select {
	case ch <- buf:
	case <-done:
	case <-s.ctx.Done():
	}
}

func (s *audioSink) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	started := s.started
	done := s.done
	if !s.closed {
		s.closed = true
		s.cancel()
	}
	s.mu.Unlock()
	if started && done != nil {
		<-done
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *audioSink) start() {
	s.ch = make(chan []byte, 64)
	s.done = make(chan struct{})
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()

	go func() {
		defer close(s.done)

		player, err := newNativePCMPlayer(s.sampleRate)
		if err != nil {
			s.setErr(fmt.Errorf("init audio player: %w", err))
			return
		}
		defer func() {
			if err := player.Cleanup(); err != nil {
				s.setErr(fmt.Errorf("cleanup audio player: %w", err))
			}
		}()

		targetBytes := audioBytesForDuration(s.sampleRate, audioTargetDuration)
		if latency := player.EstimatedLatency(); latency > 0 {
			if tuned := audioBytesForDuration(s.sampleRate, 4*latency); tuned > targetBytes {
				targetBytes = tuned
			}
		}

		var pending []byte
		var timer *time.Timer
		var timerC <-chan time.Time

		stopTimer := func() {
			if timer == nil {
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer = nil
			timerC = nil
		}
		armTimer := func() {
			if timer == nil {
				timer = time.NewTimer(audioIdleFlush)
				timerC = timer.C
			}
		}
		flush := func() bool {
			if len(pending) == 0 {
				stopTimer()
				return true
			}
			buf := make([]byte, len(pending))
			copy(buf, pending)
			pending = pending[:0]
			stopTimer()
			if err := player.Play(s.ctx, buf); err != nil {
				s.setErr(fmt.Errorf("play audio: %w", err))
				return false
			}
			return true
		}

		for {
			select {
			case audio := <-s.ch:
				if len(audio) == 0 {
					continue
				}
				pending = append(pending, audio...)
				if len(pending) >= targetBytes && !flush() {
					return
				}
				armTimer()
			case <-timerC:
				if !flush() {
					return
				}
			case <-s.ctx.Done():
				flush()
				return
			}
		}
	}()
}

func audioBytesForDuration(sampleRate int, d time.Duration) int {
	if sampleRate <= 0 || d <= 0 {
		return 0
	}
	bytesPerSecond := sampleRate * audioChannels * audioBitsPerSample / 8
	n := int((int64(bytesPerSecond)*int64(d) + int64(time.Second) - 1) / int64(time.Second))
	if n < 1 {
		return 1
	}
	return n
}

func (s *audioSink) setErr(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
}
