package llmperf

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestResponsesDoneParts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		events  []string
		want    string
		wantErr bool
	}{
		{"done only", []string{`{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"hello"}`}, "hello", false},
		{"delta and done", []string{`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`, `{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"hello"}`}, "hello", false},
		{"sibling fallback", []string{`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"one"}`, `{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"one"}`, `{"type":"response.output_text.done","output_index":0,"content_index":1,"text":"two"}`}, "onetwo", false},
		{"conflicting done", []string{`{"type":"response.output_text.delta","delta":"one"}`, `{"type":"response.output_text.done","text":"two"}`}, "", true},
		{"duplicate done", []string{`{"type":"response.output_text.done","text":"one"}`, `{"type":"response.output_text.done","text":"one"}`}, "", true},
		{"delta after done", []string{`{"type":"response.output_text.done","text":"one"}`, `{"type":"response.output_text.delta","delta":"two"}`}, "", true},
		{"changed item", []string{`{"type":"response.output_text.delta","item_id":"a","delta":"one"}`, `{"type":"response.output_text.done","item_id":"b","text":"one"}`}, "", true},
		{"lost index", []string{`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"one"}`, `{"type":"response.output_text.done","text":"one"}`}, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var state responsesStream
			var text strings.Builder
			var err error
			for _, event := range tt.events {
				var chunk ChatCompletionChunk
				chunk, _, err = state.decode([]byte(event))
				if err != nil {
					break
				}
				text.WriteString(chunk.Choices[0].Delta.Content)
			}
			if (err != nil) != tt.wantErr || err == nil && text.String() != tt.want {
				t.Fatalf("text=%q err=%v", text.String(), err)
			}
		})
	}
}

func TestResponsesDoneRefusals(t *testing.T) {
	for _, event := range []string{
		`{"type":"response.output_text.done","text":7}`,
		`{"type":"response.output_text.done","text":{},"output_index":0,"content_index":0}`,
		`{"type":"response.output_text.done","text":"a","Text":"b"}`,
		`{"type":"response.output_text.done","text":"a","content_index":0,"content_index":1}`,
		`{"type":"response.output_text.done","text":"a","Content_index":0}`,
		`{"type":"response.output_text.done","text":"a","output_index":0,"content_index":null}`,
		`{"type":"response.output_text.done","text":"a","output_index":0,"content_index":false}`,
		`{"type":"response.output_text.done","text":"a","output_index":0,"content_index":0.0}`,
		`{"type":"response.output_text.done","text":"a","output_index":0,"content_index":-1}`,
		`{"type":"response.output_text.done","text":"a","output_index":0}`,
	} {
		t.Run(event, func(t *testing.T) {
			var state responsesStream
			if _, _, err := state.decode([]byte(event)); err == nil {
				t.Fatal("accepted malformed done")
			}
		})
	}
}

func TestResponsesDoneControlledSSE(t *testing.T) {
	for _, delta := range []bool{false, true} {
		t.Run(fmt.Sprint(delta), func(t *testing.T) {
			var wire strings.Builder
			if delta {
				wire.WriteString("data: {\"type\":\"response.output_text.delta\",\"output_index\":0,\"content_index\":0,\"delta\":\"hello\"}\n\n")
			}
			wire.WriteString("data: {\"type\":\"response.output_text.done\",\"output_index\":0,\"content_index\":0,\"text\":\"hello\"}\n\n")
			wire.WriteString("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":2,\"output_tokens\":1,\"total_tokens\":3}}}\n\n")
			c := setupTestClient(t, "http://unused.invalid")
			c.protocol = "openai-responses"
			c.config.StrictMeasurement = true
			now := time.Now()
			c.measurement = &Measurement{SchemaVersion: 1, Phase: "measurement", Attempted: true, Recorded: true, requestStart: now, start: now}
			response := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(wire.String()))}
			result, err := c.handleStreamingResponse(response, now, 0, func() {}, false)
			if err != nil || messageText(result.Choices[0].Message.Content) != "hello" {
				t.Fatalf("response=%+v err=%v", result, err)
			}
			expectedView := 1
			if delta {
				expectedView = 2
			}
			if c.measurement.AIPerfContentEvents == nil || *c.measurement.AIPerfContentEvents != expectedView || c.measurement.AIPerfTTFT == nil || c.measurement.AIPerfLastContentLatency == nil {
				t.Fatalf("missing pinned content view: %+v", c.measurement)
			}
			if c.measurement.ContentEventCount != 1 || !c.measurement.reported.completionPresent || c.measurement.reported.CompletionTokens != 1 {
				t.Fatalf("measurement=%+v", c.measurement)
			}
		})
	}
}

func TestResponsesPinnedContentTiming(t *testing.T) {
	var state responsesStream
	events := []struct {
		data string
		want bool
	}{
		{`{"type":"response.output_item.added","item":{"type":"message"}}`, false},
		{`{"type":"response.reasoning_summary_text.delta","delta":"summary"}`, false},
		{`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`, true},
		{`{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"hello"}`, true},
		{`{"type":"response.output_text.done","output_index":0,"content_index":1,"text":"world"}`, true},
		{`{"type":"response.completed","response":{"status":"completed","usage":{"output_tokens":2}}}`, false},
	}
	start := time.Unix(0, 0)
	m := Measurement{requestStart: start, AIPerfContentPolicy: responsesContentPolicy, AIPerfContentEvents: new(int)}
	for index, event := range events {
		chunk, _, err := state.decode([]byte(event.data))
		if err != nil || chunk.aiperfContent != event.want {
			t.Fatalf("event%d: %+v %v", index, chunk, err)
		}
		if chunk.aiperfContent {
			recordResponsesTiming(&m, start.Add(time.Duration(index)*time.Millisecond), start)
		}
	}
	if *m.AIPerfTTFT != 2 || *m.AIPerfLastContentLatency != 4 || *m.AIPerfContentEvents != 3 || m.ContentEventCount != 0 {
		t.Fatal(m)
	}
}

func TestResponsesTimingReplay(t *testing.T) {
	valid := func() Measurement {
		return Measurement{Protocol: "openai-responses", Stream: true, FullResponseLatency: 100, AIPerfContentPolicy: responsesContentPolicy, AIPerfContentEvents: intPtr(2), AIPerfTTFT: floatPtr(0), AIPerfLastContentLatency: floatPtr(50)}
	}
	if err := validateResponsesTiming(Measurement{}); err != nil {
		t.Fatal("historical", err)
	}
	if err := validateResponsesTiming(valid()); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		mutate func(*Measurement)
	}{
		{"policy", func(m *Measurement) { m.AIPerfContentPolicy = "other" }},
		{"protocol", func(m *Measurement) { m.Protocol = "openai-chat" }},
		{"unary", func(m *Measurement) { m.Stream = false }},
		{"missing count", func(m *Measurement) { m.AIPerfContentEvents = nil }},
		{"negative count", func(m *Measurement) { m.AIPerfContentEvents = intPtr(-1) }},
		{"missing first", func(m *Measurement) { m.AIPerfTTFT = nil }},
		{"negative first", func(m *Measurement) { m.AIPerfTTFT = floatPtr(-1) }},
		{"reversing", func(m *Measurement) { m.AIPerfTTFT = floatPtr(60) }},
		{"outside response", func(m *Measurement) { m.AIPerfLastContentLatency = floatPtr(101) }},
		{"nonfinite", func(m *Measurement) { m.AIPerfTTFT = floatPtr(math.NaN()) }},
		{"zero with timing", func(m *Measurement) { m.AIPerfContentEvents = intPtr(0) }},
		{"single span", func(m *Measurement) { m.AIPerfContentEvents = intPtr(1) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()
			tt.mutate(&m)
			if validateResponsesTiming(m) == nil {
				t.Fatal("accepted invalid view")
			}
		})
	}
	m := valid()
	m.AIPerfContentEvents = intPtr(1)
	m.AIPerfLastContentLatency = floatPtr(0)
	if err := validateResponsesTiming(m); err != nil {
		t.Fatal("single legitimate zero", err)
	}
	m.AIPerfContentEvents = intPtr(0)
	m.AIPerfTTFT = nil
	m.AIPerfLastContentLatency = nil
	if err := validateResponsesTiming(m); err != nil {
		t.Fatal("zero events", err)
	}
	run := formulaRun()
	m = valid()
	record := &run.Records[0]
	record.Protocol = m.Protocol
	record.Stream = true
	record.AIPerfContentPolicy = m.AIPerfContentPolicy
	record.AIPerfContentEvents = m.AIPerfContentEvents
	record.AIPerfTTFT = m.AIPerfTTFT
	record.AIPerfLastContentLatency = m.AIPerfLastContentLatency
	if err := readFormulaRun(run); err != nil {
		t.Fatal("round trip", err)
	}
	record.AIPerfLastContentLatency = floatPtr(101)
	if readFormulaRun(run) == nil {
		t.Fatal("serialized invalid view accepted")
	}
}

func TestResponsesStateBounds(t *testing.T) {
	var state responsesStream
	for i := 0; i < 4096; i++ {
		event := fmt.Sprintf(`{"type":"response.output_text.done","output_index":0,"content_index":%d,"text":""}`, i)
		if _, _, err := state.decode([]byte(event)); err != nil {
			t.Fatal(i, err)
		}
	}
	if _, _, err := state.decode([]byte(`{"type":"response.output_text.done","output_index":0,"content_index":4096,"text":""}`)); err == nil {
		t.Fatal("part limit not enforced")
	}
	// Place state at the aggregate boundary without allocating a 16 MiB fixture.
	state = responsesStream{textBytes: 16 * 1024 * 1024}
	if _, _, err := state.decode([]byte(`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"x"}`)); err == nil {
		t.Fatal("aggregate delta budget")
	}
	state = responsesStream{textBytes: 16 * 1024 * 1024}
	if _, _, err := state.decode([]byte(`{"type":"response.output_text.done","output_index":0,"content_index":1,"text":"x"}`)); err == nil {
		t.Fatal("aggregate fallback budget")
	}
	state = responsesStream{}
	event := fmt.Sprintf(`{"type":"response.output_text.done","item_id":%q,"text":""}`, strings.Repeat("x", 2049))
	if _, _, err := state.decode([]byte(event)); err == nil {
		t.Fatal("item identity budget")
	}
}
