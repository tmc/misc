package llmperf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/grafana/sobek"
	"github.com/tmc/misc/xk6-llm-perf/internal/jsoncheck"
)

// RuntimeTemplateOptions selects a trusted, immutable Python installation and
// hash-pinned scripts/template-compat.py. Request is its JSON-only protocol.
type RuntimeTemplateOptions struct {
	Python       string          `json:"python"`
	PythonSHA256 string          `json:"pythonSHA256"`
	Script       string          `json:"script"`
	ScriptSHA256 string          `json:"scriptSHA256"`
	Request      json.RawMessage `json:"request"`
}

// RuntimeTemplateResult separates evaluator cost from HTTP request timing.
// Value is authored data, not a response adapter or token measurement.
type RuntimeTemplateResult struct {
	Result       json.RawMessage `json:"result"`
	ElapsedMS    float64         `json:"elapsed_ms"`
	ScriptSHA256 string          `json:"script_sha256"`
}

const runtimeTemplateLimit = 1 << 20

// EvaluateRuntimeTemplate evaluates one bounded JSON-only Jinja or JMESPath
// request. It does not send HTTP requests or change client metrics. The trusted
// Python runtime and worker must remain immutable throughout execution.
func EvaluateRuntimeTemplate(ctx context.Context, opts RuntimeTemplateOptions) (RuntimeTemplateResult, error) {
	start := time.Now()
	var result RuntimeTemplateResult
	if !filepath.IsAbs(opts.Python) || !filepath.IsAbs(opts.Script) || len(opts.ScriptSHA256) != 64 || len(opts.PythonSHA256) != 64 {
		return result, fmt.Errorf("runtime template requires absolute runtime and script paths and sha256 pin")
	}
	if len(opts.Request) == 0 || len(opts.Request) > runtimeTemplateLimit || strictRuntimeJSON(opts.Request) != nil {
		return result, fmt.Errorf("invalid runtime template request")
	}
	interpreter, err := openRuntimeTemplate(opts.Python)
	if err != nil {
		return result, fmt.Errorf("open runtime template interpreter")
	}
	hasher := sha256.New()
	n, hashErr := io.Copy(hasher, io.LimitReader(interpreter, 256<<20+1))
	interpreterCloseErr := interpreter.Close()
	if hashErr != nil || interpreterCloseErr != nil || n > 256<<20 || hex.EncodeToString(hasher.Sum(nil)) != opts.PythonSHA256 {
		return result, fmt.Errorf("runtime template interpreter hash mismatch")
	}
	f, err := openRuntimeTemplate(opts.Script)
	if err != nil {
		return result, fmt.Errorf("open runtime template worker")
	}
	data, err := io.ReadAll(io.LimitReader(f, 65536+1))
	closeErr := f.Close()
	if err != nil || closeErr != nil || len(data) > 65536 {
		return result, fmt.Errorf("read runtime template worker")
	}
	hash := sha256.Sum256(data)
	pin := hex.EncodeToString(hash[:])
	if pin != opts.ScriptSHA256 {
		return result, fmt.Errorf("runtime template worker hash mismatch")
	}
	var request struct {
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(opts.Request, &request); err != nil || (request.Operation != "render" && request.Operation != "select") {
		return result, fmt.Errorf("invalid runtime template operation")
	}
	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, opts.Python, "-I", "-c", string(data)) // #nosec G204 -- trusted pinned interpreter and already hashed worker bytes, no shell
	cmd.Stdin = bytes.NewReader(opts.Request)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "OPENBLAS_NUM_THREADS=1", "OMP_NUM_THREADS=1", "MKL_NUM_THREADS=1", "NUMEXPR_NUM_THREADS=1"}
	out := &runtimeTemplateOutput{}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	err = runRuntimeTemplate(cmd)
	result.ScriptSHA256 = pin
	if err != nil {
		return result, fmt.Errorf("runtime template worker failed or exceeded budget")
	}
	if err := strictRuntimeJSON(out.Bytes()); err != nil {
		return result, fmt.Errorf("invalid runtime template response JSON")
	}
	var envelope struct {
		Schema     int             `json:"schema_version"`
		Operation  string          `json:"operation"`
		Value      json.RawMessage `json:"value"`
		Provenance struct {
			Reference   string            `json:"reference_revision"`
			Versions    map[string]string `json:"dependency_versions"`
			Scope       string            `json:"scope"`
			RequestHash string            `json:"request_sha256"`
			ValueHash   string            `json:"value_sha256"`
		} `json:"provenance"`
	}
	d := json.NewDecoder(bytes.NewReader(out.Bytes()))
	d.DisallowUnknownFields()
	if err := d.Decode(&envelope); err != nil {
		return result, fmt.Errorf("invalid runtime template response")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return result, fmt.Errorf("trailing runtime template response")
	}
	v := envelope.Provenance.Versions
	if envelope.Schema != 1 || envelope.Operation != request.Operation || len(envelope.Value) == 0 || envelope.Provenance.Reference != "f10eb50a1776775d80d7f13396b0f668f14e2c03" || envelope.Provenance.Scope != "offline-json-jinja-sandbox-jmespath-v1" || len(envelope.Provenance.RequestHash) != 64 || len(envelope.Provenance.ValueHash) != 64 || len(v) != 3 || v["Jinja2"] != "3.1.6" || v["jmespath"] != "1.0.1" || v["MarkupSafe"] != "3.0.3" {
		return result, fmt.Errorf("runtime template provenance mismatch")
	}
	for _, pin := range []string{envelope.Provenance.RequestHash, envelope.Provenance.ValueHash} {
		if data, err := hex.DecodeString(pin); err != nil || len(data) != sha256.Size {
			return result, fmt.Errorf("invalid runtime template provenance hash")
		}
	}
	result.Result = append(json.RawMessage(nil), out.Bytes()...)
	result.ElapsedMS = float64(time.Since(start)) / float64(time.Millisecond)
	return result, nil
}

// Canonical request/result hashes in the envelope are the trusted pinned
// worker's receipts. They are not independently recomputed here: Python and Go
// encode JSON floats and escaped characters differently.
func strictRuntimeJSON(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("invalid utf8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	return jsoncheck.Check(d, jsoncheck.Options{})
}

type runtimeTemplateOutput struct {
	buffer bytes.Buffer
}

func (w *runtimeTemplateOutput) Len() int      { return w.buffer.Len() }
func (w *runtimeTemplateOutput) Bytes() []byte { return w.buffer.Bytes() }

func (w *runtimeTemplateOutput) Write(p []byte) (int, error) {
	if len(p) > runtimeTemplateLimit-w.Len() {
		return 0, fmt.Errorf("runtime template output exceeds budget")
	}
	return w.buffer.Write(p)
}

func (mi *moduleInstance) exportRuntimeTemplate() error {
	rt := mi.vu.Runtime()
	return mi.exports.Set("evaluateRuntimeTemplate", func(v sobek.Value) sobek.Value {
		var opts RuntimeTemplateOptions
		if err := exportValue(v, &opts); err != nil {
			return rt.NewGoError(err)
		}
		result, err := EvaluateRuntimeTemplate(mi.vu.Context(), opts)
		if err != nil {
			return rt.NewGoError(err)
		}
		value, err := jsonValue(rt, result)
		if err != nil {
			return rt.NewGoError(err)
		}
		return value
	})
}
