package serving

import (
	"strings"
	"testing"
)

func TestCapacityJSONBoundaries(t *testing.T) {
	for _, input := range []string{`{"a":1,"a":2}`, `{"evidence":null}`, `{} {}`, strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65)} {
		if err := ValidateCapacityJSON([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	if err := ValidateCapacityJSON([]byte(`{"a":0,"b":{}}`)); err != nil {
		t.Fatal(err)
	}
}
