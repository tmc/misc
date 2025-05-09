package main

import (
	"io"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

type AppState struct {
	Session           *Session
	AudioOutputFile   string
	AudioFile         *os.File
	AudioOutput       io.WriteCloser
	AudioMutex        sync.Mutex
	DebugLevel        int
	DefaultSampleRate int
	DefaultBitDepth   int
	DefaultChannels   int
	ActualSampleRate  int
}

// AudioWriteCloser extends io.WriteCloser with additional methods
type AudioWriteCloser interface {
	io.WriteCloser
	IsClosed() bool
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
	Modalities              []string            `json:"modalities,omitempty"`
	Instructions            string              `json:"instructions,omitempty"`
	Voice                   string              `json:"voice,omitempty"`
	AvailableVoices         []string            `json:"available_voices,omitempty"`
	InputAudioFormat        string              `json:"input_audio_format,omitempty"`
	OutputAudioFormat       string              `json:"output_audio_format,omitempty"`
	InputAudioTranscription *AudioTranscription `json:"input_audio_transcription,omitempty"`
	TurnDetection           TurnDetection       `json:"turn_detection,omitempty"`
	Tools                   []Tool              `json:"tools,omitempty"`
	ToolChoice              string              `json:"tool_choice,omitempty"`
	Temperature             float64             `json:"temperature,omitempty"`
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
