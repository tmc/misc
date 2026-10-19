package traceworkload

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

const maxBytes = 16 << 20
const maxRows = 100000
const maxTokens = 1000000

// Row is a normalized input to examples/lib/workload.js realize and trace.
// SourceRow is the one-based CSV data-record ordinal, excluding the header.
// Offset is milliseconds since the source trace origin, without rebasing.
type Row struct {
	ID           string  `json:"id"`
	Offset       float64 `json:"offset"`
	InputTokens  int     `json:"inputTokens"`
	OutputBudget int     `json:"outputBudget"`
	SourceRow    int     `json:"sourceRow"`
}

// Trace retains the original source identity and conversion conventions.
// SourceHash is the lowercase SHA-256 of all input bytes, including whitespace.
type Trace struct {
	Format        string `json:"format"`
	SourceHash    string `json:"sourceHash"`
	TimestampUnit string `json:"timestampUnit"`
	OffsetUnit    string `json:"offsetUnit"`
	CountPolicy   string `json:"countPolicy"`
	Rows          []Row  `json:"rows"`
}

// ReadBurstGPT imports CSV with Timestamp, Request tokens and Response tokens
// columns. Timestamp is seconds; additional columns are retained in the source
// hash but do not affect targets. Duplicate headers and malformed rows fail the
// entire import. No records are skipped, sorted or capped silently.
//
// Limits are 16 MiB, 100000 rows, 1000000 tokens per count and one day of
// nonnegative monotonic timestamps. Equal timestamps and zero input counts are
// valid. Output counts must be positive for workload.js realization; zero output
// counts are unsupported. The caller retains the original input bytes.
func ReadBurstGPT(r io.Reader) (Trace, error) {
	var trace Trace
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return trace, fmt.Errorf("read burstgpt: %w", err)
	}
	if len(data) > maxBytes {
		return trace, fmt.Errorf("burstgpt exceeds byte limit")
	}
	reader := csv.NewReader(bytes.NewReader(data))
	header, err := reader.Read()
	if err != nil {
		return trace, fmt.Errorf("read burstgpt header: %w", err)
	}
	columns := map[string]int{}
	for i, name := range header {
		if name == "" {
			return trace, fmt.Errorf("burstgpt empty header")
		}
		if _, ok := columns[name]; ok {
			return trace, fmt.Errorf("burstgpt duplicate header %q", name)
		}
		columns[name] = i
	}
	for _, name := range []string{"Timestamp", "Request tokens", "Response tokens"} {
		if _, ok := columns[name]; !ok {
			return trace, fmt.Errorf("burstgpt missing column %q", name)
		}
	}
	trace = Trace{Format: "burstgpt-csv-v1", SourceHash: fmt.Sprintf("%x", sha256.Sum256(data)), TimestampUnit: "seconds", OffsetUnit: "milliseconds", CountPolicy: "declared-input-target-and-output-budget"}
	last := float64(0)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Trace{}, fmt.Errorf("read burstgpt record %d: %w", len(trace.Rows)+1, err)
		}
		if len(trace.Rows) == maxRows {
			return Trace{}, fmt.Errorf("burstgpt exceeds row limit")
		}
		timestamp := record[columns["Timestamp"]]
		stamp, err := strconv.ParseFloat(timestamp, 64)
		offset := stamp * 1000
		if err != nil || strings.ContainsAny(timestamp, "xXpP") || math.IsNaN(offset) || math.IsInf(offset, 0) || offset < last || offset < 0 || offset > 86400000 {
			return Trace{}, fmt.Errorf("burstgpt record %d invalid timestamp in seconds", len(trace.Rows)+1)
		}
		input, err := count(record[columns["Request tokens"]], 0)
		if err != nil {
			return Trace{}, fmt.Errorf("burstgpt record %d input: %w", len(trace.Rows)+1, err)
		}
		output, err := count(record[columns["Response tokens"]], 1)
		if err != nil {
			return Trace{}, fmt.Errorf("burstgpt record %d output: %w", len(trace.Rows)+1, err)
		}
		ordinal := len(trace.Rows) + 1
		trace.Rows = append(trace.Rows, Row{ID: trace.SourceHash + ":" + strconv.Itoa(ordinal), Offset: offset, InputTokens: input, OutputBudget: output, SourceRow: ordinal})
		last = offset
	}
	if len(trace.Rows) == 0 {
		return Trace{}, fmt.Errorf("burstgpt has no records")
	}
	return trace, nil
}

func count(text string, minimum int) (int, error) {
	if text == "" || strings.IndexFunc(text, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, fmt.Errorf("invalid decimal token count")
	}
	n, err := strconv.ParseUint(text, 10, 32)
	if err != nil || n < uint64(minimum) || n > maxTokens {
		return 0, fmt.Errorf("token count outside %d..%d", minimum, maxTokens)
	}
	return int(n), nil
}
