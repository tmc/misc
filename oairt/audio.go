package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AudioPlayer defines the interface for audio playback
type AudioPlayer interface {
	Start(ctx context.Context, state *AppState, sampleRate int) error
	Write(data []byte) (int, error)
	Close() error
	IsClosed() bool
}

// FFplayPlayer implements AudioPlayer using FFplay
type FFplayPlayer struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	sampleRate int
	mutex      sync.Mutex
	state      *AppState
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	closed     bool
}

func NewFFplayPlayer(state *AppState) *FFplayPlayer {
	return &FFplayPlayer{
		state: state,
	}
}

func (p *FFplayPlayer) Start(ctx context.Context, state *AppState, sampleRate int) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.state = state
	p.sampleRate = sampleRate

	p.ctx, p.cancelFunc = context.WithCancel(ctx)

	ffplayArgs := []string{
		"-f", "s16le",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-i", "pipe:0",
		"-nodisp",
		"-autoexit",
	}

	p.cmd = exec.CommandContext(p.ctx, "ffplay", ffplayArgs...)

	var err error
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe for FFplay: %w", err)
	}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFplay: %w", err)
	}

	logVerbose("Started FFplay process",
		zap.Int("pid", p.cmd.Process.Pid),
		zap.String("command", p.cmd.String()),
	)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		err := p.cmd.Wait()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() != 123 { // Ignore exit code 123, which is used for normal termination
					logError("FFplay process ended with error", err)
				} else {
					logDebug("FFplay process ended normally")
				}
			} else {
				logError("FFplay process ended with error", err)
			}
		} else {
			logDebug("FFplay process ended normally")
		}
		p.Close()
	}()

	return nil
}

func (p *FFplayPlayer) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	if p.cancelFunc != nil {
		p.cancelFunc()
	}

	if p.stdin != nil {
		p.stdin.Close()
	}

	// Signal the process to stop
	if p.cmd != nil && p.cmd.Process != nil {
		logDebug("Attempting to stop FFplay process")
		if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
			logDebug("Failed to send interrupt signal to FFplay process, attempting to kill")
			if err := p.cmd.Process.Kill(); err != nil {
				if err != os.ErrProcessDone {
					logError("Failed to kill FFplay process", err)
				} else {
					logDebug("FFplay process had already finished")
				}
			}
		}
	}

	// Wait for the process to finish with a timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logDebug("FFplay process closed successfully")
	case <-time.After(5 * time.Second):
		logError("Timeout waiting for FFplay process to close", nil)
	}

	p.cmd = nil
	p.stdin = nil

	return nil
}

func (p *FFplayPlayer) Write(data []byte) (int, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return 0, fmt.Errorf("audio player is closed")
	}

	if p.stdin == nil {
		return 0, fmt.Errorf("audio player is not started")
	}

	return p.stdin.Write(data)
}

func (p *FFplayPlayer) IsClosed() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.closed
}

// BufferedAudioWriter implements a buffered writer for audio data
type BufferedAudioWriter struct {
	player  AudioPlayer
	buffer  []byte
	mutex   sync.Mutex
	maxSize int
	closed  bool
}

func NewBufferedAudioWriter(player AudioPlayer, maxSize int) AudioWriteCloser {
	return &BufferedAudioWriter{
		player:  player,
		buffer:  make([]byte, 0, maxSize),
		maxSize: maxSize,
	}
}

func (w *BufferedAudioWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.closed {
		return 0, fmt.Errorf("BufferedAudioWriter is closed")
	}

	w.buffer = append(w.buffer, p...)

	if len(w.buffer) >= w.maxSize {
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

	if w.closed {
		return nil
	}

	w.closed = true
	if err := w.flush(); err != nil {
		return err
	}
	return w.player.Close()
}

func (w *BufferedAudioWriter) IsClosed() bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.closed
}
