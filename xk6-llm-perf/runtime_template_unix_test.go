//go:build unix

package llmperf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRuntimeTemplateSuccessfulLeaderCleanup(t *testing.T) {
	base := runtimeTemplateFixture(t)
	for _, redirected := range []bool{false, true} {
		t.Run(fmt.Sprint(redirected), func(t *testing.T) {
			dir := t.TempDir()
			started, survivor := filepath.Join(dir, "started"), filepath.Join(dir, "survivor")
			child := fmt.Sprintf("import pathlib,time;pathlib.Path(%q).write_text('started');time.sleep(0.8);pathlib.Path(%q).write_text('survived')", started, survivor)
			redirection := ""
			if redirected {
				redirection = ",stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL"
			}
			source := fmt.Sprintf(`import pathlib,subprocess,sys,time
sys.stdin.buffer.read()
subprocess.Popen([sys.executable,'-I','-c',%q]%s)
deadline=time.monotonic()+3
while not pathlib.Path(%q).exists():
    if time.monotonic()>deadline: raise RuntimeError('child did not start')
    time.sleep(0.005)
sys.stdout.write('v' * (128 << 10))
sys.stdout.flush()
`, child, redirection, started)
			ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, base.Python, "-I", "-c", source)
			cmd.Stdin = bytes.NewReader(make([]byte, 1<<20))
			var output bytes.Buffer
			cmd.Stdout = &output
			if err := runRuntimeTemplate(cmd); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(output.Bytes(), bytes.Repeat([]byte("v"), 128<<10)) {
				t.Fatalf("leader output length: %d", output.Len())
			}
			if _, err := os.Stat(started); err != nil {
				t.Fatal("child startup was not observed")
			}
			time.Sleep(time.Second)
			if _, err := os.Stat(survivor); !os.IsNotExist(err) {
				t.Fatal("successful leader left a descendant running")
			}
		})
	}
}

func TestRuntimeTemplateProcessGroup(t *testing.T) {
	base := runtimeTemplateFixture(t)
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprint(timeout), func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "survivor")
			source := fmt.Sprintf("import subprocess,time\nsubprocess.Popen(['%s','-I','-c',%q])\n", base.Python, "import time,pathlib;time.sleep(0.8);pathlib.Path("+fmt.Sprintf("%q", marker)+").write_text('survived')")
			if timeout {
				source += "time.sleep(30)\n"
			}
			script := filepath.Join(dir, "worker.py")
			if err := os.WriteFile(script, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256([]byte(source))
			opts := base
			opts.Script = script
			opts.ScriptSHA256 = hex.EncodeToString(hash[:])
			opts.Request = json.RawMessage(`{"operation":"select"}`)
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			start := time.Now()
			_, err := EvaluateRuntimeTemplate(ctx, opts)
			if err == nil {
				t.Fatal("accepted worker without result")
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("worker cancellation exceeded budget")
			}
			time.Sleep(time.Second)
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("worker descendant survived group cleanup")
			}
		})
	}
}

func TestRuntimeTemplateRejectsFIFO(t *testing.T) {
	opts := runtimeTemplateFixture(t)
	fifo := filepath.Join(t.TempDir(), "worker")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	opts.Script = fifo
	opts.Request = json.RawMessage(`{"operation":"select"}`)
	start := time.Now()
	if _, err := EvaluateRuntimeTemplate(context.Background(), opts); err == nil {
		t.Fatal("accepted fifo")
	}
	if time.Since(start) > time.Second {
		t.Fatal("fifo open blocked")
	}
}

func TestRuntimeTemplatePipeLimit(t *testing.T) {
	opts := runtimeTemplateFixture(t)
	for _, size := range []int{runtimeTemplateLimit, runtimeTemplateLimit + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, opts.Python, "-I", "-c", fmt.Sprintf("import sys;sys.stdout.buffer.write(b'x' * %d)", size))
			var output runtimeTemplateOutput
			cmd.Stdout = &output
			err := runRuntimeTemplate(cmd)
			if (err == nil) != (size == runtimeTemplateLimit) || output.Len() > runtimeTemplateLimit {
				t.Fatalf("size=%d retained=%d error=%v", size, output.Len(), err)
			}
			if err == nil && !bytes.Equal(output.Bytes(), bytes.Repeat([]byte("x"), size)) {
				t.Fatal("output differs")
			}
		})
	}
}

func TestRuntimeTemplateReceiptLimit(t *testing.T) {
	base := runtimeTemplateFixture(t)
	for _, size := range []int{0, runtimeTemplateLimit, runtimeTemplateLimit + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			// The pinned synthetic worker tests envelope admission and output limits,
			// not actual template-library behavior or provenance attestation.
			source := fmt.Sprintf(`import hashlib,json,sys
b=sys.stdin.buffer.read()
r=json.loads(b)
out={"schema_version":1,"operation":r["operation"],"value":"x","provenance":{"reference_revision":"f10eb50a1776775d80d7f13396b0f668f14e2c03","dependency_versions":{"Jinja2":"3.1.6","jmespath":"1.0.1","MarkupSafe":"3.0.3"},"scope":"offline-json-jinja-sandbox-jmespath-v1","request_sha256":hashlib.sha256(b).hexdigest(),"value_sha256":hashlib.sha256(b'"x"').hexdigest()}}
wire=json.dumps(out)
sys.stdout.write(" "*max(0,%d-len(wire))+wire)
`, size)
			opts := base
			opts.Script = filepath.Join(t.TempDir(), "worker.py")
			if err := os.WriteFile(opts.Script, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256([]byte(source))
			opts.ScriptSHA256 = hex.EncodeToString(hash[:])
			opts.Request = json.RawMessage(`{"operation":"select"}`)
			result, err := EvaluateRuntimeTemplate(context.Background(), opts)
			if size <= runtimeTemplateLimit {
				if err != nil || len(result.Result) == 0 || (size != 0 && len(result.Result) != size) {
					t.Fatalf("valid receipt: result_bytes=%d error=%v", len(result.Result), err)
				}
			} else if err == nil || len(result.Result) != 0 {
				t.Fatalf("oversized receipt accepted: result_bytes=%d error=%v", len(result.Result), err)
			}
		})
	}
}
