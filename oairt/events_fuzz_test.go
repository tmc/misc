package oairt

import (
	"bytes"
	"encoding/json"
	"testing"
	"unicode/utf8"
)

// FuzzParseEvent guards Event JSON decoding against panics and round-trip
// regressions. Seeds are drawn from canonical event shapes the server is
// known to emit plus a handful of adversarial inputs.
//
// Invariants:
//  1. json.Unmarshal into Event must never panic, regardless of input.
//  2. If Unmarshal succeeds, the decoded Event must re-Marshal without error.
//  3. The re-marshaled bytes must Unmarshal again into a value with the same
//     Type discriminator (typed surface stability).
//
// Run:
//
//	go test -run=^$ -fuzz=FuzzParseEvent -fuzztime=30s
func FuzzParseEvent(f *testing.F) {
	for _, e := range fuzzSeedEvents() {
		data, err := json.Marshal(e)
		if err != nil {
			f.Fatalf("seed marshal: %v", err)
		}
		f.Add(data)
	}
	for _, raw := range fuzzAdversarialSeeds {
		f.Add([]byte(raw))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if !utf8.Valid(data) {
			return
		}
		var ev Event
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&ev); err != nil {
			return
		}
		out, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("re-marshal failed for decoded event %+v: %v (input=%q)", ev, err, data)
		}
		var ev2 Event
		if err := json.Unmarshal(out, &ev2); err != nil {
			t.Fatalf("re-decode failed: %v (intermediate=%s)", err, out)
		}
		if ev.Type != ev2.Type {
			t.Fatalf("type discriminator drifted: %q -> %q", ev.Type, ev2.Type)
		}
	})
}

// fuzzSeedEvents mirrors the canonical TestEventRoundTrip corpus.
func fuzzSeedEvents() []Event {
	return []Event{
		{Type: EventSessionUpdate, EventID: "evt_1", Session: &Session{Voice: "alloy"}},
		{Type: EventSessionCreated, Session: &Session{ID: "sess_a", Model: "gpt-realtime"}},
		{Type: EventSessionUpdated, Session: &Session{Voice: "echo"}},
		{Type: EventInputAudioBufferAppend, Audio: "AAAA"},
		{Type: EventInputAudioBufferCommit, EventID: "evt_3"},
		{Type: EventInputAudioBufferClear},
		{Type: EventInputAudioBufferCommitted, ItemID: "item_a"},
		{Type: EventInputAudioBufferSpeechStarted, ItemID: "item_b"},
		{Type: EventInputAudioBufferSpeechStopped, ItemID: "item_b"},
		{Type: EventConversationCreated},
		{Type: EventConversationItemCreate, Item: &Item{Type: "message", Role: "user", Content: []ItemContent{{Type: "input_text", Text: "hi"}}}},
		{Type: EventConversationItemCreated, Item: &Item{ID: "item_1", Type: "message", Role: "user"}},
		{Type: EventConversationItemTruncate, ItemID: "item_2"},
		{Type: EventConversationItemDelete, ItemID: "item_3"},
		{Type: EventResponseCreate, EventID: "evt_4"},
		{Type: EventResponseCreated, ResponseID: "resp_1"},
		{Type: EventResponseDone, ResponseID: "resp_1"},
		{Type: EventResponseOutputItemAdded, ResponseID: "resp_1", Item: &Item{Type: "message", Role: "assistant"}},
		{Type: EventResponseContentPartAdded, ResponseID: "resp_1", ContentIndex: 0},
		{Type: EventResponseTextDelta, ResponseID: "resp_1", Delta: json.RawMessage(`"hello"`)},
		{Type: EventResponseAudioDelta, ResponseID: "resp_1", Delta: json.RawMessage(`"AAAA"`)},
		{Type: EventResponseAudioTranscriptDelta, ResponseID: "resp_1", Delta: json.RawMessage(`"the cat"`)},
		{Type: EventResponseFunctionCallArgumentsDelta, ResponseID: "resp_1", Delta: json.RawMessage(`"{\"a\":1}"`)},
		{Type: EventRateLimitsUpdated},
		{Type: EventError, Error: &APIError{Type: "invalid_request_error", Code: "missing_parameter", Message: "session is required"}},
		// gpt-realtime-2 v2 surface (oairt v0.1.1):
		{Type: EventConversationItemAdded, EventID: "evt_6", PreviousItemID: "item_0", Item: &Item{ID: "item_1", Type: "message", Role: "assistant", Status: "in_progress"}},
		{Type: EventConversationItemDone, EventID: "evt_7", PreviousItemID: "item_0", Item: &Item{ID: "item_1", Type: "message", Role: "assistant", Status: "completed"}},
		{Type: EventConversationItemInputAudioTranscriptionDelta, EventID: "evt_8", ItemID: "item_2", ContentIndex: 0, Delta: json.RawMessage(`"hel"`)},
		{Type: EventSessionUpdate, EventID: "evt_9", Session: &Session{
			Reasoning:               &Reasoning{Effort: "medium"},
			InputAudioTranscription: &AudioTranscription{Model: "gpt-4o-transcribe", Language: "en", Delay: "low", Prompt: "vocabulary: ACME, OAuth"},
		}},
	}
}

// fuzzAdversarialSeeds covers shapes that have historically tripped JSON
// decoders: empty/missing discriminators, mistyped delta payloads, deeply
// nested item soup, oversized integers.
var fuzzAdversarialSeeds = []string{
	`{}`,
	`{"type":""}`,
	`{"type":"unknown.event"}`,
	`{"type":"response.audio.delta","delta":null}`,
	`{"type":"response.audio.delta","delta":1234}`,
	`{"type":"response.audio.delta","delta":["AAAA"]}`,
	`{"type":"conversation.item.created","item":{"id":"x","content":[{"type":"input_text","text":"hi"}]}}`,
	`{"type":"conversation.item.created","item":{"arguments":"{\"deeply\":{\"nested\":{\"k\":1}}}"}}`,
	`{"type":"error","error":{"type":"x","code":"y","message":"emoji-removed"}}`,
	`{"type":"x","output_index":2147483647,"content_index":-2147483648}`,
}
