package llmperf

import (
	"encoding/json"
	"fmt"
	"strings"
)

func Example() {
	run, err := ReadRun(strings.NewReader(`{"schema_version":1,"started_at":1000,"finished_at":2000,"duration":1,"records":[]}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	timeline, err := AnalyzeTimeline(run)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(timeline.Requests.Effective.Mean)
	// Output: 0
}

func ExampleTokenCounts() {
	zero := 0
	counts := TokenCounts{OutputTokens: &zero, Tokenizer: "caller:vocab@revision/template"}
	fmt.Println(*counts.OutputTokens, counts.InputTokens == nil)
	// Output: 0 true
}

func ExampleTokenCount() {
	zero := 0
	reported := TokenCount{Value: &zero, Source: "reported"}
	missing := TokenCount{Source: "unavailable"}
	fmt.Println(*reported.Value, missing.Value == nil)
	// Output: 0 true
}

func ExampleRunOptions() {
	options := RunOptions{RunID: "fixture", WorkerID: "worker-a", MaxRecords: 100, Duration: "10s"}
	data, _ := json.Marshal(options)
	fmt.Println(strings.Contains(string(data), `"workerId":"worker-a"`))
	// Output: true
}

func ExampleRawRequest() {
	request := RawRequest{Path: "/embeddings", Protocol: "json", Body: map[string]any{"model": "fixture", "input": "hello"}}
	fmt.Println(request.Path, request.Protocol)
	// Output: /embeddings json
}

func ExampleMeasurement() {
	record := Measurement{Outcome: "failure", InputTokens: TokenCount{Source: "unavailable"}, OutputTokens: TokenCount{Source: "unavailable"}}
	fmt.Println(record.Good, record.TTFT == nil, record.OutputTokens.Value == nil)
	// Output: false true true
}
