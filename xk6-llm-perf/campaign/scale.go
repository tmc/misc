package campaign

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// ScaleConfig describes a bounded, single-run multi-worker admission controller.
// Each worker is a local k6 subprocess with its own root-shared control group.
// Request and prefill quotas are partitioned so their sums never exceed the
// global caps. Zero-quota workers remain idle and send participation heartbeats.
type ScaleConfig struct {
	Binary          string   `json:"binary"`
	Script          string   `json:"script"`
	Assets          []string `json:"assets,omitempty"`
	Variable        string   `json:"variable"`
	Minimum         int      `json:"minimum"`
	Maximum         int      `json:"maximum"`
	FixedRequest    int      `json:"fixed_request"`
	FixedPrefill    int      `json:"fixed_prefill"`
	Workers         int      `json:"workers"`
	Windows         int      `json:"windows"`
	SustainWindows  int      `json:"sustain_windows"`
	Period          string   `json:"period"`
	StartupTimeout  string   `json:"startup_timeout"`
	DrainTimeout    string   `json:"drain_timeout"`
	MinCompleted    int      `json:"min_completed"`
	MaxLatency      float64  `json:"max_latency_ms"`
	MaxFailureRatio float64  `json:"max_failure_ratio"`
	MaxRecords      int      `json:"max_records"`
}

// ScaleSample is a sanitized whole-window request receipt. Identity must be
// unique across workers, VUs and iterations. Timestamps are Unix milliseconds.
type ScaleSample struct {
	ID           string  `json:"id"`
	Worker       int     `json:"worker"`
	Epoch        int     `json:"epoch"`
	DispatchedAt float64 `json:"dispatched_at"`
	FinishedAt   float64 `json:"finished_at"`
	Latency      float64 `json:"latency_ms"`
	Outcome      string  `json:"outcome"`
	Requests     int     `json:"requests"`
	Prefills     int     `json:"prefills"`
}

// ScaleWindow retains every assessment, including failures and incomplete data.
type ScaleWindow struct {
	Epoch                int           `json:"epoch"`
	StartedAt            int64         `json:"started_at"`
	FinishedAt           int64         `json:"finished_at"`
	RequestLimit         int           `json:"request_limit"`
	PrefillLimit         int           `json:"prefill_limit"`
	Sustaining           bool          `json:"sustaining"`
	ParticipatingWorkers []int         `json:"participating_workers"`
	Completed            int           `json:"completed"`
	Failures             int           `json:"failures"`
	Excluded             int           `json:"excluded"`
	Dropped              int           `json:"dropped"`
	Duplicates           int           `json:"duplicates"`
	P95                  *float64      `json:"p95_ms"`
	Passed               bool          `json:"passed"`
	Reason               string        `json:"reason"`
	Samples              []ScaleSample `json:"samples"`
}

// ScaleWorker describes a native worker process and its retained output.
type ScaleWorker struct {
	ID           int    `json:"id"`
	Directory    string `json:"directory"`
	VUs          int    `json:"vus"`
	ExitCode     int    `json:"exit_code"`
	Error        string `json:"error,omitempty"`
	LogTruncated bool   `json:"log_truncated"`
	SummaryHash  string `json:"summary_sha256,omitempty"`
}

// ScaleManifest declares clock mapping, controls and the complete assessment
// sequence. Workers run on the controller host and share its Unix epoch basis;
// arbitrary remote clocks are not silently assumed synchronized.
type ScaleManifest struct {
	SchemaVersion   int               `json:"schema_version"`
	Identity        string            `json:"identity"`
	Config          ScaleConfig       `json:"config"`
	BinaryHash      string            `json:"binary_sha256"`
	ScriptHash      string            `json:"script_sha256"`
	AssetHashes     map[string]string `json:"asset_sha256"`
	ClockPolicy     string            `json:"clock_policy"`
	PartitionPolicy string            `json:"partition_policy"`
	StartedAt       time.Time         `json:"started_at"`
	FinishedAt      time.Time         `json:"finished_at"`
	Windows         []ScaleWindow     `json:"windows"`
	Workers         []ScaleWorker     `json:"workers"`
	StopReason      string            `json:"stop_reason"`
}

func (c ScaleConfig) Validate() error {
	if c.Binary == "" || c.Script == "" {
		return fmt.Errorf("binary and script are required")
	}
	if c.Variable != "request" && c.Variable != "prefill" {
		return fmt.Errorf("scale variable must be request or prefill")
	}
	if c.Minimum < 1 || c.Maximum < c.Minimum || c.Maximum > 10000 || c.FixedRequest < 1 || c.FixedRequest > 10000 || c.FixedPrefill < 1 || c.FixedPrefill > 10000 {
		return fmt.Errorf("invalid global scale caps")
	}
	if c.Workers < 1 || c.Workers > 32 || c.Windows < 2 || c.Windows > 1000 || c.SustainWindows < 1 || c.SustainWindows >= c.Windows {
		return fmt.Errorf("invalid worker or window bounds")
	}
	if c.MinCompleted < 1 || c.MinCompleted > 100000 || c.MaxRecords < c.MinCompleted || c.MaxRecords > 100000 || !finite(c.MaxLatency) || c.MaxLatency <= 0 || !finite(c.MaxFailureRatio) || c.MaxFailureRatio < 0 || c.MaxFailureRatio > 1 {
		return fmt.Errorf("invalid assessment bounds")
	}
	period, e := time.ParseDuration(c.Period)
	if e != nil || period < 100*time.Millisecond || period > time.Hour {
		return fmt.Errorf("period must be between 100ms and 1h")
	}
	for _, s := range []string{c.StartupTimeout, c.DrainTimeout} {
		d, e := time.ParseDuration(s)
		if e != nil || d < time.Millisecond || d > time.Minute {
			return fmt.Errorf("startup and drain timeouts must be between 1ms and 1m")
		}
	}
	if period*time.Duration(c.Windows) > 24*time.Hour {
		return fmt.Errorf("assessment budget exceeds 24h")
	}
	if len(c.Assets) > 128 {
		return fmt.Errorf("too many assets")
	}
	return nil
}

type scaleControl struct {
	mu              sync.Mutex
	config          ScaleConfig
	window          *ScaleWindow
	active, stop    bool
	ready           []bool
	participating   []bool
	retained        int
	seen            map[string]bool
	protocolFailure string
}
type scaleAssignment struct {
	Epoch   int   `json:"epoch"`
	Active  bool  `json:"active"`
	Stop    bool  `json:"stop"`
	Start   int64 `json:"start"`
	End     int64 `json:"end"`
	Request int   `json:"request"`
	Prefill int   `json:"prefill"`
}

func quota(total, workers, id int) int {
	v := total / workers
	if id < total%workers {
		v++
	}
	return v
}
func (s *scaleControl) serveHTTP(w http.ResponseWriter, r *http.Request) {
	worker, e := strconv.Atoi(r.URL.Query().Get("worker"))
	if e != nil || worker < 0 || worker >= s.config.Workers {
		http.Error(w, "invalid worker", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/control":
		s.ready[worker] = true
		a := scaleAssignment{Stop: s.stop}
		if s.window != nil {
			a.Epoch = s.window.Epoch
			a.Active = s.active
			a.Start = s.window.StartedAt
			a.End = s.window.FinishedAt
			a.Request = quota(s.window.RequestLimit, s.config.Workers, worker)
			a.Prefill = quota(s.window.PrefillLimit, s.config.Workers, worker)
			if s.active {
				s.participating[worker] = true
			}
		}
		_ = json.NewEncoder(w).Encode(a)
	case r.Method == http.MethodPost && r.URL.Path == "/sample":
		b, e := io.ReadAll(io.LimitReader(r.Body, 2049))
		if e != nil || len(b) > 2048 {
			http.Error(w, "sample exceeds limit", 400)
			return
		}
		var sample ScaleSample
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if e = dec.Decode(&sample); e != nil {
			http.Error(w, "invalid sample", 400)
			return
		}
		var extra any
		if e = dec.Decode(&extra); e != io.EOF {
			http.Error(w, "sample has trailing data", 400)
			return
		}
		if sample.Requests < 0 || sample.Requests > 100000 || sample.Prefills < 0 || sample.Prefills > 100000 || sample.Worker != worker || sample.ID == "" || len(sample.ID) > 128 || !finite(sample.DispatchedAt) || !finite(sample.FinishedAt) || sample.FinishedAt < sample.DispatchedAt || !finite(sample.Latency) || sample.Latency < 0 || (sample.Outcome != "success" && sample.Outcome != "failure" && sample.Outcome != "cancelled") {
			http.Error(w, "invalid sample", 400)
			return
		}
		if s.window == nil || sample.Epoch != s.window.Epoch {
			http.Error(w, "stale epoch", 409)
			return
		}
		window := s.window
		if s.seen[sample.ID] {
			window.Duplicates++
			s.protocolFailure = "duplicate request identity"
			http.Error(w, "duplicate identity", 409)
			return
		}
		if s.retained >= s.config.MaxRecords {
			window.Dropped++
			http.Error(w, "retention exhausted", 429)
			return
		}
		s.seen[sample.ID] = true
		s.retained++
		if sample.DispatchedAt < float64(window.StartedAt) || sample.FinishedAt > float64(window.FinishedAt) {
			window.Excluded++
			w.WriteHeader(202)
			return
		}
		window.Samples = append(window.Samples, sample)
		w.WriteHeader(202)
	default:
		http.Error(w, "unsupported route", 404)
	}
}

func assess(w *ScaleWindow, participating []bool, c ScaleConfig, protocolFailure string) {
	values := []float64{}
	for _, r := range w.Samples {
		if r.Outcome == "success" {
			w.Completed++
			values = append(values, r.Latency)
		} else {
			w.Failures++
		}
	}
	for i, v := range participating {
		if v {
			w.ParticipatingWorkers = append(w.ParticipatingWorkers, i)
		}
	}
	if len(values) > 0 {
		sort.Float64s(values)
		index := int(math.Ceil(.95*float64(len(values)))) - 1
		p := values[index]
		w.P95 = &p
	}
	w.Reason = "passed"
	switch {
	case protocolFailure != "":
		w.Reason = protocolFailure
	case len(w.ParticipatingWorkers) != c.Workers:
		w.Reason = "missing_worker"
	case w.Dropped > 0:
		w.Reason = "incomplete_records"
	case w.Completed < c.MinCompleted:
		w.Reason = "insufficient_completions"
	case float64(w.Failures)/float64(w.Completed+w.Failures) > c.MaxFailureRatio:
		w.Reason = "failure_ratio"
	case w.P95 == nil || *w.P95 > c.MaxLatency:
		w.Reason = "latency"
	}
	w.Passed = w.Reason == "passed"
}

// RunScale executes one bounded adaptive run across local native k6 workers.
// It starts at Minimum, doubles the selected admission cap while assessments
// pass, then sustains the last passing cap. Failed assessments are never erased.
// The output directory must be new; finished scale runs replay their immutable
// manifest rather than restart partial workers under an existing identity.
func RunScale(ctx context.Context, c ScaleConfig, directory string) (*ScaleManifest, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	binary, e := exec.LookPath(c.Binary)
	if e != nil {
		return nil, e
	}
	binary, e = filepath.Abs(binary)
	if e != nil {
		return nil, e
	}
	script, e := filepath.Abs(c.Script)
	if e != nil {
		return nil, e
	}
	bh, e := fileHash(binary)
	if e != nil {
		return nil, e
	}
	sh, e := fileHash(script)
	if e != nil {
		return nil, e
	}
	assets := map[string]string{}
	for _, p := range c.Assets {
		hash, e := fileHash(p)
		if e != nil {
			return nil, e
		}
		assets[p] = hash
	}
	identityBytes, _ := json.Marshal(struct {
		Config         ScaleConfig
		Binary, Script string
		Assets         map[string]string
	}{c, bh, sh, assets})
	identity := digest(identityBytes)
	directory, e = filepath.Abs(directory)
	if e != nil {
		return nil, e
	}
	if e = os.Mkdir(directory, 0700); e != nil {
		return nil, fmt.Errorf("scale directory must be new: %w", e)
	}
	m := &ScaleManifest{SchemaVersion: 1, Identity: identity, Config: c, BinaryHash: bh, ScriptHash: sh, AssetHashes: assets, ClockPolicy: "same-host Unix epoch milliseconds; controller-defined whole-contained windows", PartitionPolicy: "balanced integer quotas by worker ID; zero-quota workers idle; sums bounded by global caps", StartedAt: time.Now().UTC()}
	save := func() error { return writeJSON(filepath.Join(directory, "scale.json"), m) }
	if e = save(); e != nil {
		return m, e
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return m, e
	}
	control := &scaleControl{config: c, ready: make([]bool, c.Workers), seen: map[string]bool{}}
	server := &http.Server{Handler: http.HandlerFunc(control.serveHTTP), ReadHeaderTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second, MaxHeaderBytes: 4096}
	go server.Serve(listener)
	defer server.Close()
	processCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct{ worker ScaleWorker }
	results := make(chan result, c.Workers)
	period, _ := time.ParseDuration(c.Period)
	startup, _ := time.ParseDuration(c.StartupTimeout)
	drain, _ := time.ParseDuration(c.DrainTimeout)
	maxVU := c.FixedRequest
	if c.Variable == "request" {
		maxVU = c.Maximum
	}
	maxVU = (maxVU+c.Workers-1)/c.Workers + 1
	if maxVU > 10000 {
		maxVU = 10000
	}
	if maxVU < 1 {
		maxVU = 1
	}
	for worker := 0; worker < c.Workers; worker++ {
		worker := worker
		dir := filepath.Join(directory, fmt.Sprintf("worker-%03d", worker))
		if e = os.Mkdir(dir, 0700); e != nil {
			return m, e
		}
		go func() {
			receipt := ScaleWorker{ID: worker, Directory: filepath.Base(dir), VUs: maxVU, ExitCode: -1}
			log, e := os.OpenFile(filepath.Join(dir, "process.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) // #nosec G304 -- fixed-name log in new private output directory with bounded generated worker ID; network samples cannot select paths.
			if e != nil {
				receipt.Error = e.Error()
				results <- result{receipt}
				return
			}
			defer log.Close()
			output := &boundedLog{writer: log, remaining: 16 << 20}
			duration := startup + time.Duration(c.Windows)*(period+drain) + drain + time.Second
			args := []string{"run", "--summary-export", filepath.Join(dir, "summary.json"), "--env", "SCALE_CONTROL_URL=http://" + listener.Addr().String(), "--env", "SCALE_WORKER_ID=" + strconv.Itoa(worker), "--env", "SCALE_WORKER_COUNT=" + strconv.Itoa(c.Workers), "--env", "SCALE_MAX_VUS=" + strconv.Itoa(maxVU), "--env", "SCALE_RUN_DURATION=" + duration.String(), script}
			cmd := exec.CommandContext(processCtx, binary, args...) // #nosec G204 -- explicitly caller-selected hashed local binary with fixed k6 argv, generated worker identity and declared script; never a shell.
			cmd.Dir = dir
			cmd.WaitDelay = time.Second
			cmd.Stdout = output
			cmd.Stderr = output
			e = cmd.Run()
			receipt.LogTruncated = output.truncated
			receipt.ExitCode = 0
			if e != nil {
				receipt.Error = e.Error()
				receipt.ExitCode = -1
				if ex, ok := e.(*exec.ExitError); ok {
					receipt.ExitCode = ex.ExitCode()
				}
			}
			receipt.SummaryHash, _ = fileHash(filepath.Join(dir, "summary.json"))
			results <- result{receipt}
		}()
	}
	exited := 0
	processFailed := false
	collect := func(block bool) {
		for exited < c.Workers {
			select {
			case r := <-results:
				m.Workers = append(m.Workers, r.worker)
				exited++
				if r.worker.ExitCode != 0 {
					processFailed = true
				}
				if !block {
					continue
				}
			case <-time.After(10 * time.Millisecond):
				if !block {
					return
				}
			}
		}
	}
	readyDeadline := time.Now().Add(startup)
	for {
		control.mu.Lock()
		ready := true
		for _, v := range control.ready {
			ready = ready && v
		}
		control.mu.Unlock()
		collect(false)
		if ready {
			break
		}
		if exited > 0 || ctx.Err() != nil || time.Now().After(readyDeadline) {
			m.StopReason = "startup_failed"
			if ctx.Err() != nil {
				m.StopReason = "cancelled"
			}
			cancel()
			collect(true)
			m.FinishedAt = time.Now().UTC()
			_ = save()
			return m, fmt.Errorf("workers failed startup")
		}
		time.Sleep(10 * time.Millisecond)
	}
	value, lastPassing := c.Minimum, 0
	sustaining := false
	sustainLeft := c.SustainWindows
	for epoch := 1; epoch <= c.Windows; epoch++ {
		if ctx.Err() != nil {
			m.StopReason = "cancelled"
			break
		}
		collect(false)
		if exited > 0 {
			m.StopReason = "worker_failed"
			break
		}
		request, prefill := c.FixedRequest, c.FixedPrefill
		if c.Variable == "request" {
			request = value
		} else {
			prefill = value
		}
		window := ScaleWindow{Epoch: epoch, StartedAt: time.Now().UnixMilli(), RequestLimit: request, PrefillLimit: prefill, Sustaining: sustaining}
		window.FinishedAt = window.StartedAt + period.Milliseconds()
		if e = verifyInputs(binary, script, bh, sh, assets); e != nil {
			m.StopReason = "assets_changed"
			break
		}
		m.Windows = append(m.Windows, window)
		m.Windows[len(m.Windows)-1].Reason = "assessment_in_progress"
		if e = save(); e != nil {
			m.StopReason = "checkpoint_failed"
			break
		}
		closeWindow := func(reason string) {
			control.mu.Lock()
			control.active = false
			assess(&window, control.participating, c, control.protocolFailure)
			if reason != "" {
				window.Passed = false
				window.Reason = reason
			}
			control.window = nil
			control.mu.Unlock()
			m.Windows[len(m.Windows)-1] = window
		}
		control.mu.Lock()
		control.window = &window
		control.participating = make([]bool, c.Workers)
		control.active = true
		control.mu.Unlock()
		if !pause(ctx, period) {
			m.StopReason = "cancelled"
			closeWindow("cancelled")
			break
		}
		control.mu.Lock()
		control.active = false
		control.mu.Unlock()
		if !pause(ctx, drain) {
			m.StopReason = "cancelled"
			closeWindow("cancelled")
			break
		}
		closeWindow("")
		collect(false)
		if exited > 0 {
			window.Passed = false
			window.Reason = "worker_failed"
		}
		m.Windows[len(m.Windows)-1] = window
		if e = save(); e != nil {
			m.StopReason = "checkpoint_failed"
			break
		}
		if !window.Passed && (window.Reason == "worker_failed" || window.Reason == "missing_worker" || window.Reason == "incomplete_records" || window.Reason == "duplicate request identity") {
			m.StopReason = "assessment_failed"
			break
		}
		if sustaining {
			if !window.Passed {
				m.StopReason = "sustain_failed"
				break
			}
			sustainLeft--
			if sustainLeft == 0 {
				m.StopReason = "sustain_complete"
				break
			}
			continue
		}
		if window.Passed {
			lastPassing = value
			if value == c.Maximum {
				sustaining = true
			} else {
				value *= 2
				if value > c.Maximum {
					value = c.Maximum
				}
			}
		} else {
			if lastPassing == 0 {
				m.StopReason = "no_passing_boundary"
				break
			}
			value = lastPassing
			sustaining = true
		}
	}
	if m.StopReason == "" {
		m.StopReason = "window_budget"
	}
	control.mu.Lock()
	control.active = false
	control.stop = true
	control.mu.Unlock()
	deadline := time.Now().Add(drain + time.Second)
	for exited < c.Workers && time.Now().Before(deadline) {
		collect(false)
		time.Sleep(10 * time.Millisecond)
	}
	if exited < c.Workers {
		cancel()
		collect(true)
	}
	if processFailed && m.StopReason == "sustain_complete" {
		m.StopReason = "worker_failed"
	}
	sort.Slice(m.Workers, func(i, j int) bool { return m.Workers[i].ID < m.Workers[j].ID })
	m.FinishedAt = time.Now().UTC()
	if err := save(); e == nil {
		e = err
	}
	return m, e
}
func pause(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
