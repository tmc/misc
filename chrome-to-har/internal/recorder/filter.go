package recorder

import (
	"encoding/json"
	"fmt"

	"github.com/chromedp/cdproto/har"
	"github.com/itchyny/gojq"
)

// applyJQFilter applies a JQ-style filter to the HAR entry
func (r *Recorder) applyJQFilter(entry *har.Entry) (*har.Entry, error) {
	// Convert entry to map for JQ processing
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal entry: %w", err)
	}

	var input interface{}
	if err := json.Unmarshal(entryJSON, &input); err != nil {
		return nil, fmt.Errorf("unmarshal entry: %w", err)
	}

	// Parse JQ query
	query, err := gojq.Parse(r.filter.JQExpr)
	if err != nil {
		return nil, fmt.Errorf("parse jq expression: %w", err)
	}

	// Run query
	iter := query.Run(input)
	result, ok := iter.Next()
	if !ok {
		// No match, filter out this entry
		return nil, nil
	}
	if err, ok := result.(error); ok {
		return nil, fmt.Errorf("jq execution: %w", err)
	}

	// Convert result back to HAR entry
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	var filtered har.Entry
	if err := json.Unmarshal(resultJSON, &filtered); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return &filtered, nil
}
