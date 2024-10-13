package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
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
		if err := startFFPlay(state); err != nil {
			return fmt.Errorf("Failed to start ffplay: %w", err)
		}
	}

	client := NewRealtimeClient(apiKey, state)

	client.On("*", func(event Event) {
		handleEvent(state, event)
	})

	if err := client.Connect(modelName); err != nil {
		return fmt.Errorf("Error connecting: %w", err)
	}

	// Apply initial voice and instructions if provided
	if initialVoice != "" || initialInstructions != "" {
		updateSession(client, initialVoice, initialInstructions)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-interrupt
		fmt.Println("\nReceived interrupt signal. Shutting down...")
		client.Disconnect()
		if state.AudioCmd != nil && state.AudioCmd.Process != nil {
			state.AudioCmd.Process.Kill()
		}
		os.Exit(0)
	}()

	readStdin(client)

	time.Sleep(time.Second)
	return client.Disconnect()
}

func updateSession(client *RealtimeClient, voice, instructions string) {
	if client.state.Session == nil {
		logDebug(client.state, "No active session. Cannot update session.")
		return
	}

	updateEvent := Event{
		Type:    "session.update",
		EventID: generateID("evt_"),
		Session: &Session{
			Voice:                   voice,
			Instructions:            instructions,
			Modalities:              client.state.Session.Modalities,
			InputAudioFormat:        client.state.Session.InputAudioFormat,
			OutputAudioFormat:       client.state.Session.OutputAudioFormat,
			InputAudioTranscription: client.state.Session.InputAudioTranscription,
			TurnDetection:           client.state.Session.TurnDetection,
			Tools:                   []Tool{},
			ToolChoice:              client.state.Session.ToolChoice,
			Temperature:             client.state.Session.Temperature,
		},
	}

	if voice != "" {
		isValidVoice := false
		for _, v := range client.state.Session.AvailableVoices {
			if voice == v {
				isValidVoice = true
				break
			}
		}
		if !isValidVoice {
			logDebug(client.state, "Error: Invalid voice. Supported values are: %s", strings.Join(client.state.Session.AvailableVoices, ", "))
			return
		}
	}

	logVerbose(client.state, "Sending session update event: %+v", updateEvent)
	err := client.Send(updateEvent)
	if err != nil {
		logDebug(client.state, "Error sending session update: %v", err)
	} else {
		logDebug(client.state, "Sent request to update session")
	}
}
