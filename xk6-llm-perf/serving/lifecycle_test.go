package serving

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	llmperf "github.com/tmc/misc/xk6-llm-perf"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLifecycleHelper(t *testing.T) {
	switch os.Getenv("SERVING_HELPER") {
	case "":
		return
	case "exit":
		os.Exit(3)
	case "work":
		resp, err := http.Get(os.Getenv("SERVING_URL") + "/work")
		if err != nil {
			os.Exit(4)
		}
		resp.Body.Close()
		os.Exit(0)
	case "stubborn":
		signal.Ignore(syscall.SIGTERM)
		os.WriteFile(os.Getenv("SERVING_PID"), []byte(fmt.Sprint(os.Getpid())), 0600)
		for {
			time.Sleep(time.Second)
		}
	case "server":
		if os.Getenv("SERVING_CHILD") == "yes" {
			cmd := exec.Command(os.Args[0], "-test.run=^TestLifecycleHelper$")
			cmd.Env = append(os.Environ(), "SERVING_HELPER=stubborn")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				os.Exit(6)
			}
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			f, _ := os.OpenFile(os.Getenv("SERVING_EVENTS"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			fmt.Fprintln(f, r.URL.Path)
			f.Close()
			if r.URL.Path == "/observe" {
				fmt.Fprint(w, os.Getenv("SERVING_MANIFEST"))
			} else {
				fmt.Fprint(w, "ok")
			}
		})
		if err := http.ListenAndServe(os.Getenv("SERVING_ADDR"), mux); err != nil {
			os.Exit(5)
		}
		os.Exit(0)
	}
}
func fixture(t *testing.T) (LifecycleConfig, string, string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	dir := t.TempDir()
	events := filepath.Join(dir, "events")
	pid := filepath.Join(dir, "pid")
	url := "http://" + addr
	command := Command{Path: exe, SHA256: hex.EncodeToString(h[:]), Args: []string{"-test.run=^TestLifecycleHelper$"}}
	server := command
	server.Env = []string{"SERVING_HELPER=server", "SERVING_ADDR=" + addr, "SERVING_EVENTS=" + events, "SERVING_PID=" + pid}
	workload := command
	workload.Env = []string{"SERVING_HELPER=work", "GORACE=atexit_sleep_ms=0", "SERVING_URL=" + url}
	m := llmperf.ServingManifest{SchemaVersion: 1, Arm: llmperf.ServingArm{BinaryDigest: command.SHA256, SourceRevision: strings.Repeat("b", 40), Backend: "test", EndpointIdentity: "local-test"}, ModelDigest: strings.Repeat("a", 64), TokenizerDigest: strings.Repeat("a", 64), TemplateDigest: strings.Repeat("a", 64), WorkloadDigest: strings.Repeat("a", 64), EnvironmentDigest: strings.Repeat("a", 64), Settings: map[string]llmperf.ServingSetting{}}
	for _, name := range []string{"cache", "compile", "resources", "scheduler", "dtype", "device", "temperature", "top_p", "top_k", "seed", "max_output_tokens"} {
		v := `0`
		switch name {
		case "cache", "compile", "resources", "scheduler":
			v = `{"enabled":false}`
		case "dtype", "device":
			v = `"test"`
		}
		m.Settings[name] = llmperf.ServingSetting{Effective: json.RawMessage(v), EvidenceDigest: strings.Repeat("a", 64)}
	}
	manifest, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	server.Env = append(server.Env, "SERVING_MANIFEST="+string(manifest))
	observation := sha256.Sum256(manifest)
	return LifecycleConfig{Server: server, Workload: workload, Ready: Probe{URL: url + "/ready", Status: 200}, Observe: Probe{URL: url + "/observe", Status: 200}, ExpectedEndpointIdentity: "local-test", ExpectedObservationSHA256: hex.EncodeToString(observation[:]), Warmup: []Probe{{URL: url + "/warmup", Status: 200}}, Drain: Probe{URL: url + "/drain", Status: 200}, StartupMS: 5000, WorkloadMS: 10000, StopMS: 200}, events, pid
}
func TestLifecycle(t *testing.T) {
	c, events, _ := fixture(t)
	r, err := Run(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Complete || len(r.Phases) != 8 || r.ServerPID <= 0 || r.WorkloadPID <= 0 || r.ServerPID == r.WorkloadPID || r.RunNonce == "" {
		t.Fatalf("receipt %+v", r)
	}
	b, err := os.ReadFile(events)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "/ready\n/observe\n/warmup\n/work\n/drain\n" {
		t.Fatalf("events %s", b)
	}
}
func TestLifecycleFailures(t *testing.T) {
	for _, name := range []string{"hash", "observe", "exit", "cancel", "lock", "warmup"} {
		t.Run(name, func(t *testing.T) {
			c, _, _ := fixture(t)
			ctx := context.Background()
			switch name {
			case "hash":
				c.Server.SHA256 = strings.Repeat("0", 64)
			case "observe":
				c.ExpectedObservationSHA256 = strings.Repeat("0", 64)
			case "exit":
				c.Server.Env = []string{"SERVING_HELPER=exit"}
			case "cancel":
				cc, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cc
			case "lock":
				c.RequireLock = true
			case "warmup":
				c.Warmup[0].Status = 201
			}
			r, err := Run(ctx, c)
			if err == nil || r.Complete {
				t.Fatalf("accepted %s %+v", name, r)
			}
		})
	}
}
func TestLifecycleStubbornDescendant(t *testing.T) {
	c, _, pidfile := fixture(t)
	c.Server.Env = append(c.Server.Env, "SERVING_CHILD=yes")
	r, err := Run(context.Background(), c)
	if err != nil || !r.Complete {
		t.Fatalf("%v %+v", err, r)
	}
	b, err := os.ReadFile(pidfile)
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	fmt.Sscan(string(b), &pid)
	// A killed orphan may remain a zombie until the OS reaps it; it must not run.
	if err := syscall.Kill(pid, 0); err == nil {
		out, e := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(pid)).Output()
		if e == nil && !strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
			t.Fatalf("descendant running: %s", out)
		}
	}
}

func TestLifecycleOccupiedPort(t *testing.T) {
	c, _, _ := fixture(t)
	addr := strings.TrimPrefix(strings.TrimSuffix(c.Ready.URL, "/ready"), "http://")
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	r, err := Run(context.Background(), c)
	if err == nil || r.Complete || len(r.Phases) != 0 {
		t.Fatalf("occupied endpoint accepted: %+v %v", r, err)
	}
}
func TestBoundedOutput(t *testing.T) {
	log := new(boundedLog)
	data := []byte(strings.Repeat("a", 1<<20))
	n, err := log.Write(data)
	if err != nil || n != len(data) || !log.isTruncated() || len(log.data) != 65536 {
		t.Fatalf("invalid bounded log %d %v", n, err)
	}
}

func TestLifecycleWorkloadDeadline(t *testing.T) {
	c, _, _ := fixture(t)
	c.Workload.Env = append(c.Workload.Env, "SERVING_HELPER=stubborn")
	c.WorkloadMS = 50
	r, err := Run(context.Background(), c)
	if err == nil || r.Complete || len(r.Phases) == 0 || r.Phases[len(r.Phases)-1].Name != "stop" {
		t.Fatalf("deadline accepted: %v", err)
	}
}
func TestLifecycleDrainFailure(t *testing.T) {
	c, _, _ := fixture(t)
	c.Drain.Status = 201
	r, err := Run(context.Background(), c)
	if err == nil || r.Complete {
		t.Fatal("failed drain qualified")
	}
}
