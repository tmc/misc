package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"go.uber.org/zap"
)

// AudioPlayer defines the interface for audio playback
type AudioPlayer interface {
	Start(ctx context.Context, state *AppState, sampleRate int) error
	Write(data []byte) (int, error)
	Close() error
}

// MacOSAudioPlayer implements AudioPlayer for macOS using afplay
type MacOSAudioPlayer struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	sampleRate int
	mutex      sync.Mutex
	state      *AppState
}

func NewMacOSAudioPlayer(state *AppState) *MacOSAudioPlayer {
	return &MacOSAudioPlayer{
		state: state,
	}
}

func (p *MacOSAudioPlayer) Start(ctx context.Context, state *AppState, sampleRate int) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.state = state

	if p.cmd != nil {
		return fmt.Errorf("audio player is already started")
	}

	p.sampleRate = sampleRate
	args := []string{
		"-f", "s16le",
		"-r", fmt.Sprintf("%d", sampleRate),
		"-t", "raw",
		"-c", "1",
		"-",
	}

	p.cmd = exec.CommandContext(ctx, "afplay", args...)

	var err error
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start afplay: %w", err)
	}

	go func() {
		if err := p.cmd.Wait(); err != nil {
			logError("afplay process ended with error", err)
		}
	}()

	return nil
}

func (p *MacOSAudioPlayer) Write(data []byte) (int, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.stdin == nil {
		return 0, fmt.Errorf("audio player is not started")
	}

	return p.stdin.Write(data)
}

func (p *MacOSAudioPlayer) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.cmd == nil {
		return nil
	}

	if p.stdin != nil {
		p.stdin.Close()
	}

	if err := p.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill afplay process: %w", err)
	}

	p.cmd = nil
	p.stdin = nil
	return nil
}

// BufferedAudioWriter implements a buffered writer for audio data
type BufferedAudioWriter struct {
	player  AudioPlayer
	buffer  []byte
	mutex   sync.Mutex
	maxSize int
}

func NewBufferedAudioWriter(player AudioPlayer, maxSize int) *BufferedAudioWriter {
	return &BufferedAudioWriter{
		player:  player,
		buffer:  make([]byte, 0, maxSize),
		maxSize: maxSize,
	}
}

func (w *BufferedAudioWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.buffer = append(w.buffer, p...)

	if len(w.buffer) >= w.maxSize/2 {
		if err := w.flush(); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func (w *BufferedAudioWriter) flush() error {
	if len(w.buffer) == 0 {
		return nil
	}

	_, err := w.player.Write(w.buffer)
	if err != nil {
		logError("Error writing to audio output", err, zap.Int("bytes", len(w.buffer)))
	}
	w.buffer = w.buffer[:0]
	return err
}

func (w *BufferedAudioWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if err := w.flush(); err != nil {
		return err
	}
	return w.player.Close()
}