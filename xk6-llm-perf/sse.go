package llmperf

import (
    "bufio"
    "bytes"
    "io"
    "strings"
)

type SSEEvent struct {
    Event string
    Data  string
}

type SSEReader struct {
    scanner *bufio.Scanner
}

func NewSSEReader(r io.Reader) *SSEReader {
    return &SSEReader{
        scanner: bufio.NewScanner(r),
    }
}

func (r *SSEReader) Read() (*SSEEvent, error) {
    var event SSEEvent
    var data bytes.Buffer

    for r.scanner.Scan() {
        line := r.scanner.Text()
        if line == "" {
            if data.Len() > 0 {
                event.Data = strings.TrimSpace(data.String())
                return &event, nil
            }
            continue
        }

        if strings.HasPrefix(line, "event: ") {
            event.Event = strings.TrimPrefix(line, "event: ")
        } else if strings.HasPrefix(line, "data: ") {
            data.WriteString(strings.TrimPrefix(line, "data: "))
        }
    }

    if err := r.scanner.Err(); err != nil {
        return nil, err
    }

    if data.Len() > 0 {
        event.Data = strings.TrimSpace(data.String())
        return &event, nil
    }

    return nil, io.EOF
}

