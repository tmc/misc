package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	oairt "github.com/tmc/misc/oairt"
	"go.uber.org/zap"
)

var audioDropOnce sync.Once

func handleEvent(ctx context.Context, state *AppState, event oairt.Event) {
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

			state.SetSession(event.Session)
			updateAudioParams(ctx, state, event.Session)
		} else {
			logDebug("Session event received but session data is missing", zap.String("type", event.Type))
		}

	case "response.audio.delta":
		delta, ok := event.AudioDelta()
		if !ok {
			logError("Invalid audio delta payload", fmt.Errorf("could not decode delta"))
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
			if _, err := state.AudioOutput.Write(data); err != nil {
				logError("Error writing to audio output", err)
			}
		} else if state.AudioFile == nil {
			audioDropOnce.Do(func() {
				logInfo("Received audio delta but no audio output is configured. " +
					"Pass -audio-stream to hear audio (or -audio-output FILE to save).")
			})
		}

		logDebug("Received audio delta", zap.Int("bytes", len(data)))

	case "response.audio.done":
		logDebug("Audio response completed")

	case "response.audio_transcript.delta":
		if delta, ok := event.TextDelta(); ok {
			fmt.Print(delta)
		}

	case "response.audio_transcript.done":
		fmt.Println()

	case "error":
		errorData, _ := json.Marshal(event)
		logError("Error event received", fmt.Errorf("%s", string(errorData)))

	case "conversation.item.created":
		if event.Item != nil && len(event.Item.Content) > 0 {
			if text := event.Item.Content[0].Text; text != "" {
				fmt.Printf("User: %s\n", text)
				return
			}
		}
		logDebug("Conversation item created", zap.Any("event", event))

	default:
		logDebug("Unhandled event", zap.String("type", event.Type), zap.Any("data", event))
	}
}

func updateAudioParams(_ context.Context, state *AppState, newSession *oairt.Session) error {
	if newSession == nil {
		logInfo("No session data provided. Skipping audio params update.")
		return nil
	}

	oldSampleRate := state.ActualSampleRate
	switch newSession.OutputAudioFormat {
	case "pcm16":
		state.ActualSampleRate = 24000
	default:
		logDebug("Unknown audio format. Using default sample rate.", zap.String("format", newSession.OutputAudioFormat))
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
