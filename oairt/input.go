package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

func readStdin(ctx context.Context, client *RealtimeClient) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()

		if input == "/voice" {
			if client.state.Session == nil || len(client.state.Session.AvailableVoices) == 0 {
				fmt.Println("Voice information not available. Please try again later.")
				continue
			}
			fmt.Printf("Enter new voice (available voices: %s):\n", strings.Join(client.state.Session.AvailableVoices, ", "))
			scanner.Scan()
			newVoice := scanner.Text()
			updateVoice(client, newVoice)
			continue
		}

		if input == "/instructions" {
			fmt.Println("Enter new instructions:")
			scanner.Scan()
			newInstructions := scanner.Text()
			updateInstructions(client, newInstructions)
			continue
		}

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

		err := client.Send(event)
		if err != nil {
			logDebug(client.state, "Error sending message: %v", err)
			continue
		}

		responseEvent := Event{
			Type:    "response.create",
			EventID: generateID("evt_"),
		}
		err = client.Send(responseEvent)
		if err != nil {
			logDebug(client.state, "Error sending response creation message: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		logDebug(client.state, "Error reading from stdin: %v", err)
	}
}

func updateVoice(client *RealtimeClient, newVoice string) {
	if client.state.Session == nil {
		logDebug(client.state, "No active session. Cannot update voice.")
		return
	}

	availableVoices := client.state.Session.AvailableVoices
	if len(availableVoices) == 0 {
		logDebug(client.state, "No available voices information. Cannot update voice.")
		return
	}

	for _, v := range availableVoices {
		if newVoice == v {
			updateSession(client, newVoice, "")
			return
		}
	}

	logDebug(client.state, "Invalid voice: %s. Supported values are: %s", newVoice, strings.Join(availableVoices, ", "))
}

func updateInstructions(client *RealtimeClient, newInstructions string) {
	if client.state.Session == nil {
		logDebug(client.state, "No active session. Cannot update instructions.")
		return
	}

	updateSession(client, "", newInstructions)
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
