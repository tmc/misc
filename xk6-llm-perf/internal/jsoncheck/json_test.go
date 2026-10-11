package jsoncheck_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

func ExampleCheck() {
	d := json.NewDecoder(strings.NewReader(`{"error":null}`))
	fmt.Println(jsoncheck.Check(d, jsoncheck.Options{FoldNames: true}))
	// Output: <nil>
}

func ExampleValue() {
	d := json.NewDecoder(strings.NewReader(`{} []`))
	fmt.Println(jsoncheck.Value(d, jsoncheck.Options{}))
	fmt.Println(jsoncheck.Value(d, jsoncheck.Options{}))
	// Output:
	// <nil>
	// <nil>
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name, input string
		opts        jsoncheck.Options
		number      bool
		want        error
	}{
		{name: "nested duplicates", input: `{"a":[{"b":1,"b":2}]}`, want: jsoncheck.ErrDuplicate},
		{name: "decoded names", input: `{"a":1,"\u0061":2}`, want: jsoncheck.ErrDuplicate},
		{name: "fold alias", input: `{"s":1,"\u017f":2}`, opts: jsoncheck.Options{FoldNames: true}, want: jsoncheck.ErrDuplicate},
		{name: "exact names", input: `{"s":1,"\u017f":2}`},
		{name: "separate scopes", input: `{"a":{"b":1},"c":{"b":2}}`},
		{name: "null allowed", input: `{"a":null}`},
		{name: "null refused", input: `[null]`, opts: jsoncheck.Options{RejectNull: true}, want: jsoncheck.ErrNull},
		{name: "trailing", input: `{} []`, want: jsoncheck.ErrTrailing},
		{name: "depth64", input: strings.Repeat("[", 64) + "0" + strings.Repeat("]", 64)},
		{name: "depth65", input: strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65), want: jsoncheck.ErrDepth},
		{name: "number precision", input: `1e1000`, number: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := json.NewDecoder(strings.NewReader(tt.input))
			if tt.number {
				d.UseNumber()
			}
			if err := jsoncheck.Check(d, tt.opts); err != tt.want {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
	for _, input := range []string{``, `{"a":}`, `[}`, `{"a":1`} {
		if err := jsoncheck.Check(json.NewDecoder(strings.NewReader(input)), jsoncheck.Options{}); err == nil {
			t.Fatalf("accepted malformed JSON %q", input)
		}
	}
}
