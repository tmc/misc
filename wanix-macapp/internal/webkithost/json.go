package webkithost

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func jsonUnmarshalString(s string, v any) error {
	var raw string
	if err := json.Unmarshal([]byte(s), &raw); err == nil {
		s = raw
	} else if unquoted, qerr := strconv.Unquote(s); qerr == nil {
		s = unquoted
	}
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return fmt.Errorf("decode javascript result: %w", err)
	}
	return nil
}
