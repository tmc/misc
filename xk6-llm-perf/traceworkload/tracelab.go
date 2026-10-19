package traceworkload

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ReadTraceLab reads uncompressed TraceLab JSONL without inferred subagent
// nesting. It groups sessions, orders by submission time then round index, and
// reproduces the pinned 64-token virtual prefix-block chain. Source counts remain
// separate from the upstream max(count,1) input/output targets. Reasoning is
// included in OutputBudget. Missing event timing and subagent lifecycle tools
// are refused rather than dropped or reconstructed heuristically.
func ReadTraceLab(r io.Reader) (SessionTrace, error) {
	return ReadTraceLabWithOptions(r, BlockOptions{BlockSize: 64})
}

// ReadTraceLabWithOptions reads TraceLab using an explicit virtual block size.
// The size controls metadata, prefix reuse and all aggregate expansion checks.
// It does not enable subagent reconstruction or prompt realization.
func ReadTraceLabWithOptions(r io.Reader, options BlockOptions) (SessionTrace, error) {
	if err := options.validate(); err != nil {
		return SessionTrace{}, err
	}
	trace, rows, err := readRows(r, "tracelab-jsonl-v1")
	if err != nil {
		return trace, err
	}
	trace.TimestampPolicy = "latest input event (or first output); session-relative rounded microseconds converted to milliseconds; origin Unix milliseconds"
	trace.CountPolicy = "raw counts retained; inputTarget=max(input,1); outputBudget=max(output+reasoning,1); timing proxies not server latency"
	trace.HashPolicy = fmt.Sprintf("source-defined virtual IDs in session-local namespace; %d-token whole blocks; no inferred subagent links", options.BlockSize)
	index := map[string]int{}
	for _, row := range rows {
		if err := tracelabRow(&trace, index, row, options.BlockSize); err != nil {
			return SessionTrace{}, fmt.Errorf("tracelab row %d: %w", row.ordinal, err)
		}
	}
	// Validate aggregate expansion before allocating the first virtual chain.
	if err := virtualBlockBudget(trace, rows); err != nil {
		return SessionTrace{}, err
	}
	for i := range trace.Sessions {
		s := &trace.Sessions[i]
		sort.SliceStable(s.Turns, func(a, b int) bool {
			left, right := s.Turns[a], s.Turns[b]
			if *left.Timestamp != *right.Timestamp {
				return *left.Timestamp < *right.Timestamp
			}
			return *left.RoundIndex < *right.RoundIndex
		})
		if *s.Turns[len(s.Turns)-1].Timestamp-*s.Turns[0].Timestamp > 86400 {
			return SessionTrace{}, fmt.Errorf("session timing exceeds one day")
		}
		origin := *s.Turns[0].Timestamp
		s.OriginUnixMS = pointer(origin * 1000)
		previous := []string{}
		next := 1
		var prevEnd *float64
		for j := range s.Turns {
			t := &s.Turns[j]
			at := *t.Timestamp - origin
			t.Timestamp = pointer(roundSeconds(at) * 1000)
			if prevEnd != nil {
				t.ThinkTime = pointer(roundSeconds(math.Max(at-*prevEnd, 0)) * 1000)
			}
			api := 0.0
			if t.APITime != nil {
				api = *t.APITime
				t.APITime = pointer(roundSeconds(api) * 1000)
			}
			prevEnd = pointer(at + api)
			total, prefix := *t.InputTokens, *t.PrefixTokens
			n := total / s.BlockSize
			reuse := min(prefix/s.BlockSize, len(previous), n)
			ids := append([]string{}, previous[:reuse]...)
			for len(ids) < n {
				ids = append(ids, strconv.Itoa(next))
				next++
			}
			t.HashIDs = ids
			previous = ids
		}
	}
	return trace, nil
}
func roundSeconds(v float64) float64 {
	n, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 6, 64), 64)
	return n
}
func tracelabRow(trace *SessionTrace, index map[string]int, row sourceRow, blockSize int) error {
	f := row.fields
	id, err := text(f, "session_id", true)
	if err != nil {
		return err
	}
	if err := sessionID(id); err != nil {
		return err
	}
	t := Turn{ID: strconv.Itoa(row.ordinal), SourceRow: row.ordinal, InputMode: "counts", Raw: row.raw}
	if t.RoundIndex, err = integer(f, "round_index", true, 0, 1000000); err != nil {
		return err
	}
	if t.InputTokens, err = integer(f, "input_tokens_total", true, 0, maxTokens); err != nil {
		return err
	}
	if t.PrefixTokens, err = integer(f, "prefix_tokens", true, 0, maxTokens); err != nil {
		return err
	}
	if t.NewlyAppendTokens, err = integer(f, "newly_append_tokens", true, 0, maxTokens); err != nil {
		return err
	}
	if *t.PrefixTokens+*t.NewlyAppendTokens != *t.InputTokens {
		return fmt.Errorf("prefix/new counts do not sum to total")
	}
	if t.OutputTokens, err = integer(f, "output_tokens", false, 0, maxTokens); err != nil {
		return err
	}
	if t.ReasoningTokens, err = integer(f, "reasoning_output_tokens", false, 0, maxTokens); err != nil {
		return err
	}
	t.InputTarget = pointer(max(*t.InputTokens, 1))
	output := 0
	if t.OutputTokens != nil {
		output += *t.OutputTokens
	}
	if t.ReasoningTokens != nil {
		output += *t.ReasoningTokens
	}
	if output > maxTokens {
		return fmt.Errorf("combined output exceeds token limit")
	}
	t.OutputBudget = pointer(max(output, 1))
	t.Model, err = text(f, "model", false)
	if err != nil {
		return err
	}
	if t.Model == "" {
		t.Model = "unknown"
	}
	var tools []map[string]json.RawMessage
	if raw, ok := f["tools"]; ok {
		if json.Unmarshal(raw, &tools) != nil || string(raw) == "null" {
			return fmt.Errorf("invalid tools")
		}
		for _, tool := range tools {
			name, e := text(tool, "tool_name", false)
			if e != nil {
				return e
			}
			switch name {
			case "Agent", "Task", "spawn_agent", "wait_agent", "close_agent":
				return fmt.Errorf("subagent lifecycle reconstruction is unsupported")
			}
		}
	}
	raw, ok := f["timing_events"]
	if !ok {
		return fmt.Errorf("missing timing_events")
	}
	var events []map[string]json.RawMessage
	if json.Unmarshal(raw, &events) != nil || len(events) == 0 || len(events) > 10000 {
		return fmt.Errorf("invalid timing_events")
	}
	t.Stop = "end_turn"
	t.InputType = "text"
	if kind, e := text(f, "first_input_event_type", false); e != nil {
		return e
	} else if kind == "tool_result" {
		t.InputType = "tool_result"
	}
	var ins, outs []float64
	for _, event := range events {
		kind, e := text(event, "event_type", true)
		if e != nil {
			return e
		}
		if kind == "tool_call" {
			t.Stop = "tool_use"
		}
		if _, ok := event["timestamp"]; !ok {
			continue
		}
		stamp, e := text(event, "timestamp", true)
		if e != nil {
			return e
		}
		if stamp == "" {
			continue
		}
		parsed, e := parseTimestamp(stamp)
		if e != nil {
			return e
		}
		switch kind {
		case "user_message", "tool_result":
			ins = append(ins, parsed)
		case "text", "reasoning", "tool_call":
			outs = append(outs, parsed)
		}
	}
	if len(ins) == 0 && len(outs) == 0 {
		return fmt.Errorf("no usable submission timing")
	}
	var submitted float64
	if len(ins) > 0 {
		submitted = ins[0]
		for _, n := range ins {
			submitted = math.Max(submitted, n)
		}
	} else {
		submitted = outs[0]
		for _, n := range outs {
			submitted = math.Min(submitted, n)
		}
	}
	t.Timestamp = pointer(submitted)
	if len(outs) > 0 {
		last := outs[0]
		for _, n := range outs {
			last = math.Max(last, n)
		}
		t.APITime = pointer(math.Max(last-submitted, 0))
	}
	if err := appendTurn(trace, index, id, "recorded", "session-virtual", blockSize, t); err != nil {
		return err
	}
	s := &trace.Sessions[index[id]]
	if len(s.Turns) > 1 && math.Abs(*s.Turns[0].Timestamp-submitted) > 86400 {
		return fmt.Errorf("session timing exceeds one day")
	}
	return nil
}
func parseTimestamp(s string) (float64, error) {
	// Python assumes UTC for naive timestamps. This bounded subset accepts
	// ISO date/time with at most microsecond precision, not arbitrary fromisoformat forms.
	if i := strings.IndexByte(s, '.'); i >= 0 {
		end := i + 1
		for end < len(s) && s[end] >= '0' && s[end] <= '9' {
			end++
		}
		if end-i-1 > 6 {
			return 0, fmt.Errorf("timestamp exceeds microsecond precision")
		}
	}
	layouts := []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999", "2006-01-02 15:04:05.999999"}
	for _, layout := range layouts {
		if t, e := time.Parse(layout, s); e == nil && t.Year() >= 1970 && t.Year() <= 2100 {
			return float64(t.Unix()) + float64(t.Nanosecond())/1e9, nil
		}
	}
	return 0, fmt.Errorf("invalid ISO timestamp")
}
