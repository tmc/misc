package campaign

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func scaleConfiguration() ScaleConfig {
	return ScaleConfig{Binary: "k6", Script: "examples/campaign/global-scale.js", Variable: "request", Minimum: 1, Maximum: 2, FixedRequest: 2, FixedPrefill: 2, Workers: 2, Windows: 5, SustainWindows: 1, Period: "200ms", StartupTimeout: "5s", DrainTimeout: "100ms", MinCompleted: 2, MaxLatency: 1000, MaxFailureRatio: 0, MaxRecords: 1000}
}
func TestQuotaGlobalBounds(t *testing.T) {
	for workers := 1; workers <= 32; workers++ {
		for cap := 1; cap <= 100; cap++ {
			total := 0
			for id := 0; id < workers; id++ {
				q := quota(cap, workers, id)
				if q < 0 {
					t.Fatal("negative quota")
				}
				total += q
			}
			if total != cap {
				t.Fatal("unbounded aggregate cap")
			}
		}
	}
}
func TestWindowAssessment(t *testing.T) {
	c := scaleConfiguration()
	for _, tt := range []struct {
		name          string
		window        ScaleWindow
		participating []bool
		reason        string
	}{{"passing", ScaleWindow{Samples: []ScaleSample{{Latency: 10, Outcome: "success"}, {Latency: 20, Outcome: "success"}}}, []bool{true, true}, "passed"}, {"missing worker", ScaleWindow{}, []bool{true, false}, "missing_worker"}, {"retention", ScaleWindow{Dropped: 1}, []bool{true, true}, "incomplete_records"}, {"low completion", ScaleWindow{Samples: []ScaleSample{{Latency: 10, Outcome: "success"}}}, []bool{true, true}, "insufficient_completions"}, {"failure", ScaleWindow{Samples: []ScaleSample{{Latency: 10, Outcome: "success"}, {Latency: 20, Outcome: "success"}, {Outcome: "failure"}}}, []bool{true, true}, "failure_ratio"}, {"latency", ScaleWindow{Samples: []ScaleSample{{Latency: 2000, Outcome: "success"}, {Latency: 20, Outcome: "success"}}}, []bool{true, true}, "latency"}} {
		t.Run(tt.name, func(t *testing.T) {
			w := tt.window
			assess(&w, tt.participating, c, "")
			if w.Reason != tt.reason || w.Passed != (tt.reason == "passed") {
				t.Fatalf("got %+v", w)
			}
		})
	}
}
func TestScaleControlProtocol(t *testing.T) {
	c := scaleConfiguration()
	control := &scaleControl{config: c, ready: make([]bool, 2), participating: make([]bool, 2), seen: map[string]bool{}, active: true, window: &ScaleWindow{Epoch: 1, StartedAt: 100, FinishedAt: 200, RequestLimit: 1, PrefillLimit: 2}}
	server := httptest.NewServer(http.HandlerFunc(control.serveHTTP))
	defer server.Close()
	resp, e := http.Get(server.URL + "/control?worker=1")
	if e != nil {
		t.Fatal(e)
	}
	var assignment scaleAssignment
	if e = json.NewDecoder(resp.Body).Decode(&assignment); e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if assignment.Request != 0 || assignment.Prefill != 1 || !assignment.Active {
		t.Fatal(assignment)
	}
	sample := ScaleSample{ID: "0:1:1", Worker: 0, Epoch: 1, DispatchedAt: 120, FinishedAt: 130, Latency: 10, Outcome: "success"}
	b, _ := json.Marshal(sample)
	for i := 0; i < 2; i++ {
		response, e := http.Post(server.URL+"/sample?worker=0", "application/json", bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		want := 202
		if i == 1 {
			want = 409
		}
		if response.StatusCode != want {
			t.Fatal(response.StatusCode)
		}
	}
	if len(control.window.Samples) != 1 || control.window.Duplicates != 1 {
		t.Fatal("duplicate not retained")
	}
	control.config.MaxRecords = 1
	sample.ID = "0:1:2"
	b, _ = json.Marshal(sample)
	response, e := http.Post(server.URL+"/sample?worker=0", "application/json", bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 429 || control.window.Dropped != 1 || len(control.window.Samples) != 1 {
		t.Fatal("retention budget did not refuse and account for dropped sample")
	}

	for _, payload := range []string{`{"worker":0,"id":"x","epoch":1,"dispatched_at":150,"finished_at":100,"latency_ms":2,"outcome":"success"}`, strings.Repeat("a", 2049)} {
		response, e := http.Post(server.URL+"/sample?worker=0", "application/json", strings.NewReader(payload))
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatal("malformed sample accepted")
		}
	}
}
func TestBuiltK6GlobalScale(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to signed extension binary")
	}
	binary, e := filepath.Abs(binary)
	if e != nil {
		t.Fatal(e)
	}
	for _, variable := range []string{"request", "prefill"} {
		t.Run(variable, func(t *testing.T) {
			var inflight, peak atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				current := inflight.Add(1)
				defer inflight.Add(-1)
				for {
					old := peak.Load()
					if current <= old || peak.CompareAndSwap(old, current) {
						break
					}
				}
				time.Sleep(25 * time.Millisecond)
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")

				fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\ndata: [DONE]\n\n")
			}))
			defer server.Close()
			t.Setenv("ENDPOINT_URL", server.URL)
			c := scaleConfiguration()
			c.Variable = variable
			if variable == "prefill" {
				c.FixedRequest = 4
			} else {
				c.FixedPrefill = 4
			}
			c.Binary = binary
			c.Script, e = filepath.Abs("../examples/campaign/global-scale.js")
			if e != nil {
				t.Fatal(e)
			}
			var preparedAt int64
			verify := func(binary, script, bh, sh string, assets map[string]string) error {
				time.Sleep(250 * time.Millisecond)
				return verifyInputs(binary, script, bh, sh, assets)
			}
			checkpoint := func(path string, manifest *ScaleManifest) error {
				if len(manifest.Windows) > 0 && manifest.Windows[len(manifest.Windows)-1].Reason == "admission_pending" {
					time.Sleep(250 * time.Millisecond)
					pending := manifest.Windows[len(manifest.Windows)-1]
					if pending.StartedAt != 0 || pending.FinishedAt != 0 {
						t.Fatal("pending checkpoint advertises an active window")
					}
					preparedAt = time.Now().UnixMilli()
				}
				return writeJSON(path, manifest)
			}
			m, e := runScale(context.Background(), c, filepath.Join(t.TempDir(), "scale"), verify, checkpoint)
			if e != nil {
				t.Fatal(e)
			}
			if m.StopReason != "sustain_complete" || len(m.Windows) != 3 || len(m.Workers) != 2 {
				b, _ := json.MarshalIndent(m, "", "  ")
				t.Fatalf("global scale: %s", b)
			}
			for _, window := range m.Windows {
				if window.FinishedAt-window.StartedAt != 200 {
					t.Fatal("assessment duration changed")
				}
				if !window.Passed || len(window.ParticipatingWorkers) != 2 || window.Completed < 2 {
					t.Fatalf("assessment failed: %+v", window)
				}
			}
			if m.Windows[len(m.Windows)-1].StartedAt < preparedAt {
				t.Fatal("assessment began before durable checkpoint")
			}
			if peak.Load() > 2 || peak.Load() < 2 {
				t.Fatalf("global actual peak %d want 2", peak.Load())
			}
			b, _ := json.Marshal(m)
			replayed, err := ReadScale(bytes.NewReader(b))
			if err != nil || replayed.StopReason != m.StopReason {
				t.Fatalf("replay: %v", err)
			}
			for _, worker := range m.Workers {
				if worker.ExitCode != 0 || worker.SummaryHash == "" || worker.VUs < 2 {
					t.Fatalf("worker failed: %+v", worker)
				}
			}
		})
	}
}
func ExampleScaleConfig_Validate() {
	c := scaleConfiguration()
	fmt.Println(c.Validate())
	// Output: <nil>
}
func ExampleRunScale() {
	_, err := RunScale(context.Background(), ScaleConfig{}, "unused")
	fmt.Println(err)
	// Output: binary and script are required
}

func TestScaleReadAssessmentMutation(t *testing.T) {
	c := scaleConfiguration()
	w := ScaleWindow{Epoch: 1, StartedAt: 100, FinishedAt: 300, RequestLimit: 1, PrefillLimit: 2, Samples: []ScaleSample{{ID: "1", Worker: 0, Epoch: 1, DispatchedAt: 120, FinishedAt: 130, Latency: 10, Outcome: "success"}, {ID: "2", Worker: 0, Epoch: 1, DispatchedAt: 140, FinishedAt: 160, Latency: 20, Outcome: "success"}}}
	assess(&w, []bool{true, true}, c, "")
	m := ScaleManifest{SchemaVersion: 1, Identity: "receipt", StopReason: "cancelled", Config: c, StartedAt: time.UnixMilli(100), FinishedAt: time.UnixMilli(400), Windows: []ScaleWindow{w}, Workers: []ScaleWorker{{ID: 0}, {ID: 1}}}
	b, _ := json.Marshal(m)
	if _, e := ReadScale(bytes.NewReader(b)); e != nil {
		t.Fatal(e)
	}
	m.Windows[0].Completed++
	b, _ = json.Marshal(m)
	if _, e := ReadScale(bytes.NewReader(b)); e == nil {
		t.Fatal("forged summary accepted")
	}
}
func TestBuiltK6GlobalScaleFailedAssessment(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to signed extension binary")
	}
	binary, e := filepath.Abs(binary)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(25 * time.Millisecond)
		w.WriteHeader(503)
		fmt.Fprint(w, "fixture refusal")
	}))
	defer server.Close()
	t.Setenv("ENDPOINT_URL", server.URL)
	c := scaleConfiguration()
	c.Binary = binary
	c.Script, e = filepath.Abs("../examples/campaign/global-scale.js")
	if e != nil {
		t.Fatal(e)
	}
	m, e := RunScale(context.Background(), c, filepath.Join(t.TempDir(), "failure"))
	if e != nil {
		t.Fatal(e)
	}
	if m.StopReason != "no_passing_boundary" || len(m.Windows) != 1 || m.Windows[0].Passed || m.Windows[0].Failures == 0 {
		b, _ := json.MarshalIndent(m, "", "  ")
		t.Fatalf("failed assessment lost: %s", b)
	}
}
func ExampleReadScale() {
	_, err := ReadScale(strings.NewReader(`{}`))
	fmt.Println(err)
	// Output: binary and script are required
}

func TestScaleReplayControlAndStopMutation(t *testing.T) {
	c := scaleConfiguration()
	w := ScaleWindow{Epoch: 1, RequestLimit: 1, PrefillLimit: 2, Passed: true}
	m := ScaleManifest{Config: c, Windows: []ScaleWindow{w}, StopReason: "cancelled"}
	if e := verifyScaleDecisions(&m); e != nil {
		t.Fatal(e)
	}
	m.Windows[0].RequestLimit = 2
	if e := verifyScaleDecisions(&m); e == nil {
		t.Fatal("forged first control cap accepted")
	}
	m.Windows[0] = w
	m.StopReason = "sustain_complete"
	if e := verifyScaleDecisions(&m); e == nil {
		t.Fatal("premature favorable stopping accepted")
	}
	m.StopReason = "no_passing_boundary"
	m.Windows[0].Passed = false
	if e := verifyScaleDecisions(&m); e != nil {
		t.Fatal(e)
	}
}

func TestBuiltK6GlobalScaleAdmissionCancellation(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to signed extension binary")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("ENDPOINT_URL", server.URL)
	c := scaleConfiguration()
	c.Binary = binary
	c.Script, err = filepath.Abs("../examples/campaign/global-scale.js")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checkpoint := func(path string, m *ScaleManifest) error {
		if len(m.Windows) > 0 && m.Windows[len(m.Windows)-1].Reason == "admission_pending" {
			cancel()
		}
		return writeJSON(path, m)
	}
	m, err := runScale(ctx, c, filepath.Join(t.TempDir(), "cancel"), verifyInputs, checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if m.StopReason != "cancelled" || len(m.Workers) != c.Workers || len(m.Windows) != 1 {
		t.Fatalf("cancellation: %+v", m)
	}
	if calls.Load() != 0 || m.Windows[0].StartedAt != 0 || len(m.Windows[0].Samples) != 0 {
		t.Fatal("cancelled admission was activated")
	}
	b, _ := json.Marshal(m)
	if _, err := ReadScale(bytes.NewReader(b)); err == nil {
		t.Fatal("unassessed pending receipt accepted as completed assessment")
	}
}

func TestScaleLatencyAdmissionAndReplay(t *testing.T) {
	for _, tt := range []struct {
		name          string
		span, latency float64
		accepted      bool
	}{
		{"coherent", 10, 10, true},
		{"epoch roundoff", 10.0001, 10, true},
		{"outside tolerance", 10.01, 10, false},
		{"understated latency", 1000, 10, false},
		{"overstated latency", 10, 1000, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := scaleConfiguration()
			c.Workers = 1
			c.MinCompleted = 1
			c.MaxLatency = 100
			sample := ScaleSample{ID: "latency", Worker: 0, Epoch: 1, DispatchedAt: 120, FinishedAt: 120 + tt.span, Latency: tt.latency, Outcome: "success"}
			control := &scaleControl{config: c, ready: []bool{true}, participating: []bool{true}, seen: map[string]bool{}, active: true, window: &ScaleWindow{Epoch: 1, StartedAt: 100, FinishedAt: 2000, RequestLimit: 1, PrefillLimit: 2}}
			b, err := json.Marshal(sample)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/sample?worker=0", bytes.NewReader(b))
			response := httptest.NewRecorder()
			control.serveHTTP(response, req)
			want := http.StatusBadRequest
			if tt.accepted {
				want = http.StatusAccepted
			}
			if response.Code != want {
				t.Fatalf("admission status %d want %d", response.Code, want)
			}
			if !tt.accepted && len(control.window.Samples) != 0 {
				t.Fatal("contradictory sample retained for assessment")
			}

			// Construct the replay independently: a receipt asserting a favorable
			// assessment must be refused when its own timestamp span contradicts it.
			latency := tt.latency
			window := ScaleWindow{Epoch: 1, StartedAt: 100, FinishedAt: 2000, RequestLimit: 1, PrefillLimit: 2, ParticipatingWorkers: []int{0}, Completed: 1, P95: &latency, Passed: tt.latency <= 100, Reason: "passed", Samples: []ScaleSample{sample}}
			if !window.Passed {
				window.Reason = "latency"
			}
			m := ScaleManifest{SchemaVersion: 1, Identity: "latency-fixture", Config: c, StartedAt: time.UnixMilli(100), FinishedAt: time.UnixMilli(2100), Windows: []ScaleWindow{window}, Workers: []ScaleWorker{{ID: 0}}, StopReason: "cancelled"}
			b, err = json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ReadScale(bytes.NewReader(b))
			if (err == nil) != tt.accepted {
				t.Fatalf("replay error %v accepted=%v", err, tt.accepted)
			}
		})
	}
}
