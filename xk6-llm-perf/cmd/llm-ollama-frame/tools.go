package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

// Tool admission refuses inputs that the pinned conversion, collation or
// template would accept but render differently from what was sent.
var (
	errToolSchema         = errors.New("tool schema not preserved by pinned decoding")
	errToolType           = errors.New("unsupported tool type")
	errToolsCapability    = errors.New("tools capability required")
	errContentWithCalls   = errors.New("content with tool calls")
	errMergedToolCalls    = errors.New("tool calls in merged assistant message")
	errToolCallRole       = errors.New("tool calls outside assistant message")
	errToolCallID         = errors.New("tool call id outside tool message")
	errToolCallType       = errors.New("unsupported tool call type")
	errToolCallName       = errors.New("empty tool call name")
	errEchoField          = errors.New("unsupported echo field")
	errToolArguments      = errors.New("invalid tool call arguments")
	errInexactNumber      = errors.New("number not exact in float64")
	errLoneSurrogate      = errors.New("unpaired UTF-16 surrogate escape")
	errToolBudgetExceeded = errors.New("tool budget exceeded")
)

// The tool types mirror the pinned api.Tool and api.ToolCall shapes. Field
// order, tags, PropertyType marshaling and pointer String methods determine
// the bytes the template prints for {{ .Function }} and
// {{ .Function.Arguments }}.

type tool struct {
	Type     string       `json:"type"`
	Items    any          `json:"items,omitempty"`
	Function toolFunction `json:"function"`
}

func (t tool) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}

type propertyType []string

func (pt *propertyType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*pt = []string{s}
		return nil
	}
	var a []string
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*pt = a
	return nil
}

func (pt propertyType) MarshalJSON() ([]byte, error) {
	if len(pt) == 1 {
		return json.Marshal(pt[0])
	}
	return json.Marshal([]string(pt))
}

func (pt propertyType) String() string {
	if len(pt) == 0 {
		return ""
	}
	if len(pt) == 1 {
		return pt[0]
	}
	return fmt.Sprintf("%v", []string(pt))
}

type toolProperty struct {
	AnyOf       []toolProperty `json:"anyOf,omitempty"`
	Type        propertyType   `json:"type,omitempty"`
	Items       any            `json:"items,omitempty"`
	Description string         `json:"description,omitempty"`
	Enum        []any          `json:"enum,omitempty"`
}

type toolFunctionParameters struct {
	Type       string                  `json:"type"`
	Defs       any                     `json:"$defs,omitempty"`
	Items      any                     `json:"items,omitempty"`
	Required   []string                `json:"required"`
	Properties map[string]toolProperty `json:"properties"`
}

func (t *toolFunctionParameters) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}

type toolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  toolFunctionParameters `json:"parameters"`
}

func (t *toolFunction) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}

type toolCall struct {
	Function toolCallFunction `json:"function"`
}

type toolCallFunction struct {
	Index     int                       `json:"index"`
	Name      string                    `json:"name"`
	Arguments toolCallFunctionArguments `json:"arguments"`
}

type toolCallFunctionArguments map[string]any

func (t *toolCallFunctionArguments) String() string {
	b, _ := json.Marshal(t)
	return string(b)
}

// admitTools decodes request tools as the pinned api.Tool and refuses any
// tool whose function object does not survive that decode and re-marshal
// with the same members, strings and float64-exact numbers. The template
// prints only the function, always labeled "function".
func admitTools(raw json.RawMessage) ([]tool, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		return nil, errToolSchema
	}
	if len(entries) > 64 {
		return nil, errToolBudgetExceeded
	}
	var tools []tool
	for _, e := range entries {
		var typ string
		if len(e) != 2 || e["function"] == nil || json.Unmarshal(e["type"], &typ) != nil {
			return nil, errToolSchema
		}
		if typ != "function" {
			return nil, errToolType
		}
		var t tool
		t.Type = typ
		if err := json.Unmarshal(e["function"], &t.Function); err != nil {
			return nil, errToolSchema
		}
		out, err := json.Marshal(&t.Function)
		if err != nil {
			return nil, errToolSchema
		}
		if err := sameJSON(e["function"], out); err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, nil
}

// admitToolCalls checks one message's tool fields and converts its calls as
// the pinned openai.FromCompletionToolCall does.
func admitToolCalls(m message) ([]toolCall, error) {
	if m.ToolCallID != "" && m.Role != "tool" {
		return nil, errToolCallID
	}
	if len(m.ToolCalls) == 0 {
		return nil, nil
	}
	if m.Role != "assistant" {
		return nil, errToolCallRole
	}
	if m.Content != "" {
		return nil, errContentWithCalls
	}
	calls := make([]toolCall, len(m.ToolCalls))
	for i, c := range m.ToolCalls {
		if c.Type != "function" {
			return nil, errToolCallType
		}
		if c.Function.Name == "" {
			return nil, errToolCallName
		}
		args, err := admitArguments(c.Function.Arguments)
		if err != nil {
			return nil, err
		}
		calls[i].Function.Name = c.Function.Name
		calls[i].Function.Arguments = args
	}
	return calls, nil
}

// admitArguments requires a JSON object without duplicate members, unpaired
// surrogate escapes or numbers that float64 would change.
func admitArguments(s string) (toolCallFunctionArguments, error) {
	if len(s) > 65536 {
		return nil, errToolBudgetExceeded
	}
	if loneSurrogate([]byte(s)) {
		return nil, errLoneSurrogate
	}
	d := json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	if jsoncheck.Check(d, jsoncheck.Options{}) != nil {
		return nil, errToolArguments
	}
	d = json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	var v any
	if d.Decode(&v) != nil {
		return nil, errToolArguments
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, errToolArguments
	}
	if err := exactNumbers(v); err != nil {
		return nil, err
	}
	var args toolCallFunctionArguments
	if err := json.Unmarshal([]byte(s), &args); err != nil {
		return nil, errToolArguments
	}
	return args, nil
}

func exactNumbers(v any) error {
	switch v := v.(type) {
	case json.Number:
		if !exactFloat64(string(v)) {
			return errInexactNumber
		}
	case []any:
		for _, e := range v {
			if err := exactNumbers(e); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, e := range v {
			if err := exactNumbers(e); err != nil {
				return err
			}
		}
	}
	return nil
}

// exactFloat64 reports whether the JSON number s denotes a float64 value
// exactly, so that decoding into float64 loses nothing.
func exactFloat64(s string) bool {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) {
		return false
	}
	// Bound the exponent before exact rational parsing.
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		e, err := strconv.Atoi(s[i+1:])
		if err != nil || e < -1100 || e > 400 {
			return false
		}
	}
	r, ok := new(big.Rat).SetString(s)
	return ok && r.Cmp(new(big.Rat).SetFloat64(f)) == 0
}

// sameJSON reports whether two JSON documents have the same members, array
// elements and strings, and numbers that are exact float64 values in a and
// equal in b. Member order and string escaping may differ.
func sameJSON(a, b []byte) error {
	var x, y any
	for _, p := range []struct {
		data []byte
		v    *any
	}{{a, &x}, {b, &y}} {
		d := json.NewDecoder(bytes.NewReader(p.data))
		d.UseNumber()
		if d.Decode(p.v) != nil {
			return errToolSchema
		}
	}
	return sameValue(x, y)
}

func sameValue(x, y any) error {
	switch x := x.(type) {
	case json.Number:
		y, ok := y.(json.Number)
		if !ok {
			return errToolSchema
		}
		if !exactFloat64(string(x)) {
			return errInexactNumber
		}
		fx, _ := strconv.ParseFloat(string(x), 64)
		fy, err := strconv.ParseFloat(string(y), 64)
		if err != nil || fx != fy {
			return errToolSchema
		}
	case []any:
		y, ok := y.([]any)
		if !ok || len(x) != len(y) {
			return errToolSchema
		}
		for i := range x {
			if err := sameValue(x[i], y[i]); err != nil {
				return err
			}
		}
	case map[string]any:
		y, ok := y.(map[string]any)
		if !ok || len(x) != len(y) {
			return errToolSchema
		}
		for k, v := range x {
			w, ok := y[k]
			if !ok {
				return errToolSchema
			}
			if err := sameValue(v, w); err != nil {
				return err
			}
		}
	default:
		if x != y {
			return errToolSchema
		}
	}
	return nil
}

// loneSurrogate reports whether JSON text contains a \u escape for a UTF-16
// surrogate that is not part of a valid pair. encoding/json silently decodes
// such escapes as U+FFFD.
func loneSurrogate(data []byte) bool {
	hex := func(i int) (rune, bool) {
		if i+6 > len(data) || data[i] != '\\' || data[i+1] != 'u' {
			return 0, false
		}
		v, err := strconv.ParseUint(string(data[i+2:i+6]), 16, 16)
		return rune(v), err == nil
	}
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' || i+1 >= len(data) {
			continue
		}
		if data[i+1] != 'u' {
			i++ // skip the escaped character
			continue
		}
		r, ok := hex(i)
		if !ok {
			continue
		}
		switch {
		case r >= 0xd800 && r < 0xdc00:
			lo, ok := hex(i + 6)
			if !ok || lo < 0xdc00 || lo >= 0xe000 {
				return true
			}
			i += 11
		case r >= 0xdc00 && r < 0xe000:
			return true
		default:
			i += 5
		}
	}
	return false
}
