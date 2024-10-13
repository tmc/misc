package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func startFFPlay(ctx context.Context, state *AppState) error {
	logDebug(state, "Starting ffplay...")

	sampleRate := state.ActualSampleRate
	if sampleRate == 0 {
		sampleRate = state.DefaultSampleRate
	}

	state.AudioCmd = exec.Command("ffplay",
		// "-f", "s16le",
		// "-ar", fmt.Sprintf("%d", sampleRate),
		// "-ac", fmt.Sprintf("%d", state.DefaultChannels),
		"-i", "pipe:0",
		"-nodisp",
		"-loglevel", "debug")

	var err error
	state.AudioPipe, err = state.AudioCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("Failed to create stdin pipe for ffplay: %w", err)
	}

	if state.DebugLevel > 0 {
		state.AudioCmd.Stdout = os.Stdout
		state.AudioCmd.Stderr = os.Stderr
	} else {
		state.AudioCmd.Stdout = io.Discard
		state.AudioCmd.Stderr = io.Discard
	}

	err = state.AudioCmd.Start()
	if err != nil {
		state.AudioPipe = nil
		return fmt.Errorf("Failed to start ffplay: %w", err)
	}

	logDebug(state, "ffplay started successfully with parameters: SampleRate=%d, Channels=%d",
		sampleRate, state.DefaultChannels)

	go func() {
		err := state.AudioCmd.Wait()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				logDebug(state, "ffplay process ended with error: %v, stderr: %s", err, exitErr.Stderr)
			} else {
				logDebug(state, "ffplay process ended with error: %v", err)
			}
		} else {
			logDebug(state, "ffplay process ended normally")
		}
		state.AudioPipe = nil
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
