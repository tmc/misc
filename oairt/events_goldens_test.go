package oairt

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGoldens = flag.Bool("update", false, "rewrite testdata/events/*.json from the in-memory corpus")

// TestEventGoldens snapshots every Event in eventCorpus() to
// testdata/events/<name>.json and asserts each file round-trips back to a
// byte-identical re-marshal. Run with -update to regenerate the goldens
// after a deliberate corpus change.
func TestEventGoldens(t *testing.T) {
	dir := filepath.Join("testdata", "events")
	if *updateGoldens {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	tests := eventCorpus()
	seen := make(map[string]bool, len(tests))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen[tt.name+".json"] = true
			path := filepath.Join(dir, tt.name+".json")
			canonical, err := canonicalize(tt.in)
			if err != nil {
				t.Fatalf("canonicalize: %v", err)
			}
			if *updateGoldens {
				if err := os.WriteFile(path, canonical, 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if !bytes.Equal(bytes.TrimRight(want, "\n"), bytes.TrimRight(canonical, "\n")) {
				t.Fatalf("golden mismatch for %s\n  got:  %s\n  want: %s", tt.name, canonical, want)
			}
			// Also: unmarshal the golden file straight into Event and confirm it
			// survives a fresh round-trip.
			var e Event
			dec := json.NewDecoder(bytes.NewReader(want))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&e); err != nil {
				t.Fatalf("decode golden: %v", err)
			}
			if e.Type != tt.in.Type {
				t.Fatalf("type drift: got %q, want %q", e.Type, tt.in.Type)
			}
		})
	}

	if !*updateGoldens {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("readdir: %v", err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			if !seen[e.Name()] {
				t.Errorf("orphan golden %q (no matching corpus entry; remove or add to corpus)", e.Name())
			}
		}
	}
}

// canonicalize marshals v with stable indentation so golden diffs are
// line-readable.
func canonicalize(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, b, "", "  "); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
