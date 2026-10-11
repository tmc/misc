package accuracy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkerCopyLimit(t *testing.T) {
	const limit = 1 << 20
	for _, size := range []int{limit, limit + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "output")
			if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0600); err != nil {
				t.Fatal(err)
			}
			input, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			output := boundedWriter{limit: limit}
			n, err := io.Copy(&output, input)
			if (err == nil) != (size == limit) || output.Len() > limit {
				t.Fatalf("size=%d copied=%d retained=%d error=%v", size, n, output.Len(), err)
			}
			if err == nil && !bytes.Equal(output.Bytes(), bytes.Repeat([]byte("x"), size)) {
				t.Fatal("output differs")
			}
		})
	}
}

func TestWorkerReceiptLimit(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	for _, size := range []int{0, 1 << 20, 1<<20 + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			// This synthetic pinned bridge exercises byte-budget and receipt admission,
			// not confinement or upstream grading correctness.
			body := fmt.Sprintf(`import hashlib,json,pathlib,sys
b=sys.stdin.buffer.read()
r=json.loads(b)
out={"schema_version":1,"evaluator":r["evaluator"],"source_revision":"f10eb50a1776775d80d7f13396b0f668f14e2c03","dependency_versions":r["dependency_versions"],"correct":r["prediction"]==r["reference"],"unparsed":False,"request_sha256":hashlib.sha256(b).hexdigest(),"worker_sha256":hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),"isolation":"bubblewrap-unshare-all-readonly-v1"}
wire=json.dumps(out)
sys.stdout.write(" "*max(0,%d-len(wire))+wire)
`, size)
			script := filepath.Join(t.TempDir(), "worker.py")
			if err := os.WriteFile(script, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256([]byte(body))
			opts := WorkerOptions{Python: python, Script: script, ScriptSHA256: hex.EncodeToString(hash[:]), AIPerfSource: "fixture", RuntimeRoot: "fixture", DependencyVersions: map[string]string{"fixture": "1"}}
			wire, err := json.Marshal(fixture("math-symbolic-v1"))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			report, err := EvaluateWithWorker(ctx, bytes.NewReader(wire), opts)
			if size <= 1<<20 {
				if err != nil || report.Correct != 1 || report.Worker == nil || report.Worker.CasesEvaluated != 2 {
					t.Fatalf("valid receipt: report=%+v error=%v", report, err)
				}
			} else if err == nil || report.Worker != nil {
				t.Fatalf("oversized receipt accepted: report=%+v error=%v", report, err)
			}
		})
	}
}
