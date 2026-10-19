package traceworkload

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
)

type bailianRow struct {
	id, parent int64
	order      int
	turn       Turn
}

// ReadBailian reads bounded Bailian JSONL. Parent links define sessions and
// stable turn-number sorting defines replay order. Source seconds become
// milliseconds; each row remains a full request, not a delta. Duplicate chat
// IDs, dangling parents and cycles are refused instead of salvaged.
func ReadBailian(r io.Reader) (SessionTrace, error) {
	trace, rows, err := readRows(r, "bailian-jsonl-v1")
	if err != nil {
		return trace, err
	}
	trace.TimestampPolicy = "source seconds converted to milliseconds; stable turn order; no rebasing"
	trace.CountPolicy = "full-request declared counts; recorded context; no synthesis or capping"
	trace.HashPolicy = "recorded signed integer IDs as decimal strings; file scope; 16-token blocks"
	items := make([]bailianRow, 0, len(rows))
	totalHashes := 0
	byID := map[int64]int{}
	for _, row := range rows {
		f := row.fields
		id, e := bailianID(f, "chat_id", false)
		if e != nil {
			return SessionTrace{}, fmt.Errorf("bailian row %d: %w", row.ordinal, e)
		}
		parent, e := bailianID(f, "parent_chat_id", true)
		if e != nil {
			return SessionTrace{}, fmt.Errorf("bailian row %d: %w", row.ordinal, e)
		}
		if _, ok := byID[id]; ok {
			return SessionTrace{}, fmt.Errorf("bailian duplicate chat_id %d", id)
		}
		stamp, e := number(f, "timestamp", true)
		if e != nil || *stamp > 86400 {
			return SessionTrace{}, fmt.Errorf("bailian row %d invalid timestamp seconds", row.ordinal)
		}
		*stamp *= 1000
		in, e := integer(f, "input_length", true, 0, maxTokens)
		if e != nil {
			return SessionTrace{}, e
		}
		output, e := integer(f, "output_length", true, 0, maxTokens)
		if e != nil {
			return SessionTrace{}, e
		}
		order, e := integer(f, "turn", false, 1, 1000)
		if e != nil {
			return SessionTrace{}, e
		}
		if order == nil {
			order = pointer(1)
		}
		h, e := hashes(f, &totalHashes)
		if e != nil {
			return SessionTrace{}, e
		}
		if f["type"] != nil && f["request_type"] != nil {
			return SessionTrace{}, fmt.Errorf("ambiguous Bailian request type aliases")
		}
		typeKey := "type"
		if f["request_type"] != nil {
			typeKey = "request_type"
		}
		kind, e := text(f, typeKey, false)
		if e != nil {
			return SessionTrace{}, e
		}
		if kind != "" && kind != "text" && kind != "search" && kind != "image" && kind != "file" {
			return SessionTrace{}, fmt.Errorf("unsupported Bailian request type")
		}
		t := Turn{ID: strconv.FormatInt(id, 10), SourceRow: row.ordinal, Timestamp: stamp, InputTokens: in, InputTarget: in, OutputTokens: output, OutputBudget: output, HashIDs: h, RoundIndex: order, InputMode: "counts", RequestType: kind, ParentID: pointer(strconv.FormatInt(parent, 10)), Raw: row.raw}
		byID[id] = len(items)
		items = append(items, bailianRow{id, parent, *order, t})
	}
	roots := map[int64]int64{}
	for _, item := range items {
		current := item.id
		path := []int64{}
		seen := map[int64]bool{}
		for {
			if _, ok := roots[current]; ok {
				current = roots[current]
				break
			}
			if seen[current] {
				return SessionTrace{}, fmt.Errorf("bailian parent cycle")
			}
			seen[current] = true
			path = append(path, current)
			i, ok := byID[current]
			if !ok {
				return SessionTrace{}, fmt.Errorf("bailian missing parent %d", current)
			}
			if items[i].parent == -1 {
				break
			}
			current = items[i].parent
		}
		for _, id := range path {
			roots[id] = current
		}
	}
	index := map[string]int{}
	for _, item := range items {
		if err := appendTurn(&trace, index, strconv.FormatInt(roots[item.id], 10), "recorded", "file", 16, item.turn); err != nil {
			return SessionTrace{}, err
		}
	}
	for i := range trace.Sessions {
		sort.SliceStable(trace.Sessions[i].Turns, func(a, b int) bool {
			return *trace.Sessions[i].Turns[a].RoundIndex < *trace.Sessions[i].Turns[b].RoundIndex
		})
	}
	return trace, nil
}
func bailianID(f map[string]json.RawMessage, key string, optional bool) (int64, error) {
	raw, ok := f[key]
	if !ok && optional {
		return -1, nil
	}
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	n, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || n < 0 && !(optional && n == -1) {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n, nil
}
