// Package serving supervises isolated HTTP serving benchmark runs.
package serving

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	llmperf "github.com/tmc/misc/xk6-llm-perf"
)

// Command identifies executable bytes and literal arguments. Arguments are never
// evaluated by a shell. Environment values are not included in receipts.
type Command struct {
	Path   string   `json:"path"`
	SHA256 string   `json:"sha256"`
	Args   []string `json:"args,omitempty"`
	Env    []string `json:"env,omitempty"`
	Dir    string   `json:"dir,omitempty"`
}

// Probe is one bounded HTTP operation. Status must be explicitly declared.
type Probe struct {
	URL    string `json:"url"`
	Method string `json:"method,omitempty"`
	Body   string `json:"body,omitempty"`
	Status int    `json:"status"`
}

// LifecycleConfig describes one fresh server arm. Timeout values are milliseconds.
// Observe must return the effective server configuration, not an author copy.
// ExpectedObservationSHA256 binds the exact observed bytes to prior evidence.
type LifecycleConfig struct {
	Server                    Command `json:"server"`
	Workload                  Command `json:"workload"`
	Ready                     Probe   `json:"ready"`
	Observe                   Probe   `json:"observe"`
	ExpectedEndpointIdentity  string  `json:"expected_endpoint_identity"`
	ExpectedObservationSHA256 string  `json:"expected_observation_sha256"`
	Warmup                    []Probe `json:"warmup"`
	Drain                     Probe   `json:"drain"`
	StartupMS                 int64   `json:"startup_ms"`
	WorkloadMS                int64   `json:"workload_ms"`
	StopMS                    int64   `json:"stop_ms"`
	// Lock wraps Server with an external lock executable. Its documented argv
	// protocol must execute the appended server command and hold ownership until exit.
	Lock         *Command `json:"lock,omitempty"`
	LockProtocol string   `json:"lock_protocol,omitempty"`
	RequireLock  bool     `json:"require_lock,omitempty"`
}

// PhaseReceipt records an operation without its potentially sensitive output.
type PhaseReceipt struct {
	Name     string    `json:"name"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
	SHA256   string    `json:"sha256,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// LifecycleReceipt binds executable and observed configuration bytes. Completion
// proves host supervision, not physical GPU ownership or engine parity.
type LifecycleReceipt struct {
	RunNonce             string    `json:"run_nonce"`
	LauncherPID          int       `json:"launcher_pid,omitempty"`
	ServerPID            int       `json:"server_pid,omitempty"`
	ServerProcessGroup   int       `json:"server_process_group,omitempty"`
	ServerStarted        time.Time `json:"server_started"`
	WorkloadPID          int       `json:"workload_pid,omitempty"`
	WorkloadProcessGroup int       `json:"workload_process_group,omitempty"`
	WorkloadStarted      time.Time `json:"workload_started"`

	ServerSHA256            string          `json:"server_sha256"`
	WorkloadSHA256          string          `json:"workload_sha256"`
	Observation             json.RawMessage `json:"observation,omitempty"`
	WorkHash                string          `json:"work_hash,omitempty"`
	ObservationSHA256       string          `json:"observation_sha256,omitempty"`
	LockSHA256              string          `json:"lock_sha256,omitempty"`
	LockProtocol            string          `json:"lock_protocol,omitempty"`
	Phases                  []PhaseReceipt  `json:"phases"`
	Complete                bool            `json:"complete"`
	ServerOutputSHA256      string          `json:"server_output_sha256"`
	WorkloadOutputSHA256    string          `json:"workload_output_sha256,omitempty"`
	ServerOutputTruncated   bool            `json:"server_output_truncated"`
	WorkloadOutputTruncated bool            `json:"workload_output_truncated"`
}

type boundedLog struct {
	data      []byte
	mu        sync.Mutex
	n         int
	truncated bool
}

func (b *boundedLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) < 65536 {
		n := min(len(p), 65536-len(b.data))
		b.data = append(b.data, p[:n]...)
	}
	b.n += len(p)
	if b.n > 65536 {
		b.truncated = true
		b.n = 65537
	}
	return len(p), nil
}
func (b *boundedLog) digest() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	h := sha256.Sum256(b.data)
	return hex.EncodeToString(h[:])
}
func (b *boundedLog) isTruncated() bool { b.mu.Lock(); defer b.mu.Unlock(); return b.truncated }

func verifyCommand(c Command) error {
	if len(c.Args) > 1024 || len(c.Env) > 1024 || len(c.Dir) > 4096 {
		return errors.New("command exceeds bounds")
	}
	n := 0
	for _, a := range append(append([]string{}, c.Args...), c.Env...) {
		n += len(a)
		if len(a) > 65536 || n > 1<<20 {
			return errors.New("command arguments or environment exceed bounds")
		}
	}

	if !filepath.IsAbs(c.Path) || len(c.SHA256) != 64 {
		return errors.New("command requires absolute path and sha256")
	}
	f, err := os.Open(c.Path)
	if err != nil {
		return fmt.Errorf("open executable: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Mode()&0111 == 0 {
		return errors.New("executable is not an executable regular file")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != c.SHA256 {
		return errors.New("executable sha256 mismatch")
	}
	return nil
}

func start(c Command) (*exec.Cmd, <-chan error, *boundedLog, error) {
	if err := verifyCommand(c); err != nil {
		return nil, nil, nil, err
	}
	cmd := exec.Command(c.Path, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(os.Environ(), c.Env...)
	log := new(boundedLog)
	cmd.Stdout = log
	cmd.Stderr = log
	configureGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait(); close(done) }()
	return cmd, done, log, nil
}

func probe(ctx context.Context, p Probe) ([]byte, error) {
	if p.URL == "" || p.Status < 100 || p.Status > 599 || len(p.Body) > 1<<20 {
		return nil, errors.New("probe requires URL, status and bounded body")
	}
	method := p.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, p.URL, strings.NewReader(p.Body))
	if err != nil {
		return nil, err
	}
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("probe transport failed")
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return nil, errors.New("read probe response failed")
	}
	if len(b) > 1<<20 {
		return nil, errors.New("probe response exceeds limit")
	}
	if resp.StatusCode != p.Status {
		return nil, fmt.Errorf("probe status %d, want %d", resp.StatusCode, p.Status)
	}
	return b, nil
}

func validateEndpoint(c LifecycleConfig) error {
	u, err := url.Parse(c.Ready.URL)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Port() == "" {
		return errors.New("readiness requires local HTTP endpoint with explicit port")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return errors.New("supervised server requires loopback IP endpoint")
	}
	probes := append([]Probe{c.Observe, c.Drain}, c.Warmup...)
	for _, p := range probes {
		v, e := url.Parse(p.URL)
		if e != nil || v.User != nil || v.Scheme != u.Scheme || v.Host != u.Host {
			return errors.New("lifecycle probes must share server origin")
		}
	}
	conn, e := net.DialTimeout("tcp", u.Host, 100*time.Millisecond)
	if e == nil {
		conn.Close()
		return errors.New("server endpoint already occupied")
	}
	return nil
}

// Run starts, observes, warms, exercises, drains and stops one fresh server.
// It refuses qualification when any phase or owned-process cleanup fails.
// Call sequentially for comparison arms; concurrent runs are not isolated.
func Run(ctx context.Context, c LifecycleConfig) (r LifecycleReceipt, err error) {
	if err = ctx.Err(); err != nil {
		return r, err
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return r, err
	}
	r.RunNonce = hex.EncodeToString(nonce)
	ctx, cancel := context.WithTimeout(ctx, 24*time.Hour)
	defer cancel()

	if c.StartupMS <= 0 || c.WorkloadMS <= 0 || c.StopMS <= 0 || c.StartupMS > 3600000 || c.WorkloadMS > 86400000 || c.StopMS > 60000 {
		return r, errors.New("invalid lifecycle deadlines")
	}
	if (len(c.ExpectedObservationSHA256) != 64 || c.ExpectedEndpointIdentity == "") || (len(c.Warmup) == 0 || len(c.Warmup) > 1000) || c.Drain.URL == "" {
		return r, errors.New("observation hash, warmup and drain are required")
	}
	if c.RequireLock && c.Lock == nil {
		return r, errors.New("owned lock required")
	}
	if err = validateEndpoint(c); err != nil {
		return r, err
	}
	if err = verifyCommand(c.Server); err != nil {
		return r, err
	}
	if err = verifyCommand(c.Workload); err != nil {
		return r, err
	}
	r.ServerSHA256 = c.Server.SHA256
	r.WorkloadSHA256 = c.Workload.SHA256
	launch := c.Server
	if c.Lock != nil {
		if c.LockProtocol != "append-command-v1" {
			return r, errors.New("unsupported lock protocol")
		}
		if err = verifyCommand(*c.Lock); err != nil {
			return r, err
		}
		r.LockSHA256 = c.Lock.SHA256
		r.LockProtocol = c.LockProtocol
		launch = *c.Lock
		if launch.Dir != "" && launch.Dir != c.Server.Dir {
			return r, errors.New("lock and server working directories differ")
		}
		launch.Dir = c.Server.Dir
		launch.Args = append(append([]string{}, c.Lock.Args...), append([]string{c.Server.Path}, c.Server.Args...)...)
		launch.Env = append(append([]string{}, c.Server.Env...), c.Lock.Env...)
	}
	started := PhaseReceipt{Name: "start", Started: time.Now().UTC()}
	cmd, done, log, e := start(launch)
	started.Finished = time.Now().UTC()
	started.SHA256 = launch.SHA256
	if e != nil {
		started.Error = "start server failed"
	}
	r.Phases = append(r.Phases, started)
	if e != nil {
		return r, fmt.Errorf("start server: %w", e)
	}
	r.LauncherPID = cmd.Process.Pid
	r.ServerProcessGroup = cmd.Process.Pid
	r.ServerStarted = started.Finished
	if c.Lock == nil {
		r.ServerPID = cmd.Process.Pid
	}
	defer func() {
		p := PhaseReceipt{Name: "stop", Started: time.Now().UTC()}
		e := stopGroup(cmd, done, time.Duration(c.StopMS)*time.Millisecond)
		p.Finished = time.Now().UTC()
		if e != nil {
			p.Error = e.Error()
			err = errors.Join(err, e)
		}
		r.Phases = append(r.Phases, p)
		r.ServerOutputTruncated = log.isTruncated()
		r.ServerOutputSHA256 = log.digest()
		r.Complete = err == nil
	}()
	phase := func(name string, fn func(context.Context) ([]byte, error), timeout time.Duration) ([]byte, error) {
		p := PhaseReceipt{Name: name, Started: time.Now().UTC()}
		index := len(r.Phases)
		r.Phases = append(r.Phases, p)
		pc, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		b, e := fn(pc)
		p.Finished = time.Now().UTC()
		if e != nil {
			p.Error = e.Error()
		} else {
			h := sha256.Sum256(b)
			p.SHA256 = hex.EncodeToString(h[:])
		}
		r.Phases[index] = p
		return b, e
	}
	_, err = phase("ready", func(pc context.Context) ([]byte, error) {
		for {
			select {
			case <-done:
				return nil, errors.New("server exited before readiness")
			default:
			}
			b, e := probe(pc, c.Ready)
			if e == nil {
				return b, nil
			}
			select {
			case <-pc.Done():
				return nil, errors.New("readiness deadline exceeded")
			case <-time.After(20 * time.Millisecond):
			}
		}
	}, time.Duration(c.StartupMS)*time.Millisecond)
	if err != nil {
		return r, err
	}
	b, e := phase("observe", func(pc context.Context) ([]byte, error) { return probe(pc, c.Observe) }, time.Duration(c.StartupMS)*time.Millisecond)
	if e != nil {
		return r, e
	}
	h := sha256.Sum256(b)
	r.ObservationSHA256 = hex.EncodeToString(h[:])
	if r.ObservationSHA256 != c.ExpectedObservationSHA256 {
		return r, errors.New("effective observation sha256 mismatch")
	}
	if !json.Valid(b) {
		return r, errors.New("effective observation is not JSON")
	}
	manifest, e := llmperf.ReadServingManifest(bytes.NewReader(b))
	if e != nil {
		return r, fmt.Errorf("read observed manifest: %w", e)
	}
	if manifest.Arm.BinaryDigest != c.Server.SHA256 || manifest.Arm.SourceRevision == "" || manifest.Arm.Backend == "" || manifest.Arm.EndpointIdentity != c.ExpectedEndpointIdentity {
		return r, errors.New("observed server identity does not bind launch")
	}
	r.WorkHash, e = llmperf.ServingWorkHash(*manifest)
	if e != nil {
		return r, fmt.Errorf("qualify observed manifest: %w", e)
	}
	r.Observation = append(json.RawMessage(nil), b...)
	for _, p := range c.Warmup {
		_, e = phase("warmup", func(pc context.Context) ([]byte, error) { return probe(pc, p) }, time.Duration(c.StartupMS)*time.Millisecond)
		if e != nil {
			return r, e
		}
	}
	_, err = phase("workload", func(pc context.Context) ([]byte, error) {
		wp := PhaseReceipt{Name: "workload_start", Started: time.Now().UTC()}
		w, wd, wl, e := start(c.Workload)
		wp.Finished = time.Now().UTC()
		wp.SHA256 = c.Workload.SHA256
		if e != nil {
			wp.Error = "start workload failed"
		}
		r.Phases = append(r.Phases, wp)
		if e != nil {
			return nil, e
		}
		r.WorkloadPID = w.Process.Pid
		r.WorkloadProcessGroup = w.Process.Pid
		r.WorkloadStarted = wp.Finished
		defer func() { r.WorkloadOutputTruncated = wl.isTruncated(); r.WorkloadOutputSHA256 = wl.digest() }()
		select {
		case e := <-wd:
			ce := stopGroup(w, wd, time.Duration(c.StopMS)*time.Millisecond)
			return nil, errors.Join(e, ce)
		case <-done:
			_ = stopGroup(w, wd, time.Duration(c.StopMS)*time.Millisecond)
			return nil, errors.New("server exited during workload")
		case <-pc.Done():
			_ = stopGroup(w, wd, time.Duration(c.StopMS)*time.Millisecond)
			return nil, errors.New("workload canceled or deadline exceeded")
		}
	}, time.Duration(c.WorkloadMS)*time.Millisecond)
	if err != nil {
		return r, err
	}
	_, err = phase("drain", func(pc context.Context) ([]byte, error) { return probe(pc, c.Drain) }, time.Duration(c.StartupMS)*time.Millisecond)
	if err == nil {
		select {
		case <-done:
			err = errors.New("server exited before supervised stop")
		default:
		}
	}
	return r, err
}
