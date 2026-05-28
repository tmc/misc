package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	oairt "github.com/tmc/misc/oairt"
)

const defaultInstructions = `# Role and Objective
You are playing and narrating Zelda3 in realtime.
Your immediate gameplay objective is to find Link's uncle in the castle dungeon.
Start by dismissing any text box or modal dialog, opening the chest in Link's house if it has not been opened, exiting Link's house through the bottom/south exit, then routing north toward the castle, entering it, reaching the dungeon path, and continuing until the uncle is found.

# Voice
- Act like a funny Twitch streamer: quick, playful, observant, and a little self-deprecating when the game is chaotic.
- Keep jokes short and family-safe so gameplay decisions stay clear.
- Do not overuse streamer catchphrases. Say "chat" rarely, at most once every few minutes.
- Speak naturally, briefly, and confidently during active play.
- Use short spoken preambles before tool calls, such as "Checking the room" or "Tiny gamer steps, moving down now."
- Do not include sound effects, humming, music, or onomatopoeia.

# Gameplay Rules
- Use observe before deciding what to do.
- Prefer run_input for controller actions.
- Use one bounded action at a time, then observe again.
- Use get_frame when text state is ambiguous or contradicts the visible HUD.
- If a text box, dialog, modal, or menu is on screen, press buttons to dismiss it before navigating.
- To dismiss text, try short bounded A presses first, then B or START only if A does not clear it; observe after each attempt.
- Before leaving Link's house, open the chest if it is still present or the lamp has not been collected.
- Do not head north toward the castle until Link has exited his house through the bottom/south exit.
- Do not use memory writes, teleports, item grants, or cheats.

# State Interpretation
- Treat numeric health, magic, or rupee fields as potentially stale or ambiguous unless confirmed by the HUD or game state.
- Do not say Link is at zero health or cannot act unless the visible HUD or an explicit death/game-over state confirms it.
- If a state field conflicts with visible movement or user correction, trust the correction and continue playing.
- When uncertain, verify with observe or get_frame instead of stopping progress.`

type config struct {
	apiKey       string
	realtimeURL  string
	mcpURL       string
	healthURL    string
	model        string
	voice        string
	effort       string
	outputModes  string
	audio        bool
	instructions string
	prompt       string
	framesMax    int
	timeout      time.Duration
	once         bool
	tui          bool
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "oairt-zelda3: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcp := newMCPClient(cfg.mcpURL, cfg.timeout)
	if err := checkHealth(ctx, cfg.healthURL, cfg.timeout); err != nil {
		return err
	}
	if err := mcp.initialize(ctx); err != nil {
		return err
	}
	mcpTools, err := mcp.tools(ctx)
	if err != nil {
		return err
	}
	if err := verifyTools(mcpTools); err != nil {
		return err
	}
	allowedTools := toolNameSet(mcpTools)
	interactive := cfg.tui && isInteractive(stdin, stdout)
	uiEvents := make(chan uiEvent, 256)
	defer close(uiEvents)

	client := oairt.NewClient(cfg.apiKey, clientOptions(cfg)...)
	defer client.Close()

	var audio *audioSink
	if cfg.audio && wantsAudio(cfg.outputModes) {
		audio = newAudioSink(ctx, 24000)
	}

	sessionReady := make(chan struct{})
	var sawSession atomic.Bool
	done := make(chan struct{})

	bridge := &toolBridge{
		mcp:          mcp,
		sender:       client,
		allowedTools: allowedTools,
		framesMax:    cfg.framesMax,
		out:          stdout,
		err:          stderr,
	}

	client.On("*", func(e oairt.Event) {
		switch e.Type {
		case oairt.EventSessionCreated, oairt.EventSessionUpdated:
			postUIEvent(uiEvents, uiEvent{kind: uiStatus, text: e.Type})
			if sawSession.CompareAndSwap(false, true) {
				close(sessionReady)
			}
		case oairt.EventResponseTextDelta, oairt.EventResponseOutputTextDelta,
			oairt.EventResponseAudioTranscriptDelta, oairt.EventResponseOutputAudioTranscriptDelta:
			if s, ok := e.TextDelta(); ok {
				if interactive {
					postUIEvent(uiEvents, uiEvent{kind: uiAssistantDelta, text: s})
				} else {
					fmt.Fprint(stdout, s)
				}
			}
		case oairt.EventResponseAudioDelta, oairt.EventResponseOutputAudioDelta:
			if audio != nil {
				s, ok := e.AudioDelta()
				if !ok {
					return
				}
				data, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					fmt.Fprintf(stderr, "decode audio delta: %v\n", err)
					return
				}
				audio.Write(data)
				postUIEvent(uiEvents, uiEvent{kind: uiAudio, bytes: len(data)})
			}
		case oairt.EventResponseTextDone, oairt.EventResponseOutputTextDone,
			oairt.EventResponseAudioTranscriptDone, oairt.EventResponseOutputAudioTranscriptDone:
			if interactive {
				postUIEvent(uiEvents, uiEvent{kind: uiAssistantDone})
			} else {
				fmt.Fprintln(stdout)
			}
		case oairt.EventResponseFunctionCallArgumentsDone, oairt.EventResponseOutputItemDone:
			if call, ok := e.FunctionCall(); ok {
				postUIEvent(uiEvents, uiEvent{kind: uiTool, text: call.Name})
				go bridge.handleFunctionCall(ctx, e)
			}
		case oairt.EventError:
			var msg string
			if e.Error != nil {
				msg = e.Error.Error()
			} else {
				msg = string(e.Raw)
			}
			if interactive {
				postUIEvent(uiEvents, uiEvent{kind: uiError, text: msg})
			} else {
				fmt.Fprintf(stderr, "realtime error: %s\n", msg)
			}
		}
	})

	if err := client.Connect(ctx, cfg.model); err != nil {
		return fmt.Errorf("connect realtime: %w", err)
	}
	if err := waitSession(ctx, sessionReady, 30*time.Second); err != nil {
		return err
	}
	if err := sendSessionUpdate(client, cfg, mcpTools); err != nil {
		return err
	}
	if err := sendInitialObservation(ctx, client, mcp); err != nil {
		return err
	}
	if cfg.prompt != "" {
		if err := sendUserText(client, cfg.prompt); err != nil {
			return err
		}
	}

	if interactive {
		err := runTUI(ctx, tuiOptions{
			cfg:    cfg,
			client: client,
			events: uiEvents,
			stdin:  stdin,
			stdout: stdout,
		})
		if audio != nil {
			if closeErr := audio.Close(); closeErr != nil {
				fmt.Fprintf(stderr, "audio: %v\n", closeErr)
			}
		}
		_ = releaseAll(context.Background(), mcp)
		return err
	}

	go func() {
		defer close(done)
		if cfg.once {
			<-ctx.Done()
			return
		}
		scanStdin(ctx, stdin, client, stderr)
	}()

	select {
	case <-ctx.Done():
	case <-done:
	}
	if audio != nil {
		if err := audio.Close(); err != nil {
			fmt.Fprintf(stderr, "audio: %v\n", err)
		}
	}
	_ = releaseAll(context.Background(), mcp)
	return nil
}

func parseConfig(args []string) (*config, error) {
	cfg := &config{
		mcpURL:       "http://127.0.0.1:8123/mcp",
		model:        "gpt-realtime-2",
		voice:        "marin",
		effort:       "low",
		audio:        true,
		tui:          true,
		instructions: defaultInstructions,
		framesMax:    60,
		timeout:      5 * time.Second,
	}
	fs := flag.NewFlagSet("oairt-zelda3", flag.ContinueOnError)
	fs.StringVar(&cfg.apiKey, "api-key", "", "OpenAI API key; defaults to OPENAI_API_KEY")
	fs.StringVar(&cfg.realtimeURL, "url", "", "Realtime WebSocket URL override")
	fs.StringVar(&cfg.mcpURL, "mcp-url", cfg.mcpURL, "Zelda3 MCP HTTP URL")
	fs.StringVar(&cfg.healthURL, "health-url", "", "Zelda3 health URL; defaults to MCP URL base plus /health")
	fs.StringVar(&cfg.model, "model", cfg.model, "Realtime model")
	fs.StringVar(&cfg.voice, "voice", cfg.voice, "Realtime output voice")
	fs.StringVar(&cfg.effort, "effort", cfg.effort, "reasoning effort")
	fs.StringVar(&cfg.outputModes, "output-modalities", "text", "comma-separated output modalities: text,audio")
	fs.BoolVar(&cfg.audio, "audio", cfg.audio, "play Realtime audio output when audio modality is active")
	fs.StringVar(&cfg.instructions, "instructions", cfg.instructions, "session instructions")
	fs.StringVar(&cfg.prompt, "prompt", "", "initial user prompt")
	fs.IntVar(&cfg.framesMax, "frames-max", cfg.framesMax, "maximum frames per run_input call")
	fs.DurationVar(&cfg.timeout, "timeout", cfg.timeout, "HTTP/tool timeout")
	fs.BoolVar(&cfg.once, "once", false, "send startup turn and wait for interrupt")
	fs.BoolVar(&cfg.tui, "tui", cfg.tui, "run interactive Bubble Tea terminal UI when stdin/stdout are terminals")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("api key is required; set OPENAI_API_KEY or pass -api-key")
	}
	if cfg.healthURL == "" {
		health, err := deriveHealthURL(cfg.mcpURL)
		if err != nil {
			return nil, err
		}
		cfg.healthURL = health
	}
	return cfg, nil
}

func clientOptions(cfg *config) []oairt.Option {
	var opts []oairt.Option
	if cfg.realtimeURL != "" {
		opts = append(opts, oairt.WithURL(cfg.realtimeURL))
	}
	return opts
}

func waitSession(ctx context.Context, ready <-chan struct{}, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timeout waiting for realtime session")
	}
}

func sendSessionUpdate(sender realtimeSender, cfg *config, tools []mcpTool) error {
	return sender.Send(oairt.Event{
		Type: oairt.EventSessionUpdate,
		Session: &oairt.Session{
			Type:             "realtime",
			Model:            cfg.model,
			Instructions:     cfg.instructions,
			OutputModalities: parseOutputModalities(cfg.outputModes),
			Audio: &oairt.AudioConfig{
				Input: &oairt.AudioInputConfig{
					Format: &oairt.AudioFormat{Type: "audio/pcm", Rate: 24000},
					TurnDetection: &oairt.TurnDetection{
						Type:              "server_vad",
						CreateResponse:    boolPtr(false),
						InterruptResponse: boolPtr(true),
					},
				},
				Output: &oairt.AudioOutputConfig{
					Format: &oairt.AudioFormat{Type: "audio/pcm", Rate: 24000},
					Voice:  cfg.voice,
				},
			},
			Tools:      realtimeTools(tools),
			ToolChoice: "auto",
			Reasoning:  &oairt.Reasoning{Effort: cfg.effort},
		},
	})
}

func boolPtr(v bool) *bool {
	return &v
}

func isInteractive(stdin io.Reader, stdout io.Writer) bool {
	in, ok := stdin.(*os.File)
	if !ok || in != os.Stdin {
		return false
	}
	out, ok := stdout.(*os.File)
	return ok && out == os.Stdout
}

func sendInitialObservation(ctx context.Context, sender realtimeSender, mcp *mcpClient) error {
	result, err := mcp.callTool(ctx, "observe", map[string]any{})
	if err != nil {
		return fmt.Errorf("observe: %w", err)
	}
	if err := sendUserText(sender, "Initial Zelda3 observation:\n"+string(result)+"\n\nBegin the objective now: find Link's uncle in the castle dungeon. If health fields look wrong, verify visually and keep playing."); err != nil {
		return err
	}
	return nil
}

func sendUserText(sender realtimeSender, text string) error {
	if err := sender.Send(oairt.Event{
		Type: oairt.EventConversationItemCreate,
		Item: &oairt.Item{
			Type: "message",
			Role: "user",
			Content: []oairt.ItemContent{{
				Type: "input_text",
				Text: text,
			}},
		},
	}); err != nil {
		return fmt.Errorf("send user text: %w", err)
	}
	if err := sender.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		return fmt.Errorf("send response.create: %w", err)
	}
	return nil
}

func parseList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseOutputModalities(s string) []string {
	if wantsAudio(s) {
		return []string{"audio"}
	}
	return []string{"text"}
}

func wantsAudio(s string) bool {
	for _, mode := range parseList(s) {
		if mode == "audio" {
			return true
		}
	}
	return false
}

func scanStdin(ctx context.Context, r io.Reader, sender realtimeSender, stderr io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := sendUserText(sender, line); err != nil {
			fmt.Fprintf(stderr, "send input: %v\n", err)
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "stdin: %v\n", err)
	}
}

type realtimeSender interface {
	Send(oairt.Event) error
}

type toolBridge struct {
	mcp          *mcpClient
	sender       realtimeSender
	allowedTools map[string]bool
	handled      sync.Map
	framesMax    int
	out          io.Writer
	err          io.Writer
}

func (b *toolBridge) handleFunctionCall(ctx context.Context, e oairt.Event) {
	call, ok := e.FunctionCall()
	if !ok {
		fmt.Fprintf(b.err, "function call: missing function name or call_id\n")
		return
	}
	if _, loaded := b.handled.LoadOrStore(call.CallID, true); loaded {
		return
	}
	if !b.allowedTools[call.Name] {
		b.sendToolOutput(call.CallID, map[string]any{"error": "tool not allowed"})
		return
	}
	args, err := call.ArgumentsObject()
	if err != nil {
		b.sendToolOutput(call.CallID, map[string]any{"error": err.Error()})
		return
	}
	if err := b.checkArgs(call.Name, args); err != nil {
		b.sendToolOutput(call.CallID, map[string]any{"error": err.Error()})
		if isMutatingTool(call.Name) {
			_ = releaseAll(context.Background(), b.mcp)
		}
		return
	}
	result, err := b.mcp.callTool(ctx, call.Name, args)
	if err != nil {
		if isMutatingTool(call.Name) {
			_ = releaseAll(context.Background(), b.mcp)
		}
		b.sendToolOutput(call.CallID, map[string]any{"error": err.Error()})
		return
	}
	b.sendToolOutput(call.CallID, json.RawMessage(result))
}

func (b *toolBridge) sendToolOutput(callID string, v any) {
	event, err := oairt.FunctionCallOutput(callID, v)
	if err != nil {
		event, _ = oairt.FunctionCallOutput(callID, map[string]any{"error": err.Error()})
	}
	err = b.sender.Send(event)
	if err != nil {
		fmt.Fprintf(b.err, "send tool output: %v\n", err)
		return
	}
	if err := b.sender.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		fmt.Fprintf(b.err, "send response.create: %v\n", err)
	}
}

func (b *toolBridge) checkArgs(name string, args map[string]any) error {
	if name != "run_input" {
		return nil
	}
	frames, ok, err := intArg(args, "frames")
	if err != nil {
		return err
	}
	if !ok {
		frames, ok, err = intArg(args, "count")
		if err != nil {
			return err
		}
	}
	if ok && frames > b.framesMax {
		return fmt.Errorf("frames %d exceeds maximum %d", frames, b.framesMax)
	}
	return nil
}

func intArg(args map[string]any, name string) (int, bool, error) {
	v, ok := args[name]
	if !ok {
		return 0, false, nil
	}
	switch v := v.(type) {
	case json.Number:
		i, err := v.Int64()
		return int(i), true, err
	case float64:
		return int(v), true, nil
	case int:
		return v, true, nil
	default:
		return 0, true, fmt.Errorf("%s must be a number", name)
	}
}

func isMutatingTool(name string) bool {
	switch name {
	case "run_input", "set_buttons", "release_all_inputs", "restore_snapshot":
		return true
	default:
		return false
	}
}

type mcpClient struct {
	url     string
	http    *http.Client
	nextID  atomic.Int64
	headers http.Header
}

func newMCPClient(rawURL string, timeout time.Duration) *mcpClient {
	return &mcpClient{
		url:  rawURL,
		http: &http.Client{Timeout: timeout},
		headers: http.Header{
			"Content-Type": []string{"application/json"},
			"Accept":       []string{"application/json, text/event-stream"},
		},
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type mcpTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code != 0 {
		return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message)
	}
	return "mcp error: " + e.Message
}

func (c *mcpClient) initialize(ctx context.Context) error {
	_, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "oairt-zelda3",
			"version": "dev",
		},
	})
	if err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}
	return nil
}

func (c *mcpClient) tools(ctx context.Context) ([]mcpTool, error) {
	raw, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("mcp tools/list: %w", err)
	}
	var body struct {
		Tools []mcpTool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("decode tools/list: %w", err)
	}
	return body.Tools, nil
}

func (c *mcpClient) callTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	return c.call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
}

func (c *mcpClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal rpc: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new rpc request: %w", err)
	}
	for key, values := range c.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post rpc: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read rpc response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("rpc status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out rpcResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode rpc response: %w", err)
	}
	if out.Error != nil {
		return nil, out.Error
	}
	return out.Result, nil
}

func verifyTools(tools []mcpTool) error {
	have := toolNameSet(tools)
	var missing []string
	for _, name := range []string{"observe", "release_all_inputs"} {
		if !have[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("mcp server missing tools: %s", strings.Join(missing, ", "))
	}
	return nil
}

func toolNameSet(tools []mcpTool) map[string]bool {
	out := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if tool.Name != "" {
			out[tool.Name] = true
		}
	}
	return out
}

func releaseAll(ctx context.Context, mcp *mcpClient) error {
	_, err := mcp.callTool(ctx, "release_all_inputs", map[string]any{})
	return err
}

func checkHealth(ctx context.Context, rawURL string, timeout time.Duration) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("new health request: %w", err)
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("check zelda3 health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("zelda3 health status %d", resp.StatusCode)
	}
	return nil
}

func deriveHealthURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse mcp url: %w", err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/mcp")
	u.Path = strings.TrimRight(u.Path, "/") + "/health"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func realtimeTools(tools []mcpTool) []oairt.Tool {
	out := make([]oairt.Tool, 0, len(tools))
	for _, tool := range tools {
		if tool.Name == "" {
			continue
		}
		out = append(out, oairt.Tool{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  toolParameters(tool.InputSchema),
		})
	}
	return out
}

func objectSchema(properties map[string]json.RawMessage, required []string) oairt.ToolParameters {
	if properties == nil {
		properties = make(map[string]json.RawMessage)
	}
	return oairt.ToolParameters{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}

func toolParameters(schema json.RawMessage) oairt.ToolParameters {
	if len(schema) == 0 || string(schema) == "null" {
		return objectSchema(nil, nil)
	}
	return oairt.ToolParameters{Raw: append(json.RawMessage(nil), schema...)}
}
