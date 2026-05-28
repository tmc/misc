package oairt

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Client is a connection to the OpenAI Realtime API.
//
// The zero value is not usable; create a Client with NewClient.
type Client struct {
	URL    string
	APIKey string

	conn       *websocket.Conn
	send       chan []byte
	handlers   map[string][]func(Event)
	mu         sync.Mutex
	debug      bool
	dumpFrames bool
	logger     Logger
	userAgent  string
	httpClient *http.Client      // optional, used for proxy/transport hints
	dialer     *websocket.Dialer // optional, overrides the default dialer
	dispatchWG sync.WaitGroup    // tracks per-handler dispatch goroutines
	closeOnce  sync.Once
	closed     chan struct{} // closed by Close to signal teardown
}

// Event is the wire-level envelope for a Realtime API message in either
// direction. Loosely-typed payload bytes live in Raw; helper accessors decode
// the per-type payload.
type Event struct {
	Type           string          `json:"type"`
	EventID        string          `json:"event_id,omitempty"`
	ResponseID     string          `json:"response_id,omitempty"`
	ItemID         string          `json:"item_id,omitempty"`
	PreviousItemID string          `json:"previous_item_id,omitempty"`
	OutputIndex    int             `json:"output_index,omitempty"`
	ContentIndex   int             `json:"content_index,omitempty"`
	CallID         string          `json:"call_id,omitempty"`
	Name           string          `json:"name,omitempty"`
	Arguments      string          `json:"arguments,omitempty"`
	Audio          string          `json:"audio,omitempty"`
	Session        *Session        `json:"session,omitempty"`
	Item           *Item           `json:"item,omitempty"`
	Error          *APIError       `json:"error,omitempty"`
	Delta          json.RawMessage `json:"delta,omitempty"`

	// Raw holds any fields not otherwise represented above; populated by
	// UnmarshalJSON when a server-sent event carries unrecognized keys.
	Raw json.RawMessage `json:"-"`
}

// AudioDelta returns the base64-encoded audio payload from a
// "response.audio.delta" event.
func (e Event) AudioDelta() (string, bool) {
	if len(e.Delta) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(e.Delta, &s); err != nil {
		return "", false
	}
	return s, true
}

// TextDelta returns the text payload from a "response.text.delta" or
// "response.audio_transcript.delta" event.
func (e Event) TextDelta() (string, bool) { return e.AudioDelta() }

// Session describes a Realtime session as reported by the server.
type Session struct {
	ID                      string              `json:"id,omitempty"`
	Type                    string              `json:"type,omitempty"`
	Object                  string              `json:"object,omitempty"`
	Model                   string              `json:"model,omitempty"`
	Modalities              []string            `json:"modalities,omitempty"`
	OutputModalities        []string            `json:"output_modalities,omitempty"`
	Instructions            string              `json:"instructions,omitempty"`
	Voice                   string              `json:"voice,omitempty"`
	AvailableVoices         []string            `json:"available_voices,omitempty"`
	InputAudioFormat        string              `json:"input_audio_format,omitempty"`
	OutputAudioFormat       string              `json:"output_audio_format,omitempty"`
	Audio                   *AudioConfig        `json:"audio,omitempty"`
	InputAudioTranscription *AudioTranscription `json:"input_audio_transcription,omitempty"`
	TurnDetection           *TurnDetection      `json:"turn_detection,omitempty"`
	Tools                   []Tool              `json:"tools,omitempty"`
	ToolChoice              string              `json:"tool_choice,omitempty"`
	Temperature             float64             `json:"temperature,omitempty"`
	Reasoning               *Reasoning          `json:"reasoning,omitempty"`
}

// Reasoning configures reasoning-capable Realtime models such as
// gpt-realtime-2. Effort accepts the documented enum values
// "minimal", "low", "medium", "high", or "xhigh".
type Reasoning struct {
	Effort string `json:"effort,omitempty"`
}

// AudioConfig configures the current Realtime audio session shape.
type AudioConfig struct {
	Input  *AudioInputConfig  `json:"input,omitempty"`
	Output *AudioOutputConfig `json:"output,omitempty"`
}

// AudioInputConfig configures input audio and turn detection.
type AudioInputConfig struct {
	Format        *AudioFormat   `json:"format,omitempty"`
	TurnDetection *TurnDetection `json:"turn_detection,omitempty"`
}

// AudioOutputConfig configures model audio output.
type AudioOutputConfig struct {
	Format *AudioFormat `json:"format,omitempty"`
	Voice  string       `json:"voice,omitempty"`
}

// AudioFormat describes Realtime audio encoding.
type AudioFormat struct {
	Type string `json:"type,omitempty"`
	Rate int    `json:"rate,omitempty"`
}

// AudioTranscription configures input transcription on a session.
//
// Language is a BCP-47 tag ("en", "es-MX", ...). Delay is a string enum
// ("minimal" | "low" | "medium" | "high" | "auto") that trades latency
// for accuracy; only honored by gpt-realtime-whisper. Prompt seeds the
// transcription model with style or vocabulary guidance.
type AudioTranscription struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Model    string `json:"model,omitempty"`
	Language string `json:"language,omitempty"`
	Delay    string `json:"delay,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

// TurnDetection configures voice-activity-driven turn detection.
type TurnDetection struct {
	Type              string  `json:"type,omitempty"`
	Threshold         float64 `json:"threshold,omitempty"`
	PrefixPaddingMs   int     `json:"prefix_padding_ms,omitempty"`
	SilenceDurationMs int     `json:"silence_duration_ms,omitempty"`
	CreateResponse    *bool   `json:"create_response,omitempty"`
	InterruptResponse *bool   `json:"interrupt_response,omitempty"`
}

// Tool advertises a function the model may call.
type Tool struct {
	Type        string         `json:"type,omitempty"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  ToolParameters `json:"parameters,omitempty"`
}

// ToolParameters describes a tool's JSON-Schema input.
type ToolParameters struct {
	Type       string                     `json:"type,omitempty"`
	Properties map[string]json.RawMessage `json:"properties,omitempty"`
	Required   []string                   `json:"required,omitempty"`
	Raw        json.RawMessage            `json:"-"`
}

// MarshalJSON preserves raw JSON Schema payloads when ToolParameters came from
// an external schema source such as MCP tools/list.
func (p ToolParameters) MarshalJSON() ([]byte, error) {
	if len(p.Raw) > 0 {
		return p.Raw, nil
	}
	type noMethods ToolParameters
	return json.Marshal(noMethods(p))
}

// Item is a conversation item (a message, function call, function output, ...).
type Item struct {
	ID                string         `json:"id,omitempty"`
	Object            string         `json:"object,omitempty"`
	Type              string         `json:"type,omitempty"`
	Status            string         `json:"status,omitempty"`
	Role              string         `json:"role,omitempty"`
	Content           []ItemContent  `json:"content,omitempty"`
	CallID            string         `json:"call_id,omitempty"`
	Name              string         `json:"name,omitempty"`
	Args              string         `json:"arguments,omitempty"`
	Output            string         `json:"output,omitempty"`
	ApprovalRequestID string         `json:"approval_request_id,omitempty"`
	Approve           *bool          `json:"approve,omitempty"`
	ServerLabel       string         `json:"server_label,omitempty"`
	Tools             []Tool         `json:"tools,omitempty"`
	Extra             map[string]any `json:"-"`
}

// ItemContent is one element of an Item's content array.
type ItemContent struct {
	Type       string `json:"type,omitempty"`
	Text       string `json:"text,omitempty"`
	Audio      string `json:"audio,omitempty"`
	Transcript string `json:"transcript,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
}

const (
	ItemTypeMessage             = "message"
	ItemTypeFunctionCall        = "function_call"
	ItemTypeFunctionCallOutput  = "function_call_output"
	ItemTypeMCPCall             = "mcp_call"
	ItemTypeMCPListTools        = "mcp_list_tools"
	ItemTypeMCPApprovalRequest  = "mcp_approval_request"
	ItemTypeMCPApprovalResponse = "mcp_approval_response"
	ContentTypeInputText        = "input_text"
	ContentTypeInputImage       = "input_image"
	ContentTypeInputAudio       = "input_audio"
	ContentTypeOutputText       = "output_text"
	ContentTypeOutputAudio      = "output_audio"
	ContentTypeAudioTranscript  = "audio_transcript"
)

// FunctionCall is a complete Realtime function tool call.
type FunctionCall struct {
	ID          string
	CallID      string
	Name        string
	Arguments   string
	ResponseID  string
	ItemID      string
	OutputIndex int
}

// FunctionCall returns the complete function call carried by e, if any.
func (e Event) FunctionCall() (FunctionCall, bool) {
	call := FunctionCall{
		ResponseID:  e.ResponseID,
		ItemID:      e.ItemID,
		OutputIndex: e.OutputIndex,
		CallID:      e.CallID,
		Name:        e.Name,
		Arguments:   e.Arguments,
	}
	if e.Item != nil {
		if e.Item.Type != "" && e.Item.Type != ItemTypeFunctionCall {
			return FunctionCall{}, false
		}
		call.ID = e.Item.ID
		if call.ItemID == "" {
			call.ItemID = e.Item.ID
		}
		if call.CallID == "" {
			call.CallID = e.Item.CallID
		}
		if call.Name == "" {
			call.Name = e.Item.Name
		}
		if call.Arguments == "" {
			call.Arguments = e.Item.Args
		}
	}
	if call.Arguments == "" {
		call.Arguments = "{}"
	}
	return call, call.CallID != "" && call.Name != ""
}

// ArgumentsObject decodes the call's JSON arguments.
func (c FunctionCall) ArgumentsObject() (map[string]any, error) {
	args := make(map[string]any)
	dec := json.NewDecoder(strings.NewReader(c.Arguments))
	dec.UseNumber()
	if err := dec.Decode(&args); err != nil {
		return nil, err
	}
	return args, nil
}

// FunctionCallOutput creates the conversation item event that answers a
// Realtime function call.
func FunctionCallOutput(callID string, output any) (Event, error) {
	var text string
	switch v := output.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	case json.RawMessage:
		text = string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return Event{}, err
		}
		text = string(data)
	}
	return Event{
		Type: EventConversationItemCreate,
		Item: &Item{
			Type:   ItemTypeFunctionCallOutput,
			CallID: callID,
			Output: text,
		},
	}, nil
}

// MCPApprovalResponse creates the conversation item event that approves or
// rejects a hosted MCP approval request.
func MCPApprovalResponse(approvalRequestID string, approve bool) Event {
	return Event{
		Type: EventConversationItemCreate,
		Item: &Item{
			ID:                "mcp_approval_" + approvalRequestID,
			Type:              ItemTypeMCPApprovalResponse,
			ApprovalRequestID: approvalRequestID,
			Approve:           &approve,
		},
	}
}

// APIError carries the body of an "error" event from the server.
type APIError struct {
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Param   string `json:"param,omitempty"`
	EventID string `json:"event_id,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code != "" {
		return e.Type + ": " + e.Code + ": " + e.Message
	}
	return e.Type + ": " + e.Message
}

// Logger is the minimal logging interface used by the library. The default is
// a no-op. A future option (WithLogger) will allow callers to wire in a real
// logger; until then the library emits no log output.
type Logger interface {
	Debugf(format string, args ...any)
	Errorf(format string, args ...any)
}

type nopLogger struct{}

func (nopLogger) Debugf(string, ...any) {}
func (nopLogger) Errorf(string, ...any) {}

// Event types observed on the OpenAI Realtime websocket.
//
// Client → Server:
const (
	EventSessionUpdate            = "session.update"
	EventInputAudioBufferAppend   = "input_audio_buffer.append"
	EventInputAudioBufferCommit   = "input_audio_buffer.commit"
	EventInputAudioBufferClear    = "input_audio_buffer.clear"
	EventConversationItemCreate   = "conversation.item.create"
	EventConversationItemTruncate = "conversation.item.truncate"
	EventConversationItemDelete   = "conversation.item.delete"
	EventResponseCreate           = "response.create"
	EventResponseCancel           = "response.cancel"
)

// Server → Client:
const (
	EventError                                        = "error"
	EventSessionCreated                               = "session.created"
	EventSessionUpdated                               = "session.updated"
	EventConversationCreated                          = "conversation.created"
	EventConversationItemCreated                      = "conversation.item.created"
	EventConversationItemAdded                        = "conversation.item.added"
	EventConversationItemDone                         = "conversation.item.done"
	EventConversationItemInputAudioTranscriptionDelta = "conversation.item.input_audio_transcription.delta"
	EventConversationItemInputAudioTranscriptionDone  = "conversation.item.input_audio_transcription.completed"
	EventInputAudioBufferCommitted                    = "input_audio_buffer.committed"
	EventInputAudioBufferCleared                      = "input_audio_buffer.cleared"
	EventInputAudioBufferSpeechStarted                = "input_audio_buffer.speech_started"
	EventInputAudioBufferSpeechStopped                = "input_audio_buffer.speech_stopped"
	EventResponseCreated                              = "response.created"
	EventResponseDone                                 = "response.done"
	EventResponseOutputItemAdded                      = "response.output_item.added"
	EventResponseOutputItemDone                       = "response.output_item.done"
	EventResponseContentPartAdded                     = "response.content_part.added"
	EventResponseContentPartDone                      = "response.content_part.done"
	EventResponseTextDelta                            = "response.text.delta"
	EventResponseTextDone                             = "response.text.done"
	EventResponseOutputTextDelta                      = "response.output_text.delta"
	EventResponseOutputTextDone                       = "response.output_text.done"
	EventResponseAudioDelta                           = "response.audio.delta"
	EventResponseAudioDone                            = "response.audio.done"
	EventResponseOutputAudioDelta                     = "response.output_audio.delta"
	EventResponseOutputAudioDone                      = "response.output_audio.done"
	EventResponseAudioTranscriptDelta                 = "response.audio_transcript.delta"
	EventResponseAudioTranscriptDone                  = "response.audio_transcript.done"
	EventResponseOutputAudioTranscriptDelta           = "response.output_audio_transcript.delta"
	EventResponseOutputAudioTranscriptDone            = "response.output_audio_transcript.done"
	EventResponseFunctionCallArgumentsDelta           = "response.function_call_arguments.delta"
	EventResponseFunctionCallArgumentsDone            = "response.function_call_arguments.done"
	EventMCPListToolsCompleted                        = "mcp_list_tools.completed"
	EventMCPListToolsFailed                           = "mcp_list_tools.failed"
	EventResponseMCPCallArgumentsDone                 = "response.mcp_call_arguments.done"
	EventResponseMCPCallInProgress                    = "response.mcp_call.in_progress"
	EventResponseMCPCallFailed                        = "response.mcp_call.failed"
	EventRateLimitsUpdated                            = "rate_limits.updated"
)
