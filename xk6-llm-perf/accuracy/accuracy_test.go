package accuracy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func fixture(evaluator string) Artifact {
	correct, wrong, empty := "A", "B", ""
	a := Artifact{SchemaVersion: 1, Task: "fixture", TaskVersion: "1", ReferenceVersion: "1", Evaluator: evaluator, EvaluatorVersion: "1", Model: "fixture", ModelRevision: "1", Cases: []Case{{"1", "A", &correct, "success"}, {"2", "A", &wrong, "success"}, {"3", "A", nil, "success"}, {"4", "A", &empty, "failure"}}}
	a.ReferenceSHA256, _ = ReferenceHash(a.Cases)
	return a
}
func ExampleReferenceHash() {
	h, err := ReferenceHash([]Case{{ID: "one", Reference: "A"}})
	fmt.Println(len(h), err)
	// Output: 64 <nil>
}
func ExampleEvaluate() {
	a := fixture("exact-match-v1")
	b, _ := json.Marshal(a)
	r, err := Evaluate(bytes.NewReader(b))
	fmt.Println(r.Correct, r.Total, r.Accuracy, r.Coverage, err)
	// Output: 1 4 0.25 0.5 <nil>
}
func TestAccuracyDenominators(t *testing.T) {
	for _, eval := range []string{"exact-match-v1", "choice-v1"} {
		a := fixture(eval)
		b, _ := json.Marshal(a)
		r, err := Evaluate(bytes.NewReader(b))
		if err != nil || r.Accuracy != .25 || r.Observed != 2 || r.Unavailable != 1 || r.Failed != 1 {
			t.Fatalf("report: %+v %v", r, err)
		}
		out, _ := json.Marshal(r)
		if bytes.Contains(out, []byte("prediction")) || bytes.Contains(out, []byte(`"reference":`)) {
			t.Fatal("answers leaked")
		}
	}
}
func TestInvalidArtifacts(t *testing.T) {
	for _, mutate := range []func(*Artifact){func(a *Artifact) { a.ReferenceSHA256 = "wrong" }, func(a *Artifact) { a.EvaluatorVersion = "2" }, func(a *Artifact) { a.Cases[1].ID = "1"; a.ReferenceSHA256, _ = ReferenceHash(a.Cases) }, func(a *Artifact) { a.Evaluator = "execute" }, func(a *Artifact) { a.Cases[0].Outcome = "unknown" }} {
		a := fixture("exact-match-v1")
		mutate(&a)
		b, _ := json.Marshal(a)
		if _, err := Evaluate(bytes.NewReader(b)); err == nil {
			t.Fatal("invalid accepted")
		}
	}
}
func TestRational(t *testing.T) {
	p := "0.5"
	a := fixture("rational-v1")
	a.Cases = []Case{{"one", "1/2", &p, "success"}}
	a.ReferenceSHA256, _ = ReferenceHash(a.Cases)
	b, _ := json.Marshal(a)
	r, err := Evaluate(bytes.NewReader(b))
	if err != nil || r.Correct != 1 {
		t.Fatal(r, err)
	}
}
