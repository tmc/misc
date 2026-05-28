package oairt

import (
	"encoding/json"
	"strings"
	"testing"
)

// eventCorpus is the canonical set of events oairt round-trips and
// snapshots. Tests share this single source so the golden files and the
// in-memory round-trip cannot drift apart.
func eventCorpus() []struct {
	name string
	in   Event
} {
	return []struct {
		name string
		in   Event
	}{
		{"session.update", Event{Type: EventSessionUpdate, EventID: "evt_1", Session: &Session{Voice: "alloy"}}},
		{"session.created", Event{Type: EventSessionCreated, EventID: "evt_2", Session: &Session{ID: "sess_a", Model: "gpt-realtime", Modalities: []string{"text", "audio"}}}},
		{"session.updated", Event{Type: EventSessionUpdated, Session: &Session{Voice: "echo"}}},
		{"input_audio_buffer.append", Event{Type: EventInputAudioBufferAppend, Audio: "AAAA"}},
		{"input_audio_buffer.commit", Event{Type: EventInputAudioBufferCommit, EventID: "evt_3"}},
		{"input_audio_buffer.clear", Event{Type: EventInputAudioBufferClear}},
		{"input_audio_buffer.committed", Event{Type: EventInputAudioBufferCommitted, ItemID: "item_a"}},
		{"input_audio_buffer.cleared", Event{Type: EventInputAudioBufferCleared}},
		{"input_audio_buffer.speech_started", Event{Type: EventInputAudioBufferSpeechStarted, ItemID: "item_b"}},
		{"input_audio_buffer.speech_stopped", Event{Type: EventInputAudioBufferSpeechStopped, ItemID: "item_b"}},
		{"conversation.created", Event{Type: EventConversationCreated}},
		{"conversation.item.create", Event{Type: EventConversationItemCreate, Item: &Item{Type: "message", Role: "user", Content: []ItemContent{{Type: "input_text", Text: "hi"}}}}},
		{"conversation.item.created", Event{Type: EventConversationItemCreated, Item: &Item{ID: "item_1", Type: "message", Role: "user", Content: []ItemContent{{Type: "input_text", Text: "hello"}}}}},
		{"conversation.item.truncate", Event{Type: EventConversationItemTruncate, ItemID: "item_2"}},
		{"conversation.item.delete", Event{Type: EventConversationItemDelete, ItemID: "item_3"}},
		{"response.create", Event{Type: EventResponseCreate, EventID: "evt_4"}},
		{"response.cancel", Event{Type: EventResponseCancel, ResponseID: "resp_1"}},
		{"response.created", Event{Type: EventResponseCreated, ResponseID: "resp_1"}},
		{"response.done", Event{Type: EventResponseDone, ResponseID: "resp_1"}},
		{"response.output_item.added", Event{Type: EventResponseOutputItemAdded, ResponseID: "resp_1", OutputIndex: 0, Item: &Item{Type: "message", Role: "assistant"}}},
		{"response.output_item.done", Event{Type: EventResponseOutputItemDone, ResponseID: "resp_1", OutputIndex: 0}},
		{"response.content_part.added", Event{Type: EventResponseContentPartAdded, ResponseID: "resp_1", ContentIndex: 0}},
		{"response.content_part.done", Event{Type: EventResponseContentPartDone, ResponseID: "resp_1", ContentIndex: 0}},
		{"response.text.delta", Event{Type: EventResponseTextDelta, ResponseID: "resp_1", Delta: json.RawMessage(`"hello"`)}},
		{"response.text.done", Event{Type: EventResponseTextDone, ResponseID: "resp_1"}},
		{"response.audio.delta", Event{Type: EventResponseAudioDelta, ResponseID: "resp_1", Delta: json.RawMessage(`"AAAA"`)}},
		{"response.audio.done", Event{Type: EventResponseAudioDone, ResponseID: "resp_1"}},
		{"response.audio_transcript.delta", Event{Type: EventResponseAudioTranscriptDelta, ResponseID: "resp_1", Delta: json.RawMessage(`"the cat"`)}},
		{"response.audio_transcript.done", Event{Type: EventResponseAudioTranscriptDone, ResponseID: "resp_1"}},
		{"response.function_call_arguments.delta", Event{Type: EventResponseFunctionCallArgumentsDelta, ResponseID: "resp_1", Delta: json.RawMessage(`"{\"a\":1}"`)}},
		{"response.function_call_arguments.done", Event{Type: EventResponseFunctionCallArgumentsDone, ResponseID: "resp_1"}},
		{"rate_limits.updated", Event{Type: EventRateLimitsUpdated}},
		{"error", Event{Type: EventError, Error: &APIError{Type: "invalid_request_error", Code: "missing_parameter", Message: "session is required", EventID: "evt_5"}}},
		// gpt-realtime-2 v2 surface (oairt v0.1.1):
		{"conversation.item.added", Event{Type: EventConversationItemAdded, EventID: "evt_6", PreviousItemID: "item_0", Item: &Item{ID: "item_1", Type: "message", Role: "assistant", Status: "in_progress"}}},
		{"conversation.item.done", Event{Type: EventConversationItemDone, EventID: "evt_7", PreviousItemID: "item_0", Item: &Item{ID: "item_1", Type: "message", Role: "assistant", Status: "completed"}}},
		{"conversation.item.input_audio_transcription.delta", Event{Type: EventConversationItemInputAudioTranscriptionDelta, EventID: "evt_8", ItemID: "item_2", ContentIndex: 0, Delta: json.RawMessage(`"hel"`)}},
		{"session.update_v2", Event{Type: EventSessionUpdate, EventID: "evt_9", Session: &Session{
			Reasoning:               &Reasoning{Effort: "medium"},
			InputAudioTranscription: &AudioTranscription{Model: "gpt-4o-transcribe", Language: "en", Delay: "low", Prompt: "vocabulary: ACME, OAuth"},
		}}},
	}
}

// TestEventRoundTrip exercises Marshal → Unmarshal of the canonical events
// the OpenAI Realtime API emits or accepts. Acceptance: every documented
// event Type round-trips and the type discriminator survives.
func TestEventRoundTrip(t *testing.T) {
	tests := eventCorpus()
	if got := len(tests); got < 20 {
		t.Fatalf("test corpus too small: have %d, want >= 20", got)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(data), `"type":"`+tt.in.Type+`"`) {
				t.Fatalf("marshaled JSON missing type discriminator: %s", data)
			}
			var got Event
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Type != tt.in.Type {
				t.Fatalf("type mismatch: got %q, want %q", got.Type, tt.in.Type)
			}
			data2, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("re-marshal: %v", err)
			}
			if string(data) != string(data2) {
				t.Fatalf("re-marshal differs:\n  first: %s\n second: %s", data, data2)
			}
		})
	}
}

// TestEventDelta_AudioAndText covers the typed accessors for delta payloads.
func TestEventDelta_AudioAndText(t *testing.T) {
	e := Event{Type: EventResponseAudioDelta, Delta: json.RawMessage(`"AAAA"`)}
	got, ok := e.AudioDelta()
	if !ok || got != "AAAA" {
		t.Fatalf("AudioDelta: got (%q,%v), want (AAAA,true)", got, ok)
	}
	t2 := Event{Type: EventResponseTextDelta, Delta: json.RawMessage(`"hi"`)}
	gt, ok := t2.TextDelta()
	if !ok || gt != "hi" {
		t.Fatalf("TextDelta: got (%q,%v), want (hi,true)", gt, ok)
	}
	empty := Event{Type: EventResponseAudioDone}
	if _, ok := empty.AudioDelta(); ok {
		t.Fatalf("AudioDelta on empty delta should report false")
	}
}

func TestFunctionCallInterface(t *testing.T) {
	e := Event{
		Type: EventResponseOutputItemDone,
		Item: &Item{
			Type:   ItemTypeFunctionCall,
			ID:     "fc_1",
			CallID: "call_1",
			Name:   "run_input",
			Args:   `{"frames":2}`,
		},
	}
	call, ok := e.FunctionCall()
	if !ok {
		t.Fatal("FunctionCall returned false")
	}
	if call.CallID != "call_1" || call.Name != "run_input" {
		t.Fatalf("call = %#v", call)
	}
	args, err := call.ArgumentsObject()
	if err != nil {
		t.Fatal(err)
	}
	if got := args["frames"].(json.Number).String(); got != "2" {
		t.Fatalf("frames = %q, want 2", got)
	}

	out, err := FunctionCallOutput(call.CallID, map[string]any{"ok": true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Type != EventConversationItemCreate || out.Item == nil || out.Item.Type != ItemTypeFunctionCallOutput {
		t.Fatalf("output event = %#v", out)
	}
	if out.Item.CallID != "call_1" || !strings.Contains(out.Item.Output, `"ok":true`) {
		t.Fatalf("output item = %#v", out.Item)
	}
}

// TestAPIError_Error sanity-checks the error string formatting.
func TestAPIError_Error(t *testing.T) {
	e := &APIError{Type: "t", Code: "c", Message: "m"}
	if got := e.Error(); !strings.Contains(got, "t") || !strings.Contains(got, "c") || !strings.Contains(got, "m") {
		t.Fatalf("APIError.Error() = %q", got)
	}
	var nilErr *APIError
	if nilErr.Error() != "<nil>" {
		t.Fatalf("nil APIError should stringify to <nil>")
	}
}
