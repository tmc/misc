package traceworkload

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

// BlockOptions declares the token granularity of each trace hash block.
// BlockSize must be in 1..1000000. The zero value is invalid: use the readers
// without options for format defaults. The upper bound is a local resource
// limit, not an upstream limit. Block scope remains the reader's stated scope.
type BlockOptions struct {
	BlockSize int
}

func (o BlockOptions) validate() error {
	if o.BlockSize < 1 || o.BlockSize > maxTokens {
		return fmt.Errorf("block size must be 1..%d", maxTokens)
	}
	return nil
}

// SessionTrace retains grouped trace input without synthesizing prompt text.
// TimestampPolicy, CountPolicy and HashPolicy describe conversions explicitly.
// Raw rows preserve the source fields; SourceHash identifies the complete input.
type SessionTrace struct {
	Format          string    `json:"format"`
	SourceHash      string    `json:"sourceHash"`
	TimestampPolicy string    `json:"timestampPolicy"`
	CountPolicy     string    `json:"countPolicy"`
	HashPolicy      string    `json:"hashPolicy"`
	Sessions        []Session `json:"sessions"`
}

// Session holds ordered turns. ContextMode is recorded or live; generated
// prompts require realization before execution. Hash IDs have the stated scope.
type Session struct {
	ID           string   `json:"session_id"`
	ContextMode  string   `json:"contextMode"`
	BlockSize    int      `json:"blockSize"`
	HashScope    string   `json:"hashScope"`
	OriginUnixMS *float64 `json:"originUnixMS"`
	Turns        []Turn   `json:"turns"`
}

// Turn retains source counts separately from adjusted generation targets.
// Timestamp and Delay are milliseconds under the trace's explicit policy.
// HashIDs are decimal strings, preserving integer identity through JavaScript.
// APITime and ThinkTime are derived timing proxies in milliseconds, not observed
// server latency. Raw is the original source row, including ancillary fields.
type Turn struct {
	Stop              string          `json:"stop,omitempty"`
	InputType         string          `json:"inputType,omitempty"`
	ID                string          `json:"id"`
	SourceRow         int             `json:"sourceRow"`
	Timestamp         *float64        `json:"timestamp"`
	Delay             *float64        `json:"delay"`
	InputTokens       *int            `json:"inputTokens"`
	OutputTokens      *int            `json:"outputTokens"`
	InputTarget       *int            `json:"inputTarget"`
	OutputBudget      *int            `json:"outputBudget"`
	PrefixTokens      *int            `json:"prefixTokens"`
	NewlyAppendTokens *int            `json:"newlyAppendTokens"`
	ReasoningTokens   *int            `json:"reasoningTokens"`
	HashIDs           []string        `json:"hashIDs"`
	RoundIndex        *int            `json:"roundIndex"`
	Model             string          `json:"model,omitempty"`
	RequestType       string          `json:"requestType,omitempty"`
	ParentID          *string         `json:"parentID"`
	APITime           *float64        `json:"apiTime"`
	ThinkTime         *float64        `json:"thinkTime"`
	InputMode         string          `json:"inputMode"`
	TextInput         *string         `json:"textInput,omitempty"`
	Messages          json.RawMessage `json:"messages,omitempty"`
	Payload           json.RawMessage `json:"payload,omitempty"`
	Tools             json.RawMessage `json:"tools,omitempty"`
	Extra             json.RawMessage `json:"extra,omitempty"`
	Raw               json.RawMessage `json:"raw"`
}

const (
	maxCacheBlocks      = 262144
	maxTraceOutputBytes = 64 << 20
	traceRowBytes       = 4096
	traceRawExpansion   = 24
	traceBlockBytes     = 24
)

type sourceRow struct {
	fields  map[string]json.RawMessage
	raw     json.RawMessage
	ordinal int
}

func readRows(r io.Reader, format string) (SessionTrace, []sourceRow, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return SessionTrace{}, nil, fmt.Errorf("read %s: %w", format, err)
	}
	if len(data) > maxBytes || !utf8.Valid(data) {
		return SessionTrace{}, nil, fmt.Errorf("%s exceeds byte limit or contains invalid UTF-8", format)
	}
	trace := SessionTrace{Format: format, SourceHash: fmt.Sprintf("%x", sha256.Sum256(data))}
	var rows []sourceRow
	estimatedBytes := traceRowBytes
	for i, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if len(line) > 1<<20 || len(rows) == maxRows {
			return SessionTrace{}, nil, fmt.Errorf("%s row %d exceeds row/line limit", format, i+1)
		}
		estimatedBytes += traceRowBytes + traceRawExpansion*len(line)
		if estimatedBytes > maxTraceOutputBytes {
			return SessionTrace{}, nil, fmt.Errorf("%s aggregate output budget exceeds 64 MiB", format)
		}
		d := json.NewDecoder(bytes.NewReader(line))
		d.UseNumber()
		if err := jsoncheck.Check(d, jsoncheck.Options{}); err != nil {
			return SessionTrace{}, nil, fmt.Errorf("%s row %d: %w", format, i+1, err)
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(line, &fields) != nil || fields == nil {
			return SessionTrace{}, nil, fmt.Errorf("%s row %d requires an object", format, i+1)
		}
		// encoding/json replaces lone escaped surrogates. Refuse those before any
		// decoded field can silently acquire a different identity.
		if !validEscapes(line) {
			return SessionTrace{}, nil, fmt.Errorf("%s row %d invalid Unicode escape", format, i+1)
		}
		rows = append(rows, sourceRow{fields: fields, raw: append(json.RawMessage(nil), line...), ordinal: i + 1})
	}
	if len(rows) == 0 {
		return SessionTrace{}, nil, fmt.Errorf("%s has no rows", format)
	}
	return trace, rows, nil
}

func validEscapes(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] != '"' {
			continue
		}
		for i++; i < len(data) && data[i] != '"'; i++ {
			if data[i] != '\\' {
				continue
			}
			i++
			if i >= len(data) {
				return false
			}
			if data[i] != 'u' {
				continue
			}
			if i+4 >= len(data) {
				return false
			}
			code, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
			if err != nil {
				return false
			}
			i += 4
			if code >= 0xDC00 && code <= 0xDFFF {
				return false
			}
			if code < 0xD800 || code > 0xDBFF {
				continue
			}
			if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
			if err != nil || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			i += 6
		}
	}
	return true
}

func integer(fields map[string]json.RawMessage, key string, required bool, minimum, maximum int) (*int, error) {
	raw, ok := fields[key]
	if !ok {
		if required {
			return nil, fmt.Errorf("missing %s", key)
		}
		return nil, nil
	}
	text := string(raw)
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || n < int64(minimum) || n > int64(maximum) {
		return nil, fmt.Errorf("invalid %s integer", key)
	}
	v := int(n)
	return &v, nil
}
func number(fields map[string]json.RawMessage, key string, required bool) (*float64, error) {
	raw, ok := fields[key]
	if !ok {
		if required {
			return nil, fmt.Errorf("missing %s", key)
		}
		return nil, nil
	}
	if len(raw) > 128 || len(raw) == 0 || raw[0] == '"' || raw[0] == 'n' {
		return nil, fmt.Errorf("invalid %s number", key)
	}
	n, err := strconv.ParseFloat(string(raw), 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 86400000 {
		return nil, fmt.Errorf("invalid %s number", key)
	}
	return &n, nil
}
func text(fields map[string]json.RawMessage, key string, required bool) (string, error) {
	raw, ok := fields[key]
	if !ok {
		if required {
			return "", fmt.Errorf("missing %s", key)
		}
		return "", nil
	}
	var s string
	if json.Unmarshal(raw, &s) != nil || bytes.Equal(raw, []byte("null")) || len(s) > 262144 {
		return "", fmt.Errorf("invalid %s string", key)
	}
	return s, nil
}
func hashes(fields map[string]json.RawMessage, total *int) ([]string, error) {
	raw, ok := fields["hash_ids"]
	if !ok {
		return nil, nil
	}
	scan := func(emit func(int64)) (int, error) {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		token, err := d.Token()
		if err != nil || token != json.Delim('[') {
			return 0, fmt.Errorf("invalid hash_ids")
		}
		count := 0
		for d.More() {
			token, err := d.Token()
			if err != nil {
				return 0, err
			}
			number, ok := token.(json.Number)
			if !ok {
				return 0, fmt.Errorf("invalid hash_id integer")
			}
			n, err := strconv.ParseInt(string(number), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid hash_id integer")
			}
			count++
			if count > maxCacheBlocks-*total {
				return 0, fmt.Errorf("aggregate cache block budget exceeds %d", maxCacheBlocks)
			}
			if emit != nil {
				emit(n)
			}
		}
		if _, err := d.Token(); err != nil {
			return 0, err
		}
		return count, nil
	}
	count, err := scan(nil)
	if err != nil {
		return nil, err
	}
	// Count the complete row before allocating the normalized ID slice.
	result := make([]string, 0, count)
	if _, err := scan(func(n int64) { result = append(result, strconv.FormatInt(n, 10)) }); err != nil {
		return nil, err
	}
	*total += count
	return result, nil
}

func virtualBlockBudget(trace SessionTrace, rows []sourceRow) error {
	estimated := traceRowBytes
	for _, row := range rows {
		estimated += traceRowBytes + traceRawExpansion*len(row.raw)
	}
	blocks := 0
	for _, session := range trace.Sessions {
		for _, turn := range session.Turns {
			n := *turn.InputTokens / session.BlockSize
			if n > maxCacheBlocks-blocks {
				return fmt.Errorf("aggregate virtual block budget exceeds %d", maxCacheBlocks)
			}
			blocks += n
			if traceBlockBytes*n > maxTraceOutputBytes-estimated {
				return fmt.Errorf("aggregate virtual block output budget exceeds 64 MiB")
			}
			estimated += traceBlockBytes * n
		}
	}
	return nil
}

func sessionID(id string) error {
	if id == "" || len(id) > 256 || strings.ContainsAny(id, "\x00\r\n") {
		return fmt.Errorf("invalid session_id")
	}
	return nil
}
func pointer[T any](v T) *T { return &v }
func appendTurn(trace *SessionTrace, index map[string]int, id, mode, scope string, block int, turn Turn) error {
	i, ok := index[id]
	if !ok {
		if len(trace.Sessions) == 10000 {
			return fmt.Errorf("session count exceeds limit")
		}
		i = len(trace.Sessions)
		index[id] = i
		trace.Sessions = append(trace.Sessions, Session{ID: id, ContextMode: mode, HashScope: scope, BlockSize: block})
	}
	s := &trace.Sessions[i]
	if s.ContextMode != mode {
		return fmt.Errorf("session %q mixes context modes", id)
	}
	if len(s.Turns) == 1000 {
		return fmt.Errorf("session %q exceeds turn limit", id)
	}
	if len(s.Turns) > 0 && s.Turns[0].InputMode != turn.InputMode {
		return fmt.Errorf("session %q mixes input modes", id)
	}
	s.Turns = append(s.Turns, turn)
	return nil
}
