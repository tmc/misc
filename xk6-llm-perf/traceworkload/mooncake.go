package traceworkload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// ReadMooncake reads bounded Mooncake JSONL and groups rows in file order.
// Millisecond timestamps and delays are not rebased. Recorded messages/payloads
// remain independent; live messages and generated inputs retain their context
// policy. Counts are declarations, not tokenized prompts or observed output.
// Optional fields must be omitted rather than null. No rows are skipped.
func ReadMooncake(r io.Reader) (SessionTrace, error) {
	trace, rows, err := readRows(r, "mooncake-jsonl-v1")
	if err != nil {
		return trace, err
	}
	trace.TimestampPolicy = "source timestamp/delay milliseconds; no rebasing or delay cap"
	trace.CountPolicy = "declared counts retained; no prompt synthesis or output observation"
	trace.HashPolicy = "recorded signed integer IDs as decimal strings; file scope; default block size 512"
	index := map[string]int{}
	totalHashes := 0
	for _, row := range rows {
		if err := mooncakeRow(&trace, index, row, &totalHashes); err != nil {
			return SessionTrace{}, fmt.Errorf("mooncake row %d: %w", row.ordinal, err)
		}
	}
	return trace, nil
}
func mooncakeRow(trace *SessionTrace, index map[string]int, row sourceRow, totalHashes *int) error {
	f := row.fields
	if _, ok := f["message_mode"]; ok {
		return fmt.Errorf("message_mode is unsupported; use assistant_responses")
	}
	if raw, ok := f["type"]; ok && string(raw) != `"mooncake_trace"` {
		return fmt.Errorf("invalid dataset type")
	}
	id, err := text(f, "session_id", false)
	if err != nil {
		return err
	}
	if id == "" {
		id = trace.SourceHash + ":" + strconv.Itoa(row.ordinal)
	}
	if err := sessionID(id); err != nil {
		return err
	}
	t := Turn{ID: strconv.Itoa(row.ordinal), SourceRow: row.ordinal, Raw: row.raw}
	if t.Timestamp, err = number(f, "timestamp", false); err != nil {
		return err
	}
	if t.Delay, err = number(f, "delay", false); err != nil {
		return err
	}
	if t.InputTokens, err = integer(f, "input_length", false, 0, maxTokens); err != nil {
		return err
	}
	t.InputTarget = t.InputTokens
	if t.OutputTokens, err = integer(f, "output_length", false, 0, maxTokens); err != nil {
		return err
	}
	t.OutputBudget = t.OutputTokens
	if t.HashIDs, err = hashes(f, totalHashes); err != nil {
		return err
	}
	modes := 0
	if t.InputTokens != nil {
		modes++
		t.InputMode = "counts"
	}
	if _, ok := f["text_input"]; ok {
		s, e := text(f, "text_input", true)
		if e != nil {
			return e
		}
		t.TextInput = &s
		t.InputMode = "text"
		modes++
	}
	if raw, ok := f["messages"]; ok {
		var messages []map[string]json.RawMessage
		if json.Unmarshal(raw, &messages) != nil || len(messages) == 0 || len(messages) > 1000 {
			return fmt.Errorf("invalid messages")
		}
		for _, m := range messages {
			role, e := text(m, "role", true)
			if e != nil || role == "" {
				return fmt.Errorf("message requires role")
			}
		}
		t.Messages = raw
		t.InputMode = "messages"
		modes++
	}
	if raw, ok := f["payload"]; ok {
		var payload map[string]json.RawMessage
		if json.Unmarshal(raw, &payload) != nil || len(payload) == 0 {
			return fmt.Errorf("invalid payload")
		}
		t.Payload = raw
		t.InputMode = "payload"
		modes++
	}
	if modes != 1 {
		return fmt.Errorf("exactly one input mode is required")
	}
	if _, ok := f["hash_ids"]; ok && t.InputMode != "counts" {
		return fmt.Errorf("hash_ids require input_length")
	}
	mode, err := text(f, "assistant_responses", false)
	if err != nil {
		return err
	}
	if mode == "" {
		mode = "recorded"
	}
	if mode != "live" && mode != "recorded" {
		return fmt.Errorf("invalid assistant_responses")
	}
	if mode == "live" && t.InputMode != "messages" {
		return fmt.Errorf("live mode requires messages")
	}
	if t.InputMode == "counts" || t.InputMode == "text" {
		mode = "live"
	}
	if raw, ok := f["tools"]; ok {
		var tools []map[string]json.RawMessage
		if t.InputMode != "messages" || json.Unmarshal(raw, &tools) != nil || len(tools) == 0 {
			return fmt.Errorf("invalid tools")
		}
		t.Tools = raw
	}
	if raw, ok := f["extra"]; ok {
		var extra map[string]json.RawMessage
		if json.Unmarshal(raw, &extra) != nil || bytes.Equal(raw, []byte("null")) {
			return fmt.Errorf("invalid extra")
		}
		if mode == "live" && (extra["messages"] != nil || extra["input"] != nil) {
			return fmt.Errorf("live extra overrides history")
		}
		t.Extra = raw
	}
	return appendTurn(trace, index, id, mode, "file", 512, t)
}
