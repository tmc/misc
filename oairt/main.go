package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Parse command-line flags
	config, err := parseFlags()
	if err != nil {
		return fmt.Errorf("error parsing flags: %w", err)
	}

	// Initialize logger
	if err := initLogger(config.DebugLevel); err != nil {
		return fmt.Errorf("error initializing logger: %w", err)
	}
	defer logger.Sync()

	// Initialize AppState
	state := &AppState{
		DefaultSampleRate: 24000,
		DefaultBitDepth:   16,
		DefaultChannels:   1,
		DebugLevel:        config.DebugLevel,
		AudioOutputFile:   config.AudioOutputFile,
	}

	// Setup audio output file if specified
	if state.AudioOutputFile != "" {
		if err := setupAudioOutputFile(state); err != nil {
			return fmt.Errorf("error setting up audio output file: %w", err)
		}
		defer state.AudioFile.Close()
	}

	// Setup audio streaming if enabled
	if config.AudioStream {
		if err := setupAudioStreaming(ctx, state); err != nil {
			return fmt.Errorf("error setting up audio streaming: %w", err)
		}
		defer state.AudioOutput.Close()
	}

	// Create and connect the realtime client
	client, err := setupRealtimeClient(ctx, config, state)
	if err != nil {
		return fmt.Errorf("error setting up realtime client: %w", err)
	}
	defer client.Disconnect()

	// Apply initial voice and instructions if provided
	if config.InitialVoice != "" || config.InitialInstructions != "" {
		inputHandler := NewInputHandler(client, state)
		if err := inputHandler.updateSession(config.InitialVoice, config.InitialInstructions); err != nil {
			logError("Failed to set initial voice or instructions", err)
		}
	}

	// Start input handling
	inputHandler := NewInputHandler(client, state)
	go inputHandler.ReadStdin(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

type Config struct {
	APIKey              string
	AudioOutputFile     string
	AudioStream         bool
	DebugLevel          int
	InitialVoice        string
	InitialInstructions string
	ModelName           string
}

func parseFlags() (*Config, error) {
	config := &Config{}

	flag.StringVar(&config.APIKey, "api-key", "", "OpenAI API Key (overrides OPENAI_API_KEY env variable)")
	flag.StringVar(&config.AudioOutputFile, "audio-output", "", "File to save audio output to")
	flag.BoolVar(&config.AudioStream, "audio-stream", false, "Stream audio in real-time")
	flag.IntVar(&config.DebugLevel, "debug", 0, "Debug level (0=off, 1=debug, 2=verbose)")
	flag.StringVar(&config.InitialVoice, "voice", "", "Initial voice to use")
	flag.StringVar(&config.InitialInstructions, "instructions", "", "Initial instructions for the AI")
	flag.StringVar(&config.ModelName, "model", "gpt-4o-realtime-preview-2024-10-01", "Model name to use")
	flag.Parse()

	if config.APIKey == "" {
		config.APIKey = os.Getenv("OPENAI_API_KEY")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required. Set OPENAI_API_KEY environment variable or use -api-key flag")
	}

	return config, nil
}

func setupAudioOutputFile(state *AppState) error {
	var err error
	state.AudioFile, err = os.OpenFile(state.AudioOutputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error creating or truncating audio output file: %w", err)
	}
	logDebug("Audio output file created", zap.String("file", state.AudioOutputFile))
	return nil
}

func setupAudioStreaming(ctx context.Context, state *AppState) error {
	player := NewMacOSAudioPlayer(state)
	if err := player.Start(ctx, state, state.DefaultSampleRate); err != nil {
		return fmt.Errorf("failed to start audio playback: %w", err)
	}
	state.AudioOutput = NewBufferedAudioWriter(player, 8192) // 8KB buffer
	return nil
}

func setupRealtimeClient(ctx context.Context, config *Config, state *AppState) (*RealtimeClient, error) {
	client := NewRealtimeClient(config.APIKey, state, WithDebug(state.DebugLevel > 0), WithDumpFrames(state.DebugLevel > 1))

	client.On("*", func(event Event) {
		handleEvent(ctx, state, event)
	})

	if err := client.Connect(ctx, config.ModelName); err != nil {
		return nil, fmt.Errorf("error connecting: %w", err)
	}

	return client, nil
}
