package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	llm "github.com/tmc/misc/xk6-llm-perf"
	collector "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func named(d Document, name string) Statistic {
	for _, s := range d.Metrics {
		if s.Name == name {
			return s
		}
	}
	return Statistic{}
}

func TestCanonicalLatencyBoundaries(t *testing.T) {
	r := runFixture()
	last, firstOutput, itl, fullITL, maxChunk := 70., 30., 12.5, 20., 40.
	n := 5
	r.OutputTokens, r.OutputThroughput = 5, 5
	r.MissingOutputCounts = 0
	m := &r.Records[0]
	m.LastContentLatency, m.TTFO, m.ITL, m.FullResponseITL, m.MaxChunkLatency = &last, &firstOutput, &itl, &fullITL, &maxChunk
	m.OutputTokens = llm.TokenCount{Value: &n, Source: "reported"}
	decode := 4
	m.DecodeTokens = &decode
	m.FirstChunkSource = "assumed_one"
	m.HTTPTrace = &llm.HTTPTrace{Waiting: m.TTFT}
	d, err := Summarize(r)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]float64{"request_latency": 100, "full_response_latency": 100, "last_content_latency": 70, "ttft": 20, "ttfo": 30, "itl": 12.5, "full_response_itl": 20, "max_chunk_latency": 40, "http_waiting": 20} {
		s := named(d, name)
		if s.Unit != timingUnit(name) || s.Count != 1 || s.Mean == nil || *s.Mean != want {
			t.Fatalf("%s: %+v, want %g", name, s, want)
		}
	}
	if s := named(d, "http_dns"); s.Count != 0 || s.Mean != nil {
		t.Fatal("missing reused connection DNS became zero")
	}
	if d.SchemaVersion != 2 || d.PercentileMethod != "nearest_rank" || d.StandardDeviationMethod != "population" || d.RetainedSuccesses != 1 || named(d, "output_tokens").Sources["reported"] != 1 {
		t.Fatalf("metadata lost: %+v", d)
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"last_content_latency"`)) || !bytes.Contains(data, []byte(`"percentile_method":"nearest_rank"`)) {
		t.Fatal(string(data))
	}
	var b bytes.Buffer
	if err := CSV(&b, d); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows[1:] {
		if row[0] == "last_content_latency" {
			found = row[3] == "70" && row[10] == "nearest_rank"
		}
	}
	if !found {
		t.Fatal("CSV endpoint/estimator mismatch")
	}
	b.Reset()
	if err := HTML(&b, d); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "last_content_latency") || !strings.Contains(b.String(), "full_response_latency") {
		t.Fatal("HTML omitted latency distinction")
	}
	payload, err := OTLP(r)
	if err != nil {
		t.Fatal(err)
	}
	var req collector.ExportMetricsServiceRequest
	if err := protojson.Unmarshal(payload, &req); err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, metric := range req.ResourceMetrics[0].ScopeMetrics[0].Metrics {
		got[metric.Name] = metric.GetGauge().DataPoints[0].GetAsDouble()
	}
	if got["gen_ai.client.operation.duration"] != .1 || got["llmperf.client.last_content_latency"] != .07 || got["llmperf.client.full_response_latency"] != .1 {
		t.Fatal(got)
	}
	live, err := liveMLflow(r, "run")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(live, []byte(`"key":"live.llmperf.client.last_content_latency","value":0.07`)) {
		t.Fatal(string(live))
	}
	batch, err := MLflowBatch(d, "run", 1200)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(batch, []byte(`"key":"last_content_latency.mean","value":70`)) || !bytes.Contains(batch, []byte(`"key":"llmperf.schema_version","value":"2"`)) {
		t.Fatal(string(batch))
	}
}

func TestNearestRankAndCoverage(t *testing.T) {
	s := stats("example", "ms", []float64{1, 2, 10, 20})
	if *s.P50 != 2 || *s.P90 != 20 || *s.Mean != 8.25 {
		t.Fatalf("unexpected estimator: %+v", s)
	}
	if two := stats("known", "ms", []float64{1, 3}); two.Std == nil || *two.Std != 1 {
		t.Fatal("expected population standard deviation 1")
	}
	r := runFixture()
	r.Attempts, r.Successes, r.DroppedRecords = 2, 2, 1
	r.ExcludedBoundaryRequests, r.InflightAtEnd = 3, 1
	d, err := Summarize(r)
	if err != nil {
		t.Fatal(err)
	}
	if d.RetainedRecords != 1 || d.RetainedSuccesses != 1 || d.DroppedRecords != 1 || d.ExcludedBoundaryRequests != 3 || d.InflightAtEnd != 1 || d.Timeline != nil || d.TimelineUnavailable == "" {
		t.Fatalf("coverage: %+v", d)
	}
	if named(d, "output_throughput").Mean != nil {
		t.Fatal("missing counts became zero throughput")
	}
	if named(d, "input_tokens").Sources["reported"] != 1 || named(d, "output_tokens").Sources["unavailable"] != 1 {
		t.Fatal("token provenance lost")
	}
}

func TestContentEventStatistics(t *testing.T) {
	r := runFixture()
	m := &r.Records[0]
	first, last, second := 20., 70., 40.
	m.TTFT, m.LastContentLatency, m.TTST = &first, &last, &second
	m.TTSTPolicy = "second recognized content event; not a physical second-token observation"
	m.ContentEventCount = 3
	m.ContentIntervals = []float64{20, 30}
	d, err := Summarize(r)
	if err != nil {
		t.Fatal(err)
	}
	if s := named(d, "content_inter_chunk_latency"); s.Count != 2 || s.Mean == nil || *s.Mean != 25 {
		t.Fatal(s)
	}
	if s := named(d, "ttst"); s.Mean == nil || *s.Mean != 40 {
		t.Fatal(s)
	}
	if !strings.Contains(d.TTSTPolicy, "not a physical") || d.ContentIntervalUnavailableRequests != 0 {
		t.Fatal("event policy lost")
	}
	payload, err := OTLP(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(payload, []byte("llmperf.client.ttst")) {
		t.Fatal("OTLP omitted event TTST")
	}
}
