package accuracy

import (
	"bytes"
	"encoding/json"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

func uniqueJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	return jsoncheck.Check(d, jsoncheck.Options{FoldNames: true})
}
