package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("CAMPAIGN_HELPER") == "1" {
		helper()
		return
	}
	os.Exit(m.Run())
}
func helper() {
	summary := ""
	value := "1"
	for i, a := range os.Args {
		if a == "--summary-export" && i+1 < len(os.Args) {
			summary = os.Args[i+1]
		}
		if a == "--env" && i+1 < len(os.Args) && strings.HasPrefix(os.Args[i+1], "LOAD=") {
			value = strings.TrimPrefix(os.Args[i+1], "LOAD=")
		}
	}
	if os.Getenv("CAMPAIGN_DISTRIBUTION") == "1" {
		os.WriteFile(filepath.Join(filepath.Dir(summary), "distribution.json"), []byte(`[1,2,3,4]`), 0600)
	}
	if value == "slow" {
		time.Sleep(time.Minute)
	}
	if path := os.Getenv("CAMPAIGN_MUTATE_ASSET"); path != "" {
		os.WriteFile(path, []byte("changed"), 0600)
	}
	number, _ := strconv.ParseFloat(value, 64)
	payload := fmt.Sprintf(`{"metrics":{"score":{"values":{"avg":%g}},"sla":{"values":{"avg":%g}}}}`, number, number)
	if value == "missing" {
		payload = `{"metrics":{}}`
	}
	if value == "null" {
		payload = `{"metrics":{"score":{"values":{"avg":null}}}}`
	}
	if e := os.WriteFile(summary, []byte(payload), 0600); e != nil {
		os.Exit(3)
	}
	if value == "bad" {
		os.Exit(2)
	}
	os.Exit(0)
}
func configuration(t *testing.T) Config {
	t.Helper()
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	script := filepath.Join(t.TempDir(), "fixture.js")
	if e = os.WriteFile(script, []byte("fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	return Config{Binary: binary, Script: script, Mode: "grid", Dimensions: []Dimension{{"LOAD", []string{"1", "2"}}}, Objective: Metric{"score", "avg"}, MaxCandidates: 10, MaxAttempts: 30, MinRuns: 2, MaxRuns: 3, Convergence: "cv", Tolerance: 0, Timeout: "5s", MaxDuration: "30s"}
}
func TestConvergence(t *testing.T) {
	for _, tt := range []struct {
		name, method string
		v            []float64
		tol          float64
		want         bool
	}{{"stable", "cv", []float64{2, 2, 2}, 0, true}, {"unstable", "cv", []float64{1, 3}, .1, false}, {"minimum", "cv", []float64{1}, 1, false}, {"zero", "cv", []float64{0, 0}, 0, true}, {"zero varied", "cv", []float64{-1, 1}, 1, false}, {"range", "range", []float64{9, 10, 11}, .2, true}, {"CI full width", "ci95", []float64{9, 10, 11}, .3, true}, {"nonfinite", "cv", []float64{1, math.Inf(1)}, 1, false}, {"unknown", "bad", []float64{1, 1}, 1, false}} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Converged(tt.v, tt.method, tt.tol, 2); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
func TestRunResumeFailureAndAssets(t *testing.T) {
	t.Setenv("CAMPAIGN_HELPER", "1")
	c := configuration(t)
	c.Dimensions[0].Values = []string{"1", "bad", "missing", "null", "2"}
	c.Retries = 1
	directory := t.TempDir()
	m, e := Run(context.Background(), c, directory)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Candidates) != 5 || m.BestID != cellID(map[string]string{"LOAD": "2"}) {
		t.Fatalf("manifest: %+v", m)
	}
	for _, cell := range m.Candidates {
		if len(cell.Attempts) != 2 {
			t.Fatalf("attempts: %+v", cell)
		}
		for _, a := range cell.Attempts {
			if _, e = os.Stat(filepath.Join(directory, a.Directory, "attempt.json")); e != nil {
				t.Fatal(e)
			}
		}
	}
	bad := m.Candidates[1]
	if !bad.Failed || bad.Feasible || bad.Attempts[0].SummaryHash == "" || bad.Attempts[0].ExitCode != 2 {
		t.Fatalf("failure lost: %+v", bad)
	}
	again, e := Run(context.Background(), c, directory)
	if e != nil {
		t.Fatal(e)
	}
	if len(again.Candidates) != 5 || len(again.Candidates[1].Attempts) != 2 {
		t.Fatal("resume duplicated work")
	}
	if e = os.WriteFile(c.Script, []byte("changed"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Run(context.Background(), c, directory); e == nil {
		t.Fatal("changed asset accepted")
	}
}
func TestInterruptedAttemptResume(t *testing.T) {
	t.Setenv("CAMPAIGN_HELPER", "1")
	c := configuration(t)
	c.Retries = 1
	c.Dimensions[0].Values = []string{"1"}
	directory := t.TempDir()
	m, e := Run(context.Background(), c, directory)
	if e != nil {
		t.Fatal(e)
	}
	cell := &m.Candidates[0]
	if e = os.RemoveAll(filepath.Join(directory, cell.Attempts[1].Directory)); e != nil {
		t.Fatal(e)
	}
	cell.Attempts = cell.Attempts[:1]
	cell.Attempts[0].Error = "interrupted before completion"
	cell.Attempts[0].ExitCode = -1
	cell.Attempts[0].Value = nil
	cell.Attempts[0].SummaryHash = ""
	cell.Attempts[0].Passed = false
	if e = os.Remove(filepath.Join(directory, cell.Attempts[0].Directory, "attempt.json")); e != nil {
		t.Fatal(e)
	}
	cell.Converged = false
	cell.Feasible = false
	if e = writeJSON(filepath.Join(directory, "campaign.json"), m); e != nil {
		t.Fatal(e)
	}
	m, e = Run(context.Background(), c, directory)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Candidates[0].Attempts) != 3 || m.Candidates[0].Feasible || m.BestID != "" {
		t.Fatalf("interrupted attempt hidden: %+v", m.Candidates[0])
	}
}
func TestTimeoutAndLock(t *testing.T) {
	t.Setenv("CAMPAIGN_HELPER", "1")
	c := configuration(t)
	c.Timeout = "20ms"
	c.Dimensions[0].Values = []string{"slow"}
	dir := t.TempDir()
	m, e := Run(context.Background(), c, dir)
	if e != nil {
		t.Fatal(e)
	}
	if !m.Candidates[0].Failed || m.Candidates[0].Attempts[0].ExitCode == 0 {
		t.Fatal("timeout was accepted")
	}
	if e = os.WriteFile(filepath.Join(dir, ".lock"), nil, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Run(context.Background(), c, dir); e == nil {
		t.Fatal("concurrent writer accepted")
	}
}
func TestPlanners(t *testing.T) {
	for _, mode := range []string{"grid", "halton", "adaptive"} {
		t.Run(mode, func(t *testing.T) {
			c := Config{Mode: mode, Seed: 123, Dimensions: []Dimension{{"A", []string{"1", "2", "3"}}, {"B", []string{"x", "y"}}}}
			m := &Manifest{}
			seen := map[string]bool{}
			for i := 0; i < 6; i++ {
				p := next(c, grid(c.Dimensions), m)
				id := cellID(p)
				if p == nil || seen[id] {
					t.Fatal("duplicate/missing cell")
				}
				seen[id] = true
				m.Candidates = append(m.Candidates, Candidate{ID: id, Parameters: p})
				m.BestID = id
			}
			if next(c, grid(c.Dimensions), m) != nil {
				t.Fatal("unbounded planner")
			}
		})
	}
	c := Config{Mode: "scale", Dimensions: []Dimension{{"A", []string{"1", "2", "3", "4", "5", "6", "7", "8"}}}}
	m := &Manifest{}
	var got []string
	for {
		p := next(c, grid(c.Dimensions), m)
		if p == nil {
			break
		}
		v, _ := strconv.Atoi(p["A"])
		got = append(got, p["A"])
		m.Candidates = append(m.Candidates, Candidate{ID: cellID(p), Parameters: p, Converged: true, Feasible: v <= 5})
	}
	if strings.Join(got, ",") != "1,2,4,8,6,5" {
		t.Fatal(got)
	}
}
func TestBounds(t *testing.T) {
	c := configuration(t)
	for _, modify := range []func(*Config){func(c *Config) { c.MaxAttempts = 0 }, func(c *Config) { c.Dimensions[0].Name = "PATH" }, func(c *Config) { c.Mode = "scale" }, func(c *Config) { c.MinRuns = 1 }, func(c *Config) { c.Tolerance = math.NaN() }} {
		bad := c
		bad.Dimensions = append([]Dimension(nil), c.Dimensions...)
		modify(&bad)
		if bad.Validate() == nil {
			t.Fatal("invalid config accepted")
		}
	}
}
func TestBuiltK6Campaign(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to the signed extension binary")
	}
	binary, e := filepath.Abs(binary)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"fixture","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()
	t.Setenv("CAMPAIGN_ENDPOINT", server.URL)
	script := filepath.Join(t.TempDir(), "fixture.js")
	source := `import llm from 'k6/x/llm-perf';import { check } from 'k6';import { Counter } from 'k6/metrics';const score=new Counter('campaign_score');export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_requests:['count>0']}};const client=new llm.Client({baseURL:__ENV.CAMPAIGN_ENDPOINT,model:'fixture'});export default function(){const r=client.chat.completions.create({messages:[{role:'user',content:'hi'}],stream:false});check(r,{ok:r=>!(r instanceof Error)&&r.choices[0].message.content==='ok'});score.add(Number(__ENV.LOAD));}`
	if e = os.WriteFile(script, []byte(source), 0600); e != nil {
		t.Fatal(e)
	}
	c := configuration(t)
	c.Binary = binary
	c.Script = script
	c.Objective = Metric{"campaign_score", "count"}
	m, e := Run(context.Background(), c, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if m.BestID != cellID(map[string]string{"LOAD": "2"}) || len(m.Candidates) != 2 {
		b, _ := json.MarshalIndent(m, "", "  ")
		t.Fatalf("native campaign failed: %s", b)
	}
}

func TestDistributionRun(t *testing.T) {
	t.Setenv("CAMPAIGN_HELPER", "1")
	t.Setenv("CAMPAIGN_DISTRIBUTION", "1")
	c := configuration(t)
	c.Convergence = "distribution"
	c.DistributionFile = "distribution.json"
	dir := t.TempDir()
	m, e := Run(context.Background(), c, dir)
	if e != nil {
		t.Fatal(e)
	}
	if m.BestID == "" || !m.Candidates[0].Converged {
		t.Fatal("stable distribution did not converge")
	}
	path := filepath.Join(dir, m.Candidates[0].Attempts[0].Directory, c.DistributionFile)
	if e = os.WriteFile(path, []byte(`[2,3,4,5]`), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Run(context.Background(), c, dir); e == nil {
		t.Fatal("changed sample artifact accepted")
	}
}
func TestDurationAndAttemptBudgets(t *testing.T) {
	t.Setenv("CAMPAIGN_HELPER", "1")
	c := configuration(t)
	c.MaxAttempts = 1
	m, e := Run(context.Background(), c, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if m.StopReason != "attempt_budget" || len(m.Candidates) != 1 || len(m.Candidates[0].Attempts) != 1 || m.BestID != "" {
		t.Fatalf("budget bypass: %+v", m)
	}
	c.MaxAttempts = 10
	c.MaxDuration = "1ms"
	c.Dimensions[0].Values = []string{"slow"}
	m, e = Run(context.Background(), c, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if m.StopReason != "duration_budget" {
		t.Fatalf("duration budget: %+v", m)
	}
}

func TestBuiltK6SingleRunScale(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY to signed extension binary")
	}
	binary, e := filepath.Abs(binary)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer server.Close()
	t.Setenv("ENDPOINT_URL", server.URL)
	t.Setenv("SCALE_WINDOWS", "3")
	t.Setenv("SCALE_WINDOW_MS", "100")
	t.Setenv("SCALE_MIN_REQUESTS", "1")
	t.Setenv("SCALE_MAX_RATE", "1")
	t.Setenv("SCALE_SUSTAIN_WINDOWS", "2")
	script, e := filepath.Abs("../examples/campaign/window-scale.js")
	if e != nil {
		t.Fatal(e)
	}
	c := configuration(t)
	c.Binary = binary
	c.Script = script
	c.Dimensions = nil
	c.Objective = Metric{"campaign_scale_windows", "count"}
	directory := t.TempDir()
	m, e := Run(context.Background(), c, directory)
	if e != nil {
		t.Fatal(e)
	}
	if m.BestID == "" || len(m.Candidates) != 1 {
		b, _ := json.MarshalIndent(m, "", "  ")
		t.Fatalf("single run scale failed: %s", b)
	}
	for _, a := range m.Candidates[0].Attempts {
		log, e := os.ReadFile(filepath.Join(directory, a.Directory, "process.log"))
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(string(log), "scale_window") || !strings.Contains(string(log), "sustaining") {
			t.Fatal("window receipts absent")
		}
	}
}

func TestScaleFailedAssessment(t *testing.T) {
	t.Setenv("CAMPAIGN_HELPER", "1")
	c := configuration(t)
	c.Mode = "scale"
	c.Dimensions[0].Values = []string{"1", "2"}
	c.Objective = Metric{"missing", "avg"}
	c.Constraints = []Constraint{{Metric: Metric{"sla", "avg"}, Direction: "le", Bound: 5}}
	m, e := Run(context.Background(), c, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if m.StopReason != "scale_assessment_failed" || len(m.Candidates) != 1 {
		t.Fatalf("failed assessment reported boundary: %+v", m)
	}
}

func TestCheckpointAttemptMutation(t *testing.T) {
	t.Setenv("CAMPAIGN_HELPER", "1")
	for _, field := range []string{"value", "passed", "exit", "error"} {
		t.Run(field, func(t *testing.T) {
			c := configuration(t)
			directory := t.TempDir()
			m, e := Run(context.Background(), c, directory)
			if e != nil {
				t.Fatal(e)
			}
			a := &m.Candidates[0].Attempts[0]
			switch field {
			case "value":
				v := 123.
				a.Value = &v
			case "passed":
				a.Passed = false
			case "exit":
				a.ExitCode = 2
			case "error":
				a.Error = "fake"
			}
			if e = writeJSON(filepath.Join(directory, "campaign.json"), m); e != nil {
				t.Fatal(e)
			}
			if _, e = Run(context.Background(), c, directory); e == nil {
				t.Fatal("mutated attempt accepted")
			}
		})
	}
}

func TestAssetsChangeDuringExecution(t *testing.T) {
	t.Setenv("CAMPAIGN_HELPER", "1")
	c := configuration(t)
	asset := filepath.Join(t.TempDir(), "asset.json")
	if e := os.WriteFile(asset, []byte("original"), 0600); e != nil {
		t.Fatal(e)
	}
	c.Assets = []string{asset}
	t.Setenv("CAMPAIGN_MUTATE_ASSET", asset)
	m, e := Run(context.Background(), c, t.TempDir())
	if e == nil || m == nil || m.StopReason != "assets_changed" || len(m.Candidates) != 1 || len(m.Candidates[0].Attempts) != 1 || m.BestID != "" {
		t.Fatalf("asset drift ignored: %+v %v", m, e)
	}
}

func TestCheckpointDeletionRefusal(t *testing.T) {
	t.Setenv("CAMPAIGN_HELPER", "1")
	for _, field := range []string{"candidate", "attempt"} {
		t.Run(field, func(t *testing.T) {
			c := configuration(t)
			directory := t.TempDir()
			m, e := Run(context.Background(), c, directory)
			if e != nil {
				t.Fatal(e)
			}
			if field == "candidate" {
				m.Candidates = m.Candidates[1:]
			} else {
				m.Candidates[0].Attempts = m.Candidates[0].Attempts[:1]
			}
			if e = writeJSON(filepath.Join(directory, "campaign.json"), m); e != nil {
				t.Fatal(e)
			}
			if _, e = Run(context.Background(), c, directory); e == nil {
				t.Fatal("deleted durable receipt accepted")
			}
		})
	}
}
