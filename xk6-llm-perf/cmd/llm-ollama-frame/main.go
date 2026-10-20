// Command llm-ollama-frame replays one pinned Ollama 0.12.7 chat template offline.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

const templateSHA256 = "ae370d884f108d16e7cc8fd5259ebc5773a0afa6e078b11f4ed7e39a27e0dfc4"

const sourceRevision = "c88647104d1621b5021d6e3ca18fb8a1f5cdd394"

type request struct {
	Model           string    `json:"model"`
	Messages        []message `json:"messages"`
	ReasoningEffort string    `json:"reasoning_effort"`
	Reasoning       *struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
	Tools         json.RawMessage `json:"tools"`
	Stream        bool            `json:"stream"`
	MaxTokens     *uint64         `json:"max_tokens"`
	Temperature   *float64        `json:"temperature"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
}
type message struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// UnmarshalJSON decodes an OpenAI request message strictly. As in the pinned
// openai.FromChatRequest, content may be omitted only with tool calls.
func (m *message) UnmarshalJSON(data []byte) error {
	var v struct {
		Role       string           `json:"role"`
		Content    *string          `json:"content"`
		ToolCalls  []openaiToolCall `json:"tool_calls"`
		ToolCallID string           `json:"tool_call_id"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return err
	}
	if v.ToolCalls != nil && len(v.ToolCalls) == 0 {
		return errEmptyToolCalls
	}
	if v.Content == nil && v.ToolCalls == nil {
		return fmt.Errorf("message content required")
	}
	*m = message{Role: v.Role, ToolCalls: v.ToolCalls, ToolCallID: v.ToolCallID}
	if v.Content != nil {
		m.Content = *v.Content
	}
	return nil
}

// openaiToolCall is the pinned openai.ToolCall request shape, without the
// streaming-only index.
type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// defaultMessage is a model-level message from /api/show. Defaults remain
// text-only: api.Message tool calls and tool names are refused.
type defaultMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type show struct {
	Renderer     string          `json:"renderer,omitempty"`
	Parser       string          `json:"parser,omitempty"`
	RemoteModel  string          `json:"remote_model,omitempty"`
	RemoteHost   string          `json:"remote_host,omitempty"`
	Template     string          `json:"template"`
	Capabilities []string        `json:"capabilities"`
	System       json.RawMessage `json:"system,omitempty"`
	Messages     json.RawMessage `json:"messages,omitempty"`
}

// reconstruction records source-derived values, never engine observations.
// Pointers preserve omitted generation declarations separately from explicit zero.
type reconstruction struct {
	Think       bool    `json:"think"`
	Stream      bool    `json:"stream"`
	Temperature float64 `json:"temperature"`
	MaxTokens   *uint64 `json:"max_tokens"`
}

func reconstruct(r request, think bool) reconstruction {
	temperature := 1.0 // pinned OpenAI conversion sets this even when omitted.
	if r.Temperature != nil {
		temperature = *r.Temperature
	}
	return reconstruction{think, r.Stream, temperature, r.MaxTokens}
}

type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, fmt.Errorf("rendered prompt exceeds budget")
	}
	return b.Buffer.Write(p)
}

func render(s show, r request) (string, bool, error) {
	// This reconstructs the pinned text renderer before context truncation.
	// Engine tokenizer/options and alternate renderers are not reconstructed.
	if fmt.Sprintf("%x", sha256.Sum256([]byte(s.Template))) != templateSHA256 {
		return "", false, fmt.Errorf("unsupported serving template")
	}
	if r.Model != "qwen3:0.6b" || len(r.Messages) == 0 || s.Renderer != "" || s.Parser != "" || s.RemoteModel != "" || s.RemoteHost != "" {
		return "", false, fmt.Errorf("unsupported request population")
	}
	tools, err := admitTools(r.Tools)
	if err != nil {
		return "", false, err
	}
	if len(tools) > 0 && !slices.Contains(s.Capabilities, "tools") {
		return "", false, errToolsCapability
	}
	// The pinned OpenAI parser uses a signed int for max_tokens. A uint64
	// declaration outside that range cannot identify the same accepted request.
	if r.MaxTokens != nil && *r.MaxTokens > math.MaxInt64 {
		return "", false, fmt.Errorf("unsupported max tokens")
	}
	messages, system, err := frameMessages(s, r)
	if err != nil {
		return "", false, err
	}
	if r.ReasoningEffort != "" && r.ReasoningEffort != "none" {
		return "", false, fmt.Errorf("unsupported reasoning effort")
	}
	if r.Reasoning != nil && r.Reasoning.Effort != "" && r.Reasoning.Effort != "none" {
		return "", false, fmt.Errorf("unsupported reasoning effort")
	}
	if !slices.Contains(s.Capabilities, "thinking") {
		return "", false, fmt.Errorf("thinking capability required")
	}
	effort := r.ReasoningEffort
	if r.Reasoning != nil {
		effort = r.Reasoning.Effort
	}
	think := true // routes.go defaults absent Think for a thinking-capable model.
	switch effort {
	case "":
	case "none":
		think = false
	default:
		return "", false, fmt.Errorf("unsupported reasoning effort")
	}
	t, err := template.New("ollama").Parse(s.Template)
	if err != nil {
		return "", false, fmt.Errorf("parse template: %w", err)
	}
	var b boundedBuffer
	values := map[string]any{"System": system, "Tools": tools, "Messages": messages, "Think": think, "IsThinkSet": true, "ThinkLevel": effort}
	if err = t.Execute(&b, values); err != nil {
		return "", false, fmt.Errorf("render template: %w", err)
	}
	return b.String(), think, nil
}

// templateMessage mirrors the api.Message fields the pinned template reads.
type templateMessage struct {
	Role, Content, Thinking string
	ToolCalls               []toolCall
}

// frameMessages follows the pinned routes.go defaults/filterThinkTags and
// template.go collate text paths. It deliberately does not truncate context.
func frameMessages(s show, r request) ([]templateMessage, string, error) {
	var system string
	if len(s.System) > 0 && json.Unmarshal(s.System, &system) != nil {
		return nil, "", fmt.Errorf("invalid model system")
	}
	var defaults []defaultMessage
	if len(s.Messages) > 0 && string(bytes.TrimSpace(s.Messages)) != "null" {
		if decodeJSON(s.Messages, &defaults, true, false) != nil {
			return nil, "", fmt.Errorf("invalid model messages")
		}
	}
	var messages []message
	if r.Messages[0].Role != "system" && system != "" {
		messages = append(messages, message{Role: "system", Content: system})
	}
	for _, m := range defaults {
		// api.Message.UnmarshalJSON normalizes model-message roles.
		role := strings.ToLower(m.Role)
		if role == "tool" {
			return nil, "", fmt.Errorf("unsupported text message")
		}
		messages = append(messages, message{Role: role, Content: m.Content})
	}
	messages = append(messages, r.Messages...)
	if len(messages) > 64 || len(system) > 65536 {
		return nil, "", fmt.Errorf("message budget exceeded")
	}
	lastUser := -1
	for i, m := range messages {
		if len(m.Content) > 65536 || m.Role != "system" && m.Role != "user" && m.Role != "assistant" && m.Role != "tool" {
			return nil, "", fmt.Errorf("unsupported text message")
		}
		if m.Role == "user" {
			lastUser = i
		}
	}
	var systems []string
	var collated []templateMessage
	for i, m := range messages {
		calls, err := admitToolCalls(m)
		if err != nil {
			return nil, "", err
		}
		if m.Role == "assistant" && i < lastUser {
			m.Content = filterThinking(m.Content)
		}
		if m.Role == "system" {
			systems = append(systems, m.Content)
		}
		// Pinned collate merges consecutive non-tool messages of one role,
		// keeping only the first message's tool calls, which content then hides.
		if n := len(collated); n > 0 && collated[n-1].Role == m.Role && m.Role != "tool" {
			if len(collated[n-1].ToolCalls) > 0 || len(calls) > 0 {
				return nil, "", errMergedToolCalls
			}
			collated[n-1].Content += "\n\n" + m.Content
		} else {
			collated = append(collated, templateMessage{Role: m.Role, Content: m.Content, ToolCalls: calls})
		}
	}
	return collated, strings.Join(systems, "\n\n"), nil
}

// filterThinking is the one-shot result of the pinned thinking.Parser for the
// complete assistant content, as called by filterThinkTags. Partial opening
// tags and unterminated thinking yield no visible content.
func filterThinking(content string) string {
	trimmed := strings.TrimLeftFunc(content, unicode.IsSpace)
	if rest, ok := strings.CutPrefix(trimmed, "<think>"); ok {
		_, visible, closed := strings.Cut(rest, "</think>")
		if !closed {
			return ""
		}
		return strings.TrimLeftFunc(visible, unicode.IsSpace)
	}
	if strings.HasPrefix("<think>", trimmed) {
		return ""
	}
	return content
}

func decodeJSON(data []byte, value any, strict, allowNull bool) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("invalid UTF-8")
	}
	if loneSurrogate(data) {
		return errLoneSurrogate
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := jsoncheck.Check(d, jsoncheck.Options{FoldNames: true, RejectNull: !allowNull}); err != nil {
		return err
	}
	d = json.NewDecoder(bytes.NewReader(data))
	if strict {
		d.DisallowUnknownFields()
	}
	return d.Decode(value)
}

func pinned(path, want string) ([]byte, error) {
	if len(want) != 64 {
		return nil, fmt.Errorf("SHA-256 pin required")
	}
	f, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("input exceeds budget")
	}
	h := sha256.Sum256(data)
	if hex.EncodeToString(h[:]) != want {
		return nil, fmt.Errorf("input hash mismatch")
	}
	return data, nil
}

func run(args []string, out io.Writer) error {
	f := flag.NewFlagSet("llm-ollama-frame", flag.ContinueOnError)
	sp := f.String("model-show", "", "retained model-show JSON")
	sh := f.String("model-show-sha256", "", "required model-show byte hash")
	rp := f.String("request", "", "retained OpenAI request JSON")
	rh := f.String("request-sha256", "", "required request byte hash")
	version := f.String("provider-version", "", "observed provider version; must be 0.12.7")
	input := f.String("tokenizer-input", "", "explicit destination for sensitive rendered tokenizer input")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 || *version != "0.12.7" || *input == "" {
		return fmt.Errorf("pinned provider version and tokenizer destination required")
	}
	sb, err := pinned(*sp, *sh)
	if err != nil {
		return fmt.Errorf("model show: %w", err)
	}
	rb, err := pinned(*rp, *rh)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	var s show
	var r request
	if decodeJSON(sb, &s, false, true) != nil || decodeJSON(rb, &r, true, false) != nil {
		return fmt.Errorf("invalid JSON input")
	}
	text, think, err := render(s, r)
	if err != nil {
		return err
	}
	b, err := json.Marshal(struct {
		Text    string `json:"text"`
		Special bool   `json:"add_special_tokens"`
	}{text, false})
	if err != nil {
		return err
	}
	target, err := os.OpenFile(*input, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = target.Write(append(b, '\n'))
	closeErr := target.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	h := sha256.Sum256([]byte(text))
	th := sha256.Sum256([]byte(s.Template))
	return json.NewEncoder(out).Encode(map[string]any{"schema_version": 2, "provider_version": *version, "source_revision": sourceRevision, "model": r.Model, "model_show_sha256": *sh, "request_sha256": *rh, "template_sha256": hex.EncodeToString(th[:]), "rendered_sha256": hex.EncodeToString(h[:]), "reconstructed": reconstruct(r, think), "engine_observation_status": "unobserved", "context_policy": "untruncated-renderer-reconstruction", "tool_policy": "lossless-typed-or-refused", "scope": "offline pinned-source/template reconstruction; actual engine prompt not captured"})
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "llm-ollama-frame: refused")
		os.Exit(1)
	}
}
