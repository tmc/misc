package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tmc/macgo"
	oairt "github.com/tmc/misc/oairt"
	"go.uber.org/zap"
)

func main() {
	cfg := macgo.NewConfig().
		WithPermissions(macgo.Microphone, macgo.Network).
		WithAdHocSign().
		WithDebug()

	if err := macgo.Start(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting macgo: %v\n", err)
		os.Exit(1)
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	config, err := parseFlags()
	if err != nil {
		return fmt.Errorf("error parsing flags: %w", err)
	}

	if err := initLogger(config.DebugLevel); err != nil {
		return fmt.Errorf("error initializing logger: %w", err)
	}
	defer logger.Sync()

	state := &AppState{
		DefaultSampleRate: 24000,
		DefaultBitDepth:   16,
		DefaultChannels:   1,
		DebugLevel:        config.DebugLevel,
		AudioOutputFile:   config.AudioOutputFile,
	}

	if state.AudioOutputFile != "" {
		if err := setupAudioOutputFile(state); err != nil {
			return fmt.Errorf("error setting up audio output file: %w", err)
		}
		defer state.AudioFile.Close()
	}

	if config.AudioStream {
		if err := setupAudioStreaming(ctx, state); err != nil {
			return fmt.Errorf("error setting up audio streaming: %w", err)
		}
		defer state.AudioOutput.Close()
	}

	client, err := setupRealtimeClient(ctx, config, state)
	if err != nil {
		return fmt.Errorf("error setting up realtime client: %w", err)
	}
	defer client.Close()

	if config.AudioStream {
		if err := startAudioInput(ctx, state, client); err != nil {
			logError("Failed to start audio input", err)
		}
	}

	if err := waitForInitialSession(ctx, state); err != nil {
		return err
	}

	if err := applyInitialSession(client, config); err != nil {
		logError("Failed to apply initial session config", err)
	}

	inputHandler := NewInputHandler(client, state)
	inputDone := make(chan struct{})
	go func() {
		inputHandler.ReadStdin(ctx)
		close(inputDone)
	}()

	done := make(chan struct{})
	go func() {
		runEventLoop(ctx, state, inputDone)
		close(done)
	}()

	select {
	case <-sigChan:
		logInfo("Received interrupt signal, shutting down")
		cancel()
	case <-done:
		logInfo("Event loop finished, shutting down")
	}

	if err := performShutdown(state); err != nil {
		logError("Error during shutdown", err)
	}

	return nil
}

func waitForInitialSession(ctx context.Context, state *AppState) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if state.CurrentSession() != nil {
			logInfo("Initial session received")
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("timeout waiting for initial session")
}

func runEventLoop(ctx context.Context, _ *AppState, inputDone <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logInfo("Event loop context cancelled")
			return
		case <-inputDone:
			logInfo("Input handler finished")
			return
		case <-ticker.C:
			logDebug("Event loop heartbeat")
		}
	}
}

func performShutdown(state *AppState) error {
	logDebug("Beginning shutdown process")
	time.Sleep(500 * time.Millisecond)

	if state.AudioOutput != nil {
		logDebug("Closing audio output")
		if err := state.AudioOutput.Close(); err != nil {
			logError("Error closing audio output", err)
		}
	}

	logInfo("Shutdown complete")
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
	URL                 string

	Temperature       float64
	TemperatureSet    bool
	ReasoningEffort   string
	Modalities        string
	InputAudioFormat  string
	OutputAudioFormat string

	VADType          string
	VADThreshold     float64
	VADThresholdSet  bool
	VADPrefixPadding int
	VADSilenceMs     int

	TranscriptionEnabled    bool
	TranscriptionEnabledSet bool
	TranscriptionModel      string
	TranscriptionLanguage   string
	TranscriptionPrompt     string
	TranscriptionDelay      string

	ToolChoice string
}

func parseFlags() (*Config, error) {
	config := &Config{}

	flag.StringVar(&config.APIKey, "api-key", "", "OpenAI API Key (overrides OPENAI_API_KEY env variable)")
	flag.StringVar(&config.AudioOutputFile, "audio-output", "", "File to save audio output to")
	flag.BoolVar(&config.AudioStream, "audio-stream", true, "Stream audio in real-time (use -audio-stream=false to disable)")
	flag.IntVar(&config.DebugLevel, "debug", 0, "Debug level (0=off, 1=debug, 2=verbose)")
	flag.StringVar(&config.InitialVoice, "voice", "", "Initial voice to use")
	flag.StringVar(&config.InitialInstructions, "instructions", "", "Initial instructions for the AI")
	flag.StringVar(&config.ModelName, "model", "gpt-realtime-2", "Model name to use")
	flag.StringVar(&config.URL, "url", "", "Override Realtime WebSocket URL")

	flag.Func("temperature", "Sampling temperature (0.0-2.0)", func(s string) error {
		_, err := fmt.Sscanf(s, "%f", &config.Temperature)
		if err == nil {
			config.TemperatureSet = true
		}
		return err
	})
	flag.StringVar(&config.ReasoningEffort, "effort", "", "Reasoning effort: minimal, low, medium, high, xhigh (gpt-realtime-2)")
	flag.StringVar(&config.Modalities, "modalities", "", "Comma-separated modalities: text,audio")
	flag.StringVar(&config.InputAudioFormat, "input-audio-format", "", "Input audio format (e.g. pcm16, g711_ulaw)")
	flag.StringVar(&config.OutputAudioFormat, "output-audio-format", "", "Output audio format (e.g. pcm16, g711_ulaw)")

	flag.StringVar(&config.VADType, "vad-type", "", "Turn-detection type (e.g. server_vad, none)")
	flag.Func("vad-threshold", "VAD activation threshold (0.0-1.0)", func(s string) error {
		_, err := fmt.Sscanf(s, "%f", &config.VADThreshold)
		if err == nil {
			config.VADThresholdSet = true
		}
		return err
	})
	flag.IntVar(&config.VADPrefixPadding, "vad-prefix-padding-ms", 0, "VAD prefix padding (ms)")
	flag.IntVar(&config.VADSilenceMs, "vad-silence-ms", 0, "VAD silence duration (ms)")

	flag.Func("transcription-enabled", "Enable input audio transcription (true/false)", func(s string) error {
		switch strings.ToLower(s) {
		case "1", "true", "t", "yes", "on":
			config.TranscriptionEnabled = true
		case "0", "false", "f", "no", "off":
			config.TranscriptionEnabled = false
		default:
			return fmt.Errorf("invalid bool: %q", s)
		}
		config.TranscriptionEnabledSet = true
		return nil
	})
	flag.StringVar(&config.TranscriptionModel, "transcription-model", "", "Transcription model (e.g. whisper-1)")
	flag.StringVar(&config.TranscriptionLanguage, "transcription-language", "", "Transcription language (BCP-47)")
	flag.StringVar(&config.TranscriptionPrompt, "transcription-prompt", "", "Transcription prompt")
	flag.StringVar(&config.TranscriptionDelay, "transcription-delay", "", "Transcription delay")

	flag.StringVar(&config.ToolChoice, "tool-choice", "", "Tool choice (auto, none, required)")

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
	if err := checkAudioDependencies(); err != nil {
		return err
	}

	player := NewAudioPlayer(state)
	if err := player.Start(ctx, state, state.DefaultSampleRate); err != nil {
		return fmt.Errorf("failed to start audio playback: %w", err)
	}
	state.AudioEngine = player
	state.AudioOutput = NewBufferedAudioWriter(player, 8192)

	logInfo("Audio streaming setup completed successfully")
	return nil
}

func startAudioInput(ctx context.Context, state *AppState, client *oairt.Client) error {
	if state.AudioEngine == nil {
		return nil
	}

	err := state.AudioEngine.StartRecording(ctx, 24000, func(data []byte) {
		_ = client.SendAudio(data)
	})
	if err != nil {
		return fmt.Errorf("failed to start recording: %w", err)
	}
	logInfo("Audio recording started")
	return nil
}

func applyInitialSession(client *oairt.Client, config *Config) error {
	sess := &oairt.Session{}
	dirty := false

	if config.InitialVoice != "" {
		sess.Voice = config.InitialVoice
		dirty = true
	}
	if config.InitialInstructions != "" {
		sess.Instructions = config.InitialInstructions
		dirty = true
	}
	if config.Modalities != "" {
		parts := strings.Split(config.Modalities, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			sess.Modalities = out
			dirty = true
		}
	}
	if config.InputAudioFormat != "" {
		sess.InputAudioFormat = config.InputAudioFormat
		dirty = true
	}
	if config.OutputAudioFormat != "" {
		sess.OutputAudioFormat = config.OutputAudioFormat
		dirty = true
	}
	if config.TemperatureSet {
		sess.Temperature = config.Temperature
		dirty = true
	}
	if config.ToolChoice != "" {
		sess.ToolChoice = config.ToolChoice
		dirty = true
	}
	if config.ReasoningEffort != "" {
		sess.Reasoning = &oairt.Reasoning{Effort: config.ReasoningEffort}
		dirty = true
	}
	if config.VADType != "" || config.VADThresholdSet || config.VADPrefixPadding != 0 || config.VADSilenceMs != 0 {
		td := &oairt.TurnDetection{
			Type:              config.VADType,
			PrefixPaddingMs:   config.VADPrefixPadding,
			SilenceDurationMs: config.VADSilenceMs,
		}
		if config.VADThresholdSet {
			td.Threshold = config.VADThreshold
		}
		sess.TurnDetection = td
		dirty = true
	}
	if config.TranscriptionEnabledSet || config.TranscriptionModel != "" || config.TranscriptionLanguage != "" || config.TranscriptionPrompt != "" || config.TranscriptionDelay != "" {
		sess.InputAudioTranscription = &oairt.AudioTranscription{
			Enabled:  config.TranscriptionEnabled,
			Model:    config.TranscriptionModel,
			Language: config.TranscriptionLanguage,
			Prompt:   config.TranscriptionPrompt,
			Delay:    config.TranscriptionDelay,
		}
		dirty = true
	}

	if !dirty {
		return nil
	}

	evt := oairt.Event{
		Type:    "session.update",
		EventID: generateID("evt_"),
		Session: sess,
	}
	if err := client.Send(evt); err != nil {
		return fmt.Errorf("send session.update: %w", err)
	}
	logInfo("Sent initial session.update with CLI overrides")
	return nil
}

func setupRealtimeClient(ctx context.Context, config *Config, state *AppState) (*oairt.Client, error) {
	opts := []oairt.Option{
		oairt.WithDebug(state.DebugLevel > 0),
		oairt.WithDumpFrames(state.DebugLevel > 1),
	}
	if config.URL != "" {
		opts = append(opts, oairt.WithURL(config.URL))
	}
	client := oairt.NewClient(config.APIKey, opts...)

	client.On("*", func(event oairt.Event) {
		handleEvent(ctx, state, event)
	})

	if err := client.Connect(ctx, config.ModelName); err != nil {
		return nil, fmt.Errorf("error connecting: %w", err)
	}

	return client, nil
}
