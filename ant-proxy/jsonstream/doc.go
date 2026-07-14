// Package jsonstream streams JSON objects from various formats.
//
// Stream handles three JSON formats transparently:
//
//   - JSON arrays: [{"id":1}, {"id":2}]
//   - Single objects: {"id":1}
//   - NDJSON: {"id":1}\n{"id":2}\n
//
// For all formats, Stream yields objects one at a time.
//
// # Usage
//
// Stream with an io.Reader:
//
//	resp, _ := http.Get(url)
//	defer resp.Body.Close()
//	for msg, err := range jsonstream.Stream(resp.Body) {
//		if err != nil {
//			log.Fatal(err)
//		}
//		var obj MyType
//		json.Unmarshal(msg, &obj)
//		// process obj
//	}
//
// Stream from bytes:
//
//	data := []byte(`[{"id":1}, {"id":2}]`)
//	for msg, err := range jsonstream.Bytes(data) {
//		if err != nil {
//			log.Fatal(err)
//		}
//		// process msg
//	}
//
// # Format Detection
//
// Stream peeks at the first token to determine format:
//
//   - If the first byte is '[', it reads array elements
//   - Otherwise, it reads objects separated by newlines
//
// A single object is treated as one-element NDJSON.
//
// This works for streaming APIs like OpenAI, Anthropic, Gemini,
// which may return either JSON arrays or NDJSON depending on
// their streaming configuration.
package jsonstream
