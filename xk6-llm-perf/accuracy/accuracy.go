package accuracy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Case associates a caller supplied output with a stable reference identity.
// A nil Prediction is unavailable, while an empty string is an observed answer.
type Case struct {
	ID         string  `json:"id"`
	Reference  string  `json:"reference"`
	Prediction *string `json:"prediction"`
	Outcome    string  `json:"outcome"`
}

// Artifact pins the task, reference dataset, evaluator and model identity.
type Artifact struct {
	PromptSHA256     string `json:"prompt_sha256,omitempty"`
	SchemaVersion    int    `json:"schema_version"`
	Task             string `json:"task"`
	TaskVersion      string `json:"task_version"`
	ReferenceVersion string `json:"reference_version"`
	ReferenceSHA256  string `json:"reference_sha256"`
	Evaluator        string `json:"evaluator"`
	EvaluatorVersion string `json:"evaluator_version"`
	Model            string `json:"model"`
	ModelRevision    string `json:"model_revision"`
	Cases            []Case `json:"cases"`
}

// Result retains all case outcomes without copying sensitive answer text.
type Result struct {
	Unparsed bool   `json:"unparsed"`
	ID       string `json:"id"`
	Status   string `json:"status"`
	Correct  bool   `json:"correct"`
}

// Report uses all declared cases for accuracy and observed successful answers for coverage.
type Report struct {
	Worker           *WorkerReceipt `json:"worker,omitempty"`
	Unparsed         int            `json:"unparsed"`
	PromptSHA256     string         `json:"prompt_sha256,omitempty"`
	SchemaVersion    int            `json:"schema_version"`
	Task             string         `json:"task"`
	TaskVersion      string         `json:"task_version"`
	ReferenceVersion string         `json:"reference_version"`
	ReferenceSHA256  string         `json:"reference_sha256"`
	Evaluator        string         `json:"evaluator"`
	EvaluatorVersion string         `json:"evaluator_version"`
	Model            string         `json:"model"`
	ModelRevision    string         `json:"model_revision"`
	ArtifactSHA256   string         `json:"artifact_sha256"`
	Total            int            `json:"total"`
	Correct          int            `json:"correct"`
	Observed         int            `json:"observed"`
	Unavailable      int            `json:"unavailable"`
	Failed           int            `json:"failed"`
	Accuracy         float64        `json:"accuracy"`
	Coverage         float64        `json:"coverage"`
	Results          []Result       `json:"results"`
}

// ReferenceHash hashes the ordered identities and references using canonical JSON.
func ReferenceHash(cases []Case) (string, error) {
	type reference struct {
		ID        string `json:"id"`
		Reference string `json:"reference"`
	}
	r := make([]reference, len(cases))
	for i, c := range cases {
		r[i] = reference{c.ID, c.Reference}
	}
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

var rational = regexp.MustCompile(`^[+-]?[0-9]+(?:\.[0-9]+)?(?:/[+-]?[0-9]+)?$`)

// Evaluate reads a bounded strict artifact and verifies its declared reference hash.
// aiperf-exact-match-v1 uses the pinned Python whitespace set;
// exact-match-v1 is Go-trimmed, case-sensitive equality; choice-v1 requires a bare
// A-Z first line; rational-v1 compares finite decimal/fraction values exactly.
func Evaluate(reader io.Reader) (Report, error) { return evaluate(reader, nil) }

type grader func(evaluator, pred, ref string) (bool, bool, error)

func evaluate(reader io.Reader, external grader) (Report, error) {
	b, err := io.ReadAll(io.LimitReader(reader, 16<<20+1))
	if err != nil {
		return Report{}, fmt.Errorf("read artifact: %w", err)
	}
	if len(b) > 16<<20 || !utf8.Valid(b) {
		return Report{}, fmt.Errorf("artifact exceeds budget or invalid UTF-8")
	}
	if err := uniqueJSON(b); err != nil {
		return Report{}, fmt.Errorf("decode artifact: %w", err)
	}
	var a Artifact
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return Report{}, fmt.Errorf("decode artifact: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return Report{}, fmt.Errorf("trailing artifact data")
	}
	if a.SchemaVersion != 1 || a.EvaluatorVersion != "1" || len(a.Cases) == 0 || len(a.Cases) > 100000 {
		return Report{}, fmt.Errorf("invalid schema, evaluator version or case count")
	}
	for _, s := range []string{a.Task, a.TaskVersion, a.ReferenceVersion, a.Model, a.ModelRevision} {
		if s == "" || len(s) > 256 {
			return Report{}, fmt.Errorf("missing or oversized version identity")
		}
	}
	switch a.Evaluator {
	case "aiperf-exact-match-v1", "exact-match-v1", "choice-v1", "rational-v1", "gsm8k-v1", "mmlu-v1", "mmlu-pro-v1", "math-numeric-v1", "aime-numeric-v1", "math-symbolic-v1", "lighteval-expr-v1", "lighteval-latex-v1", "lighteval-gpqa-v1", "code-execution-v1":
	default:
		return Report{}, fmt.Errorf("unsupported evaluator")
	}
	hash, err := ReferenceHash(a.Cases)
	if err != nil || hash != a.ReferenceSHA256 {
		return Report{}, fmt.Errorf("reference hash mismatch")
	}
	artifactHash := sha256.Sum256(b)
	r := Report{PromptSHA256: a.PromptSHA256, SchemaVersion: 1, Task: a.Task, TaskVersion: a.TaskVersion, ReferenceVersion: a.ReferenceVersion, ReferenceSHA256: hash, Evaluator: a.Evaluator, EvaluatorVersion: a.EvaluatorVersion, Model: a.Model, ModelRevision: a.ModelRevision, ArtifactSHA256: hex.EncodeToString(artifactHash[:]), Total: len(a.Cases)}
	seen := map[string]bool{}
	for _, c := range a.Cases {
		if c.ID == "" || len(c.ID) > 256 || seen[c.ID] || len(c.Reference) > 65536 || c.Prediction != nil && len(*c.Prediction) > 65536 {
			return Report{}, fmt.Errorf("invalid case identity or answer size")
		}
		seen[c.ID] = true
		item := Result{ID: c.ID}
		switch c.Outcome {
		case "failure", "timeout", "cancelled":
			item.Status = "failed"
			r.Failed++
		case "success":
			if c.Prediction == nil {
				item.Status = "unavailable"
				r.Unavailable++
			} else {
				r.Observed++
				item.Status = "incorrect"
				pred, ref := strings.TrimSpace(*c.Prediction), strings.TrimSpace(c.Reference)
				switch a.Evaluator {
				case "aiperf-exact-match-v1":
					item.Correct, item.Unparsed = aiperfExactMatch(*c.Prediction, c.Reference)
				case "exact-match-v1":
					item.Unparsed = pred == "" && ref != ""
					item.Correct = pred != "" && pred == ref
				case "choice-v1":
					p := strings.TrimSpace(strings.SplitN(pred, "\n", 2)[0])
					if len(ref) != 1 || ref[0] < 'A' || ref[0] > 'Z' {
						return Report{}, fmt.Errorf("invalid choice reference")
					}
					item.Correct = len(p) == 1 && p == ref
				case "rational-v1":
					if len(ref) > 256 || len(pred) > 256 {
						return Report{}, fmt.Errorf("numeric answer exceeds budget")
					}
					if !rational.MatchString(ref) {
						return Report{}, fmt.Errorf("invalid rational reference")
					}
					if !rational.MatchString(pred) {
						item.Unparsed = true
						break
					}
					x, ok := new(big.Rat).SetString(ref)
					if !ok {
						return Report{}, fmt.Errorf("invalid rational reference")
					}
					y, ok := new(big.Rat).SetString(pred)
					item.Correct = ok && x.Cmp(y) == 0
				default:
					if externalEvaluator(a.Evaluator) {
						if external == nil {
							return Report{}, fmt.Errorf("external evaluator requires pinned confined worker")
						}
						item.Correct, item.Unparsed, err = external(a.Evaluator, pred, ref)
					} else {
						item.Correct, item.Unparsed, err = gradeExtended(a.Evaluator, pred, ref)
					}
					if err != nil {
						return Report{}, err
					}
				}
				if item.Correct {
					item.Status = "correct"
					r.Correct++
				}
			}
		default:
			return Report{}, fmt.Errorf("unknown case outcome")
		}
		if item.Unparsed {
			r.Unparsed++
		}
		r.Results = append(r.Results, item)
	}
	r.Accuracy = float64(r.Correct) / float64(r.Total)
	r.Coverage = float64(r.Observed) / float64(r.Total)
	return r, nil
}
