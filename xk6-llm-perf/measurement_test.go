package llmperf

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

func TestUsagePresence(t *testing.T) {
	for _, tt := range []struct {
		input   string
		present bool
	}{{`{}`, false}, {`{"completion_tokens":null}`, false}, {`{"completion_tokens":0}`, true}} {
		var u Usage
		if err := json.Unmarshal([]byte(tt.input), &u); err != nil {
			t.Fatal(err)
		}
		if u.completionPresent != tt.present {
			t.Fatalf("%s: presence = %v", tt.input, u.completionPresent)
		}
	}
	var u Usage
	if json.Unmarshal([]byte(`{"completion_tokens":-1}`), &u) == nil {
		t.Fatal("negative count accepted")
	}
}

func TestTokenCountAndITL(t *testing.T) {
	for _, tt := range []struct {
		name             string
		output           int
		first            *int
		strict, reported bool
		want             *float64
		source           string
	}{
		{"bundled", 6, intPtr(2), true, true, floatPtr(25), "client_declared"},
		{"fallback", 6, nil, true, true, floatPtr(20), "assumed_one"},
		{"one token", 1, nil, true, true, nil, "unavailable"},
		{"missing strict", 0, nil, true, false, nil, "unavailable"},
		{"reported zero", 0, nil, false, true, nil, "unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{config: Config{StrictMeasurement: tt.strict}}
			start := time.Now().Add(-time.Second)
			m := &Measurement{start: start, requestStart: start, first: start.Add(100 * time.Millisecond), last: start.Add(200 * time.Millisecond), reported: Usage{CompletionTokens: tt.output, completionPresent: tt.reported}, counts: &TokenCounts{FirstChunkTokens: tt.first}, estimatedOutput: 12, Outcome: "success"}
			c.finishMeasurement(m)
			if (m.ITL == nil) != (tt.want == nil) || m.ITL != nil && *m.ITL != *tt.want {
				t.Fatalf("ITL=%v; want %v", m.ITL, tt.want)
			}
			if m.FirstChunkSource != tt.source {
				t.Fatalf("source=%s; want %s", m.FirstChunkSource, tt.source)
			}
			if tt.strict && !tt.reported && m.OutputTokens.Value != nil {
				t.Fatal("strict count invented")
			}
			if tt.reported && *m.OutputTokens.Value != tt.output {
				t.Fatal("reported zero replaced")
			}
		})
	}
}

func intPtr(n int) *int           { return &n }
func floatPtr(n float64) *float64 { return &n }

func TestDiscrepancyCountsRequestOnce(t *testing.T) {
	c := setupTestClient(t, "http://example.test")
	samples := make(chan metrics.SampleContainer, 100)
	c.metrics.samples = samples
	m := &Measurement{start: time.Now(), requestStart: time.Now(), Outcome: "success", Attempted: true, Recorded: true, counts: &TokenCounts{InputTokens: intPtr(10), OutputTokens: intPtr(20)}, reported: Usage{PromptTokens: 12, CompletionTokens: 25, promptPresent: true, completionPresent: true}}
	c.finishMeasurement(m)
	if !m.UsageDiscrepancy || *m.PromptDifference != 20 || *m.OutputDifference != 25 {
		t.Fatalf("differences=%+v", m)
	}
	if got := metricValues(samples)["llm_usage_discrepant_requests"]; len(got) != 1 || got[0] != 1 {
		t.Fatalf("discrepancy samples=%v", got)
	}
}

func TestMeasurementJavaScriptAndFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["measurement"] != nil {
			t.Error("local token declarations sent to provider")
		}
		if temp, ok := req["temperature"]; !ok || temp != float64(0) {
			t.Errorf("temperature zero lost: %v", req)
		}
		if req["model"] == "fail" {
			http.Error(w, "private provider text", 503)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`)
	}))
	defer server.Close()
	rt := modulestest.NewRuntime(t)
	if err := rt.SetupModuleSystem(map[string]any{importPath: new(rootModule)}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.RunOnEventLoop(fmt.Sprintf(`const llm = require(%q);`, importPath)); err != nil {
		t.Fatal(err)
	}
	samples := make(chan metrics.SampleContainer, 200)
	rt.MoveToVUContext(&lib.State{Samples: samples, Tags: lib.NewVUStateTags(rt.VU.InitEnvField.Registry.RootTagSet()), BuiltinMetrics: rt.BuiltinMetrics})
	_, err := rt.RunOnEventLoop(fmt.Sprintf(`
 llm.beginMeasurement({runId:"fixture",maxRecords:10});
 const c = new llm.Client({baseURL:%q,model:"fixture",strictMeasurement:true});
 const req = {messages:[{role:"user",content:"secret prompt"}],temperature:0};
 const ok = c.chat.completions.create(req);
 if (ok.measurement.output_tokens.value !== 0 || ok.measurement.output_tokens.source !== "reported") throw new Error("zero provenance: "+JSON.stringify(ok));
 const bad = c.chat.completions.create({...req,model:"fail"});
 if (!(bad instanceof Error) || bad.measurement.outcome !== "failure") throw new Error("failure record");
 const summary = llm.endMeasurement();
 if (summary.attempts !== 2 || summary.successes !== 1 || summary.failures !== 1 || summary.good_requests !== 1 || summary.goodput <= 0) throw new Error(JSON.stringify(summary));
 const encoded=JSON.stringify(summary);
 if (encoded.includes("secret prompt") || encoded.includes("private provider text")) throw new Error("secret in manifest");
 `, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	values := metricValues(samples)
	if len(values["llm_slo_compliance"]) != 2 || values["llm_slo_compliance"][1] != 0 {
		t.Fatal("failure omitted from SLO denominator")
	}
}

func TestPrefillWaitHonorsClientTimeout(t *testing.T) {
	c := setupTestClient(t, "http://example.test")
	c.config.Model = "fixture"
	c.config.Timeout = "15ms"
	c.config.PrefillConcurrency = 1
	c.limiters = &prefillLimiters{}
	release, err := c.limiters.acquire(c.vu.Context(), c.config)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	samples := make(chan metrics.SampleContainer, 100)
	c.metrics.samples = samples
	result := c.Complete(sobek.FunctionCall{Arguments: []sobek.Value{c.vu.Runtime().ToValue(CompletionRequest{Messages: []Message{{Role: "user", Content: "hi"}}})}})
	err, ok := result.Export().(error)
	if !ok || !strings.Contains(err.Error(), "wait for prefill slot") {
		t.Fatalf("result=%v", result)
	}
	if c.measurement.Attempted || c.measurement.AdmissionWait < 10 {
		t.Fatalf("measurement=%+v", c.measurement)
	}
	if len(metricValues(samples)["llm_attempts"]) != 0 {
		t.Fatal("queue failure became network attempt")
	}
}

func TestRunWindowAndBounds(t *testing.T) {
	var r runState
	if err := r.begin(RunOptions{MaxRecords: 1}); err != nil {
		t.Fatal(err)
	}
	if err := r.begin(RunOptions{}); err == nil {
		t.Fatal("nested window accepted")
	}
	for i := 0; i < 2; i++ {
		r.add(Measurement{Recorded: true, Attempted: true, Outcome: "success", Good: true, requestStart: time.Now(), InputTokens: count(3, "reported"), OutputTokens: count(4, "reported")})
	}
	s, err := r.finish()
	if err != nil {
		t.Fatal(err)
	}
	if s.Attempts != 2 || s.OutputTokens != 8 || s.DroppedRecords != 1 || len(s.Records) != 1 || s.OutputThroughput != 8/s.Duration {
		t.Fatalf("summary=%+v", s)
	}
	if _, err := r.finish(); err == nil {
		t.Fatal("window ended twice")
	}
}

func TestRequestCancellationPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	c := setupTestClient(t, server.URL)
	c.config.Model = "fixture"
	c.config.StrictMeasurement = true
	c.metrics.samples = make(chan metrics.SampleContainer, 100)
	req := CompletionRequest{Messages: []Message{{Role: "user", Content: "fixture"}}, CancelAfter: "10ms"}
	v, err := jsonValue(c.vu.Runtime(), req)
	if err != nil {
		t.Fatal(err)
	}
	result := c.Complete(sobek.FunctionCall{Arguments: []sobek.Value{v}})
	if _, ok := result.Export().(error); !ok || c.measurement.Outcome != "cancelled" || !c.measurement.Attempted {
		t.Fatalf("result=%v measurement=%+v", result, c.measurement)
	}
}

func TestRecordByteBudgetPreservesAccounting(t *testing.T) {
	var r runState
	if err := r.begin(RunOptions{MaxRecords: 10, MaxRecordBytes: 1}); err != nil {
		t.Fatal(err)
	}
	r.add(Measurement{Attempted: true, Recorded: true, Outcome: "success", requestStart: time.Now(), InputTokens: count(2, "reported"), OutputTokens: count(4, "reported")})
	s, err := r.finish()
	if err != nil {
		t.Fatal(err)
	}
	if s.Attempts != 1 || s.OutputTokens != 4 || s.DroppedRecords != 1 || len(s.Records) != 0 || s.RetainedRecordBytes != 0 {
		t.Fatalf("bounded run=%+v", s)
	}
}
