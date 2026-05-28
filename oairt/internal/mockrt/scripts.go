package mockrt

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Canonical helpers used across oairt tests. Each returns a Script so callers
// can compose with Then.

// ScriptSessionCreated emits a single session.created frame and idles.
func ScriptSessionCreated(t testing.TB, sessionID, model string) Script {
	t.Helper()
	return Script{SendJSON(t, map[string]any{
		"type":     "session.created",
		"event_id": "evt_session_created",
		"session": map[string]any{
			"id":     sessionID,
			"object": "realtime.session",
			"model":  model,
		},
	})}
}

// ScriptAudioDelta emits session.created, n response.audio.delta frames each
// carrying the literal base64 string "AAAA", then a response.audio.done.
func ScriptAudioDelta(t testing.TB, n int) Script {
	t.Helper()
	out := ScriptSessionCreated(t, "sess_audio", "gpt-realtime-test")
	for i := range n {
		out = append(out, SendJSON(t, map[string]any{
			"type":     "response.audio.delta",
			"event_id": fmt.Sprintf("evt_audio_%d", i),
			"delta":    "AAAA",
		}))
	}
	out = append(out, SendJSON(t, map[string]any{
		"type":     "response.audio.done",
		"event_id": "evt_audio_done",
	}))
	return out
}

// ScriptError emits session.created followed by an error frame.
func ScriptError(t testing.TB, code, message string) Script {
	t.Helper()
	return ScriptSessionCreated(t, "sess_error", "gpt-realtime-test").Then(Script{
		SendJSON(t, map[string]any{
			"type":     "error",
			"event_id": "evt_error",
			"error": map[string]any{
				"type":    "invalid_request_error",
				"code":    code,
				"message": message,
			},
		}),
	})
}

// ScriptAbruptClose emits session.created then closes the connection without
// a graceful close frame.
func ScriptAbruptClose(t testing.TB) Script {
	t.Helper()
	return ScriptSessionCreated(t, "sess_abrupt", "gpt-realtime-test").Then(Script{
		{CloseCode: 1006},
	})
}

// ScriptTranscriptDone emits three response.audio_transcript.delta frames
// followed by a .done.
func ScriptTranscriptDone(t testing.TB, parts ...string) Script {
	t.Helper()
	if len(parts) == 0 {
		parts = []string{"hello", " ", "world"}
	}
	out := ScriptSessionCreated(t, "sess_tx", "gpt-realtime-test")
	for i, p := range parts {
		delta, _ := json.Marshal(p)
		out = append(out, Step{Send: mustJSON(map[string]any{
			"type":     "response.audio_transcript.delta",
			"event_id": fmt.Sprintf("evt_tx_%d", i),
			"delta":    json.RawMessage(delta),
		})})
	}
	out = append(out, Step{Send: mustJSON(map[string]any{
		"type":     "response.audio_transcript.done",
		"event_id": "evt_tx_done",
	})})
	return out
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
