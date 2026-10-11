package accuracy

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestUniqueJSON(t *testing.T) {
	for _, input := range []string{`{"error":null}`, `{"a":{"b":1},"c":[{"b":2}]}`, `{"choices":[{"index":0,"message":{"content":""}}]}`} {
		if err := uniqueJSON([]byte(input)); err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range []string{`{"error":{},"error":null}`, `{"error":{},"Error":null}`, `{"s":1,"\u017f":2}`, `{"choices":[{"index":1,"index":0}]}`, `{"choices":[{"message":{"content":"B","content":"A"}}]}`, `{"x":[{"a":1,"a":2}]}`, `{"a":1,"\u0061":2}`, `{} {}`, strings.Repeat(`[`, 66) + `0` + strings.Repeat(`]`, 66)} {
		if err := uniqueJSON([]byte(input)); err == nil {
			t.Fatalf("ambiguous JSON accepted: %s", input)
		}
	}
}

func TestArtifactDuplicateMembers(t *testing.T) {
	a := fixture("exact-match-v1")
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{strings.Replace(string(data), `"schema_version":1`, `"schema_version":0,"schema_version":1`, 1), strings.Replace(string(data), `"prediction":"A"`, `"prediction":"B","prediction":"A"`, 1)} {
		if _, err := Evaluate(bytes.NewBufferString(input)); err == nil {
			t.Fatal("duplicate artifact evidence accepted")
		}
	}
	if _, err := Evaluate(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}
