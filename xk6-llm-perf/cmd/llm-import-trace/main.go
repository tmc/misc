// Command llm-import-trace normalizes bounded vendor JSONL trace sessions.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tmc/misc/xk6-llm-perf/traceworkload"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "llm-import-trace:", err)
		os.Exit(1)
	}
}

type replayRequest struct {
	Path     string                     `json:"path"`
	Protocol string                     `json:"protocol"`
	Body     map[string]json.RawMessage `json:"body"`
}
type replayTurn struct {
	ID      string             `json:"id"`
	Request replayRequest      `json:"request"`
	Source  traceworkload.Turn `json:"source"`
}
type replaySession struct {
	BlockSize       int          `json:"block_size"`
	HashScope       string       `json:"hash_scope"`
	ID              string       `json:"session_id"`
	SourceHash      string       `json:"source_sha256"`
	SourcePolicy    string       `json:"source_policy"`
	TimestampPolicy string       `json:"timestamp_policy"`
	Turns           []replayTurn `json:"turns"`
}

func run(args []string, in io.Reader, out io.Writer) error {
	flags := flag.NewFlagSet("llm-import-trace", flag.ContinueOnError)
	blockSize := flags.Int("block-size", 0, "explicit hash block tokens for mooncake/tracelab (format default when omitted)")
	format := flags.String("format", "", "mooncake, bailian or tracelab JSONL")
	output := flags.String("output", "normalized", "normalized or sessions")
	policy := flags.String("execution-policy", "", "sessions requires serial-recorded (original timing is metadata only)")
	path := flags.String("path", "", "API path for sessions export")
	protocol := flags.String("protocol", "", "json or openai-chat response adapter")
	model := flags.String("model", "", "explicit model for authored messages/text; does not rewrite payloads")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || (*output != "normalized" && *output != "sessions") {
		return fmt.Errorf("usage: llm-import-trace -format mooncake|bailian|tracelab [-output sessions -execution-policy serial-recorded -path /chat/completions -protocol openai-chat]")
	}
	if *output == "normalized" && (*policy != "" || *path != "" || *protocol != "" || *model != "") {
		return fmt.Errorf("execution flags require sessions output")
	}
	blockSet := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "block-size" {
			blockSet = true
		}
	})
	if blockSet && (*blockSize < 1 || *blockSize > 1000000) {
		return fmt.Errorf("block size must be 1..1000000")
	}
	var trace traceworkload.SessionTrace
	var err error
	switch *format {
	case "mooncake":
		if blockSet {
			trace, err = traceworkload.ReadMooncakeWithOptions(in, traceworkload.BlockOptions{BlockSize: *blockSize})
		} else {
			trace, err = traceworkload.ReadMooncake(in)
		}
	case "bailian":
		if blockSet {
			return fmt.Errorf("block-size override is unsupported for bailian; default is 16")
		}
		trace, err = traceworkload.ReadBailian(in)
	case "tracelab":
		if blockSet {
			trace, err = traceworkload.ReadTraceLabWithOptions(in, traceworkload.BlockOptions{BlockSize: *blockSize})
		} else {
			trace, err = traceworkload.ReadTraceLab(in)
		}
	default:
		return fmt.Errorf("unsupported format")
	}
	if err != nil {
		return err
	}
	var value any = trace
	if *output == "sessions" {
		if !utf8.ValidString(*path) || !utf8.ValidString(*model) || *policy != "serial-recorded" || len(*path) > 2048 || !strings.HasPrefix(*path, "/") || strings.HasPrefix(*path, "//") || strings.ContainsAny(*path, "?#\\ \r\n\t") || strings.Contains(*path, "..") || (*protocol != "json" && *protocol != "openai-chat") || len(*model) > 1024 {
			return fmt.Errorf("sessions requires explicit serial-recorded policy, valid path and json/openai-chat protocol")
		}
		value, err = export(trace, *path, *protocol, *model)
		if err != nil {
			return err
		}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode trace: %w", err)
	}
	if len(data) > 64<<20 {
		return fmt.Errorf("output exceeds 64 MiB")
	}
	if _, err := out.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write trace: %w", err)
	}
	return nil
}

func export(trace traceworkload.SessionTrace, path, protocol, model string) ([]replaySession, error) {
	// Reserve the source-reader expansion budget plus repeated caller flags
	// before constructing any replay body. Long paths/models are per-turn data.
	estimated := 4096
	for _, session := range trace.Sessions {
		for _, turn := range session.Turns {
			n := 4096 + 24*(len(turn.Raw)+len(path)+len(model))
			if n > (64<<20)-estimated {
				return nil, fmt.Errorf("aggregate replay output budget exceeds 64 MiB")
			}
			estimated += n
		}
	}
	result := make([]replaySession, 0, len(trace.Sessions))
	for _, s := range trace.Sessions {
		if s.ContextMode != "recorded" && !(len(s.Turns) == 1 && s.Turns[0].InputMode == "text") {
			return nil, fmt.Errorf("session %q requires live context assembly; execution unsupported", s.ID)
		}
		row := replaySession{ID: s.ID, BlockSize: s.BlockSize, HashScope: s.HashScope, SourceHash: trace.SourceHash, SourcePolicy: "serial-recorded authored-body replay; source arrival/delay not scheduled; no cache realization", TimestampPolicy: trace.TimestampPolicy}
		for _, t := range s.Turns {
			if t.InputMode == "counts" || len(t.HashIDs) > 0 {
				return nil, fmt.Errorf("turn %s is count/cache-only; realize against pinned tokenizer assets before execution", t.ID)
			}
			body := map[string]json.RawMessage{}
			if t.InputMode == "payload" {
				if t.Extra != nil {
					return nil, fmt.Errorf("payload extra merge is unsupported")
				}
				if err := json.Unmarshal(t.Payload, &body); err != nil {
					return nil, err
				}
			} else {
				if protocol != "openai-chat" || model == "" {
					return nil, fmt.Errorf("authored text/messages requires openai-chat and explicit model")
				}
				if t.InputMode == "messages" {
					body["messages"] = t.Messages
				} else {
					body["messages"], _ = json.Marshal([]map[string]string{{"role": "user", "content": *t.TextInput}})
				}
				body["model"], _ = json.Marshal(model)
				if t.Tools != nil {
					body["tools"] = t.Tools
				}
				if t.OutputBudget != nil {
					if *t.OutputBudget == 0 {
						return nil, fmt.Errorf("zero output budget is not executable")
					}
					body["max_tokens"], _ = json.Marshal(*t.OutputBudget)
				}
				if t.Extra != nil {
					var extra map[string]json.RawMessage
					if err := json.Unmarshal(t.Extra, &extra); err != nil {
						return nil, err
					}
					for k, v := range extra {
						body[k] = v
					}
				}
			}
			b, _ := json.Marshal(body)
			d := json.NewDecoder(bytes.NewReader(b))
			d.UseNumber()
			var decoded any
			if err := d.Decode(&decoded); err != nil {
				return nil, err
			}
			if err := safeNumbers(decoded); err != nil {
				return nil, fmt.Errorf("turn %s: %w", t.ID, err)
			}
			row.Turns = append(row.Turns, replayTurn{ID: t.ID, Request: replayRequest{Path: path, Protocol: protocol, Body: body}, Source: t})
		}
		result = append(result, row)
	}
	return result, nil
}
func safeNumbers(value any) error {
	switch v := value.(type) {
	case json.Number:
		if len(v) > 128 {
			return fmt.Errorf("number spelling exceeds budget")
		}
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) || math.Trunc(f) == f && math.Abs(f) > 9007199254740991 {
			return fmt.Errorf("number exceeds JavaScript range")
		}
		if f == 0 {
			m, _, _ := strings.Cut(strings.ToLower(string(v)), "e")
			if math.Signbit(f) || strings.Trim(m, "0.+-") != "" {
				return fmt.Errorf("number loses precision")
			}
			return nil
		}
		a, ok := new(big.Rat).SetString(string(v))
		if !ok {
			return fmt.Errorf("invalid number")
		}
		b, ok := new(big.Rat).SetString(strconv.FormatFloat(f, 'g', -1, 64))
		if !ok || a.Cmp(b) != 0 {
			return fmt.Errorf("number loses precision")
		}
	case []any:
		for _, item := range v {
			if err := safeNumbers(item); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range v {
			if err := safeNumbers(item); err != nil {
				return err
			}
		}
	}
	return nil
}
