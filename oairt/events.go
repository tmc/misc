package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

func handleEvent(ctx context.Context, state *AppState, event Event) {
	switch event.Type {
	case "session.created", "session.update":
		if event.Session != nil {
			logDebug("Session event received",
				zap.String("type", event.Type),
				zap.String("id", event.Session.ID),
				zap.String("model", event.Session.Model),
				zap.Strings("modalities", event.Session.Modalities),
				zap.String("instructions", event.Session.Instructions),
				zap.String("inputAudioFormat", event.Session.InputAudioFormat),
				zap.String("outputAudioFormat", event.Session.OutputAudioFormat),
				zap.String("voice", event.Session.Voice),
				zap.Strings("availableVoices", event.Session.AvailableVoices),
			)
			if event.Session.InputAudioTranscription != nil {
				logDebug("Input Audio Transcription",
					zap.Bool("enabled", event.Session.InputAudioTranscription.Enabled),
					zap.String("model", event.Session.InputAudioTranscription.Model),
				)
			}
			logVerbose("Full session data", zap.Any("session", event.Session))

			// Update the AppState with the latest session information
			state.Session = event.Session

			updateAudioParams(ctx, state, event.Session)
		} else {
			logDebug("Session event received but session data is missing", zap.String("type", event.Type))
		}

	case "response.audio.delta":
		delta, ok := event.Delta.(string)
		if !ok {
			logError("Invalid audio delta type", fmt.Errorf("expected string, got %T", event.Delta))
			return
		}
		data, err := base64.StdEncoding.DecodeString(delta)
		if err != nil {
			logError("Error decoding audio data", err)
			return
		}

		state.AudioMutex.Lock()
		defer state.AudioMutex.Unlock()

		if state.AudioFile != nil {
			if _, err := state.AudioFile.Write(data); err != nil {
				logError("Error writing to audio file", err)
			}
			state.AudioFile.Sync()
		}

		if state.AudioOutput != nil {
			_, err := state.AudioOutput.Write(data)
			if err != nil {
				logError("Error writing to audio output", err)
			}
		} else {
			logDebug("AudioOutput is nil, skipping audio playback")
		}

		logDebug("Received audio delta", zap.Int("bytes", len(data)))

	case "response.audio.done":
		logDebug("Audio response completed")

	case "response.audio_transcript.delta":
		if delta, ok := event.Delta.(string); ok {
			fmt.Print(delta) // Print transcript in default color
		}

	case "response.audio_transcript.done":
		fmt.Println() // New line after transcript is complete

	case "error":
		errorData, _ := json.Marshal(event)
		logError("Error event received", fmt.Errorf("%s", string(errorData)))

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
		logDebug("Conversation item created", zap.Any("event", event))

	default:
		logDebug("Unhandled event", zap.String("type", event.Type), zap.Any("data", event))
	}
}

func updateAudioParams(ctx context.Context, state *AppState, newSession *Session) error {
	if newSession == nil {
		logInfo("No session data provided. Skipping audio params update.")
		return nil
	}

	state.Session = newSession

	oldSampleRate := state.ActualSampleRate
	switch state.Session.OutputAudioFormat {
	case "pcm16":
		state.ActualSampleRate = 24000 // or whatever the correct rate is
	default:
		logDebug("Unknown audio format. Using default sample rate.", zap.String("format", state.Session.OutputAudioFormat))
		state.ActualSampleRate = state.DefaultSampleRate
	}

	if oldSampleRate != state.ActualSampleRate {
		logDebug("Sample rate changed",
			zap.Int("oldRate", oldSampleRate),
			zap.Int("newRate", state.ActualSampleRate),
		)
	}
	return nil
}
