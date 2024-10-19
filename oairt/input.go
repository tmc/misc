package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
)

type InputHandler struct {
	client *RealtimeClient
	state  *AppState
}

func NewInputHandler(client *RealtimeClient, state *AppState) *InputHandler {
	return &InputHandler{
		client: client,
		state:  state,
	}
}

func (h *InputHandler) ReadStdin(ctx context.Context) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()

		if err := h.handleInput(ctx, input); err != nil {
			logError("Error handling input", err)
		}
	}

	if err := scanner.Err(); err != nil {
		logError("Error reading from stdin", err)
	}
}

func (h *InputHandler) handleInput(ctx context.Context, input string) error {
	switch {
	case input == "/voice":
		return h.handleVoiceCommand()
	case input == "/instructions":
		return h.handleInstructionsCommand()
	default:
		return h.sendUserMessage(input)
	}
}

func (h *InputHandler) handleVoiceCommand() error {
	if h.client.state.Session == nil || len(h.client.state.Session.AvailableVoices) == 0 {
		logInfo("Voice information not available. Please try again later.")
		return nil
	}

	availableVoices := strings.Join(h.client.state.Session.AvailableVoices, ", ")
	fmt.Printf("Enter new voice (available voices: %s):\n", availableVoices)

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("failed to read new voice input")
	}
	newVoice := scanner.Text()

	return h.updateVoice(newVoice)
}

func (h *InputHandler) handleInstructionsCommand() error {
	fmt.Println("Enter new instructions:")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("failed to read new instructions input")
	}
	newInstructions := scanner.Text()

	return h.updateInstructions(newInstructions)
}

func (h *InputHandler) updateVoice(newVoice string) error {
	if h.client.state.Session == nil {
		return fmt.Errorf("no active session")
	}

	availableVoices := h.client.state.Session.AvailableVoices
	if len(availableVoices) == 0 {
		return fmt.Errorf("no available voices information")
	}

	for _, v := range availableVoices {
		if newVoice == v {
			return h.updateSession(newVoice, "")
		}
	}

	return fmt.Errorf("invalid voice: %s. Supported values are: %s", newVoice, strings.Join(availableVoices, ", "))
}

func (h *InputHandler) updateInstructions(newInstructions string) error {
	if h.client.state.Session == nil {
		return fmt.Errorf("no active session")
	}

	return h.updateSession("", newInstructions)
}

func (h *InputHandler) updateSession(voice, instructions string) error {
	if h.client.state.Session == nil {
		return fmt.Errorf("no active session")
	}

	updateEvent := Event{
		Type:    "session.update",
		EventID: generateID("evt_"),
		Session: &Session{
			Voice:                   voice,
			Instructions:            instructions,
			Modalities:              h.client.state.Session.Modalities,
			InputAudioFormat:        h.client.state.Session.InputAudioFormat,
			OutputAudioFormat:       h.client.state.Session.OutputAudioFormat,
			InputAudioTranscription: h.client.state.Session.InputAudioTranscription,
			TurnDetection:           h.client.state.Session.TurnDetection,
			Tools:                   []Tool{},
			ToolChoice:              h.client.state.Session.ToolChoice,
			Temperature:             h.client.state.Session.Temperature,
		},
	}

	logDebug("Sending session update event", zap.Any("event", updateEvent))
	err := h.client.Send(updateEvent)
	if err != nil {
		return fmt.Errorf("error sending session update: %w", err)
	}

	logInfo("Sent request to update session")
	return nil
}

func (h *InputHandler) sendUserMessage(input string) error {
	event := Event{
		Type:    "conversation.item.create",
		EventID: generateID("evt_"),
		Item: map[string]interface{}{
			"type": "message",
			"role": "user",
			"content": []map[string]string{
				{"type": "input_text", "text": input},
			},
		},
	}

	err := h.client.Send(event)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	responseEvent := Event{
		Type:    "response.create",
		EventID: generateID("evt_"),
	}
	err = h.client.Send(responseEvent)
	if err != nil {
		return fmt.Errorf("error sending response creation message: %w", err)
	}

	return nil
}
