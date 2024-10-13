package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/fatih/color"
)

func handleEvent(ctx context.Context, state *AppState, event Event) {
	grey := color.New(color.FgHiBlack)
	switch event.Type {
	case "session.created", "session.update":
		if event.Session != nil {
			logDebug(state, "%s: ID=%s, Model=%s", event.Type, event.Session.ID, event.Session.Model)
			logDebug(state, "Modalities: %v", event.Session.Modalities)
			logDebug(state, "Instructions: %s", event.Session.Instructions)
			logDebug(state, "Audio formats: Input=%s, Output=%s", event.Session.InputAudioFormat, event.Session.OutputAudioFormat)
			logDebug(state, "Voice: %s", event.Session.Voice)
			logDebug(state, "Available Voices: %v", event.Session.AvailableVoices)
			if event.Session.InputAudioTranscription != nil {
				logDebug(state, "Input Audio Transcription: Enabled=%v, Model=%s",
					event.Session.InputAudioTranscription.Enabled,
					event.Session.InputAudioTranscription.Model)
			}
			logVerbose(state, "Full session data: %+v", event.Session)

			// Update the AppState with the latest session information
			state.Session = event.Session

			updateAudioParams(ctx, state, event.Session)
		} else {
			logDebug(state, "%s event received, but session data is missing", event.Type)
		}

	case "response.audio.delta":
		delta, ok := event.Delta.(string)
		if !ok {
			logDebug(state, "Error: expected string for audio delta, got %T", event.Delta)
			return
		}
		data, err := base64.StdEncoding.DecodeString(delta)
		if err != nil {
			logDebug(state, "Error decoding audio data: %v", err)
			return
		}

		state.AudioMutex.Lock()
		defer state.AudioMutex.Unlock()

		if state.AudioFile != nil {
			n, err := state.AudioFile.Write(data)
			if err != nil {
				logDebug(state, "Error writing to audio file: %v", err)
			} else {
				logVerbose(state, "Wrote %d bytes to audio file", n)
			}
			state.AudioFile.Sync()
		}

		if state.AudioPipe != nil {
			n, err := state.AudioPipe.Write(data)
			if err != nil {
				if err == io.ErrClosedPipe {
					logDebug(state, "Audio pipe is closed. Attempting to restart audio playback.")
					err := restartAudioPlayback(ctx, state)
					if err != nil {
						logError(state, "Failed to restart audio playback: %v", err)
					}
				} else {
					logDebug(state, "Error writing to audio pipe: %v", err)
				}
			} else {
				logVerbose(state, "Wrote %d bytes to audio pipe", n)
			}
		}

		logVerbose(state, "Received audio delta: %d bytes", len(data))
		if state.DebugLevel > 1 {
			logVerbose(state, "Audio data (base64): %s", delta)
		}

	case "response.audio.done":
		logDebug(state, "Audio response completed")

	case "response.audio_transcript.delta":
		if delta, ok := event.Delta.(string); ok {
			fmt.Print(delta) // Print transcript in default color
		}

	case "response.audio_transcript.done":
		fmt.Println() // New line after transcript is complete

	case "error":
		errorData, _ := json.Marshal(event)
		color.Red("Error: %s", string(errorData))

	case "conversation.item.created":
		if event.Item != nil {
			if content, ok := event.Item["content"].([]interface{}); ok && len(content) > 0 {
				if textContent, ok := content[0].(map[string]interface{}); ok {
					if text, ok := textContent["text"].(string); ok && text != "" {
						fmt.Printf("User: %s\n", text) // Print user input in default color
						return
					}
				}
			}
		}
		grey.Printf("Event: %s - Data: %s\n", event.Type, mustMarshal(event))

	default:
		grey.Printf("Event: %s - Data: %s\n", event.Type, mustMarshal(event))
	}
}
