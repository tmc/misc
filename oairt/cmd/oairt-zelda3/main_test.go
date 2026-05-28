package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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

func TestBridgeSendsGetFrameAsImageInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc: %v", err)
		}
		if req.Method != "tools/call" {
			t.Fatalf("unexpected method %q", req.Method)
		}
		writeRPCResult(t, w, req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "image", "data": "aGVsbG8=", "mimeType": "image/png"},
				{"type": "text", "text": "Primary engine frame"},
			},
		})
	}))
	defer srv.Close()

	sender := &captureSender{}
	bridge := &toolBridge{
		mcp:          newMCPClient(srv.URL, 0),
		sender:       sender,
		allowedTools: toolNameSet([]mcpTool{{Name: "get_frame"}}),
	}
	bridge.handleFunctionCall(context.Background(), oairt.Event{
		Type:      oairt.EventResponseFunctionCallArgumentsDone,
		Name:      "get_frame",
		CallID:    "call_frame",
		Arguments: `{}`,
	})

	events := sender.Events()
	if len(events) != 3 {
		t.Fatalf("sent %d events, want 3", len(events))
	}
	if !strings.Contains(events[0].Item.Output, `"image_sent":true`) {
		t.Fatalf("function output = %q, want image_sent", events[0].Item.Output)
	}
	if strings.Contains(events[0].Item.Output, "aGVsbG8=") {
		t.Fatalf("function output included raw image data: %q", events[0].Item.Output)
	}
	item := events[1].Item
	if item == nil || len(item.Content) != 2 {
		t.Fatalf("image user item = %#v, want two content parts", item)
	}
	if item.Content[1].Type != oairt.ContentTypeInputImage || item.Content[1].ImageURL != "data:image/png;base64,aGVsbG8=" {
		t.Fatalf("image content = %#v", item.Content[1])
	}
	if events[2].Type != oairt.EventResponseCreate {
		t.Fatalf("third event = %q, want response.create", events[2].Type)
	}
}

func TestBridgeSerializesMutatingTools(t *testing.T) {
	var active int32
	var maxActive int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc: %v", err)
		}
		if req.Method != "tools/call" {
			t.Fatalf("unexpected method %q", req.Method)
		}
		now := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&maxActive)
			if now <= old || atomic.CompareAndSwapInt32(&maxActive, old, now) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		writeRPCResult(t, w, req.ID, map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}})
	}))
	defer srv.Close()

	sender := &captureSender{}
	bridge := &toolBridge{
		mcp:          newMCPClient(srv.URL, 0),
		sender:       sender,
		allowedTools: toolNameSet([]mcpTool{{Name: "run_input"}}),
		framesMax:    60,
	}
	var wg sync.WaitGroup
	for _, callID := range []string{"call_1", "call_2"} {
		wg.Add(1)
		go func(callID string) {
			defer wg.Done()
			bridge.handleFunctionCall(context.Background(), oairt.Event{
				Type:      oairt.EventResponseFunctionCallArgumentsDone,
				Name:      "run_input",
				CallID:    callID,
				Arguments: `{"buttons":["RIGHT"],"frames":1}`,
			})
		}(callID)
	}
	wg.Wait()
	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("max concurrent mutating calls = %d, want 1", got)
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

func TestToolCallDetail(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "run_input",
			args: map[string]any{"buttons": []any{"RIGHT", "A"}, "frames": float64(12)},
			want: "run_input RIGHT+A 12f",
		},
		{
			name: "observe",
			args: map[string]any{},
			want: "observe game state",
		},
		{
			name: "read_memory",
			args: map[string]any{"address": float64(0xF36D)},
			want: "read_memory $F36D",
		},
	}
	for _, tt := range tests {
		if got := toolCallDetail(tt.name, tt.args); got != tt.want {
			t.Fatalf("toolCallDetail(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestToolDoneDetailSummarizesTextContent(t *testing.T) {
	result := json.RawMessage(`{"content":[{"type":"text","text":"frame=12 link=(1,2) hp=24/24"}]}`)
	got := toolDoneDetail("run_input RIGHT 2f", result, nil, time.Now())
	if !strings.Contains(got, "frame=12 link=(1,2) hp=24/24") {
		t.Fatalf("toolDoneDetail = %q, want result summary", got)
	}
}

func TestTUIToolCallLogsOneEntry(t *testing.T) {
	var m tuiModel
	m.applyEvent(uiEvent{kind: uiTool, text: "run_input RIGHT 2f"})
	if len(m.logs) != 0 {
		t.Fatalf("uiTool logged %d entries, want 0", len(m.logs))
	}
	m.applyEvent(uiEvent{kind: uiToolDone, text: "run_input RIGHT 2f -> frame=12"})
	if len(m.logs) != 1 {
		t.Fatalf("tool call logged %d entries, want 1", len(m.logs))
	}
	if got, want := m.logs[0], "tool call: run_input RIGHT 2f -> frame=12"; got != want {
		t.Fatalf("log entry = %q, want %q", got, want)
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

func TestSendInitialContextIncludesFrame(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc: %v", err)
		}
		if req.Method != "tools/call" {
			t.Fatalf("unexpected method %q", req.Method)
		}
		var params struct {
			Name string `json:"name"`
		}
		b, _ := json.Marshal(req.Params)
		if err := json.Unmarshal(b, &params); err != nil {
			t.Fatalf("decode params: %v", err)
		}
		calls = append(calls, params.Name)
		switch params.Name {
		case "observe":
			writeRPCResult(t, w, req.ID, map[string]any{
				"content": []map[string]any{{"type": "text", "text": "Link in bed"}},
			})
		case "get_frame":
			writeRPCResult(t, w, req.ID, map[string]any{
				"content": []map[string]any{
					{"type": "image", "data": "aGVsbG8=", "mimeType": "image/png"},
					{"type": "text", "text": "Primary engine frame"},
				},
			})
		default:
			t.Fatalf("unexpected tool %q", params.Name)
		}
	}))
	defer srv.Close()

	sender := &captureSender{}
	if err := sendInitialContext(context.Background(), sender, newMCPClient(srv.URL, 0), map[string]bool{"get_frame": true}, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(calls, ","); got != "observe,get_frame" {
		t.Fatalf("calls = %q, want observe,get_frame", got)
	}
	events := sender.Events()
	if len(events) != 2 {
		t.Fatalf("sent %d events, want 2", len(events))
	}
	item := events[0].Item
	if item == nil || len(item.Content) != 2 {
		t.Fatalf("initial item = %#v, want text and image", item)
	}
	if item.Content[0].Type != oairt.ContentTypeInputText || !strings.Contains(item.Content[0].Text, "Initial Zelda3 observation") {
		t.Fatalf("text content = %#v", item.Content[0])
	}
	if item.Content[1].Type != oairt.ContentTypeInputImage || item.Content[1].ImageURL != "data:image/png;base64,aGVsbG8=" {
		t.Fatalf("image content = %#v", item.Content[1])
	}
	if events[1].Type != oairt.EventResponseCreate {
		t.Fatalf("second event = %q, want response.create", events[1].Type)
	}
}

func TestImageDataURLFromMCPFrame(t *testing.T) {
	got, desc, err := imageDataURLFromMCPFrame(json.RawMessage(`{"content":[{"type":"image","data":"aGVsbG8=","mimeType":"image/png"},{"type":"text","text":"Primary"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "data:image/png;base64,aGVsbG8=" {
		t.Fatalf("image data URL = %q", got)
	}
	if desc != "Primary" {
		t.Fatalf("description = %q, want Primary", desc)
	}
}

func TestLoadSaveSlotCallsMCP(t *testing.T) {
	var gotName string
	var gotSlot float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc: %v", err)
		}
		if req.Method != "tools/call" {
			t.Fatalf("method = %q, want tools/call", req.Method)
		}
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		b, _ := json.Marshal(req.Params)
		if err := json.Unmarshal(b, &params); err != nil {
			t.Fatalf("decode params: %v", err)
		}
		gotName = params.Name
		gotSlot, _ = params.Arguments["slot"].(float64)
		writeRPCResult(t, w, req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": "Loaded save slot 2"}},
		})
	}))
	defer srv.Close()

	if err := loadSaveSlot(context.Background(), newMCPClient(srv.URL, 0), map[string]bool{"load_save_slot": true}, 2); err != nil {
		t.Fatal(err)
	}
	if gotName != "load_save_slot" || gotSlot != 2 {
		t.Fatalf("call = %s slot %.0f, want load_save_slot 2", gotName, gotSlot)
	}
}

func TestRunLoadsSaveSlotBeforeInitialContext(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			writeRPCResult(t, w, req.ID, map[string]any{"tools": fakeMCPTools()})
		case "tools/call":
			var params struct {
				Name string `json:"name"`
			}
			b, _ := json.Marshal(req.Params)
			if err := json.Unmarshal(b, &params); err != nil {
				t.Fatalf("decode params: %v", err)
			}
			mu.Lock()
			calls = append(calls, params.Name)
			mu.Unlock()
			switch params.Name {
			case "get_frame":
				writeRPCResult(t, w, req.ID, map[string]any{
					"content": []map[string]any{
						{"type": "image", "data": "aGVsbG8=", "mimeType": "image/png"},
						{"type": "text", "text": "Primary engine frame"},
					},
				})
			default:
				writeRPCResult(t, w, req.ID, map[string]any{
					"content": []map[string]any{{"type": "text", "text": "ok"}},
				})
			}
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
	}))
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
			"-load-save-slot", "2",
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
	mu.Lock()
	got := strings.Join(calls, ",")
	mu.Unlock()
	if got != "load_save_slot,observe,get_frame,release_all_inputs" {
		t.Fatalf("tool calls = %q, want load_save_slot,observe,get_frame,release_all_inputs", got)
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
		"snagging the lantern from the chest in Link's house",
		"open the chest and collect the lantern first",
		"Act like a funny Twitch streamer",
		`Say "chat" rarely`,
		"Vary your vocabulary across turns",
		"dismissing any text box or modal dialog",
		"exiting Link's house through the bottom/south exit",
		"press buttons to dismiss it before navigating",
		"Do not head north toward the castle until Link has exited his house",
		"If an action fails",
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

func TestScanStdinMicToggleCommitsAudio(t *testing.T) {
	restore := stubMicRecorder(t, []byte{0, 1, 2, 3})
	defer restore()

	sender := &captureSender{}
	scanStdin(context.Background(), strings.NewReader("/mic\n/mic\n"), sender, io.Discard, true)

	want := []string{
		oairt.EventInputAudioBufferAppend,
		oairt.EventInputAudioBufferCommit,
		oairt.EventResponseCreate,
	}
	if len(sender.events) != len(want) {
		t.Fatalf("sent %d events, want %d: %#v", len(sender.events), len(want), sender.events)
	}
	for i, typ := range want {
		if sender.events[i].Type != typ {
			t.Fatalf("event %d type = %q, want %q", i, sender.events[i].Type, typ)
		}
	}
	if sender.events[0].Audio == "" {
		t.Fatal("input_audio_buffer.append missing encoded audio")
	}
}

func TestScanStdinMicCancelClearsAudio(t *testing.T) {
	restore := stubMicRecorder(t, []byte{0, 1})
	defer restore()

	sender := &captureSender{}
	scanStdin(context.Background(), strings.NewReader("/mic\n/mic cancel\n"), sender, io.Discard, true)

	want := []string{
		oairt.EventInputAudioBufferAppend,
		oairt.EventInputAudioBufferClear,
	}
	if len(sender.events) != len(want) {
		t.Fatalf("sent %d events, want %d: %#v", len(sender.events), len(want), sender.events)
	}
	for i, typ := range want {
		if sender.events[i].Type != typ {
			t.Fatalf("event %d type = %q, want %q", i, sender.events[i].Type, typ)
		}
	}
}

func TestParseConfigMicFlag(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	cfg, err := parseConfig([]string{"-mic=false"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.mic {
		t.Fatal("mic flag = true, want false")
	}
}

func TestParseConfigVisionFlag(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	cfg, err := parseConfig([]string{"-vision=false"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.vision {
		t.Fatal("vision flag = true, want false")
	}
}

type captureSender struct {
	mu     sync.Mutex
	events []oairt.Event
}

func (s *captureSender) Send(e oairt.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *captureSender) Events() []oairt.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]oairt.Event(nil), s.events...)
}

func TestResponseGateQueuesCreateWhileActive(t *testing.T) {
	sender := &captureSender{}
	gate := newResponseGate(sender, io.Discard)

	if err := gate.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		t.Fatal(err)
	}
	if err := gate.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		t.Fatal(err)
	}
	if got := sender.Events(); len(got) != 1 {
		t.Fatalf("sent %d events before done, want 1", len(got))
	}

	gate.observe(oairt.Event{Type: oairt.EventResponseDone})
	events := sender.Events()
	if len(events) != 2 {
		t.Fatalf("sent %d events after done, want 2", len(events))
	}
	for i, event := range events {
		if event.Type != oairt.EventResponseCreate {
			t.Fatalf("event %d type = %q, want response.create", i, event.Type)
		}
	}
}

func TestResponseGateAllowsCreateAfterDone(t *testing.T) {
	sender := &captureSender{}
	gate := newResponseGate(sender, io.Discard)

	if err := gate.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		t.Fatal(err)
	}
	gate.observe(oairt.Event{Type: oairt.EventResponseDone})
	if err := gate.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		t.Fatal(err)
	}

	if got := sender.Events(); len(got) != 2 {
		t.Fatalf("sent %d events, want 2", len(got))
	}
}

type fakeMicRecorder struct {
	data []byte
	cb   func([]byte)
}

func (r *fakeMicRecorder) Start(context.Context) error {
	r.cb(r.data)
	return nil
}

func (r *fakeMicRecorder) Stop() error {
	return nil
}

func stubMicRecorder(t *testing.T, data []byte) func() {
	t.Helper()
	old := newMicRecorder
	newMicRecorder = func(_ int, cb func([]byte)) micRecorderAPI {
		return &fakeMicRecorder{data: data, cb: cb}
	}
	return func() {
		newMicRecorder = old
	}
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
			if params.Name != "observe" && params.Name != "get_frame" && params.Name != "load_save_slot" && params.Name != "release_all_inputs" {
				t.Fatalf("unexpected tool %q", params.Name)
			}
			if params.Name == "get_frame" {
				writeRPCResult(t, w, req.ID, map[string]any{
					"content": []map[string]any{
						{"type": "image", "data": "aGVsbG8=", "mimeType": "image/png"},
						{"type": "text", "text": "Primary engine frame"},
					},
				})
				return
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
		{Name: "get_frame", Description: "Get frame", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
		{Name: "load_save_slot", Description: "Load save slot", InputSchema: json.RawMessage(`{"type":"object","properties":{"slot":{"type":"integer","minimum":0,"maximum":9}},"required":["slot"]}`)},
		{Name: "release_all_inputs", Description: "Release buttons", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
		{Name: "run_input", Description: "Run input", InputSchema: json.RawMessage(`{"type":"object","properties":{"buttons":{"type":"array","items":{"type":"string"}},"frames":{"type":"integer"}},"required":["buttons","frames"]}`)},
		{Name: "write_memory", Description: "Write memory", InputSchema: json.RawMessage(`{"type":"object","properties":{"address":{"type":"integer"},"data":{"type":"string"}},"required":["address","data"]}`)},
	}
}
