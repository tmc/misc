package accuracy

import (
	"bytes"
	"encoding/json"
	"testing"
	"unicode/utf8"
)

func TestPythonStripCodepoints(t *testing.T) {
	// Exhaustive chr(i).strip()=="" oracle from Python 3.11.16 on DGX.
	points := []rune{9, 10, 11, 12, 13, 28, 29, 30, 31, 32, 133, 160, 5760, 8192, 8193, 8194, 8195, 8196, 8197, 8198, 8199, 8200, 8201, 8202, 8232, 8233, 8239, 8287, 12288}
	want := make(map[rune]bool)
	for _, r := range points {
		want[r] = true
	}
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if pythonSpace(r) != want[r] {
			t.Fatalf("strip set differs at U+%04X", r)
		}
	}
}

func TestAIPerfExactMatch(t *testing.T) {
	for _, tt := range []struct {
		name, prediction, reference string
		correct, unparsed           bool
	}{
		{"literal", "A", "A", true, false},
		{"outer whitespace", " \tA\r\n", "A", true, false},
		{"python control whitespace", "\x1cA\x1f", "\x1dA\x1e", true, false},
		{"unicode whitespace", "\u00a0A\u3000", "A", true, false},
		{"empty", "", "A", false, true},
		{"whitespace only", "\x1c \n", "A", false, true},
		{"empty reference", "A", "", false, false},
		{"both empty", "", "", false, false},
		{"case", "a", "A", false, false},
		{"inner space", "A B", "AB", false, false},
		{"explanation", "The answer is A.", "A", false, false},
		{"zero width", "\u200bA", "A", false, false},
		{"bom", "\ufeffA", "A", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := fixture("aiperf-exact-match-v1")
			a.Cases = []Case{{ID: "one", Reference: tt.reference, Prediction: &tt.prediction, Outcome: "success"}}
			a.ReferenceSHA256, _ = ReferenceHash(a.Cases)
			b, _ := json.Marshal(a)
			r, err := Evaluate(bytes.NewReader(b))
			if err != nil || r.Results[0].Correct != tt.correct || r.Results[0].Unparsed != tt.unparsed {
				t.Fatalf("report=%+v err=%v", r, err)
			}
		})
	}
	// The existing evaluator retains its established Go whitespace semantics.
	a := fixture("exact-match-v1")
	p := "\x1cA\x1f"
	a.Cases = []Case{{ID: "one", Reference: "A", Prediction: &p, Outcome: "success"}}
	a.ReferenceSHA256, _ = ReferenceHash(a.Cases)
	b, _ := json.Marshal(a)
	r, err := Evaluate(bytes.NewReader(b))
	if err != nil || r.Correct != 0 {
		t.Fatalf("default changed: %+v %v", r, err)
	}
}
