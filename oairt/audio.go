package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

type BufferedAudioWriter struct {
	pipe    io.WriteCloser
	buffer  []byte
	mutex   sync.Mutex
	maxSize int
}

func NewBufferedAudioWriter(pipe io.WriteCloser, maxSize int) *BufferedAudioWriter {
	return &BufferedAudioWriter{
		pipe:    pipe,
		buffer:  make([]byte, 0, maxSize),
		maxSize: maxSize,
	}
}

func (w *BufferedAudioWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if len(w.buffer)+len(p) > w.maxSize {
		if err := w.flush(); err != nil {
			return 0, err
		}
	}

	w.buffer = append(w.buffer, p...)
	return len(p), nil
}

func (w *BufferedAudioWriter) flush() error {
	if len(w.buffer) == 0 {
		return nil
	}

	_, err := w.pipe.Write(w.buffer)
	w.buffer = w.buffer[:0]
	return err
}

func (w *BufferedAudioWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if err := w.flush(); err != nil {
		return err
	}
	return w.pipe.Close()
}

func startFFPlay(ctx context.Context, state *AppState) error {
	logDebug(state, "Starting ffplay...")

	sampleRate := state.ActualSampleRate
	if sampleRate == 0 {
		sampleRate = state.DefaultSampleRate
	}

	ffplayArgs := []string{
		"-f", "s16le",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-i", "pipe:0",
		"-nodisp",
		"-loglevel", "warning",
	}

	state.AudioCmd = exec.CommandContext(ctx, "ffplay", ffplayArgs...)

	var err error
	state.AudioPipe, err = state.AudioCmd.StdinPipe()
	if err != nil {
		return NewAppError(err, "Failed to create stdin pipe for ffplay", "AUDIO_PIPE_ERROR")
	}

	if state.AudioPipe == nil {
		return NewAppError(nil, "Audio pipe is nil after creation", "AUDIO_PIPE_NIL")
	}

	if state.DebugLevel > 0 {
		state.AudioCmd.Stdout = os.Stdout
		state.AudioCmd.Stderr = os.Stderr
	} else {
		state.AudioCmd.Stdout = io.Discard
		state.AudioCmd.Stderr = io.Discard
	}

	logDebug(state, "Starting ffplay with command: %v", state.AudioCmd.Args)

	err = state.AudioCmd.Start()
	if err != nil {
		state.AudioPipe = nil
		return NewAppError(err, "Failed to start ffplay", "FFPLAY_START_ERROR")
	}

	state.AudioOutput = NewBufferedAudioWriter(state.AudioPipe, 8192) // 8KB buffer

	logDebug(state, "ffplay started successfully with parameters: SampleRate=%d", sampleRate)
	go func() {
		errChan := make(chan error, 1)
		go func() {
			errChan <- state.AudioCmd.Wait()
		}()

		select {
		case <-ctx.Done():
			logDebug(state, "Context cancelled, stopping ffplay")
			if err := state.AudioCmd.Process.Signal(os.Interrupt); err != nil {
				logDebug(state, "Error sending interrupt signal to ffplay: %v", err)
			}
		case err := <-errChan:
			if err != nil {
				logDebug(state, "ffplay process ended with error: %v", err)
			} else {
				logDebug(state, "ffplay process ended")
			}
		}
		state.AudioPipe = nil
		state.AudioOutput = nil
	}()

	return nil
}

func updateAudioParams(ctx context.Context, state *AppState, newSession *Session) error {
	if newSession == nil {
		logInfo(state, "No session data provided. Skipping audio params update.")
		return nil
	}

	state.Session = newSession

	oldSampleRate := state.ActualSampleRate
	switch state.Session.OutputAudioFormat {
	case "pcm16":
		state.ActualSampleRate = 24000 // or whatever the correct rate is
	default:
		logDebug(state, "Unknown audio format: %s. Using default sample rate.", state.Session.OutputAudioFormat)
		state.ActualSampleRate = state.DefaultSampleRate
	}

	if oldSampleRate != state.ActualSampleRate {
		logDebug(state, "Sample rate changed from %d to %d Hz. Restarting audio playback.", oldSampleRate, state.ActualSampleRate)
		if state.AudioPipe != nil && state.AudioCmd != nil {
			state.AudioCmd.Process.Kill()
			return startFFPlay(ctx, state)
		}
	}
	return nil
}

func restartAudioPlayback(ctx context.Context, state *AppState) error {
	if state.AudioCmd != nil && state.AudioCmd.Process != nil {
		state.AudioCmd.Process.Kill()
	}
	return startFFPlay(ctx, state)
}
