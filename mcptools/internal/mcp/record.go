package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Entry represents a single MCP message with direction.
type Entry struct {
	Dir  string          // "in" or "out"
	Data json.RawMessage // preserves original JSON-RPC 2.0 formatting
}

// Message represents a JSON-RPC 2.0 message.
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *json.Number    `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	} `json:"error,omitempty"`
}

// WriteTo writes an entry in the standard MCP recording format.
func (e *Entry) WriteTo(w io.Writer) error {
	_, err := fmt.Fprintf(w, "mcp-%s %s\n", e.Dir, e.Data)
	return err
}

// LoadRecording reads a recording file and returns all entries.
func LoadRecording(r io.Reader) ([]Entry, error) {
	var entries []Entry
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Bytes()
		if len(line) < 4 || !bytes.HasPrefix(line, []byte("mcp-")) {
			continue
		}

		parts := bytes.SplitN(line[4:], []byte(" "), 2)
		if len(parts) != 2 {
			continue
		}

		dir := string(parts[0])
		if dir != "in" && dir != "out" {
			continue
		}

		if !json.Valid(parts[1]) {
			continue
		}

		// Verify it's a valid JSON-RPC 2.0 message
		var msg Message
		if err := json.Unmarshal(parts[1], &msg); err != nil {
			continue
		}
		if msg.JSONRPC != "2.0" {
			continue
		}

		entries = append(entries, Entry{
			Dir:  dir,
			Data: json.RawMessage(parts[1]),
		})
	}
	return entries, s.Err()
}
