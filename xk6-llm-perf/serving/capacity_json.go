package serving

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

// ValidateCapacityJSON rejects duplicate keys, explicit nulls, excessive nesting
// and trailing values in submitted cell declarations. Omit unavailable fields.
func ValidateCapacityJSON(data []byte) error { return validateCapacityJSON(data, true) }
func validateCapacityJSON(data []byte, rejectNull bool) error {
	err := jsoncheck.Check(json.NewDecoder(bytes.NewReader(data)), jsoncheck.Options{RejectNull: rejectNull})
	switch err {
	case jsoncheck.ErrDepth:
		return fmt.Errorf("capacity: JSON nesting exceeds limit")
	case jsoncheck.ErrNull:
		return fmt.Errorf("capacity: explicit null")
	case jsoncheck.ErrDuplicate:
		return fmt.Errorf("capacity: duplicate JSON member")
	case jsoncheck.ErrTrailing:
		return fmt.Errorf("capacity: trailing JSON")
	}
	return err
}
