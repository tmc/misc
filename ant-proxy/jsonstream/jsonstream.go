// Package jsonstream streams JSON arrays, objects, or NDJSON.
package jsonstream

import (
	"bytes"
	"encoding/json"
	"io"
)

// Stream yields JSON messages from arrays [{},...], single objects {}, or NDJSON {}\n{}.
func Stream(r io.Reader) func(yield func(json.RawMessage, error) bool) {
	return func(yield func(json.RawMessage, error) bool) {
		var buf bytes.Buffer
		tee := io.TeeReader(r, &buf)
		d := json.NewDecoder(tee)
		t, err := d.Token()
		if err != nil {
			if err != io.EOF {
				yield(nil, err)
			}
			return
		}
		// Reconstruct stream: buffered data + remaining original stream
		fullStream := io.MultiReader(&buf, r)
		d = json.NewDecoder(fullStream)

		if t == json.Delim('[') {
			d.Token() // consume '['
			for d.More() {
				var m json.RawMessage
				if e := d.Decode(&m); e != nil {
					yield(nil, e)
					return
				}
				if !yield(m, nil) {
					return
				}
			}
			return
		}
		// NDJSON or single object
		for {
			var m json.RawMessage
			if e := d.Decode(&m); e == io.EOF {
				return
			} else if e != nil {
				yield(nil, e)
				return
			}
			if !yield(m, nil) {
				return
			}
		}
	}
}

// Bytes is Stream for []byte.
func Bytes(b []byte) func(yield func(json.RawMessage, error) bool) {
	return Stream(bytes.NewReader(b))
}
