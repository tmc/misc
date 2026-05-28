package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	oairt "github.com/tmc/misc/oairt"
	"github.com/tmc/misc/oairt/internal/mockrt"
)

func TestDeriveHealthURL(t *testing.T) {
	got, err := deriveHealthURL("http://127.0.0.1:8123/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://127.0.0.1:8123/health" {
		t.Fatalf("deriveHealthURL = %q", got)
	}
}

func TestBridgeCallsMCPAndSendsFunctionOutput(t *testing.T) {
	var sawRunInput bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc: %v", err)
		}
		switch req.Method {
		case "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			b, _ := json.Marshal(req.Params)
			if err := json.Unmarshal(b, &params); err != nil {
				t.Fatalf("decode params: %v", err)
			}
			if params.Name != "run_input" {
				t.Fatalf("tool = %q, want run_input", params.Name)
			}
			sawRunInput = true
			writeRPCResult(t, w, req.ID, map[string]any{"content": []map[string]any{{"type": "text", "text": "frame=12"}}})
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
	}))
	defer srv.Close()

	sender := &captureSender{}
	bridge := &toolBridge{
		mcp:          newMCPClient(srv.URL, 0),
		sender:       sender,
		allowedTools: toolNameSet([]mcpTool{{Name: "run_input"}}),
		framesMax:    60,
	}
	bridge.handleFunctionCall(context.Background(), oairt.Event{
		Type:      oairt.EventResponseFunctionCallArgumentsDone,
		Name:      "run_input",
		CallID:    "call_1",
		Arguments: `{"buttons":["RIGHT"],"frames":2}`,
	})

	if !sawRunInput {
		t.Fatal("MCP run_input was not called")
	}
	if len(sender.events) != 2 {
		t.Fatalf("sent %d events, want 2", len(sender.events))
	}
	out := sender.events[0]
	if out.Type != oairt.EventConversationItemCreate || out.Item == nil || out.Item.Type != "function_call_output" {
		t.Fatalf("first event = %#v, want function_call_output item", out)
	}
	if out.Item.CallID != "call_1" {
		t.Fatalf("call_id = %q, want call_1", out.Item.CallID)
	}
	if !strings.Contains(out.Item.Output, "frame=12") {
		t.Fatalf("output = %q, want frame text", out.Item.Output)
	}
	if sender.events[1].Type != oairt.EventResponseCreate {
		t.Fatalf("second event = %q, want response.create", sender.events[1].Type)
	}
}

func TestBridgeRejectsOversizedRunInput(t *testing.T) {
	sender := &captureSender{}
	bridge := &toolBridge{
		mcp:          newMCPClient("http://127.0.0.1:1/mcp", 0),
		sender:       sender,
		allowedTools: toolNameSet([]mcpTool{{Name: "run_input"}}),
		framesMax:    5,
	}
	bridge.handleFunctionCall(context.Background(), oairt.Event{
		Type:      oairt.EventResponseFunctionCallArgumentsDone,
		Name:      "run_input",
		CallID:    "call_2",
		Arguments: `{"buttons":["RIGHT"],"frames":6}`,
	})
	if len(sender.events) != 2 {
		t.Fatalf("sent %d events, want 2", len(sender.events))
	}
	if !strings.Contains(sender.events[0].Item.Output, "exceeds maximum") {
		t.Fatalf("output = %q, want max-frame error", sender.events[0].Item.Output)
	}
}

func TestParseConfigRequiresAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := parseConfig([]string{"-mcp-url", "http://127.0.0.1:8123/mcp"})
	if err == nil {
		t.Fatal("parseConfig succeeded without an API key")
	}
}

func TestSendSessionUpdateUsesCurrentRealtimeShape(t *testing.T) {
	sender := &captureSender{}
	cfg := &config{
		model:        "gpt-realtime-2",
		voice:        "marin",
		effort:       "low",
		outputModes:  "text,audio",
		instructions: "play zelda",
	}
	tools := fakeMCPTools()
	if err := sendSessionUpdate(sender, cfg, tools); err != nil {
		t.Fatal(err)
	}
	if len(sender.events) != 1 {
		t.Fatalf("sent %d events, want 1", len(sender.events))
	}
	sess := sender.events[0].Session
	if sess == nil {
		t.Fatal("session is nil")
	}
	if sess.Type != "realtime" || sess.Model != "gpt-realtime-2" {
		t.Fatalf("session type/model = %q/%q", sess.Type, sess.Model)
	}
	if got := strings.Join(sess.OutputModalities, ","); got != "audio" {
		t.Fatalf("output modalities = %q", got)
	}
	if sess.Audio == nil || sess.Audio.Output == nil || sess.Audio.Output.Voice != "marin" {
		t.Fatalf("audio output not configured: %#v", sess.Audio)
	}
	if sess.Audio.Output.Format == nil || sess.Audio.Output.Format.Rate != 24000 {
		t.Fatalf("audio output rate not configured: %#v", sess.Audio.Output.Format)
	}
	if sess.Audio.Input == nil || sess.Audio.Input.Format == nil || sess.Audio.Input.Format.Rate != 24000 {
		t.Fatalf("audio input rate not configured: %#v", sess.Audio)
	}
	if sess.Audio.Input.TurnDetection == nil || sess.Audio.Input.TurnDetection.Type != "server_vad" {
		t.Fatalf("audio input turn detection not configured: %#v", sess.Audio.Input)
	}
	if sess.Audio.Input.TurnDetection.CreateResponse == nil || *sess.Audio.Input.TurnDetection.CreateResponse {
		t.Fatalf("turn detection create_response = %#v, want false", sess.Audio.Input.TurnDetection.CreateResponse)
	}
	if len(sess.Tools) != len(tools) {
		t.Fatalf("session has %d tools, want %d", len(sess.Tools), len(tools))
	}
	got := make(map[string]bool)
	for _, tool := range sess.Tools {
		got[tool.Name] = true
	}
	for _, tool := range tools {
		if !got[tool.Name] {
			t.Fatalf("session missing MCP tool %q", tool.Name)
		}
	}
	data, err := json.Marshal(sess.Tools)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"address"`) {
		t.Fatalf("session tools did not preserve MCP input schema: %s", data)
	}
}

func TestRunConnectsRealtimeAndMCP(t *testing.T) {
	mcp := newFakeMCPServer(t)
	defer mcp.Close()

	rt := mockrt.New(t, mockrt.Script{
		mockrt.SendJSON(t, map[string]any{
			"type":     "session.created",
			"event_id": "evt_session",
			"session": map[string]any{
				"id":    "sess_1",
				"model": "gpt-realtime-2",
			},
		}),
		{Expect: expectSessionUpdateTools(fakeMCPTools())},
		{Expect: expectEventType("conversation.item.create")},
		{Expect: expectEventType("response.create")},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	go func() {
		errc <- run(ctx, []string{
			"-api-key", "test-key",
			"-url", rt.URL,
			"-mcp-url", mcp.URL + "/mcp",
			"-health-url", mcp.URL + "/health",
		}, pr, io.Discard, io.Discard)
	}()

	if err := rt.WaitDone(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	cancel()
	_ = pw.Close()
	select {
	case err := <-errc:
		if err != nil && err != context.Canceled {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not exit after cancellation")
	}
}

func TestParseOutputModalitiesNormalizesAudio(t *testing.T) {
	if got := strings.Join(parseOutputModalities("text,audio"), ","); got != "audio" {
		t.Fatalf("text,audio normalized to %q, want audio", got)
	}
	if got := strings.Join(parseOutputModalities("text"), ","); got != "text" {
		t.Fatalf("text normalized to %q, want text", got)
	}
}

func TestDefaultInstructionsAvoidFalseZeroHealth(t *testing.T) {
	for _, want := range []string{
		"find Link's uncle in the castle dungeon",
		"opening the chest in Link's house",
		"Act like a funny Twitch streamer",
		`Say "chat" rarely`,
		"dismissing any text box or modal dialog",
		"exiting Link's house through the bottom/south exit",
		"press buttons to dismiss it before navigating",
		"Do not head north toward the castle until Link has exited his house",
		"Do not say Link is at zero health",
		"trust the correction and continue playing",
	} {
		if !strings.Contains(defaultInstructions, want) {
			t.Fatalf("defaultInstructions missing %q", want)
		}
	}
}

func TestAudioSinkWriteAfterCloseDoesNotStart(t *testing.T) {
	sink := newAudioSink(context.Background(), 24000)
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	sink.Write([]byte{0, 0})
	sink.mu.Lock()
	started := sink.started
	sink.mu.Unlock()
	if started {
		t.Fatal("audio sink started after close")
	}
}

type captureSender struct {
	events []oairt.Event
}

func (s *captureSender) Send(e oairt.Event) error {
	s.events = append(s.events, e)
	return nil
}

func writeRPCResult(t *testing.T, w http.ResponseWriter, id int64, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}); err != nil {
		t.Fatalf("encode rpc: %v", err)
	}
}

func expectEventType(want string) func([]byte) error {
	return func(data []byte) error {
		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		if event.Type != want {
			return fmt.Errorf("event type = %q, want %q; frame=%s", event.Type, want, data)
		}
		return nil
	}
}

func newFakeMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	tools := fakeMCPTools()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"ok"}`)
			return
		}
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc: %v", err)
		}
		switch req.Method {
		case "initialize":
			writeRPCResult(t, w, req.ID, map[string]any{"protocolVersion": "2024-11-05"})
		case "tools/list":
			writeRPCResult(t, w, req.ID, map[string]any{"tools": tools})
		case "tools/call":
			var params struct {
				Name string `json:"name"`
			}
			b, _ := json.Marshal(req.Params)
			if err := json.Unmarshal(b, &params); err != nil {
				t.Fatalf("decode params: %v", err)
			}
			if params.Name != "observe" && params.Name != "release_all_inputs" {
				t.Fatalf("unexpected tool %q", params.Name)
			}
			writeRPCResult(t, w, req.ID, map[string]any{
				"content": []map[string]any{{"type": "text", "text": "frame=1"}},
			})
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
	}))
}

func expectSessionUpdateTools(want []mcpTool) func([]byte) error {
	return func(data []byte) error {
		var event oairt.Event
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		if event.Type != oairt.EventSessionUpdate || event.Session == nil {
			return fmt.Errorf("event = %s, want session.update with session", data)
		}
		got := make(map[string]bool)
		for _, tool := range event.Session.Tools {
			got[tool.Name] = true
		}
		for _, tool := range want {
			if !got[tool.Name] {
				return fmt.Errorf("session.update missing tool %q; frame=%s", tool.Name, data)
			}
		}
		return nil
	}
}

func fakeMCPTools() []mcpTool {
	return []mcpTool{
		{Name: "observe", Description: "Observe game", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
		{Name: "release_all_inputs", Description: "Release buttons", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
		{Name: "run_input", Description: "Run input", InputSchema: json.RawMessage(`{"type":"object","properties":{"buttons":{"type":"array","items":{"type":"string"}},"frames":{"type":"integer"}},"required":["buttons","frames"]}`)},
		{Name: "write_memory", Description: "Write memory", InputSchema: json.RawMessage(`{"type":"object","properties":{"address":{"type":"integer"},"data":{"type":"string"}},"required":["address","data"]}`)},
	}
}
