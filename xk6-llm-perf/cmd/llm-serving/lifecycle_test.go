package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUniqueJSON(t *testing.T) {
	for _, input := range []string{`{"server":{},"server":{}}`, `{"server":{"path":"a","path":"b"}}`, `{"server":null}`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)} {
		if err := uniqueJSON(json.NewDecoder(strings.NewReader(input))); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	if err := uniqueJSON(json.NewDecoder(strings.NewReader(`{"server":{"path":"a"},"warmup":[]}`))); err != nil {
		t.Fatal(err)
	}
}
func TestLifecycleConfigBounds(t *testing.T) {
	for _, data := range []string{strings.Repeat(" ", (1<<20)+1), `{} {}`, `{"unknown":true}`} {
		file := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := runLifecycle([]string{"-config", file}); err == nil {
			t.Fatal("accepted invalid config")
		}
	}
}
