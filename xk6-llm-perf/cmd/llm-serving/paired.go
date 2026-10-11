package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	llmperf "github.com/tmc/misc/xk6-llm-perf"
	"github.com/tmc/misc/xk6-llm-perf/serving"
)

type pairedExecution struct {
	Pair                   int                     `json:"pair"`
	Arm                    string                  `json:"arm"`
	Lifecycle              serving.LifecycleConfig `json:"lifecycle"`
	RunArtifact            string                  `json:"run_artifact"`
	OfferedPerSecond       float64                 `json:"offered_per_second"`
	CapacityBounds         *serving.CapacityBounds `json:"capacity_bounds,omitempty"`
	WorkloadGeometry       string                  `json:"workload_geometry"`
	CapacityProducerSHA256 string                  `json:"capacity_producer_sha256"`
	CapacityArtifact       string                  `json:"capacity_artifact"`
}
type pairedConfig struct {
	Plan             serving.PairPlan  `json:"plan"`
	Runs             []pairedExecution `json:"runs"`
	archiveDirectory string
}
type pairedAttempt struct {
	Pair                 int                      `json:"pair"`
	Arm                  string                   `json:"arm"`
	Lifecycle            serving.LifecycleReceipt `json:"lifecycle"`
	RunSHA256            string                   `json:"run_sha256,omitempty"`
	CapacitySHA256       string                   `json:"capacity_sha256,omitempty"`
	CapacityStatus       string                   `json:"capacity_status,omitempty"`
	ArtifactsDirectory   string                   `json:"artifacts_directory,omitempty"`
	QualificationReasons []string                 `json:"qualification_reasons,omitempty"`
	Error                string                   `json:"error,omitempty"`
}
type pairedReceipt struct {
	Config       pairedConfig             `json:"config"`
	ConfigSHA256 string                   `json:"config_sha256"`
	Experiment   serving.PairedExperiment `json:"experiment"`
	Attempts     []pairedAttempt          `json:"attempts"`
	Result       *serving.PairedResult    `json:"result,omitempty"`
	Complete     bool                     `json:"complete"`
}

func fileBytes(path string) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if err != nil {
		return nil, "", err
	}
	if len(b) > 64<<20 {
		return nil, "", fmt.Errorf("artifact exceeds 64 MiB")
	}
	h := sha256.Sum256(b)
	return b, hex.EncodeToString(h[:]), nil
}

// executePaired creates a fresh supervised server for each preregistered arm.
// It retains the failed attempt and stops immediately without retrying.
func executePaired(ctx context.Context, c pairedConfig) (out pairedReceipt, err error) {
	out.Config = sanitizedPairedConfig(c)
	canonical, _ := json.Marshal(c)
	hash := sha256.Sum256(canonical)
	out.ConfigSHA256 = hex.EncodeToString(hash[:])
	out.Experiment.Plan = c.Plan
	expected, e := serving.PlanPairs(c.Plan.Seed, len(c.Plan.Orders), c.Plan.Metrics)
	if e != nil {
		return out, e
	}
	if int64(len(c.Plan.Orders))*10000*int64(len(c.Plan.Metrics)) > 20000000 {
		return out, fmt.Errorf("paired bootstrap preregistration exceeds evaluation budget")
	}
	if c.Plan.Primary != expected.Primary {
		return out, fmt.Errorf("primary metric differs from preregistration")
	}
	if c.Plan.Version != 1 || len(c.Runs) != 2*len(expected.Orders) {
		return out, fmt.Errorf("invalid paired execution population")
	}
	if c.Plan.Policy != nil {
		if _, err := serving.PairPolicyHash(*c.Plan.Policy); err != nil {
			return out, err
		}
	}
	policyHash := ""
	policyEvidence := serving.PairPolicy{}
	paths := map[string]bool{}
	canonicalPath := func(path string) (string, error) {
		absolute, e := filepath.Abs(path)
		if e != nil {
			return "", e
		}
		parent, e := filepath.EvalSymlinks(filepath.Dir(absolute))
		if e != nil {
			return "", e
		}
		return filepath.Join(parent, filepath.Base(absolute)), nil
	}
	for i, x := range c.Runs {
		if math.IsNaN(x.OfferedPerSecond) || math.IsInf(x.OfferedPerSecond, 0) || x.OfferedPerSecond < 0 {
			return out, fmt.Errorf("invalid offered rate")
		}
		if x.CapacityBounds != nil {
			b := x.CapacityBounds
			for _, v := range []float64{b.MaxLagMilliseconds, b.MaxCPUPercent, b.MaxRSSBytes, b.MaxConnectionFraction, b.MaxPipelineWaitMilliseconds} {
				if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
					return out, fmt.Errorf("invalid capacity bound")
				}
			}
			if b.MaxCPUPercent <= 0 || b.MaxRSSBytes <= 0 || b.MaxConnectionFraction <= 0 || b.MaxConnectionFraction >= 1 {
				return out, fmt.Errorf("incomplete capacity bounds")
			}
		}
		if i > 0 {
			first := c.Runs[0].CapacityBounds
			if (first == nil) != (x.CapacityBounds == nil) || first != nil && *first != *x.CapacityBounds {
				return out, fmt.Errorf("capacity bounds differ between arms")
			}
		}
		if i > 0 && (x.OfferedPerSecond != c.Runs[0].OfferedPerSecond || x.WorkloadGeometry != c.Runs[0].WorkloadGeometry) {
			return out, fmt.Errorf("offered load or geometry differs between arms")
		}
		if i > 0 && x.Lifecycle.Workload.SHA256 != c.Runs[0].Lifecycle.Workload.SHA256 {
			return out, fmt.Errorf("client executable differs between arms")
		}
		if x.Pair != i/2 || x.Arm != string(expected.Orders[i/2][i%2]) || c.Plan.Orders[i/2] != expected.Orders[i/2] {
			return out, fmt.Errorf("execution differs from preregistered order")
		}
		resolved, e := canonicalPath(x.RunArtifact)
		if e != nil {
			return out, e
		}
		if paths[resolved] || x.RunArtifact == "" {
			return out, fmt.Errorf("run artifacts must be unique")
		}
		paths[resolved] = true
		if _, e := os.Stat(x.RunArtifact); !os.IsNotExist(e) {
			return out, fmt.Errorf("run artifact already exists or cannot be inspected")
		}
	}
	for _, x := range c.Runs {
		a := pairedAttempt{Pair: x.Pair, Arm: x.Arm}
		r := serving.PairRun{Pair: x.Pair, Arm: x.Arm}
		started := time.Now()
		a.Lifecycle, e = serving.Run(ctx, x.Lifecycle)
		if e == nil && !a.Lifecycle.Complete {
			e = fmt.Errorf("lifecycle incomplete")
		}
		if e == nil && c.archiveDirectory != "" {
			dir := filepath.Join(c.archiveDirectory, fmt.Sprintf("pair-%d-%s", x.Pair, x.Arm))
			if e = os.Mkdir(dir, 0700); e == nil {
				a.ArtifactsDirectory = dir
				x, e = snapshotPairedArtifacts(x, dir)
			}
		}
		if e == nil {
			e = readPairedArtifacts(x, a.Lifecycle, started, &a, &r, c.Plan.Metrics, c.Plan.Policy)
		}
		if e == nil && r.Policy != nil {
			policyEvidence, e = serving.MergePairPolicy(policyEvidence, *r.Policy)
		}
		if e == nil && r.PolicyHash != "" {
			if policyHash != "" && policyHash != r.PolicyHash {
				e = fmt.Errorf("measurement or SLO policy differs between runs")
			}
			policyHash = r.PolicyHash
		}
		if e != nil {
			a.Error = e.Error()
		}
		out.Attempts = append(out.Attempts, a)
		out.Experiment.Runs = append(out.Experiment.Runs, r)
		if e != nil {
			return out, e
		}
	}
	verified := map[string]serving.PairRun{}
	for _, r := range out.Experiment.Runs {
		verified[r.ProcessID] = r
	}
	result, e := serving.AnalyzePaired(out.Experiment, serving.PairedOptions{Seed: c.Plan.Seed, Resamples: 10000, Confidence: .95, Verify: func(r serving.PairRun) error {
		v, ok := verified[r.ProcessID]
		if !ok || v.Pair != r.Pair || v.Arm != r.Arm || v.ArtifactHash != r.ArtifactHash || v.BinaryHash != r.BinaryHash || v.WorkHash != r.WorkHash || v.PolicyHash != r.PolicyHash || v.PolicyComplete != r.PolicyComplete || !v.WorkAccepted {
			return fmt.Errorf("pair has no matching verified runtime artifacts")
		}
		return nil
	}, VerifyPolicy: func(r serving.PairRun, p serving.PairPolicy) error {
		expected, err := serving.PairPolicyHash(p)
		if err != nil {
			return err
		}
		v, ok := verified[r.ProcessID]
		if !ok || v.ArtifactHash != r.ArtifactHash || v.PolicyHash != r.PolicyHash || !v.PolicyComplete || !r.PolicyComplete || v.PolicyHash != expected {
			return fmt.Errorf("policy has no matching verified run receipts")
		}
		return nil
	}})
	if e != nil {
		return out, e
	}
	if result.Qualified {
		result.Assumptions += "; sampled client-capacity protocol only; unsampled spikes and engine parity remain unqualified"
		for i := range result.Effects {
			result.Effects[i].StatisticalStatus += "; sampled client capacity only"
		}
	}
	out.Result = &result
	out.Complete = true
	return out, nil
}
func readPairedArtifacts(x pairedExecution, l serving.LifecycleReceipt, started time.Time, a *pairedAttempt, r *serving.PairRun, metrics []string, policy *serving.PairPolicy) error {
	b, h, err := fileBytes(x.RunArtifact)
	if err != nil {
		return err
	}
	a.RunSHA256 = h
	if l.RunNonce == "" || l.LauncherPID <= 0 || l.WorkloadPID <= 0 || l.ServerStarted.IsZero() {
		return fmt.Errorf("lifecycle lacks independent process identity")
	}
	r.ProcessID = fmt.Sprintf("%s:%d:%s", l.RunNonce, l.LauncherPID, l.ServerStarted.Format(time.RFC3339Nano))
	r.ArtifactHash = h
	r.BinaryHash = l.ServerSHA256
	r.WorkHash = l.WorkHash
	run, err := llmperf.ReadRun(bytes.NewReader(b))
	if err != nil {
		return err
	}
	if run.StartedAt < float64(started.UnixMilli())-2 {
		return fmt.Errorf("run predates supervised process")
	}
	bounded := false
	for _, phase := range l.Phases {
		if phase.Name == "workload" {
			bounded = run.StartedAt >= float64(phase.Started.UnixMilli())-2 && run.FinishedAt <= float64(phase.Finished.UnixMilli())+2
		}
	}
	if !bounded {
		return fmt.Errorf("run outside supervised workload phase")
	}
	actualPolicy, policyComplete, err := serving.RunPairPolicy(run)
	if err != nil {
		return err
	}
	r.Policy = actualPolicy
	r.PolicyComplete = policyComplete
	if actualPolicy != nil && policy != nil {
		if _, err := serving.MergePairPolicy(*policy, *actualPolicy); err != nil {
			return err
		}
	}
	if policyComplete {
		r.PolicyHash, err = serving.PairPolicyHash(*actualPolicy)
		if err != nil {
			return err
		}
		if policy != nil {
			expected, err := serving.PairPolicyHash(*policy)
			if err != nil {
				return err
			}
			if expected != r.PolicyHash {
				return fmt.Errorf("run measurement or SLO policy differs from preregistration")
			}
		}
	} else {
		a.QualificationReasons = append(a.QualificationReasons, "measurement or explicit SLO policy evidence is unavailable")
	}
	if policy == nil {
		a.QualificationReasons = append(a.QualificationReasons, "measurement and SLO policy were not preregistered")
	}
	if run.Attempts == 0 || run.Failures != 0 || run.DroppedRecords != 0 || run.ExcludedBoundaryRequests != 0 || run.InflightAtEnd != 0 {
		return fmt.Errorf("work population incomplete")
	}
	for _, m := range run.Records {
		w := m.Work
		if w == nil || w.Status != "accepted" || w.Expectation.ExpectedModel == "" || len(w.Expectation.AllowedFinishReasons) == 0 || !w.Expectation.RequireExactUsage || w.Expectation.MinOutputTokens == nil || w.Expectation.MaxOutputTokens == nil {
			return fmt.Errorf("work acceptance lacks required model, finish, usage or length evidence")
		}
	}
	if run.ServingManifest == nil {
		return fmt.Errorf("run lacks effective manifest")
	}
	observed, err := llmperf.ReadServingManifest(bytes.NewReader(l.Observation))
	if err != nil {
		return err
	}
	if run.ServingManifest.Arm != observed.Arm {
		return fmt.Errorf("run arm identity differs from observed server")
	}
	origin, err := pairedEndpointOrigin(x.Lifecycle.Observe.URL)
	if err != nil {
		return err
	}
	for _, m := range run.Records {
		got, err := pairedEndpointOrigin(m.Endpoint)
		if err != nil || got != origin {
			return fmt.Errorf("request endpoint differs from supervised server")
		}
	}
	wh, err := llmperf.ServingWorkHash(*run.ServingManifest)
	if err != nil {
		return err
	}
	if wh != l.WorkHash {
		return fmt.Errorf("run and observed server work differ")
	}
	probe, err := serving.VerifyPinnedCapacityProbe(x.CapacityArtifact, l.WorkloadSHA256, x.CapacityProducerSHA256)
	if err != nil {
		return fmt.Errorf("capacity archive: %w", err)
	}
	_, h, err = fileBytes(filepath.Join(x.CapacityArtifact, "receipt.json"))
	if err != nil {
		return err
	}
	a.CapacitySHA256 = h
	r.WorkAccepted = true
	r.ClientQualified = false // Promoted only after measured resource and offered-load gates.
	a.CapacityStatus = "DIAGNOSTIC"
	if x.CapacityBounds != nil && x.WorkloadGeometry != "" {
		cr, err := serving.EvaluateVerifiedProbe(x.CapacityArtifact, l.WorkloadSHA256, x.WorkloadGeometry, *x.CapacityBounds)
		if err != nil {
			return err
		}
		a.CapacityStatus = cr.Status
		a.QualificationReasons = append(a.QualificationReasons, cr.Reasons...)
		if cr.Status == "UNQUALIFIED" {
			return fmt.Errorf("capacity exceeds preregistered bounds")
		}
		r.ClientQualified = cr.Status == "SAMPLED_CAPACITY"
	} else {
		a.QualificationReasons = append(a.QualificationReasons, "resource bounds or comparable workload geometry were not preregistered")
	}
	qualifiedOffer, reason, err := checkPairedOffer(probe.Rate, x.OfferedPerSecond, run, x.CapacityBounds)
	if err != nil {
		return err
	}
	if !qualifiedOffer {
		r.ClientQualified = false
		a.QualificationReasons = append(a.QualificationReasons, reason)
	}
	for _, m := range run.Records {
		if probe.MaxConnsPerHost <= 0 || m.MaxConnsPerHost != probe.MaxConnsPerHost || m.TransportProfile != probe.VerifiedTransportProfile || m.TransportProfile != "shared-http2-enabled-v1" || m.Stream || m.Protocol != "openai-chat" || m.RequestBodyBytes != probe.VerifiedRequestBodyBytes || m.ResponseBodyBytes != probe.VerifiedResponseBodyBytes {
			r.ClientQualified = false
			a.QualificationReasons = append(a.QualificationReasons, "real workload transport or byte geometry differs from measured capacity cell")
			break
		}
	}
	r.Successful = true
	r.Metrics = map[string]float64{}
	analysis, err := llmperf.AnalyzeRequests(run)
	if err != nil {
		return err
	}
	for _, name := range metrics {
		switch name {
		case "goodput":
			r.Metrics[name] = run.Goodput
		case "output_throughput":
			r.Metrics[name] = run.OutputThroughput
		case "input_throughput":
			r.Metrics[name] = run.InputThroughput
		default:
			const suffix = ".mean"
			if len(name) <= len(suffix) || name[len(name)-len(suffix):] != suffix {
				return fmt.Errorf("unknown aggregate metric %q", name)
			}
			m, ok := analysis.Metrics[name[:len(name)-len(suffix)]]
			if !ok || m.Mean == nil || m.Samples != run.Successes {
				return fmt.Errorf("metric %q lacks full accepted process population", name)
			}
			r.Metrics[name] = *m.Mean
		}
	}
	return nil

}
func runPaired(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("paired", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("config", "", "paired execution configuration")
	output := flags.String("output", "", "new receipt file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *input == "" || *output == "" || flags.NArg() != 0 {
		return 2
	}
	b, _, err := fileBytes(*input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var c pairedConfig
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(b))); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		fmt.Fprintln(stderr, "trailing paired configuration")
		return 1
	}
	f, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	prereg, err := os.OpenFile(*output+".plan.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		f.Close()
		fmt.Fprintln(stderr, err)
		return 1
	}
	original, _ := json.Marshal(c)
	originalHash := sha256.Sum256(original)
	err = json.NewEncoder(prereg).Encode(struct {
		ConfigSHA256 string       `json:"config_sha256"`
		Config       pairedConfig `json:"config"`
	}{hex.EncodeToString(originalHash[:]), sanitizedPairedConfig(c)})
	if err == nil {
		err = prereg.Sync()
	}
	closePlanErr := prereg.Close()
	if err == nil {
		err = closePlanErr
	}
	if err != nil {
		f.Close()
		fmt.Fprintln(stderr, err)
		return 1
	}
	c.archiveDirectory = *output + ".artifacts"
	if err := os.Mkdir(c.archiveDirectory, 0700); err != nil {
		f.Close()
		fmt.Fprintln(stderr, err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	receipt, runErr := executePaired(ctx, c)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(receipt)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if runErr != nil {
		fmt.Fprintln(stderr, runErr)
		return 1
	}
	fmt.Fprintln(stdout, *output)
	return 0
}

func sanitizedPairedConfig(c pairedConfig) pairedConfig {
	b, _ := json.Marshal(c)
	var sanitized pairedConfig
	_ = json.Unmarshal(b, &sanitized)
	hash := func(s string) string { h := sha256.Sum256([]byte(s)); return "sha256:" + hex.EncodeToString(h[:]) }
	command := func(cmd *serving.Command) {
		for i, v := range cmd.Env {
			key, _, ok := strings.Cut(v, "=")
			if ok {
				cmd.Env[i] = key + "=" + hash(v)
			} else {
				cmd.Env[i] = hash(v)
			}
		}
		for i, v := range cmd.Args {
			cmd.Args[i] = hash(v)
		}
	}
	probe := func(p *serving.Probe) {
		if p.Body != "" {
			p.Body = hash(p.Body)
		}
		u, err := url.Parse(p.URL)
		if err == nil {
			if u.User != nil {
				u.User = nil
			}
			u.RawQuery = ""
			u.ForceQuery = false
			u.Fragment = ""
			u.RawFragment = ""
			p.URL = u.String()
		}
	}
	for i := range sanitized.Runs {
		l := &sanitized.Runs[i].Lifecycle
		command(&l.Server)
		command(&l.Workload)
		if l.Lock != nil {
			command(l.Lock)
		}
		probe(&l.Ready)
		probe(&l.Observe)
		probe(&l.Drain)
		for j := range l.Warmup {
			probe(&l.Warmup[j])
		}
	}
	return sanitized
}

func snapshotPairedArtifacts(x pairedExecution, dir string) (pairedExecution, error) {
	copy := func(source, target string) error {
		b, _, err := fileBytes(source)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0400)
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		return err
	}
	if err := copy(x.RunArtifact, filepath.Join(dir, "run.json")); err != nil {
		return x, err
	}
	target := filepath.Join(dir, "capacity")
	if err := os.Mkdir(target, 0700); err != nil {
		return x, err
	}
	for _, name := range []string{"receipt.json", "probe.js", "producer.py", "resources.json", "wire.json", "rows.json", "summary.json"} {
		if err := copy(filepath.Join(x.CapacityArtifact, name), filepath.Join(target, name)); err != nil {
			return x, err
		}
	}
	x.RunArtifact = filepath.Join(dir, "run.json")
	x.CapacityArtifact = target
	return x, nil
}

func pairedEndpointOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return "", fmt.Errorf("invalid endpoint origin")
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return u.Scheme + "://" + strings.ToLower(u.Hostname()) + ":" + port, nil
}

// checkPairedOffer compares declared open load, not completed requests divided
// by elapsed time. An inclusive end arrival adds a request without raising the
// offered rate. Missing preregistration or schedule evidence stays diagnostic.
func checkPairedOffer(capacityRate int, offered float64, run llmperf.RunSummary, bounds *serving.CapacityBounds) (bool, string, error) {
	if math.IsNaN(offered) || math.IsInf(offered, 0) || offered < 0 {
		return false, "", fmt.Errorf("invalid offered rate")
	}
	if offered == 0 {
		return false, "offered load was not preregistered", nil
	}
	if run.Attempts <= 0 || run.Duration <= 0 || math.IsNaN(run.Duration) || math.IsInf(run.Duration, 0) {
		return false, "", fmt.Errorf("invalid offered schedule population")
	}
	if float64(capacityRate) < offered {
		return false, "", fmt.Errorf("capacity offer below preregistered real offer")
	}
	if math.Abs(float64(run.Attempts)-run.Duration*offered) > 1 {
		return false, "", fmt.Errorf("measured population differs from offered schedule")
	}
	schedule := make([]float64, 0, len(run.Records))
	for _, m := range run.Records {
		if m.ScheduledAt == nil || m.ArrivalLag == nil {
			return false, "retained offered-schedule observations are incomplete", nil
		}
		if math.IsNaN(*m.ScheduledAt) || math.IsInf(*m.ScheduledAt, 0) || math.IsNaN(*m.ArrivalLag) || math.IsInf(*m.ArrivalLag, 0) || *m.ArrivalLag < 0 {
			return false, "", fmt.Errorf("invalid offered schedule observation")
		}
		if bounds != nil && *m.ArrivalLag > bounds.MaxLagMilliseconds {
			return false, "", fmt.Errorf("real dispatch lag exceeds preregistered bound")
		}
		schedule = append(schedule, *m.ScheduledAt)
	}
	if len(schedule) != run.Attempts {
		return false, "retained offered-schedule observations are incomplete", nil
	}
	sort.Float64s(schedule)
	for i, v := range schedule {
		if math.Abs(v-(schedule[0]+float64(i)*1000/offered)) > 2 {
			return false, "", fmt.Errorf("retained offered schedule differs from preregistration")
		}
	}
	return true, "", nil
}
