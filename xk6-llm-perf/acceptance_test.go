package llmperf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkAcceptance(t *testing.T) {
	zero, one, two := 0, 1, 2
	for _, tt := range []struct {
		name        string
		expectation WorkExpectation
		body        string
		want        string
	}{
		{"exact zero", WorkExpectation{RequireExactUsage: true, MinOutputTokens: &zero, MaxOutputTokens: &zero, AllowedFinishReasons: []string{"stop"}, ExpectedModel: "served"}, `{"model":"served","choices":[{"message":{"content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`, "accepted"},
		{"wrong length", WorkExpectation{MinOutputTokens: &two}, `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`, "rejected"},
		{"contradictory total", WorkExpectation{RequireExactUsage: true}, `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":99}}`, "rejected"},
		{"missing total", WorkExpectation{RequireExactUsage: true}, `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`, "unavailable"},
		{"partial consistent usage", WorkExpectation{RequireExactUsage: true}, `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}],"usage":{"completion_tokens":1,"total_tokens":2}}`, "unavailable"},
		{"partial contradictory usage", WorkExpectation{RequireExactUsage: true}, `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}],"usage":{"completion_tokens":1,"total_tokens":0}}`, "rejected"},
		{"missing usage", WorkExpectation{RequireExactUsage: true}, `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}]}`, "unavailable"},
		{"early eos", WorkExpectation{AllowedFinishReasons: []string{"stop"}, MaxOutputTokens: &two}, `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`, "accepted"},
		{"wrong model", WorkExpectation{ExpectedModel: "expected"}, `{"model":"other","choices":[{"message":{"content":"x"},"finish_reason":"stop"}]}`, "rejected"},
		{"oversized model", WorkExpectation{ExpectedModel: "expected"}, `{"model":"` + strings.Repeat("x", 1025) + `","choices":[{"message":{"content":"x"},"finish_reason":"stop"}]}`, "rejected"},
		{"digest mismatch", WorkExpectation{TextDigest: strings.Repeat("0", 64)}, `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}]}`, "rejected"},
		{"token IDs absent", WorkExpectation{TokenIDsDigest: strings.Repeat("0", 64)}, `{"choices":[{"message":{"content":"x"},"finish_reason":"stop"}]}`, "unavailable"},
		{"empty below minimum", WorkExpectation{MinOutputTokens: &one}, `{"choices":[{"message":{"content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`, "rejected"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if _, ok := body["work"]; ok {
					t.Error("expectation leaked to provider")
				}
				fmt.Fprint(w, tt.body)
			}))
			defer server.Close()
			c := setupTestClient(t, server.URL)
			value := protocolRequest(t, c, CompletionRequest{Work: &tt.expectation, Messages: []Message{{Role: "user", Content: "x"}}})
			b, _ := json.Marshal(value)
			var response CompletionResponse
			if err := json.Unmarshal(b, &response); err != nil {
				t.Fatal(err, string(b))
			}
			m := response.Measurement
			if m == nil || m.Outcome != "success" || !m.Good || m.Work.Status != tt.want {
				t.Fatalf("unexpected response: %s", b)
			}
			if err := validateWorkMeasurement(m); err != nil {
				t.Fatal(err)
			}
			m.Work.Status = "invented"
			if validateWorkMeasurement(m) == nil {
				t.Fatal("forged status accepted")
			}
		})
	}
}

func TestWorkOversizedModel(t *testing.T) {
	m := &Measurement{Outcome: "success", Work: &WorkReceipt{}}
	observeWork(m, CompletionResponse{Model: strings.Repeat("x", 1025)})
	if m.Work.ObservedModel != "" || !m.Work.ObservedModelOversized {
		t.Fatalf("work = %+v", m.Work)
	}
	m.Work.Status, m.Work.Reasons = classifyWork(m)
	if err := validateWorkMeasurement(m); err != nil {
		t.Fatal(err)
	}
	m.Work.ObservedModel = "x"
	if validateWorkMeasurement(m) == nil {
		t.Fatal("model retained beside oversized flag")
	}
}

func TestWorkFailureCannotBeAccepted(t *testing.T) {
	m := &Measurement{Outcome: "failure", Work: &WorkReceipt{Expectation: WorkExpectation{}}}
	m.Work.Status, m.Work.Reasons = classifyWork(m)
	if m.Work.Status != "rejected" {
		t.Fatal(m.Work)
	}
	if err := validateWorkMeasurement(m); err != nil {
		t.Fatal(err)
	}
	m.Work.Status = "accepted"
	m.Work.Reasons = nil
	if validateWorkMeasurement(m) == nil {
		t.Fatal("failure accepted")
	}
}

func ExampleWorkExpectation() {
	zero := 0
	w := WorkExpectation{MinOutputTokens: &zero, AllowedFinishReasons: []string{"stop"}}
	b, _ := json.Marshal(w)
	fmt.Println(string(b))
	// Output: {"allowedFinishReasons":["stop"],"minOutputTokens":0}
}

func TestWorkExpectationJSON(t *testing.T) {
	for _, data := range []string{`{"requireExactUsage":null}`, `{"minOutputTokens":null}`, `{"minOutputTokens":0,"minOutputTokens":1}`, `{"unknown":true}`, `{"allowedFinishReasons":[""]}`, `{"minOutputTokens":2,"maxOutputTokens":1}`} {
		var w WorkExpectation
		if json.Unmarshal([]byte(data), &w) == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}

func TestWorkReplayAccounting(t *testing.T) {
	run := analysisFixture()
	for i := range run.Records {
		m := &run.Records[i]
		m.Work = &WorkReceipt{Expectation: WorkExpectation{}}
		if m.Outcome == "success" && m.OutputTokens.Source == "reported" {
			m.Work.ReportedOutputTokens = m.OutputTokens.Value
		}
		m.Work.Status, m.Work.Reasons = classifyWork(m)
		addWorkCounts(&run, *m)
	}
	if _, err := AnalyzeRequests(run); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*RunSummary){
		func(r *RunSummary) { r.WorkAccepted++ },
		func(r *RunSummary) { r.WorkQualified = 0 },
		func(r *RunSummary) { r.Records[2].Work.Status = "accepted"; r.Records[2].Work.Reasons = nil },
		func(r *RunSummary) { r.WorkGoodput = 1 },
	} {
		b, _ := json.Marshal(run)
		var invalid RunSummary
		if err := json.Unmarshal(b, &invalid); err != nil {
			t.Fatal(err)
		}
		mutate(&invalid)
		if _, err := AnalyzeRequests(invalid); err == nil {
			t.Fatal("direct analysis accepted forged work")
		}
		b, _ = json.Marshal(invalid)
		if _, err := ReadRun(strings.NewReader(string(b))); err == nil {
			t.Fatal("replay accepted forged work")
		}
	}
	dropped := run
	dropped.Records = dropped.Records[:2]
	dropped.DroppedRecords = 1
	if err := validateWorkRun(dropped); err != nil {
		t.Fatal(err)
	}
	dropped.WorkRejected = 0
	dropped.WorkQualified = 2
	if err := validateWorkRun(dropped); err != nil {
		t.Fatal("valid dropped unqualified legacy attempt refused", err)
	}
}

func TestWorkDroppedFailureCannotBecomeAccepted(t *testing.T) {
	run := analysisFixture()
	run.Records = run.Records[:2]
	run.DroppedRecords = 1
	run.WorkQualified = 1
	run.WorkAccepted = 1
	if err := validateWorkRun(run); err == nil {
		t.Fatal("dropped failure became accepted")
	}
	run.WorkAccepted = 0
	run.WorkUnavailable = 1
	if err := validateWorkRun(run); err == nil {
		t.Fatal("dropped failure became unavailable qualification")
	}
	run.WorkUnavailable = 0
	run.WorkRejected = 1
	if err := validateWorkRun(run); err != nil {
		t.Fatal("valid dropped failure rejected", err)
	}
}

func TestWorkUsageReplayContradictions(t *testing.T) {
	one, two, total := 1, 1, 99
	m := Measurement{Outcome: "success", InputTokens: count(one, "reported"), OutputTokens: count(two, "reported"), ProviderTotalTokens: &total, Work: &WorkReceipt{Expectation: WorkExpectation{RequireExactUsage: true}, ReportedOutputTokens: &two}}
	m.Work.Status, m.Work.Reasons = classifyWork(&m)
	if m.Work.Status != "rejected" || len(m.Work.Reasons) != 1 || m.Work.Reasons[0] != "exact_usage_inconsistent" {
		t.Fatal(m.Work)
	}
	if err := validateWorkMeasurement(&m); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Measurement){
		func(m *Measurement) {
			m.Work.Status = "unavailable"
			m.Work.Reasons = []string{"exact_usage_unavailable"}
		},
		func(m *Measurement) { m.Work.ExactUsage = true },
		func(m *Measurement) { m.Work.ReportedOutputTokens = nil },
		func(m *Measurement) { n := 2; m.ProviderTotalTokens = &n },
	} {
		b, _ := json.Marshal(m)
		var forged Measurement
		if err := json.Unmarshal(b, &forged); err != nil {
			t.Fatal(err)
		}
		mutate(&forged)
		if err := validateWorkMeasurement(&forged); err == nil {
			t.Fatal("forged work evidence accepted")
		}
	}
	// Explicit consistent zero and partial/missing evidence remain distinguishable.
	zero := 0
	for _, c := range []struct {
		input, output TokenCount
		total         *int
		exact         bool
		state         string
	}{
		{count(zero, "reported"), count(zero, "reported"), &zero, true, "consistent"},
		{count(one, "estimated"), count(one, "reported"), &total, false, "incomplete"},
		{count(one, "reported"), count(one, "reported"), nil, false, "incomplete"},
	} {
		observation := m
		observation.InputTokens, observation.OutputTokens, observation.ProviderTotalTokens = c.input, c.output, c.total
		observation.Work = &WorkReceipt{Expectation: WorkExpectation{RequireExactUsage: true}, ExactUsage: c.exact, ReportedOutputTokens: c.output.Value}
		observation.Work.Status, observation.Work.Reasons = classifyWork(&observation)
		if got := workUsageState(&observation); got != c.state {
			t.Fatal(got, c.state)
		}
		if err := validateWorkMeasurement(&observation); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorkUsageSumDoesNotOverflow(t *testing.T) {
	maximum := int(^uint(0) >> 1)
	for _, tt := range []struct {
		input, output, total int
		want                 string
	}{
		{0, maximum, maximum, "consistent"},
		{maximum, 0, maximum, "consistent"},
		{maximum, maximum, maximum, "inconsistent"},
		{maximum, 1, 0, "inconsistent"},
	} {
		m := Measurement{InputTokens: count(tt.input, "reported"), OutputTokens: count(tt.output, "reported"), ProviderTotalTokens: &tt.total, Work: &WorkReceipt{}}
		m.reported = Usage{PromptTokens: tt.input, CompletionTokens: tt.output, TotalTokens: tt.total, promptPresent: true, completionPresent: true, totalPresent: true}
		observeWork(&m, CompletionResponse{})
		if got := workUsageState(&m); got != tt.want {
			t.Fatal(got, tt.want)
		}
		if m.Work.ExactUsage != (tt.want == "consistent") {
			t.Fatal("observed sum overflowed", m.Work)
		}
	}
}

func TestWorkUsageForgedRunReplay(t *testing.T) {
	run := analysisFixture()
	run.Records = run.Records[:1]
	run.Attempts, run.Successes, run.Failures = 1, 1, 0
	run.InputTokens, run.OutputTokens = 2, 4
	run.InputThroughput = float64(run.InputTokens) / run.Duration
	run.OutputThroughput = float64(run.OutputTokens) / run.Duration
	m := &run.Records[0]
	total := 99
	m.ProviderTotalTokens = &total
	m.Work = &WorkReceipt{Expectation: WorkExpectation{RequireExactUsage: true}, ReportedOutputTokens: m.OutputTokens.Value}
	m.Work.Status, m.Work.Reasons = classifyWork(m)
	addWorkCounts(&run, *m)
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRun(strings.NewReader(string(data))); err != nil {
		t.Fatal(err)
	}
	m.Work.Status = "unavailable"
	m.Work.Reasons = []string{"exact_usage_unavailable"}
	run.WorkRejected = 0
	run.WorkUnavailable = 1
	data, err = json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRun(strings.NewReader(string(data))); err == nil {
		t.Fatal("forged unavailable contradiction accepted by ReadRun")
	}
	if _, err := AnalyzeRequests(run); err == nil {
		t.Fatal("forged unavailable contradiction accepted by direct analysis")
	}
}
