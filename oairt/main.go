package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	state := &AppState{
		DefaultSampleRate: 24000,
		DefaultBitDepth:   16,
		DefaultChannels:   1,
	}
	var apiKey string
	var audioStream bool
	var initialVoice string
	var initialInstructions string
	var modelName string

	flag.StringVar(&apiKey, "api-key", "", "OpenAI API Key (overrides OPENAI_API_KEY env variable)")
	flag.StringVar(&state.AudioOutputFile, "audio-output", "", "File to save audio output to")
	flag.BoolVar(&audioStream, "audio-stream", false, "Stream audio in real-time")
	flag.IntVar(&state.DebugLevel, "debug", 0, "Debug level (0=off, 1=debug, 2=verbose)")
	flag.StringVar(&initialVoice, "voice", "", "Initial voice to use")
	flag.StringVar(&initialInstructions, "instructions", "", "Initial instructions for the AI")
	flag.StringVar(&modelName, "model", "gpt-4o-realtime-preview-2024-10-01", "Model name to use")
	flag.Parse()

	apiKey = os.Getenv("OPENAI_API_KEY")
	if flag.Lookup("api-key").Value.String() != "" {
		apiKey = flag.Lookup("api-key").Value.String()
	}

	if apiKey == "" {
		return fmt.Errorf("API key is required. Set OPENAI_API_KEY environment variable or use -api-key flag.")
	}

	if state.AudioOutputFile != "" {
		var err error
		state.AudioFile, err = os.OpenFile(state.AudioOutputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.Fatalf("Error creating or truncating audio output file: %v", err)
		}
		logDebug(state, "Audio output file created: %s", state.AudioOutputFile)
		defer func() {
			if err := state.AudioFile.Close(); err != nil {
				logDebug(state, "Error closing audio file: %v", err)
			}
			logDebug(state, "Audio output file closed")
		}()
	}

	if audioStream {
		// Check if ffplay is installed
		_, err := exec.LookPath("ffplay")
		if err != nil {
			return fmt.Errorf("ffplay not found. Please install FFmpeg to use audio streaming.")
		}
		if err := startFFPlay(ctx, state); err != nil {
			return fmt.Errorf("Failed to start ffplay: %w", err)
		}
	}

	client := NewRealtimeClient(apiKey, state, WithDebug(state.DebugLevel > 0), WithDumpFrames(state.DebugLevel > 1))

	client.On("*", func(event Event) {
		handleEvent(ctx, state, event)
	})

	if err := client.Connect(ctx, modelName); err != nil {
		return fmt.Errorf("Error connecting: %w", err)
	}

	// Apply initial voice and instructions if provided
	if initialVoice != "" || initialInstructions != "" {
		updateSession(client, initialVoice, initialInstructions)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		fmt.Println("\nContext cancelled. Shutting down...")
		client.Disconnect()
		if state.AudioCmd != nil && state.AudioCmd.Process != nil {
			state.AudioCmd.Process.Kill()
		}
	}()

	readStdin(ctx, client)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
		return client.Disconnect()
	}
}
