package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	oairt "github.com/tmc/misc/oairt"
	"go.uber.org/zap"
)

type InputHandler struct {
	client *oairt.Client
	state  *AppState
}

func NewInputHandler(client *oairt.Client, state *AppState) *InputHandler {
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
	sess := h.state.CurrentSession()
	if sess == nil || len(sess.AvailableVoices) == 0 {
		logInfo("Voice information not available. Please try again later.")
		return nil
	}

	availableVoices := strings.Join(sess.AvailableVoices, ", ")
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
	sess := h.state.CurrentSession()
	if sess == nil {
		return fmt.Errorf("no active session")
	}
	if len(sess.AvailableVoices) == 0 {
		return fmt.Errorf("no available voices information")
	}
	for _, v := range sess.AvailableVoices {
		if newVoice == v {
			return h.updateSession(newVoice, "")
		}
	}
	return fmt.Errorf("invalid voice: %s. Supported values are: %s", newVoice, strings.Join(sess.AvailableVoices, ", "))
}

func (h *InputHandler) updateInstructions(newInstructions string) error {
	if h.state.CurrentSession() == nil {
		return fmt.Errorf("no active session")
	}
	return h.updateSession("", newInstructions)
}

func (h *InputHandler) updateSession(voice, instructions string) error {
	sess := h.state.CurrentSession()
	if sess == nil {
		return fmt.Errorf("no active session")
	}

	updateEvent := oairt.Event{
		Type:    "session.update",
		EventID: generateID("evt_"),
		Session: &oairt.Session{
			Voice:                   voice,
			Instructions:            instructions,
			Modalities:              sess.Modalities,
			InputAudioFormat:        sess.InputAudioFormat,
			OutputAudioFormat:       sess.OutputAudioFormat,
			InputAudioTranscription: sess.InputAudioTranscription,
			TurnDetection:           sess.TurnDetection,
			Tools:                   []oairt.Tool{},
			ToolChoice:              sess.ToolChoice,
			Temperature:             sess.Temperature,
		},
	}

	logDebug("Sending session update event", zap.Any("event", updateEvent))
	if err := h.client.Send(updateEvent); err != nil {
		return fmt.Errorf("error sending session update: %w", err)
	}

	logInfo("Sent request to update session")
	return nil
}

func (h *InputHandler) sendUserMessage(input string) error {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	event := oairt.Event{
		Type:    oairt.EventConversationItemCreate,
		EventID: generateID("evt_"),
		Item: &oairt.Item{
			Type: "message",
			Role: "user",
			Content: []oairt.ItemContent{
				{Type: "input_text", Text: input},
			},
		},
	}

	if err := h.client.Send(event); err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	responseEvent := oairt.Event{
		Type:    "response.create",
		EventID: generateID("evt_"),
	}
	if err := h.client.Send(responseEvent); err != nil {
		return fmt.Errorf("error sending response creation message: %w", err)
	}

	return nil
}
