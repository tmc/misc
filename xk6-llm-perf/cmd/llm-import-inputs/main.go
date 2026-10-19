// Command llm-import-inputs converts bounded AIPerf inputs.json sessions for k6.
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
	"math/big"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

const maxInput = 16 << 20

type request struct {
	Path     string          `json:"path"`
	Protocol string          `json:"protocol"`
	Body     json.RawMessage `json:"body"`
}
type turn struct {
	ID      string  `json:"id"`
	Request request `json:"request"`
}
type session struct {
	ID           string `json:"session_id"`
	SourceSHA256 string `json:"source_sha256"`
	Turns        []turn `json:"turns"`
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "llm-import-inputs:", err)
		os.Exit(1)
	}
}

func run(args []string, in io.Reader, out io.Writer) error {
	flags := flag.NewFlagSet("llm-import-inputs", flag.ContinueOnError)
	path := flags.String("path", "", "API path for every authored payload")
	protocol := flags.String("protocol", "", "response adapter: json or openai-chat")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || len(*path) > 2048 || !strings.HasPrefix(*path, "/") || strings.HasPrefix(*path, "//") || strings.ContainsAny(*path, "?#\\") || strings.Contains(*path, "..") || strings.ContainsAny(*path, "\r\n\t ") || (*protocol != "json" && *protocol != "openai-chat") {
		return fmt.Errorf("usage: llm-import-inputs -path /chat/completions -protocol json|openai-chat < inputs.json")
	}
	data, err := io.ReadAll(io.LimitReader(in, maxInput+1))
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	if len(data) > maxInput {
		return fmt.Errorf("input exceeds 16 MiB")
	}
	if !utf8.Valid(data) || !validStrings(data) {
		return fmt.Errorf("input contains invalid Unicode")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := jsoncheck.Check(d, jsoncheck.Options{}); err != nil {
		return fmt.Errorf("check input: %w", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	if len(root) != 1 || root["data"] == nil {
		return fmt.Errorf("input requires only data")
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(root["data"], &entries); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	if len(entries) < 1 || len(entries) > 10000 {
		return fmt.Errorf("data requires 1..10000 sessions")
	}
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	seen := make(map[string]bool)
	result := make([]session, 0, len(entries))
	total := 0
	for i, entry := range entries {
		var id string
		if len(entry) != 2 || entry["session_id"] == nil || entry["payloads"] == nil {
			return fmt.Errorf("data[%d] requires only session_id and payloads", i)
		}
		if err := json.Unmarshal(entry["session_id"], &id); err != nil || id == "" || len(id) > 256 || strings.ContainsAny(id, "\x00\r\n") {
			return fmt.Errorf("data[%d] invalid session_id", i)
		}
		if seen[id] {
			return fmt.Errorf("data[%d] duplicate session_id", i)
		}
		seen[id] = true
		var payloads []json.RawMessage
		if err := json.Unmarshal(entry["payloads"], &payloads); err != nil {
			return fmt.Errorf("data[%d] invalid payloads", i)
		}
		total += len(payloads)
		if len(payloads) < 1 || len(payloads) > 1000 || total > 100000 {
			return fmt.Errorf("data[%d] exceeds turn bounds (1..1000/session, 100000 total)", i)
		}
		s := session{ID: id, SourceSHA256: hash, Turns: make([]turn, 0, len(payloads))}
		for j, payload := range payloads {
			var body map[string]any
			d := json.NewDecoder(bytes.NewReader(payload))
			d.UseNumber()
			if len(payload) > 1<<20 || d.Decode(&body) != nil || body == nil {
				return fmt.Errorf("data[%d] payloads[%d] requires an object of at most 1 MiB", i, j)
			}
			if err := numbers(body); err != nil {
				return fmt.Errorf("data[%d] payloads[%d]: %w", i, j, err)
			}
			s.Turns = append(s.Turns, turn{ID: strconv.Itoa(j), Request: request{Path: *path, Protocol: *protocol, Body: payload}})
		}
		result = append(result, s)
	}
	// Buffer the bounded result so a validation failure never emits partial output.
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode sessions: %w", err)
	}
	encoded = append(encoded, '\n')
	if _, err := out.Write(encoded); err != nil {
		return fmt.Errorf("write sessions: %w", err)
	}
	return nil
}

// numbers refuses values whose decimal meaning would change after JavaScript
// number decoding and Go JSON serialization in Client.request.
func numbers(v any) error {
	switch v := v.(type) {
	case json.Number:
		if len(v) > 128 {
			return fmt.Errorf("number spelling exceeds budget")
		}
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return fmt.Errorf("number is outside JavaScript range")
		}
		if math.Trunc(f) == f && math.Abs(f) > 9007199254740991 {
			return fmt.Errorf("integer exceeds JavaScript safe range")
		}
		if f == 0 {
			mantissa, _, _ := strings.Cut(strings.ToLower(string(v)), "e")
			if math.Signbit(f) || strings.Trim(mantissa, "0.+-") != "" {
				return fmt.Errorf("number changes through JavaScript serialization")
			}
			return nil
		}
		want, ok := new(big.Rat).SetString(string(v))
		if !ok {
			return fmt.Errorf("invalid decimal number")
		}
		got, ok := new(big.Rat).SetString(strconv.FormatFloat(f, 'g', -1, 64))
		if !ok || want.Cmp(got) != 0 || f == 0 && math.Signbit(f) {
			return fmt.Errorf("number changes through JavaScript serialization")
		}
	case []any:
		for _, item := range v {
			if err := numbers(item); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range v {
			if err := numbers(item); err != nil {
				return err
			}
		}
	}
	return nil
}

// validStrings rejects lone UTF-16 surrogate escapes that encoding/json would
// otherwise replace. Valid surrogate pairs retain their Unicode meaning.
func validStrings(data []byte) bool {
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
