package llmperf

import (
	"bytes"
	"encoding/json"
	"go.k6.io/k6/v2/metrics"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestReplayIntegrity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RunSummary)
	}{
		{"latency", func(r *RunSummary) { r.Records[0].FullResponseLatency = 99999 }},
		{"negative output timing", func(r *RunSummary) { r.Records[0].TTFO = floatPtr(-1) }},
		{"late output timing", func(r *RunSummary) { r.Records[0].TTFO = floatPtr(99999) }},
		{"negative itl", func(r *RunSummary) { r.Records[0].ITL = floatPtr(-1) }},
		{"missing count", func(r *RunSummary) { r.MissingOutputCounts = 1 }},
		{"estimated count", func(r *RunSummary) { r.EstimatedInputCounts = 1 }},
		{"outcome count", func(r *RunSummary) { r.Successes++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := analysisFixture()
			tc.mutate(&r)
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReadRun(bytes.NewReader(data)); err == nil {
				t.Fatal("malformed replay accepted")
			}
			if _, err := AnalyzeTimeline(r); err == nil {
				t.Fatal("direct timeline bypassed accounting")
			}
		})
	}
}

func TestMissingDerivedRateRoundtrip(t *testing.T) {
	for _, field := range []string{"goodput", "input_throughput", "output_throughput"} {
		t.Run(field, func(t *testing.T) {
			r := analysisFixture()
			r.Goodput = 0
			r.InputThroughput = 1
			r.OutputThroughput = 2
			data, _ := json.Marshal(r)
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			delete(fields, field)
			data, _ = json.Marshal(fields)
			got, err := ReadRun(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			if err := WriteRun(&b, got); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadRun(&b); err != nil {
				t.Fatal(err)
			}
			fields[field] = json.RawMessage("null")
			data, _ = json.Marshal(fields)
			if _, err := ReadRun(bytes.NewReader(data)); err == nil {
				t.Fatal("null rate accepted")
			}
		})
	}
}

func TestStreamingDetailOnlyUsage(t *testing.T) {
	for _, before := range []bool{false, true} {
		t.Run(map[bool]string{false: "after", true: "before"}[before], func(t *testing.T) {
			c := setupTestClient(t, "http://example.test")
			c.measurement = &Measurement{}
			details := `data: {"choices":[],"usage":{"prompt_tokens_details":{"cached_tokens":2},"completion_tokens_details":{"reasoning_tokens":1}}}` + "\n\n"
			main := `data: {"choices":[{"index":0,"delta":{"content":"hello"}}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}` + "\n\n"
			body := main + details
			if before {
				body = details + main
			}
			body += "data: [DONE]\n\n"
			_, err := c.handleStreamingResponse(&http.Response{Body: io.NopCloser(strings.NewReader(body))}, time.Now(), 3, func() {}, false)
			if err != nil {
				t.Fatal(err)
			}
			u := c.measurement.reported
			if u.CachedTokens == nil || *u.CachedTokens != 2 || u.ReasoningTokens == nil || *u.ReasoningTokens != 1 || u.PromptTokens != 3 || u.CompletionTokens != 4 {
				t.Fatalf("usage=%+v", u)
			}
		})
	}
}

func TestDistributedRefusesPopulationPolicies(t *testing.T) {
	for _, policy := range []string{"", "completion only", "unknown"} {
		a, b := workerFixture("a", 0), workerFixture("b", 0)
		b.Run.WindowPolicy = policy
		if _, err := MergeRuns([]WorkerRun{a, b}, MergeOptions{ExpectedWorkers: []string{"a", "b"}}); err == nil {
			t.Fatalf("accepted population %q", policy)
		}
	}
}

func TestDistributedRefusesDifferentSLO(t *testing.T) {
	a, b := workerFixture("a", 0), workerFixture("b", 0)
	b.Run.Records[0].SLO.MaxTTFT = 100
	if _, err := MergeRuns([]WorkerRun{a, b}, MergeOptions{ExpectedWorkers: []string{"a", "b"}}); err == nil {
		t.Fatal("different SLO receipts accepted")
	}
}

func TestContentEventObservations(t *testing.T) {
	start := time.Now().Add(-time.Second)
	m := &Measurement{start: start, requestStart: start, responseEnd: start.Add(300 * time.Millisecond), Outcome: "success"}
	m.content(start.Add(100*time.Millisecond), false, Usage{})
	m.content(start.Add(200*time.Millisecond), true, Usage{})
	m.content(start.Add(200*time.Millisecond), true, Usage{})
	(&Client{}).finishMeasurement(m)
	if m.TTST == nil || *m.TTST != 200 || m.ContentEventCount != 3 || len(m.ContentIntervals) != 2 || m.ContentIntervals[0] != 100 || m.ContentIntervals[1] != 0 {
		t.Fatalf("content=%+v", m)
	}
	for i := 0; i < 5000; i++ {
		m.content(start.Add(200*time.Millisecond), true, Usage{})
	}
	if len(m.ContentIntervals) != 4096 || !m.ContentIntervalsTruncated {
		t.Fatal("unbounded event vector")
	}
}

func TestPartialReplayStillValidatesRetainedTiming(t *testing.T) {
	r := analysisFixture()
	r.DroppedRecords = 1
	r.Records = r.Records[:2]
	r.Records[0].TTFO = floatPtr(-1)
	data, _ := json.Marshal(r)
	if _, err := ReadRun(bytes.NewReader(data)); err == nil {
		t.Fatal("partial retained timing bypass")
	}
}

func TestFirstChunkOriginSurvivesFallback(t *testing.T) {
	for _, declared := range []bool{false, true} {
		for _, first := range []int{0, 6, 7} {
			start := time.Now().Add(-time.Second)
			m := &Measurement{start: start, requestStart: start, first: start.Add(100 * time.Millisecond), last: start.Add(200 * time.Millisecond), reported: Usage{CompletionTokens: 6, completionPresent: true}, Outcome: "success"}
			want := "reported"
			if declared {
				m.counts = &TokenCounts{FirstChunkTokens: intPtr(first)}
				want = "client_declared"
			} else {
				m.firstChunk = intPtr(first)
			}
			(&Client{}).finishMeasurement(m)
			if m.ITL == nil || *m.ITL != 20 || m.FirstChunkOrigin != want || m.FirstChunkTokens == nil || *m.FirstChunkTokens != first || m.FirstChunkUnavailable == "" {
				t.Fatalf("fallback=%+v", m)
			}
		}
	}
}

func TestDetailUsageZeroNullAndNegative(t *testing.T) {
	var u Usage
	if err := json.Unmarshal([]byte(`{"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}`), &u); err != nil || u.CachedTokens == nil || *u.CachedTokens != 0 || u.ReasoningTokens == nil || *u.ReasoningTokens != 0 {
		t.Fatalf("zero usage=%+v err=%v", u, err)
	}
	base := Usage{PromptTokens: 3, CompletionTokens: 4, promptPresent: true, completionPresent: true, CachedTokens: intPtr(2), ReasoningTokens: intPtr(1)}
	if err := json.Unmarshal([]byte(`{"prompt_tokens_details":{"cached_tokens":null},"completion_tokens_details":{"reasoning_tokens":null}}`), &u); err != nil {
		t.Fatal(err)
	}
	got, err := mergeUsage(base, u)
	if err != nil {
		t.Fatal(err)
	}
	if got.CachedTokens == nil || *got.CachedTokens != 2 || got.ReasoningTokens == nil || *got.ReasoningTokens != 1 || got.PromptTokens != 3 || got.CompletionTokens != 4 {
		t.Fatalf("null erased usage=%+v", got)
	}
	if err := json.Unmarshal([]byte(`{"prompt_tokens_details":{"cached_tokens":-1}}`), &u); err == nil {
		t.Fatal("negative detail accepted")
	}
}

func TestReplayTimestampRounding(t *testing.T) {
	r := analysisFixture()
	r.Records[0].FullResponseLatency += .0015
	if _, err := AnalyzeTimeline(r); err != nil {
		t.Fatal(err)
	}
	r.Records[0].FullResponseLatency += .001
	if _, err := AnalyzeTimeline(r); err == nil {
		t.Fatal("latency beyond serialization bound accepted")
	}
}

func TestPartialAggregateBounds(t *testing.T) {
	r := analysisFixture()
	r.DroppedRecords = 2
	r.Records = r.Records[:1]
	r.Successes = 0
	r.Failures = 3
	r.InputTokens = 0
	r.OutputTokens = 0
	data, _ := json.Marshal(r)
	if _, err := ReadRun(bytes.NewReader(data)); err == nil {
		t.Fatal("retained success exceeds aggregate accepted")
	}
}
func TestReplayEventConsistency(t *testing.T) {
	for _, change := range []string{"span", "second"} {
		r := analysisFixture()
		m := &r.Records[0]
		m.TTFT = floatPtr(10)
		m.LastContentLatency = floatPtr(30)
		m.TTST = floatPtr(30)
		m.ContentEventCount = 2
		m.ContentIntervals = []float64{20}
		m.TTSTPolicy = "second recognized content event; not a physical second-token observation"
		if change == "span" {
			m.ContentIntervals[0] = 21
		} else {
			m.TTST = floatPtr(15)
		}
		if _, err := AnalyzeTimeline(r); err == nil {
			t.Fatalf("accepted contradictory %s", change)
		}
	}
}

func TestPartialCompletenessRequiresDroppedSuccess(t *testing.T) {
	r := analysisFixture()
	r.DroppedRecords = 1
	r.Records = r.Records[:2]
	r.MissingOutputCounts = 1
	data, _ := json.Marshal(r)
	if _, err := ReadRun(bytes.NewReader(data)); err == nil {
		t.Fatal("dropped failure used to invent missing successful count")
	}
}

func TestContentObservationMetrics(t *testing.T) {
	for _, tc := range []struct {
		name      string
		intervals []time.Duration
		truncated bool
		mean, max float64
		samples   int
	}{
		{"single event", nil, false, 0, 0, 0},
		{"known zero", []time.Duration{0, 0}, false, 0, 0, 1},
		{"several events", []time.Duration{0, 25 * time.Millisecond, 75 * time.Millisecond}, false, 100.0 / 3, 75, 1},
		{"truncated", []time.Duration{25 * time.Millisecond, 75 * time.Millisecond}, true, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metric := testMetrics(t, metrics.NewRegistry())
			samples := make(chan metrics.SampleContainer, 100)
			metric.samples = samples
			start := time.Now().Add(-time.Second)
			m := &Measurement{start: start, requestStart: start, responseEnd: start.Add(300 * time.Millisecond), Outcome: "success", Recorded: true, Attempted: true}
			event := start.Add(100 * time.Millisecond)
			m.content(event, false, Usage{})
			for _, gap := range tc.intervals {
				event = event.Add(gap)
				m.content(event, true, Usage{})
			}
			m.ContentIntervalsTruncated = tc.truncated
			(&Client{metrics: metric}).finishMeasurement(m)
			values := metricValues(samples)
			for name, want := range map[string]float64{"llm_mean_content_interval": tc.mean, "llm_max_content_interval": tc.max} {
				if len(values[name]) != tc.samples {
					t.Fatalf("%s sample count=%d; want %d", name, len(values[name]), tc.samples)
				}
				if tc.samples > 0 && values[name][0] != want {
					t.Fatalf("%s=%v; want %v", name, values[name][0], want)
				}
			}
			if len(tc.intervals) > 0 && (len(values["llm_dispatch_ttst"]) != 1 || values["llm_dispatch_ttst"][0] != 100+float64(tc.intervals[0])/float64(time.Millisecond)) {
				t.Fatalf("TTST metrics=%v", values)
			}
			if _, ok := metric.metrics["llm_content_inter_chunk_latency"]; ok {
				t.Fatal("unshipped per-event trend remains registered")
			}
		})
	}
}

func TestFinishTimestampUsesDispatchAnchor(t *testing.T) {
	start := time.Now()
	// A later independent Unix-clock sample may drift relative to monotonic time.
	dispatch := millis(start) + .01
	m := &Measurement{requestStart: start, start: start, responseEnd: start.Add(100 * time.Millisecond), DispatchedAt: dispatch, Attempted: true, Outcome: "success", ScheduledAt: floatPtr(dispatch - 10)}
	(&Client{}).finishMeasurement(m)
	if m.FullResponseLatency != 100 || m.FinishedAt != dispatch+100 || m.TimestampBasis != "dispatch Unix clock anchor plus monotonic elapsed time" {
		t.Fatalf("record=%+v", m)
	}
	if m.ScheduledResponseLatency == nil || *m.ScheduledResponseLatency != 110 {
		t.Fatalf("scheduled latency=%v", m.ScheduledResponseLatency)
	}
	r := RunSummary{StartedAt: dispatch - 1, FinishedAt: dispatch + 101}
	m.Phase = "measurement"
	if err := validateRecordTiming(*m, r); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionFailureTimestampUsesRequestAnchor(t *testing.T) {
	start := time.Now()
	begin := millis(start) + .01
	m := &Measurement{start: start, StartedAt: begin, responseEnd: start.Add(100 * time.Millisecond), Outcome: "failure", ScheduledAt: floatPtr(begin - 10)}
	(&Client{}).finishMeasurement(m)
	if m.FullResponseLatency != 100 || m.FinishedAt != begin+100 || m.TimestampBasis != "request Unix clock anchor plus monotonic elapsed time" || m.ScheduledResponseLatency == nil || *m.ScheduledResponseLatency != 110 {
		t.Fatalf("record=%+v", m)
	}
}
