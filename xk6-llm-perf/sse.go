package llmperf

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type SSEEvent struct {
	Event string
	Data  string
}

type SSEReader struct{ scanner *bufio.Scanner }

func NewSSEReader(r io.Reader) *SSEReader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return &SSEReader{scanner: scanner}
}

func (r *SSEReader) Read() (*SSEEvent, error) {
	var event SSEEvent
	var data strings.Builder
	hasData := false
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			if hasData {
				event.Data = data.String()
				return &event, nil
			}
			event = SSEEvent{}
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			event.Event = value
		case "data":
			if hasData {
				data.WriteByte('\n')
			}
			data.WriteString(value)
			hasData = true
			if data.Len() > 4*1024*1024 {
				return nil, fmt.Errorf("SSE event exceeds 4 MiB")
			}
		}
	}
	if err := r.scanner.Err(); err != nil {
		return nil, err
	}
	if hasData {
		event.Data = data.String()
		return &event, nil
	}
	return nil, io.EOF
}
