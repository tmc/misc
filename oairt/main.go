package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

type AppState struct {
	Session           *Session
	AudioOutputFile   string
	AudioFile         *os.File
	AudioPipe         io.WriteCloser
	AudioCmd          *exec.Cmd
	AudioMutex        sync.Mutex
	DebugLevel        int
	DefaultSampleRate int
	DefaultBitDepth   int
	DefaultChannels   int
}

type RealtimeClient struct {
	URL        string
	APIKey     string
	conn       *websocket.Conn
	send       chan []byte
	handlers   map[string][]func(Event)
	mu         sync.Mutex
	debug      bool
	dumpFrames bool
	state      *AppState
}

type Event struct {
	Type         string                 `json:"type"`
	EventID      string                 `json:"event_id"`
	ResponseID   string                 `json:"response_id,omitempty"`
	ItemID       string                 `json:"item_id,omitempty"`
	OutputIndex  int                    `json:"output_index,omitempty"`
	ContentIndex int                    `json:"content_index,omitempty"`
	Delta        interface{}            `json:"delta,omitempty"`
	Item         map[string]interface{} `json:"item,omitempty"`
	Error        map[string]interface{} `json:"error,omitempty"`
	Session      *Session               `json:"session,omitempty"`
}

type Session struct {
	ID                      string              `json:"id,omitempty"`
	Object                  string              `json:"object,omitempty"`
	Model                   string              `json:"model,omitempty"`
	Modalities              []string            `json:"modalities"`
	Instructions            string              `json:"instructions"`
	Voice                   string              `json:"voice"`
	InputAudioFormat        string              `json:"input_audio_format"`
	OutputAudioFormat       string              `json:"output_audio_format"`
	InputAudioTranscription *AudioTranscription `json:"input_audio_transcription"`
	TurnDetection           TurnDetection       `json:"turn_detection"`
	Tools                   []Tool              `json:"tools"`
	ToolChoice              string              `json:"tool_choice"`
	Temperature             float64             `json:"temperature"`
	MaxOutputTokens         int                 `json:"max_output_tokens,omitempty"`
}

type AudioTranscription struct {
	Enabled bool   `json:"enabled"`
	Model   string `json:"model"`
}

type TurnDetection struct {
	Type              string  `json:"type"`
	Threshold         float64 `json:"threshold"`
	PrefixPaddingMs   int     `json:"prefix_padding_ms"`
	SilenceDurationMs int     `json:"silence_duration_ms"`
}

type Tool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

type ToolParameters struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

func NewRealtimeClient(apiKey string, state *AppState) *RealtimeClient {
	return &RealtimeClient{
		URL:        "wss://api.openai.com/v1/realtime",
		APIKey:     apiKey,
		handlers:   make(map[string][]func(Event)),
		send:       make(chan []byte, 256),
		debug:      state.DebugLevel > 0,
		dumpFrames: state.DebugLevel > 1,
		state:      state,
	}
}

func (c *RealtimeClient) Connect(model string) error {
	u, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("error parsing URL: %v", err)
	}

	if model != "" {
		q := u.Query()
		q.Set("model", model)
		u.RawQuery = q.Encode()
	}

	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+c.APIKey)
	headers.Add("OpenAI-Beta", "realtime=v1")
	headers.Add("User-Agent", "OpenAI-Realtime-Client/1.0")

	c.logf("Connecting to %s", u.String())
	c.logf("Headers: %v", headers)

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
	}

	conn, resp, err := dialer.Dial(u.String(), headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("websocket handshake failed with status %d: %s\nResponse body: %s", resp.StatusCode, err, string(body))
		}
		return fmt.Errorf("error connecting to websocket: %v", err)
	}
	c.conn = conn

	if resp != nil {
		c.logf("Connected with status: %s", resp.Status)
		c.logf("Response headers: %v", resp.Header)
	}

	if c.dumpFrames {
		c.logf("WebSocket handshake request headers:")
		for k, v := range resp.Request.Header {
			c.logf("%s: %s", k, v)
		}
		c.logf("WebSocket handshake response status: %s", resp.Status)
		c.logf("WebSocket handshake response headers:")
		for k, v := range resp.Header {
			c.logf("%s: %s", k, v)
		}
	}

	go c.readPump()
	go c.writePump()

	return nil
}

func (c *RealtimeClient) Disconnect() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *RealtimeClient) Send(event Event) error {
	c.logf("Sending event: %s", mustMarshal(event))
	return c.conn.WriteJSON(event)
}

func (c *RealtimeClient) On(eventType string, handler func(Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[eventType] = append(c.handlers[eventType], handler)
}

func (c *RealtimeClient) readPump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logf("Error reading from websocket: %v", err)
			}
			break
		}

		if c.dumpFrames {
			c.logf("Received raw frame: %s", message)
		}

		var event Event
		if err := json.Unmarshal(message, &event); err != nil {
			c.logf("Error unmarshaling event: %v", err)
			continue
		}

		c.logf("Received event: %s", message)
		c.handleEvent(event)
	}
}

func (c *RealtimeClient) writePump() {
	ticker := time.NewTicker(time.Second * 30)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if c.dumpFrames {
				c.logf("Sending raw frame: %s", message)
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.logf("Error getting next writer: %v", err)
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				c.logf("Error closing writer: %v", err)
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logf("Error writing ping message: %v", err)
				return
			}
		}
	}
}

func (c *RealtimeClient) handleEvent(event Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	handlers := c.handlers[event.Type]
	for _, handler := range handlers {
		go handler(event)
	}

	allHandlers := c.handlers["*"]
	for _, handler := range allHandlers {
		go handler(event)
	}
}

func (c *RealtimeClient) logf(format string, v ...interface{}) {
	if c.debug {
		logDebug(c.state, format, v...)
	}
}

func logDebug(state *AppState, format string, v ...interface{}) {
	if state.DebugLevel > 0 {
		color.Set(color.FgHiCyan)
		log.Printf(format, v...)
		color.Unset()
	}
}

func logVerbose(state *AppState, format string, v ...interface{}) {
	if state.DebugLevel > 1 {
		color.Set(color.FgHiMagenta)
		log.Printf(format, v...)
		color.Unset()
	}
}

func generateID(prefix string) string {
	const charset = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	b := make([]byte, 21-len(prefix))
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return prefix + string(b)
}

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func handleEvent(state *AppState, event Event) {
	grey := color.New(color.FgHiBlack)
	switch event.Type {
	case "session.created", "session.update":
		if event.Session != nil {
			logDebug(state, "%s: ID=%s, Model=%s", event.Type, event.Session.ID, event.Session.Model)
			logDebug(state, "Modalities: %v", event.Session.Modalities)
			logDebug(state, "Instructions: %s", event.Session.Instructions)
			logDebug(state, "Audio formats: Input=%s, Output=%s", event.Session.InputAudioFormat, event.Session.OutputAudioFormat)
			logDebug(state, "Voice: %s", event.Session.Voice)
			if event.Session.InputAudioTranscription != nil {
				logDebug(state, "Input Audio Transcription: Enabled=%v, Model=%s",
					event.Session.InputAudioTranscription.Enabled,
					event.Session.InputAudioTranscription.Model)
			}
			logVerbose(state, "Full session data: %+v", event.Session)

			updateAudioParams(state, event.Session)
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
					go restartAudioPlayback(state)
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

func updateAudioParams(state *AppState, newSession *Session) {
	if newSession == nil {
		return
	}

	state.Session = newSession

	switch state.Session.OutputAudioFormat {
	case "pcm16":
		state.DefaultSampleRate = 16000
	default:
		logDebug(state, "Unknown audio format: %s. Using default sample rate.", state.Session.OutputAudioFormat)
	}

	// if state.AudioPipe != nil && state.AudioCmd != nil {
	// 	fmt.Println("Audio format changed. Restarting audio playback...")
	// 	state.AudioCmd.Process.Kill()
	// 	startFFPlay(state)
	// }
}

func restartAudioPlayback(state *AppState) {
	fmt.Println("Restarting audio playback...")
	state.AudioCmd.Process.Kill()
	if state.AudioCmd != nil && state.AudioCmd.Process != nil {
		state.AudioCmd.Process.Kill()
	}
	startFFPlay(state)
}

func readStdin(client *RealtimeClient) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()

		if input == "/voice" {
			fmt.Println("Enter new voice (e.g., alloy, echo, fable, onyx, nova, shimmer):")
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

func main() {
	state := &AppState{
		DefaultSampleRate: 16000,
		DefaultBitDepth:   16,
		DefaultChannels:   1,
	}

	var apiKey string
	var audioStream bool
	var initialVoice string
	var initialInstructions string

	flag.StringVar(&apiKey, "api-key", "", "OpenAI API Key (overrides OPENAI_API_KEY env variable)")
	flag.StringVar(&state.AudioOutputFile, "audio-output", "", "File to save audio output to")
	flag.BoolVar(&audioStream, "audio-stream", false, "Stream audio in real-time")
	flag.IntVar(&state.DebugLevel, "debug", 0, "Debug level (0=off, 1=debug, 2=verbose)")
	flag.StringVar(&initialVoice, "voice", "", "Initial voice to use (e.g., alloy, echo, fable, onyx, nova, shimmer)")
	flag.StringVar(&initialInstructions, "instructions", "", "Initial instructions for the AI")
	flag.Parse()

	apiKey = os.Getenv("OPENAI_API_KEY")
	if flag.Lookup("api-key").Value.String() != "" {
		apiKey = flag.Lookup("api-key").Value.String()
	}

	if apiKey == "" {
		log.Fatal("API key is required. Set OPENAI_API_KEY environment variable or use -api-key flag.")
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
			log.Fatal("ffplay not found. Please install FFmpeg to use audio streaming.")
		}
		startFFPlay(state)
	}

	client := NewRealtimeClient(apiKey, state)

	client.On("*", func(event Event) {
		handleEvent(state, event)
	})

	err := client.Connect("gpt-4o-realtime-preview-2024-10-01")
	if err != nil {
		log.Fatalf("Error connecting: %v", err)
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
	client.Disconnect()
}

func startFFPlay(state *AppState) {
	logDebug(state, "Starting ffplay...")

	state.AudioCmd = exec.Command("ffplay",
		"-f", "s16le",
		"-i", "pipe:0",
		"-nodisp",
		"-autoexit",
		"-loglevel", "warning") // Changed from "quiet" to "warning" for more info

	var err error
	state.AudioPipe, err = state.AudioCmd.StdinPipe()
	if err != nil {
		logDebug(state, "Failed to create stdin pipe for ffplay: %v", err)
		return
	}

	// Capture stdout and stderr only if debug is enabled
	state.AudioCmd.Stdout = os.Stdout
	state.AudioCmd.Stderr = os.Stderr
	if state.DebugLevel > 0 {
	}

	err = state.AudioCmd.Start()
	if err != nil {
		logDebug(state, "Failed to start ffplay: %v", err)
		state.AudioPipe = nil
		return
	}

	logDebug(state, "ffplay started successfully")

	go func() {
		fmt.Println("waiting")
		err := state.AudioCmd.Wait()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				logDebug(state, "ffplay process ended with error: %v, stderr: %s", err, exitErr.Stderr)
			} else {
				logDebug(state, "ffplay process ended with error: %v", err)
			}
		} else {
			logDebug(state, "ffplay process ended normally")
		}
		state.AudioPipe = nil
	}()
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

	logVerbose(client.state, "Sending session update event: %+v", updateEvent)
	err := client.Send(updateEvent)
	if err != nil {
		logDebug(client.state, "Error sending session update: %v", err)
	} else {
		logDebug(client.state, "Sent request to update session")
	}
}

func updateVoice(client *RealtimeClient, newVoice string) {
	updateSession(client, newVoice, "")
}

func updateInstructions(client *RealtimeClient, newInstructions string) {
	updateSession(client, "", newInstructions)
}
