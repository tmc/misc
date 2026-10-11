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
	"reflect"
	"time"
)

// WorkerOptions pins an explicitly selected, trusted confinement bridge.
// Python and its runtime are caller provisioned; the bridge verifies its clean
// AIPerf source and dependency pins before importing them. It refuses unsupported
// operating systems or missing bubblewrap instead of executing answers on host.
type WorkerOptions struct {
	Python             string
	Script             string
	ScriptSHA256       string
	AIPerfSource       string
	RuntimeRoot        string
	DependencyVersions map[string]string
}

// WorkerReceipt records checked provenance for the external evaluator bridge.
type WorkerReceipt struct {
	CasesEvaluated     int               `json:"cases_evaluated"`
	ScriptSHA256       string            `json:"script_sha256"`
	SourceRevision     string            `json:"source_revision"`
	DependencyVersions map[string]string `json:"dependency_versions"`
	Isolation          string            `json:"isolation"`
}

const evaluatorRevision = "f10eb50a1776775d80d7f13396b0f668f14e2c03"

func externalEvaluator(name string) bool {
	switch name {
	case "math-symbolic-v1", "lighteval-expr-v1", "lighteval-latex-v1", "lighteval-gpqa-v1", "code-execution-v1":
		return true
	}
	return false
}

// EvaluateWithWorker evaluates an artifact through a hash-pinned bridge when its
// evaluator requires symbolic, lighteval or code-execution support. Worker output
// is bounded and its request hash, script hash, source and dependencies must match.
// No response text, stderr or credentials are copied into the report.
func EvaluateWithWorker(ctx context.Context, reader io.Reader, opts WorkerOptions) (Report, error) {
	if opts.Python == "" || opts.Script == "" || opts.AIPerfSource == "" || opts.RuntimeRoot == "" || len(opts.DependencyVersions) == 0 || len(opts.DependencyVersions) > 128 {
		return Report{}, fmt.Errorf("missing worker configuration")
	}
	script, err := os.Open(opts.Script)
	if err != nil {
		return Report{}, fmt.Errorf("open worker script")
	}
	data, err := io.ReadAll(io.LimitReader(script, 1<<20+1))
	closeErr := script.Close()
	if err != nil || closeErr != nil || len(data) > 1<<20 {
		return Report{}, fmt.Errorf("worker script exceeds budget or read failed")
	}
	hash := sha256.Sum256(data)
	scriptHash := hex.EncodeToString(hash[:])
	if scriptHash != opts.ScriptSHA256 {
		return Report{}, fmt.Errorf("worker script hash mismatch")
	}
	for packageName, version := range opts.DependencyVersions {
		if packageName == "" || version == "" || len(packageName) > 128 || len(version) > 128 {
			return Report{}, fmt.Errorf("invalid dependency version pin")
		}
	}
	casesEvaluated := 0
	grade := func(evaluator, pred, ref string) (bool, bool, error) {
		request := struct {
			SchemaVersion      int               `json:"schema_version"`
			Evaluator          string            `json:"evaluator"`
			Prediction         string            `json:"prediction"`
			Reference          string            `json:"reference"`
			DependencyVersions map[string]string `json:"dependency_versions"`
		}{1, evaluator, pred, ref, opts.DependencyVersions}
		body, err := json.Marshal(request)
		if err != nil {
			return false, false, err
		}
		requestHash := sha256.Sum256(body)
		caseCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
		defer cancel()
		cmd := exec.CommandContext(caseCtx, opts.Python, opts.Script, "--allow-confined-execution", "--aiperf-source", opts.AIPerfSource, "--source-revision", evaluatorRevision, "--runtime-root", opts.RuntimeRoot) // #nosec G204 -- explicit trusted caller launcher and hash-verified script; fixed arguments without a shell
		cmd.Stdin = bytes.NewReader(body)
		out := &boundedWriter{limit: 1 << 20}
		cmd.Stdout = out
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			return false, false, fmt.Errorf("confined worker failed or exceeded budget")
		}
		var result struct {
			SchemaVersion      int               `json:"schema_version"`
			Evaluator          string            `json:"evaluator"`
			SourceRevision     string            `json:"source_revision"`
			DependencyVersions map[string]string `json:"dependency_versions"`
			Correct            *bool             `json:"correct"`
			Unparsed           *bool             `json:"unparsed"`
			RequestSHA256      string            `json:"request_sha256"`
			WorkerSHA256       string            `json:"worker_sha256"`
			Isolation          string            `json:"isolation"`
		}
		if err := uniqueJSON(out.Bytes()); err != nil {
			return false, false, fmt.Errorf("invalid worker response")
		}
		d := json.NewDecoder(bytes.NewReader(out.Bytes()))
		d.DisallowUnknownFields()
		if err := d.Decode(&result); err != nil {
			return false, false, fmt.Errorf("invalid worker response")
		}
		if err := d.Decode(new(any)); err != io.EOF {
			return false, false, fmt.Errorf("trailing worker response")
		}
		if result.SchemaVersion != 1 || result.Evaluator != evaluator || result.SourceRevision != evaluatorRevision || result.WorkerSHA256 != scriptHash || result.RequestSHA256 != hex.EncodeToString(requestHash[:]) || result.Isolation != "bubblewrap-unshare-all-readonly-v1" || !reflect.DeepEqual(result.DependencyVersions, opts.DependencyVersions) || result.Correct == nil || result.Unparsed == nil {
			return false, false, fmt.Errorf("worker provenance mismatch")
		}
		casesEvaluated++
		return *result.Correct, *result.Unparsed, nil
	}
	r, err := evaluate(reader, grade)
	if err != nil {
		return r, err
	}
	if casesEvaluated > 0 {
		r.Worker = &WorkerReceipt{casesEvaluated, scriptHash, evaluatorRevision, opts.DependencyVersions, "bubblewrap-unshare-all-readonly-v1"}
	}
	return r, nil
}

type boundedWriter struct {
	buffer bytes.Buffer
	limit  int
}

func (w *boundedWriter) Len() int      { return w.buffer.Len() }
func (w *boundedWriter) Bytes() []byte { return w.buffer.Bytes() }

func (w *boundedWriter) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.Len() {
		return 0, fmt.Errorf("output budget exceeded")
	}
	return w.buffer.Write(p)
}
