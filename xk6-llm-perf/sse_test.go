package llmperf

import (
	"io"
	"strings"
	"testing"
)

func TestSSEReader(t *testing.T) {
	tests := []struct{ name, input, event, data string }{
		{"no space", "event:chunk\ndata:hello\n\n", "chunk", "hello"},
		{"multiline", "data: first\ndata: second\n\n", "", "first\nsecond"},
		{"whitespace", "data:  hello  \n\n", "", " hello  "},
		{"comments and CRLF", ":keepalive\r\ndata: hello\r\n\r\n", "", "hello"},
		{"empty data", "data:\n\n", "", ""},
		{"reset event", "event:ignored\n\ndata:hello\n\n", "", "hello"},
		{"EOF", "data:hello", "", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewSSEReader(strings.NewReader(tt.input))
			got, err := reader.Read()
			if err != nil {
				t.Fatal(err)
			}
			if got.Event != tt.event || got.Data != tt.data {
				t.Fatalf("Read = %#v; want event %q data %q", got, tt.event, tt.data)
			}
			if _, err := reader.Read(); err != io.EOF {
				t.Fatalf("next Read = %v; want EOF", err)
			}
		})
	}
}
