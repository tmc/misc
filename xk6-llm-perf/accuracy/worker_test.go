package accuracy

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
	"strings"
	"testing"
)

func ExampleEvaluateWithWorker() {
	_, err := EvaluateWithWorker(context.Background(), bytes.NewReader(nil), WorkerOptions{})
	fmt.Println(err)
	// Output: missing worker configuration
}
func TestWorkerProtocol(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	script := filepath.Join(t.TempDir(), "protocol_fixture.py")
	body := `import hashlib,json,pathlib,sys
b=sys.stdin.buffer.read()
r=json.loads(b)
out={"schema_version":1,"evaluator":r["evaluator"],"source_revision":"f10eb50a1776775d80d7f13396b0f668f14e2c03","dependency_versions":r["dependency_versions"],"correct":r["prediction"]==r["reference"],"unparsed":False,"request_sha256":hashlib.sha256(b).hexdigest(),"worker_sha256":hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),"isolation":"bubblewrap-unshare-all-readonly-v1"}
print(json.dumps(out))
`
	os.WriteFile(script, []byte(body), 0600)
	h := sha256.Sum256([]byte(body))
	opts := WorkerOptions{Python: python, Script: script, ScriptSHA256: hex.EncodeToString(h[:]), AIPerfSource: "fixture", RuntimeRoot: "fixture", DependencyVersions: map[string]string{"fixture": "1"}}
	a := fixture("math-symbolic-v1")
	b, _ := json.Marshal(a)
	r, err := EvaluateWithWorker(context.Background(), bytes.NewReader(b), opts)
	if err != nil || r.Correct != 1 || r.Worker == nil || r.Worker.CasesEvaluated != 2 {
		t.Fatal(r, err)
	}
	// A pinned worker still cannot overwrite a score with duplicate members.
	duplicateBody := strings.Replace(body, `print(json.dumps(out))`, `print(json.dumps(out)[:-1]+',"correct":false}')`, 1)
	if err := os.WriteFile(script, []byte(duplicateBody), 0600); err != nil {
		t.Fatal(err)
	}
	duplicateHash := sha256.Sum256([]byte(duplicateBody))
	opts.ScriptSHA256 = hex.EncodeToString(duplicateHash[:])
	if _, err := EvaluateWithWorker(context.Background(), bytes.NewReader(b), opts); err == nil {
		t.Fatal("duplicate worker score accepted")
	}
	// This tests the trusted bridge wire contract, not actual confinement or math parity.
	opts.ScriptSHA256 = "wrong"
	if _, err := EvaluateWithWorker(context.Background(), bytes.NewReader(b), opts); err == nil {
		t.Fatal("wrong script hash accepted")
	}
}
