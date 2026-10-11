package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	llmperf "github.com/tmc/misc/xk6-llm-perf"
	"github.com/tmc/misc/xk6-llm-perf/serving"
)

func TestPairedProcessHelper(t *testing.T) {
	mode := os.Getenv("PAIRED_HELPER")
	if mode == "" {
		return
	}
	if mode == "server" {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/observe" {
				fmt.Fprint(w, os.Getenv("PAIRED_MANIFEST"))
				return
			}
			if os.Getenv("PAIRED_SAMPLED") == "1" {
				fmt.Fprint(w, `{"model":"capacity-fixture","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			} else {
				fmt.Fprint(w, `{"model":"served"}`)
			}
		})
		if err := http.ListenAndServe(os.Getenv("PAIRED_ADDR"), nil); err != nil {
			os.Exit(2)
		}
		return
	}
	start := float64(time.Now().UnixMilli())
	payload := []byte(`{"prompt":"x"}`)
	client := &http.Client{Transport: &http.Transport{MaxConnsPerHost: 16, ForceAttemptHTTP2: true}}
	res, err := client.Post(os.Getenv("PAIRED_URL")+"/work", "application/json", bytes.NewReader(payload))
	if err != nil {
		os.Exit(3)
	}
	response, _ := io.ReadAll(res.Body)
	res.Body.Close()
	time.Sleep(time.Millisecond)
	requestFinish := float64(time.Now().UnixMilli())
	finish := requestFinish
	if os.Getenv("PAIRED_SAMPLED") == "1" {
		finish = start + 100
		time.Sleep(time.Until(time.UnixMilli(int64(finish))))
	}
	zero := 0
	var m llmperf.ServingManifest
	if json.Unmarshal([]byte(os.Getenv("PAIRED_MANIFEST")), &m) != nil {
		os.Exit(4)
	}
	if os.Getenv("PAIRED_BAD") == "arm" {
		m.Arm.BinaryDigest = strings.Repeat("f", 64)
	}
	w := &llmperf.WorkReceipt{Status: "accepted", Expectation: llmperf.WorkExpectation{ExpectedModel: "served", AllowedFinishReasons: []string{"stop"}, RequireExactUsage: true, MinOutputTokens: &zero, MaxOutputTokens: &zero}, ObservedModel: "served", FinishReasons: []string{"stop"}, ExactUsage: true, ReportedOutputTokens: &zero}
	r := llmperf.RunSummary{SchemaVersion: 1, RunID: fmt.Sprint(os.Getpid()), ServingManifest: &m, StartedAt: start, FinishedAt: finish, Duration: (finish - start) / 1000, Attempts: 1, Successes: 1, GoodRequests: 1, WorkQualified: 1, WorkAccepted: 1, WorkGoodRequests: 1, WorkGoodput: 1000 / (finish - start), Goodput: 1000 / (finish - start), Records: []llmperf.Measurement{{SchemaVersion: 1, Endpoint: os.Getenv("PAIRED_URL") + "/work", Phase: "measurement", Attempted: true, Recorded: true, Good: true, Outcome: "success", DispatchedAt: start, FinishedAt: requestFinish, FullResponseLatency: requestFinish - start, RequestBodyBytes: int64(len(payload)), ResponseBodyBytes: int64(len(response)), MaxConnsPerHost: 16, TransportProfile: "shared-http2-enabled-v1", Protocol: "openai-chat", ProviderTotalTokens: &zero, InputTokens: llmperf.TokenCount{Value: &zero, Source: "reported"}, OutputTokens: llmperf.TokenCount{Value: &zero, Source: "reported"}, Work: w}}}
	if os.Getenv("PAIRED_SAMPLED") == "1" {
		lag := 0.0
		latency := requestFinish - start
		r.Records[0].ScheduledAt = &start
		r.Records[0].ArrivalLag = &lag
		r.Records[0].ScheduledResponseLatency = &latency
	}
	if os.Getenv("PAIRED_BAD") == "geometry" {
		r.Records[0].RequestBodyBytes++
	}
	if os.Getenv("PAIRED_BAD") == "endpoint" {
		r.Records[0].Endpoint = "http://foreign.invalid/work"
	}
	if os.Getenv("PAIRED_SAMPLED") == "1" {
		one, two := 1, 2
		r.InputTokens = 1
		r.OutputTokens = 1
		r.InputThroughput = 1 / r.Duration
		r.OutputThroughput = 1 / r.Duration
		r.Records[0].Model = "capacity-fixture"
		r.Records[0].InputTokens.Value = &one
		r.Records[0].OutputTokens.Value = &one
		r.Records[0].ProviderTotalTokens = &two
		w.Expectation.ExpectedModel = "capacity-fixture"
		w.Expectation.MinOutputTokens = &one
		w.Expectation.MaxOutputTokens = &one
		w.ObservedModel = "capacity-fixture"
		w.ReportedOutputTokens = &one
	}
	r.MeasurementPolicy = "dispatch-content-v1/success-token-counts-v1/success-slo-v1"
	r.WindowPolicy = "attempt started and completed inside window; successful token totals"
	r.WarmupPolicy = "phase=measurement and per-client warmup exhausted"
	r.Records[0].SLO = &llmperf.SLOReceipt{MaxTPOT: new(float64), MaxE2E: new(float64)}
	if os.Getenv("PAIRED_SLO") == "strict" {
		r.Records[0].SLO.MaxTTFT = 1
		r.Records[0].Good = false
		r.GoodRequests = 0
		r.Goodput = 0
		r.WorkGoodRequests = 0
		r.WorkGoodput = 0
	}
	if os.Getenv("PAIRED_POLICY_MISSING") == "slo" {
		r.Records[0].SLO = nil
	}
	if os.Getenv("PAIRED_POLICY_MISSING") == "measurement" {
		r.MeasurementPolicy = ""
	}
	b, _ := json.Marshal(r)
	if os.WriteFile(os.Getenv("PAIRED_ARTIFACT"), b, 0600) != nil {
		os.Exit(5)
	}
}
func pairedProcessConfig(t *testing.T) pairedConfig {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(b)
	digest := hex.EncodeToString(hash[:])
	plan, _ := serving.PlanPairs(1, 2, []string{"goodput"})
	plan.Policy = pairedTestRunPolicy()
	c := pairedConfig{Plan: plan}
	dir := t.TempDir()
	for i, order := range plan.Orders {
		for j := range order {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := l.Addr().String()
			l.Close()
			url := "http://" + addr
			m := llmperf.ServingManifest{SchemaVersion: 1, ModelDigest: strings.Repeat("a", 64), TokenizerDigest: strings.Repeat("a", 64), TemplateDigest: strings.Repeat("a", 64), WorkloadDigest: strings.Repeat("a", 64), EnvironmentDigest: strings.Repeat("a", 64), Settings: map[string]llmperf.ServingSetting{}, Arm: llmperf.ServingArm{BinaryDigest: digest, SourceRevision: strings.Repeat("b", 40), Backend: "test", EndpointIdentity: "paired-test"}}
			for _, name := range []string{"cache", "compile", "resources", "scheduler", "dtype", "device", "temperature", "top_p", "top_k", "seed", "max_output_tokens"} {
				v := `0`
				if name == "cache" || name == "compile" || name == "resources" || name == "scheduler" {
					v = `{"enabled":false}`
				}
				if name == "dtype" || name == "device" {
					v = `"test"`
				}
				m.Settings[name] = llmperf.ServingSetting{Effective: json.RawMessage(v), EvidenceDigest: strings.Repeat("a", 64)}
			}
			manifest, _ := json.Marshal(m)
			obs := sha256.Sum256(manifest)
			artifact := filepath.Join(dir, fmt.Sprintf("run-%d-%d.json", i, j))
			server := serving.Command{Path: exe, SHA256: digest, Args: []string{"-test.run=^TestPairedProcessHelper$"}, Env: []string{"PAIRED_HELPER=server", "PAIRED_ADDR=" + addr, "PAIRED_MANIFEST=" + string(manifest)}}
			workload := server
			workload.Env = []string{"PAIRED_HELPER=work", "PAIRED_URL=" + url, "PAIRED_MANIFEST=" + string(manifest), "PAIRED_ARTIFACT=" + artifact}
			c.Runs = append(c.Runs, pairedExecution{Pair: i, Arm: string(order[j]), RunArtifact: artifact, CapacityArtifact: filepath.Join(dir, "missing-capacity.json"), Lifecycle: serving.LifecycleConfig{Server: server, Workload: workload, Ready: serving.Probe{URL: url + "/ready", Status: 200}, Observe: serving.Probe{URL: url + "/observe", Status: 200}, ExpectedEndpointIdentity: "paired-test", ExpectedObservationSHA256: hex.EncodeToString(obs[:]), Warmup: []serving.Probe{{URL: url + "/warmup", Status: 200}}, Drain: serving.Probe{URL: url + "/drain", Status: 200}, StartupMS: 5000, WorkloadMS: 5000, StopMS: 1000}})
		}
	}
	return c
}
func TestPairedRetainsActualFailedAttempt(t *testing.T) {
	c := pairedProcessConfig(t)
	r, err := executePaired(context.Background(), c)
	if err == nil || r.Complete || len(r.Attempts) != 1 || !r.Attempts[0].Lifecycle.Complete || r.Attempts[0].Error == "" || r.Attempts[0].RunSHA256 == "" {
		t.Fatalf("attempts=%d complete=%v error=%v", len(r.Attempts), r.Complete, err)
	}
	if !strings.Contains(err.Error(), "capacity") {
		t.Fatal(err)
	}
	if r.Experiment.Runs[0].ProcessID == "" {
		t.Fatal("actual process identity absent")
	}
}
func TestPairedRefusesStaleArtifact(t *testing.T) {
	c := pairedProcessConfig(t)
	if err := os.WriteFile(c.Runs[0].RunArtifact, []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := executePaired(context.Background(), c)
	if err == nil || len(r.Attempts) != 0 {
		t.Fatal("launched with stale artifact", err)
	}
}
func pairedCapacityFixture(t *testing.T, c *pairedConfig) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "capacity")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := serving.CapacityProbe{SchemaVersion: 1, Kind: "registered-k6-fixed-response-control", BinarySHA256: c.Runs[0].Lifecycle.Workload.SHA256, Rate: 1000, Duration: 1, Attempted: 1000, Accepted: 1000, WireRequests: 1000}
	rows := make([]map[string]any, 1000)
	wire := make([]float64, 1000)
	for i := range rows {
		wire[i] = float64(i)
		rows[i] = map[string]any{"vu": 1, "iteration": i, "scheduledAt": 1000 + i, "lag": 0, "start": 1000 + i, "finish": 1000 + float64(i) + .5, "result": map[string]any{"measurement": map[string]any{"transport_profile": "shared-http2-enabled-v1", "max_connections_per_host": 16, "protocol": "openai-chat", "request_body_bytes": 14, "response_body_bytes": 18, "stream": false, "outcome": "success", "parsing_latency_ms": 0, "serialization_latency_ms": 0, "work": map[string]any{"status": "accepted", "observed_model": "capacity-fixture", "exact_usage": true, "reported_output_tokens": 1, "finish_reasons": []string{"stop"}, "expectation": map[string]any{"expectedModel": "capacity-fixture", "requireExactUsage": true, "minOutputTokens": 1, "maxOutputTokens": 1, "allowedFinishReasons": []string{"stop"}}}}}, "pipeline": map[string]any{"max_wait_nanoseconds": 0, "generation": 1, "attempted": i + 1, "delivered": i + 1, "cancelled": 0, "inflight": 0}}
	}
	write := func(name string, v any) string {
		b, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(b)
		return hex.EncodeToString(h[:])
	}
	if err := os.WriteFile(filepath.Join(dir, "probe.js"), []byte("test fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte("test fixture"))
	p.ScriptSHA256 = hex.EncodeToString(h[:])
	producer := []byte("test producer fixture")
	if err := os.WriteFile(filepath.Join(dir, "producer.py"), producer, 0600); err != nil {
		t.Fatal(err)
	}
	ph := sha256.Sum256(producer)
	p.ProducerSHA256 = hex.EncodeToString(ph[:])
	p.WorkloadGeometry = "openai-chat/json/prompt-utf8-1/content-utf8-2/output-reported-1/v1"
	lag := 0.0
	p.MaxDispatchLagMilliseconds = &lag
	p.ParsingNanoseconds = &lag
	p.SerializationNanoseconds = &lag
	resources := []map[string]any{}
	for i := 0; i < 8; i++ {
		resources = append(resources, map[string]any{"at": .9 + float64(i)*.2, "ticks": 0, "start_ticks": 1, "rss": 1, "sockets": 1, "descriptors": 0})
	}
	p.AllocatedVUs = 1
	p.TicksPerSecond = 100
	p.PeakRSSBytes = 1
	p.PeakSocketDescriptors = 1
	p.ResourcesSHA256 = write("resources.json", resources)
	p.WireSHA256 = write("wire.json", wire)
	p.RowsSHA256 = write("rows.json", rows)
	p.SummarySHA256 = write("summary.json", map[string]any{"metrics": map[string]any{"iterations": map[string]any{"values": map[string]any{"count": 1000}}, "dropped_iterations": map[string]any{"values": map[string]any{"count": 0}}}})
	write("receipt.json", p)
	for i := range c.Runs {
		c.Runs[i].CapacityArtifact = dir
		c.Runs[i].CapacityProducerSHA256 = p.ProducerSHA256
	}
}
func TestPairedSupervisedComparison(t *testing.T) {
	c := pairedProcessConfig(t)
	pairedCapacityFixture(t, &c)
	r, err := executePaired(context.Background(), c)
	if err != nil || !r.Complete || r.Result == nil || r.Result.Qualified || len(r.Attempts) != 4 {
		t.Fatalf("attempts=%d complete=%v error=%v", len(r.Attempts), r.Complete, err)
	}
	for _, a := range r.Attempts {
		if !a.Lifecycle.Complete || a.RunSHA256 == "" || a.CapacitySHA256 == "" {
			t.Fatal("unbound attempt")
		}
	}
}
func TestPairedRefusesAliasedArtifact(t *testing.T) {
	c := pairedProcessConfig(t)
	c.Runs[1].RunArtifact = filepath.Join(filepath.Dir(c.Runs[0].RunArtifact), ".", filepath.Base(c.Runs[0].RunArtifact))
	r, err := executePaired(context.Background(), c)
	if err == nil || len(r.Attempts) != 0 {
		t.Fatal("launched aliased artifact", err)
	}
}
func TestPairedConfigSanitized(t *testing.T) {
	c := pairedProcessConfig(t)
	c.Runs[0].Lifecycle.Server.Env = []string{"TOKEN=supersecret"}
	c.Runs[0].Lifecycle.Server.Args = []string{"supersecret"}
	s := sanitizedPairedConfig(c)
	b, _ := json.Marshal(s)
	if strings.Contains(string(b), "supersecret") {
		t.Fatal("secret archived")
	}
	if c.Runs[0].Lifecycle.Server.Env[0] != "TOKEN=supersecret" {
		t.Fatal("execution configuration mutated")
	}
}
func TestPairedConfigSanitizesProbeURLs(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:1234/ready#private-fragment",
		"http://127.0.0.1:1234/ready#private%2Dfragment",
		"http://user:secret@127.0.0.1:1234/ready?secret=value#private-fragment",
		"http://127.0.0.1:1234/ready?",
	} {
		t.Run(raw, func(t *testing.T) {
			c := pairedProcessConfig(t)
			c.Runs[0].Lifecycle.Ready.URL = raw
			s := sanitizedPairedConfig(c)
			if got := s.Runs[0].Lifecycle.Ready.URL; got != "http://127.0.0.1:1234/ready" {
				t.Fatalf("sanitized URL = %q", got)
			}
			if c.Runs[0].Lifecycle.Ready.URL != raw {
				t.Fatal("execution configuration mutated")
			}
		})
	}
}

func TestPairedCommandArchivesSanitizedPlanAndInputs(t *testing.T) {
	c := pairedProcessConfig(t)
	pairedCapacityFixture(t, &c)
	dir := t.TempDir()
	input := filepath.Join(dir, "config.json")
	output := filepath.Join(dir, "receipt.json")
	b, _ := json.Marshal(c)
	if err := os.WriteFile(input, b, 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runPaired([]string{"-config", input, "-output", output}, &stdout, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	b, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var receipt pairedReceipt
	if err = json.Unmarshal(b, &receipt); err != nil {
		t.Fatal(err)
	}
	if !receipt.Complete || receipt.Result.Qualified || len(receipt.Attempts) != 4 {
		t.Fatal("invalid diagnostic receipt")
	}
	for _, a := range receipt.Attempts {
		if a.ArtifactsDirectory == "" {
			t.Fatal("missing archived snapshot")
		}
		if _, err := os.Stat(filepath.Join(a.ArtifactsDirectory, "capacity", "producer.py")); err != nil {
			t.Fatal(err)
		}
	}
	b, err = os.ReadFile(output + ".plan.json")
	if err != nil || strings.Contains(string(b), "PAIRED_HELPER=server") {
		t.Fatal("unsanitized preregistration", err)
	}
}
func TestPairedPrimaryRejectedBeforeLaunch(t *testing.T) {
	c := pairedProcessConfig(t)
	c.Plan.Primary = "undeclared"
	r, err := executePaired(context.Background(), c)
	if err == nil || len(r.Attempts) != 0 {
		t.Fatal("launched undeclared primary", err)
	}
}

func TestPairedForeignEvidenceRejected(t *testing.T) {
	for _, kind := range []string{"arm", "endpoint"} {
		t.Run(kind, func(t *testing.T) {
			c := pairedProcessConfig(t)
			pairedCapacityFixture(t, &c)
			c.Runs[0].Lifecycle.Workload.Env = append(c.Runs[0].Lifecycle.Workload.Env, "PAIRED_BAD="+kind)
			r, err := executePaired(context.Background(), c)
			if err == nil || r.Complete || len(r.Attempts) != 1 {
				t.Fatalf("attempts=%d error=%v", len(r.Attempts), err)
			}
			if !strings.Contains(err.Error(), kind) {
				t.Fatal(err)
			}
		})
	}
}

// The capacity archive below is synthetic validation input. These tests prove
// gate composition and subprocess supervision, not physical client capacity.
func pairedSampledFixture(t *testing.T, c *pairedConfig) {
	t.Helper()
	pairedCapacityFixture(t, c)
	dir := c.Runs[0].CapacityArtifact
	b, err := os.ReadFile(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p serving.CapacityProbe
	if err = json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	p.MaxConnsPerHost = 16
	b, err = os.ReadFile(filepath.Join(dir, "rows.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	response := `{"model":"capacity-fixture","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
	for _, row := range rows {
		m := row["result"].(map[string]any)["measurement"].(map[string]any)
		m["response_body_bytes"] = len(response)
	}
	b, _ = json.Marshal(rows)
	if err = os.WriteFile(filepath.Join(dir, "rows.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	p.RowsSHA256 = hex.EncodeToString(h[:])
	b, _ = json.Marshal(p)
	if err = os.WriteFile(filepath.Join(dir, "receipt.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	for i := range c.Runs {
		c.Runs[i].OfferedPerSecond = 10
		c.Runs[i].WorkloadGeometry = p.WorkloadGeometry
		c.Runs[i].CapacityBounds = &serving.CapacityBounds{MaxCPUPercent: 100, MaxRSSBytes: 1000000, MaxConnectionFraction: .5, MaxLagMilliseconds: 10, MaxPipelineWaitMilliseconds: 1}
		c.Runs[i].Lifecycle.Server.Env = append(c.Runs[i].Lifecycle.Server.Env, "PAIRED_SAMPLED=1")
		c.Runs[i].Lifecycle.Workload.Env = append(c.Runs[i].Lifecycle.Workload.Env, "PAIRED_SAMPLED=1")
	}
}
func TestPairedSampledGateComposition(t *testing.T) {
	c := pairedProcessConfig(t)
	pairedSampledFixture(t, &c)
	r, err := executePaired(context.Background(), c)
	if err != nil || !r.Complete || !r.Result.Qualified {
		t.Fatalf("attempts=%d error=%v reasons=%v", len(r.Attempts), err, r.Attempts[0].QualificationReasons)
	}
	for _, a := range r.Attempts {
		if a.CapacityStatus != "SAMPLED_CAPACITY" {
			t.Fatal(a.CapacityStatus)
		}
	}
	if !strings.Contains(r.Result.Effects[0].StatisticalStatus, "sampled client capacity") {
		t.Fatal("scope missing")
	}
}
func TestPairedSampledBoundFailure(t *testing.T) {
	c := pairedProcessConfig(t)
	pairedSampledFixture(t, &c)
	for i := range c.Runs {
		c.Runs[i].CapacityBounds.MaxRSSBytes = .5
	}
	r, err := executePaired(context.Background(), c)
	if err == nil || r.Complete || len(r.Attempts) != 1 {
		t.Fatal("bound did not stop comparison", err)
	}
}

func TestPairedGeometryMismatchRemainsDiagnostic(t *testing.T) {
	c := pairedProcessConfig(t)
	pairedSampledFixture(t, &c)
	for i := range c.Runs {
		c.Runs[i].Lifecycle.Workload.Env = append(c.Runs[i].Lifecycle.Workload.Env, "PAIRED_BAD=geometry")
	}
	r, err := executePaired(context.Background(), c)
	if err != nil || !r.Complete || r.Result.Qualified || !strings.Contains(r.Attempts[0].QualificationReasons[len(r.Attempts[0].QualificationReasons)-1], "geometry") {
		t.Fatalf("error=%v", err)
	}
}

func TestPairedCapacityRequiredResources(t *testing.T) {
	for _, field := range []string{"at", "ticks", "start_ticks", "rss", "sockets", "descriptors"} {
		for _, omitted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/omitted=%t", field, omitted), func(t *testing.T) {
				c := pairedProcessConfig(t)
				pairedSampledFixture(t, &c)
				x := c.Runs[0]
				if _, err := serving.VerifyCapacityProbe(x.CapacityArtifact, x.Lifecycle.Workload.SHA256); err != nil {
					t.Fatal(err)
				}
				mutateCapacityResources(t, x.CapacityArtifact, func(rows []map[string]any, p *serving.CapacityProbe) {
					for _, row := range rows {
						if omitted {
							delete(row, field)
						} else {
							row[field] = nil
						}
					}
				})
				if _, err := serving.VerifyCapacityProbe(x.CapacityArtifact, x.Lifecycle.Workload.SHA256); err == nil || !strings.Contains(err.Error(), "missing required numeric field") {
					t.Fatalf("missing observation accepted or wrong refusal: %v", err)
				}
			})
		}
	}
	t.Run("measured zeros and unavailable descriptor limit", func(t *testing.T) {
		c := pairedProcessConfig(t)
		pairedSampledFixture(t, &c)
		x := c.Runs[0]
		mutateCapacityResources(t, x.CapacityArtifact, func(rows []map[string]any, p *serving.CapacityProbe) {
			for _, row := range rows {
				for _, field := range []string{"ticks", "rss", "sockets", "descriptors"} {
					row[field] = 0
				}
				row["descriptor_limit"] = nil
			}
			p.PeakCPUPercent = 0
			p.PeakRSSBytes = 0
			p.PeakSocketDescriptors = 0
			p.PeakFileDescriptors = 0
			p.FileDescriptorLimit = nil
		})
		result, err := serving.EvaluateVerifiedProbe(x.CapacityArtifact, x.Lifecycle.Workload.SHA256, x.WorkloadGeometry, *x.CapacityBounds)
		if err != nil || result.Status != "SAMPLED_CAPACITY" {
			t.Fatalf("measured zero refused: %+v %v", result, err)
		}
	})
}

func mutateCapacityResources(t *testing.T, dir string, change func([]map[string]any, *serving.CapacityProbe)) {
	t.Helper()
	resourcePath := filepath.Join(dir, "resources.json")
	data, err := os.ReadFile(resourcePath)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(dir, "receipt.json")
	data, err = os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var p serving.CapacityProbe
	if err = json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	change(rows, &p)
	data, err = json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(resourcePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	p.ResourcesSHA256 = hex.EncodeToString(hash[:])
	data, err = json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(receiptPath, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPairedOfferedRateBoundary(t *testing.T) {
	run := llmperf.RunSummary{Duration: 2, Attempts: 21}
	for i := 0; i < 21; i++ {
		at := 1000 + float64(i)*100
		lag := 0.0
		run.Records = append(run.Records, llmperf.Measurement{ScheduledAt: &at, ArrivalLag: &lag})
	}
	for _, tt := range []struct {
		name          string
		offer         float64
		wantQualified bool
		wantError     bool
	}{{"inclusive end", 10, true, false}, {"higher offer", 11, false, true}, {"unknown offer", 0, false, false}} {
		t.Run(tt.name, func(t *testing.T) {
			qualified, reason, err := checkPairedOffer(10, tt.offer, run, nil)
			if qualified != tt.wantQualified || (err != nil) != tt.wantError {
				t.Fatalf("qualified=%v reason=%q error=%v", qualified, reason, err)
			}
			if !qualified && err == nil && reason == "" {
				t.Fatal("diagnostic reason absent")
			}
		})
	}
	run.Records[0].ScheduledAt = nil
	qualified, reason, err := checkPairedOffer(10, 10, run, nil)
	if qualified || err != nil || reason == "" {
		t.Fatal("missing schedule promoted", qualified, reason, err)
	}
}

func pairedTestRunPolicy() *serving.PairPolicy {
	return &serving.PairPolicy{MeasurementPolicy: "dispatch-content-v1/success-token-counts-v1/success-slo-v1", WindowPolicy: "attempt started and completed inside window; successful token totals", WarmupPolicy: "phase=measurement and per-client warmup exhausted", SLO: &llmperf.SLOReceipt{MaxTPOT: new(float64), MaxE2E: new(float64)}}
}
func TestPairedSLOConfoundRejected(t *testing.T) {
	c := pairedProcessConfig(t)
	pairedSampledFixture(t, &c)
	for i := range c.Runs {
		mode := "disabled"
		if c.Runs[i].Arm == "A" {
			mode = "strict"
		}
		c.Runs[i].Lifecycle.Workload.Env = append(c.Runs[i].Lifecycle.Workload.Env, "PAIRED_SLO="+mode)
	}
	r, err := executePaired(context.Background(), c)
	if err == nil || r.Complete || !strings.Contains(err.Error(), "slo") {
		t.Fatal("policy change admitted", err)
	}
}
func TestPairedPolicyEvidence(t *testing.T) {
	for _, name := range []string{"same strict unavailable timing", "missing SLO", "missing measurement", "missing preregistration"} {
		t.Run(name, func(t *testing.T) {
			c := pairedProcessConfig(t)
			pairedSampledFixture(t, &c)
			switch name {
			case "same strict unavailable timing":
				c.Plan.Policy.SLO.MaxTTFT = 1
				for i := range c.Runs {
					c.Runs[i].Lifecycle.Workload.Env = append(c.Runs[i].Lifecycle.Workload.Env, "PAIRED_SLO=strict")
				}
			case "missing SLO":
				for i := range c.Runs {
					c.Runs[i].Lifecycle.Workload.Env = append(c.Runs[i].Lifecycle.Workload.Env, "PAIRED_POLICY_MISSING=slo")
				}
			case "missing measurement":
				for i := range c.Runs {
					c.Runs[i].Lifecycle.Workload.Env = append(c.Runs[i].Lifecycle.Workload.Env, "PAIRED_POLICY_MISSING=measurement")
				}
			case "missing preregistration":
				c.Plan.Policy = nil
			}
			r, err := executePaired(context.Background(), c)
			if err != nil || !r.Complete {
				t.Fatal(err)
			}
			want := name == "same strict unavailable timing"
			if r.Result.Qualified != want {
				t.Fatalf("qualified=%v", r.Result.Qualified)
			}
			if want && (r.Result.Effects[0].Delta != 0 || r.Result.Effects[0].Ratio != nil) {
				t.Fatal("same-policy zero goodput changed")
			}
		})
	}
}
