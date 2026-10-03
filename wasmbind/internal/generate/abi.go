package generate

import (
	"encoding/json"
	"fmt"
	"go/format"
	"strings"

	"github.com/tmc/misc/wasmbind/internal/schema"
)

// abiSource keeps the typed API converters and ownership machinery, replacing
// JavaScript values with owned wire values. It never imports syscall/js.
func abiSource(a *schema.API, source []byte) ([]byte, error) {
	s := string(source)
	start := strings.Index(s, "func main() {")
	end := strings.Index(s, "// Keep imports stable")
	if start < 0 || end < start {
		return nil, fmt.Errorf("missing ABI runtime boundary")
	}
	s = s[:start] + s[end:]
	s = strings.Replace(s, `"syscall/js"`, `"encoding/binary";"time";"unsafe";"strings"`, 1)
	s = strings.ReplaceAll(s, "js.", "abi")
	raw, _ := templates.ReadFile("templates/abi.go.txt")
	s += strings.ReplaceAll(string(raw), "__SCHEMA__", fmt.Sprintf("%q", mustJSON(a.Manifest)))
	formatted, err := format.Source([]byte(s))
	if err != nil {
		return nil, fmt.Errorf("format ABI bridge: %w", err)
	}
	return formatted, nil
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
