package accuracy

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func ExampleEvaluate_aiperfExactMatch() {
	prediction := "\x1cA\x1f"
	a := fixture("aiperf-exact-match-v1")
	a.Cases = []Case{{ID: "one", Reference: "A", Prediction: &prediction, Outcome: "success"}}
	a.ReferenceSHA256, _ = ReferenceHash(a.Cases)
	data, _ := json.Marshal(a)
	report, err := Evaluate(bytes.NewReader(data))
	fmt.Println(report.Correct, report.Total, err)
	// Output: 1 1 <nil>
}
