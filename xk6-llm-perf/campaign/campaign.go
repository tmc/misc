package campaign

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Dimension binds finite values to a k6 environment variable.
type Dimension struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// Metric selects a k6 summary metric statistic.
type Metric struct {
	Name string `json:"name"`
	Stat string `json:"stat"`
}

// Constraint requires a metric to meet a bound. Direction is "le" or "ge".
type Constraint struct {
	Metric    Metric  `json:"metric"`
	Direction string  `json:"direction"`
	Bound     float64 `json:"bound"`
}

// Config describes a finite experiment. Environment is inherited by the child
// but never recorded; inject credentials there, not in parameter dimensions.
type Config struct {
	Binary           string       `json:"binary"`
	Script           string       `json:"script"`
	Assets           []string     `json:"assets,omitempty"`
	Dimensions       []Dimension  `json:"dimensions"`
	Mode             string       `json:"mode"`
	Seed             uint64       `json:"seed"`
	Objective        Metric       `json:"objective"`
	Minimize         bool         `json:"minimize"`
	Constraints      []Constraint `json:"constraints"`
	MaxCandidates    int          `json:"max_candidates"`
	MaxAttempts      int          `json:"max_attempts"`
	MinRuns          int          `json:"min_runs"`
	MaxRuns          int          `json:"max_runs"`
	Retries          int          `json:"retries"`
	DistributionFile string       `json:"distribution_file,omitempty"`
	Convergence      string       `json:"convergence"`
	Tolerance        float64      `json:"tolerance"`
	Timeout          string       `json:"timeout"`
	MaxDuration      string       `json:"max_duration"`
}

// Attempt retains failed executions and missing or invalid summary metrics.
type Attempt struct {
	Number            int       `json:"number"`
	StartedAt         time.Time `json:"started_at"`
	Duration          float64   `json:"duration_seconds"`
	ExitCode          int       `json:"exit_code"`
	Error             string    `json:"error,omitempty"`
	Directory         string    `json:"directory"`
	SummaryHash       string    `json:"summary_sha256,omitempty"`
	Value             *float64  `json:"value,omitempty"`
	Passed            bool      `json:"passed"`
	DistributionHash  string    `json:"distribution_sha256,omitempty"`
	DistributionCount int       `json:"distribution_count,omitempty"`
	LogTruncated      bool      `json:"log_truncated"`
}

// Candidate identifies one parameter cell and all executions of that cell.
type Candidate struct {
	ID         string            `json:"id"`
	Parameters map[string]string `json:"parameters"`
	Attempts   []Attempt         `json:"attempts"`
	Converged  bool              `json:"converged"`
	Failed     bool              `json:"failed"`
	Mean       *float64          `json:"mean,omitempty"`
	Feasible   bool              `json:"feasible"`
}

// Manifest is a checkpoint and the final comparison receipt. Failed cells are
// retained; the best cell is selected only from feasible, converged candidates.
type Manifest struct {
	SchemaVersion  int               `json:"schema_version"`
	Identity       string            `json:"identity"`
	Config         Config            `json:"config"`
	ScriptHash     string            `json:"script_sha256"`
	BinaryHash     string            `json:"binary_sha256"`
	AssetHashes    map[string]string `json:"asset_sha256,omitempty"`
	StartedAt      time.Time         `json:"started_at"`
	ElapsedSeconds float64           `json:"elapsed_seconds"`
	Candidates     []Candidate       `json:"candidates"`
	BestID         string            `json:"best_id,omitempty"`
	StopReason     string            `json:"stop_reason"`
}

var envName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// Validate checks bounds before any process is started.
func (c Config) Validate() error {
	if c.Binary == "" || c.Script == "" {
		return fmt.Errorf("binary and script are required")
	}
	switch c.Mode {
	case "grid", "halton", "adaptive", "scale", "bayesian":
	default:
		return fmt.Errorf("unknown mode %q", c.Mode)
	}
	if c.MaxCandidates < 1 || c.MaxCandidates > 10000 || c.MaxAttempts < 1 || c.MaxAttempts > 10000 {
		return fmt.Errorf("candidate or attempt budget out of range")
	}
	if c.MinRuns < 2 || c.MaxRuns < c.MinRuns || c.MaxRuns > 1000 || c.Retries < 0 || c.Retries > 100 {
		return fmt.Errorf("invalid run or retry bounds")
	}
	if c.Convergence != "cv" && c.Convergence != "ci95" && c.Convergence != "range" && c.Convergence != "distribution" {
		return fmt.Errorf("unknown convergence method")
	}
	if c.Convergence == "distribution" {
		if c.MaxRuns > 32 {
			return fmt.Errorf("distribution convergence allows at most 32 runs")
		}
		if c.DistributionFile == "" || filepath.IsAbs(c.DistributionFile) || filepath.Clean(c.DistributionFile) != c.DistributionFile || c.DistributionFile == "." || strings.HasPrefix(c.DistributionFile, "..") {
			return fmt.Errorf("distribution_file must be a clean relative artifact path")
		}
	}
	if !finite(c.Tolerance) || c.Tolerance < 0 || c.Tolerance > 1 {
		return fmt.Errorf("tolerance must be between zero and one")
	}
	for _, s := range []string{c.Timeout, c.MaxDuration} {
		d, e := time.ParseDuration(s)
		if e != nil || d <= 0 || d > 7*24*time.Hour {
			return fmt.Errorf("duration must be positive and at most 7d")
		}
	}
	if c.Objective.Name == "" || c.Objective.Stat == "" {
		return fmt.Errorf("objective metric and statistic are required")
	}
	for _, v := range c.Constraints {
		if v.Metric.Name == "" || v.Metric.Stat == "" || !finite(v.Bound) || (v.Direction != "le" && v.Direction != "ge") {
			return fmt.Errorf("invalid constraint")
		}
	}
	if len(c.Dimensions) > 12 {
		return fmt.Errorf("at most 12 dimensions are allowed")
	}
	n := 1
	cellBytes := 0
	if len(c.Assets) > 128 {
		return fmt.Errorf("at most 128 declared assets are allowed")
	}
	seen := map[string]bool{}
	for _, d := range c.Dimensions {
		if !envName.MatchString(d.Name) || seen[d.Name] || d.Name == "PATH" || d.Name == "HOME" || d.Name == "TMPDIR" {
			return fmt.Errorf("invalid or duplicate dimension %q", d.Name)
		}
		seen[d.Name] = true
		if len(d.Values) == 0 || len(d.Values) > 10000 {
			return fmt.Errorf("invalid dimension size")
		}
		maximum := 0
		values := map[string]bool{}
		for _, v := range d.Values {
			if len(v) > 4096 || values[v] {
				return fmt.Errorf("duplicate or oversized dimension value")
			}
			values[v] = true
			if len(v) > maximum {
				maximum = len(v)
			}
		}
		cellBytes += maximum + len(d.Name)
		if cellBytes > 2048 {
			return fmt.Errorf("parameter cell exceeds 2048 bytes")
		}
		if n > 10000/len(d.Values) {
			return fmt.Errorf("grid exceeds 10000 cells")
		}
		n *= len(d.Values)
	}
	if c.Mode == "scale" {
		if len(c.Dimensions) != 1 || len(c.Constraints) == 0 {
			return fmt.Errorf("scale requires one dimension and SLA constraints")
		}
		last := math.Inf(-1)
		for _, s := range c.Dimensions[0].Values {
			v, e := strconv.ParseFloat(s, 64)
			if e != nil || !finite(v) || v <= last || v <= 0 {
				return fmt.Errorf("scale values must be positive and strictly increasing")
			}
			last = v
		}
	}
	return nil
}
func finite(v float64) bool  { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func fileHash(path string) (string, error) {
	f, e := os.Open(path) // #nosec G304 -- CLI/API callers explicitly choose local assets and receipt files; network sample input cannot select paths.
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func cellID(p map[string]string) string { b, _ := json.Marshal(p); return digest(b)[:24] }

// Converged evaluates run-level stability. CV uses sample standard deviation;
// ci95 uses a normal 1.96 interval, not an exact small-sample t interval. Range
// requires the relative full observed range to fit the tolerance. Zero means
// require identical zero values. Nonfinite values never converge.
func Converged(values []float64, method string, tolerance float64, minimum int) bool {
	if minimum < 2 || len(values) < minimum || !finite(tolerance) || tolerance < 0 {
		return false
	}
	mean := 0.0
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, v := range values {
		if !finite(v) {
			return false
		}
		mean += v / float64(len(values))
		lo = math.Min(lo, v)
		hi = math.Max(hi, v)
	}
	if mean == 0 {
		return lo == 0 && hi == 0
	}
	sum := 0.0
	for _, v := range values {
		sum += (v - mean) * (v - mean)
	}
	sd := math.Sqrt(sum / float64(len(values)-1))
	score := 0.0
	switch method {
	case "cv":
		score = sd / math.Abs(mean)
	case "ci95":
		score = 2 * 1.96 * sd / math.Sqrt(float64(len(values))) / math.Abs(mean)
	case "range":
		score = (hi - lo) / math.Abs(mean)
	default:
		return false
	}
	return score <= tolerance
}

// Run starts or resumes a campaign in directory. A file lock refuses concurrent
// writers. Config, script, and binary hashes must match on resume. Every attempt
// is checkpointed before launch; interrupted attempts remain failed receipts.
func Run(ctx context.Context, c Config, directory string) (*Manifest, error) {
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
	sh, e := fileHash(script)
	if e != nil {
		return nil, e
	}
	bh, e := fileHash(binary)
	if e != nil {
		return nil, e
	}
	assets := map[string]string{}
	for _, asset := range c.Assets {
		hash, err := fileHash(asset)
		if err != nil {
			return nil, fmt.Errorf("hash asset: %w", err)
		}
		if _, ok := assets[asset]; ok {
			return nil, fmt.Errorf("duplicate asset")
		}
		assets[asset] = hash
	}
	identityBytes, _ := json.Marshal(struct {
		Config                 Config
		ScriptHash, BinaryHash string
		Assets                 map[string]string
	}{c, sh, bh, assets})
	identity := digest(identityBytes)
	directory, e = filepath.Abs(directory)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(directory, 0700); e != nil {
		return nil, e
	}
	lock, e := os.OpenFile(filepath.Join(directory, ".lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) // #nosec G304 -- caller-selected private output directory; exclusive fixed-name lock, no network-derived path.
	if e != nil {
		return nil, fmt.Errorf("lock campaign: %w", e)
	}
	defer os.Remove(filepath.Join(directory, ".lock"))
	if e = lock.Close(); e != nil {
		return nil, fmt.Errorf("close campaign lock: %w", e)
	}
	checkpoint := filepath.Join(directory, "campaign.json")
	m := &Manifest{SchemaVersion: 1, Identity: identity, Config: c, ScriptHash: sh, BinaryHash: bh, AssetHashes: assets, StartedAt: time.Now().UTC()}
	if b, e := readLimited(checkpoint, 64<<20); e == nil {
		if e = json.Unmarshal(b, m); e != nil {
			return nil, e
		}
		if m.SchemaVersion != 1 || m.Identity != identity || m.ScriptHash != sh || m.BinaryHash != bh {
			return nil, fmt.Errorf("checkpoint configuration or assets differ")
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if e = validateCheckpoint(m, c); e != nil {
		return nil, e
	}
	savedAssets, _ := json.Marshal(m.AssetHashes)
	currentAssets, _ := json.Marshal(assets)
	if string(savedAssets) != string(currentAssets) {
		return nil, fmt.Errorf("checkpoint assets differ")
	}
	if e = verifyAttempts(m, directory); e != nil {
		return nil, e
	}
	for i := range m.Candidates {
		update(&m.Candidates[i], c)
		if e = updateDistribution(&m.Candidates[i], c, directory); e != nil {
			return nil, e
		}
	}
	m.StopReason = ""
	timeout, _ := time.ParseDuration(c.Timeout)
	budget, _ := time.ParseDuration(c.MaxDuration)
	start := time.Now()
	previousElapsed := m.ElapsedSeconds
	save := func() error {
		m.ElapsedSeconds = previousElapsed + time.Since(start).Seconds()
		selectBest(m)
		return writeJSON(checkpoint, m)
	}
	defer func() { _ = save() }()
	cells := grid(c.Dimensions)
	attempts := 0
	for _, cell := range m.Candidates {
		attempts += len(cell.Attempts)
	}
	for {
		if ctx.Err() != nil {
			m.StopReason = "cancelled"
			return m, ctx.Err()
		}
		if time.Duration((previousElapsed+time.Since(start).Seconds())*float64(time.Second)) >= budget {
			m.StopReason = "duration_budget"
			break
		}
		if attempts >= c.MaxAttempts {
			m.StopReason = "attempt_budget"
			break
		}
		ci := -1
		for i := range m.Candidates {
			if !m.Candidates[i].Converged && !m.Candidates[i].Failed && len(m.Candidates[i].Attempts) < c.MaxRuns+c.Retries {
				ci = i
				break
			}
		}
		if ci < 0 {
			if len(m.Candidates) >= c.MaxCandidates {
				m.StopReason = "candidate_budget"
				break
			}
			p := next(c, cells, m)
			if p == nil {
				m.StopReason = "space_exhausted"
				if c.Mode == "scale" {
					m.StopReason = "scale_boundary"
					for _, cell := range m.Candidates {
						if cell.Failed || !cell.Converged {
							m.StopReason = "scale_assessment_failed"
							break
						}
					}
				}
				break
			}
			m.Candidates = append(m.Candidates, Candidate{ID: cellID(p), Parameters: p})
			ci = len(m.Candidates) - 1
		}
		if e = verifyInputs(binary, script, bh, sh, assets); e != nil {
			m.StopReason = "assets_changed"
			return m, e
		}
		cell := &m.Candidates[ci]
		a := Attempt{Number: len(cell.Attempts) + 1, StartedAt: time.Now().UTC(), ExitCode: -1, Error: "interrupted before completion"}
		a.Directory = filepath.Join(cell.ID, fmt.Sprintf("attempt-%04d", a.Number))
		cell.Attempts = append(cell.Attempts, a)
		attempts++
		if e = save(); e != nil {
			return m, e
		}
		attemptDir := filepath.Join(directory, a.Directory)
		remaining := budget - time.Duration(m.ElapsedSeconds*float64(time.Second))
		if remaining < timeout {
			timeout = remaining
		}
		a = execute(ctx, binary, script, attemptDir, cell.Parameters, c, timeout, a)
		if err := verifyInputs(binary, script, bh, sh, assets); err != nil {
			a.Error = err.Error()
			a.Passed = false
			m.StopReason = "assets_changed"
		}
		cell.Attempts[len(cell.Attempts)-1] = a
		update(cell, c)
		if e = updateDistribution(cell, c, directory); e != nil {
			return m, e
		}
		if e = writeJSON(filepath.Join(attemptDir, "attempt.json"), struct {
			Identity, ID string
			Parameters   map[string]string
			Attempt      Attempt
		}{identity, cell.ID, cell.Parameters, a}); e != nil {
			return m, e
		}
		if e = save(); e != nil {
			return m, e
		}
		if m.StopReason == "assets_changed" {
			return m, fmt.Errorf("campaign inputs changed during attempt")
		}
	}
	if e = save(); e != nil {
		return m, e
	}
	return m, nil
}
func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".checkpoint-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(append(b, '\n'))
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, path)
}
func execute(ctx context.Context, binary, script, dir string, p map[string]string, c Config, timeout time.Duration, a Attempt) Attempt {
	start := time.Now()
	if e := os.MkdirAll(dir, 0700); e != nil {
		a.Error = e.Error()
		return a
	}
	log, e := os.OpenFile(filepath.Join(dir, "process.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) // #nosec G304 -- fixed-name log under caller-selected private campaign directory and validated hashed candidate/attempt identity.
	if e != nil {
		a.Error = e.Error()
		return a
	}
	defer log.Close()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	summary := filepath.Join(dir, "summary.json")
	args := []string{"run", "--summary-export", summary}
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--env", k+"="+p[k])
	}
	args = append(args, script)
	cmd := exec.CommandContext(runCtx, binary, args...) // #nosec G204 -- explicitly caller-selected, hash-checked local binary; fixed k6 argv structure with --env values, never a shell.
	cmd.Dir = dir
	output := &boundedLog{writer: log, remaining: 16 << 20}
	cmd.Stdout = output
	cmd.Stderr = output
	e = cmd.Run()
	a.Duration = time.Since(start).Seconds()
	a.LogTruncated = output.truncated
	a.ExitCode = 0
	a.Error = ""
	if e != nil {
		a.ExitCode = -1
		if ee, ok := e.(*exec.ExitError); ok {
			a.ExitCode = ee.ExitCode()
		}
		a.Error = e.Error()
	}
	b, e := readLimited(summary, 16<<20)
	if e != nil {
		a.Error = "read summary: " + e.Error()
		return a
	}
	a.SummaryHash = digest(b)
	var s struct {
		Metrics map[string]json.RawMessage `json:"metrics"`
	}
	if e = json.Unmarshal(b, &s); e != nil {
		a.Error = "decode summary: " + e.Error()
		return a
	}
	metric := func(m Metric) (float64, bool) {
		var object map[string]json.RawMessage
		if json.Unmarshal(s.Metrics[m.Name], &object) != nil {
			return 0, false
		}
		if nested, ok := object["values"]; ok {
			if json.Unmarshal(nested, &object) != nil {
				return 0, false
			}
		}
		raw, ok := object[m.Stat]
		if !ok || string(raw) == "null" {
			return 0, false
		}
		var v float64
		if json.Unmarshal(raw, &v) != nil {
			return 0, false
		}
		return v, finite(v)
	}
	v, ok := metric(c.Objective)
	if !ok {
		a.Error = "objective metric unavailable"
		return a
	}
	a.Value = &v
	if c.Convergence == "distribution" {
		values, hash, err := distribution(filepath.Join(dir, c.DistributionFile))
		if err != nil {
			a.Error = "read distribution: " + err.Error()
			return a
		}
		a.DistributionHash = hash
		a.DistributionCount = len(values)
	}
	a.Passed = true
	for _, constraint := range c.Constraints {
		v, ok := metric(constraint.Metric)
		if !ok {
			a.Passed = false
			a.Error = "constraint metric unavailable"
			break
		}
		if constraint.Direction == "le" && v > constraint.Bound || constraint.Direction == "ge" && v < constraint.Bound {
			a.Passed = false
		}
	}
	return a
}
func readLimited(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path) // #nosec G304 -- CLI/API callers explicitly choose local assets and receipt files; network sample input cannot select paths.
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e == nil && int64(len(b)) > limit {
		e = fmt.Errorf("file exceeds %d bytes", limit)
	}
	return b, e
}
func update(cell *Candidate, c Config) {
	cell.Mean = nil
	values := []float64{}
	failed := 0
	cell.Feasible = true
	for _, a := range cell.Attempts {
		if a.Error != "" || a.ExitCode != 0 || a.Value == nil {
			failed++
			cell.Feasible = false
			continue
		}
		values = append(values, *a.Value)
		if !a.Passed {
			cell.Feasible = false
		}
	}
	if len(values) > 0 {
		mean := 0.0
		for _, v := range values {
			mean += v / float64(len(values))
		}
		cell.Mean = &mean
	}
	cell.Converged = Converged(values, c.Convergence, c.Tolerance, c.MinRuns)
	cell.Failed = failed > c.Retries || len(values) >= c.MaxRuns && !cell.Converged || len(cell.Attempts) >= c.MaxRuns+c.Retries && !cell.Converged
}
func selectBest(m *Manifest) {
	m.BestID = ""
	best := math.Inf(-1)
	if m.Config.Minimize {
		best = math.Inf(1)
	}
	for _, c := range m.Candidates {
		if !c.Converged || !c.Feasible || c.Failed || c.Mean == nil {
			continue
		}
		if m.BestID == "" || !m.Config.Minimize && *c.Mean > best || m.Config.Minimize && *c.Mean < best {
			m.BestID = c.ID
			best = *c.Mean
		}
	}
}

// boundedLog keeps draining stdout after the retention limit. Both streams use
// the same writer, which os/exec serializes for identical comparable writers.
type boundedLog struct {
	writer    io.Writer
	remaining int
	truncated bool
}

func (w *boundedLog) Write(p []byte) (int, error) {
	n := len(p)
	keep := n
	if keep > w.remaining {
		keep = w.remaining
		w.truncated = true
	}
	if keep > 0 {
		written, e := w.writer.Write(p[:keep])
		w.remaining -= written
		if e != nil {
			return written, e
		}
	}
	return n, nil
}
func validateCheckpoint(m *Manifest, c Config) error {
	if len(m.Candidates) > c.MaxCandidates || !finite(m.ElapsedSeconds) || m.ElapsedSeconds < 0 {
		return fmt.Errorf("invalid checkpoint bounds")
	}
	configBytes, _ := json.Marshal(c)
	savedBytes, _ := json.Marshal(m.Config)
	if string(configBytes) != string(savedBytes) {
		return fmt.Errorf("checkpoint config differs")
	}
	allowed := map[string]bool{}
	for _, p := range grid(c.Dimensions) {
		allowed[cellID(p)] = true
	}
	seen := map[string]bool{}
	total := 0
	for _, cell := range m.Candidates {
		if cell.ID != cellID(cell.Parameters) || !allowed[cell.ID] || seen[cell.ID] {
			return fmt.Errorf("invalid checkpoint candidate")
		}
		seen[cell.ID] = true
		if len(cell.Attempts) > c.MaxRuns+c.Retries {
			return fmt.Errorf("invalid checkpoint run count")
		}
		for i, a := range cell.Attempts {
			if a.Number != i+1 || a.Directory != filepath.Join(cell.ID, fmt.Sprintf("attempt-%04d", i+1)) || !finite(a.Duration) || a.Duration < 0 || a.Value != nil && !finite(*a.Value) {
				return fmt.Errorf("invalid checkpoint attempt")
			}
		}
		total += len(cell.Attempts)
	}
	if total > c.MaxAttempts {
		return fmt.Errorf("checkpoint exceeds attempt budget")
	}
	return nil
}
