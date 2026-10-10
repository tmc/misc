package accuracy

import (
	"os/exec"
	"strings"
	"testing"
)

func TestExtendedGraders(t *testing.T) {
	tests := []struct {
		name, evaluator, pred, ref string
		correct, unparsed, bad     bool
	}{
		{"gsm marker", "gsm8k-v1", "working 3. #### 1,200.00", "calculation #### 1200", true, false, false},
		{"gsm fallback", "gsm8k-v1", "12 before, answer 24.0", "#### 24", true, true, false},
		{"gsm absent", "gsm8k-v1", "no number", "#### 24", false, true, false},
		{"gsm tolerance", "gsm8k-v1", "#### 24.0000005", "#### 24", true, false, false},
		{"gsm first marker", "gsm8k-v1", "#### 12 #### 24", "#### 12", true, false, false},
		{"gsm invalid ref", "gsm8k-v1", "#### 12", "12", false, false, true},
		{"mmlu bare first", "mmlu-v1", " B\nanswer is A", "B", true, false, false},
		{"mmlu cot corrected", "mmlu-v1", "A. 0 B. 4 C. 2 D. 6\nanswer is A; answer is (B)", "B", true, false, false},
		{"mmlu colon", "mmlu-v1", "A. 0\nAnswer: C\nAnswer: B", "B", true, true, false},
		{"mmlu first fallback", "mmlu-v1", "A. reasoning B\nC", "A", true, true, false},
		{"pro last fallback", "mmlu-pro-v1", "A. reasoning B\nJ", "J", true, true, false},
		{"pro bare first", "mmlu-pro-v1", "J\nA", "J", true, false, false},
		{"pro unicode word", "mmlu-pro-v1", "éAé", "A", false, true, false},
		{"pro no word match", "mmlu-pro-v1", "FLAG", "A", false, true, false},
		{"out of range ref", "mmlu-v1", "J", "J", false, false, true},
		{"math boxed", "math-numeric-v1", `work \boxed{7} then \boxed{\frac{1}{2}}`, `0.5`, true, false, false},
		{"math phrase fraction", "math-numeric-v1", `the answer is \frac{1}{2}.`, `1/2`, true, true, false},
		{"math decimal", "math-numeric-v1", `the answer is 0.5.`, `1/2`, true, true, false},
		{"math last number", "math-numeric-v1", "12 earlier 24 final", "24", true, true, false},
		{"math no percent variants", "math-numeric-v1", `\boxed{50}`, "0.5", false, false, false},
		{"math no symbolic", "math-numeric-v1", `\boxed{\sqrt{2}}`, `2^{1/2}`, false, false, false},
		{"math exact symbolic spelling", "math-numeric-v1", `\boxed{\sqrt{2}}`, `\sqrt{2}`, true, false, false},
		{"math huge exponent", "math-numeric-v1", `\boxed{1e100000000}`, "1", false, false, false},
		{"math zero denominator", "math-numeric-v1", `\boxed{1/0}`, "1", false, false, false},
		{"aime", "aime-numeric-v1", `\boxed{024}`, "24", true, false, false},
		{"unknown", "unknown", "a", "a", false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			correct, unparsed, err := gradeExtended(tt.evaluator, tt.pred, tt.ref)
			if correct != tt.correct || unparsed != tt.unparsed || (err != nil) != tt.bad {
				t.Fatalf("got correct=%v unparsed=%v err=%v", correct, unparsed, err)
			}
		})
	}
	if _, _, err := gradeExtended("gsm8k-v1", strings.Repeat("x", 65537), "#### 1"); err == nil {
		t.Fatal("accepted oversized answer")
	}
}

func TestBoxedBounds(t *testing.T) {
	if _, ok := boxedAnswer(`\boxed{\frac{1}{2}`); ok {
		t.Fatal("accepted unterminated box")
	}
	if _, ok := boxedAnswer(`\boxed{` + strings.Repeat("{", 129) + "2" + strings.Repeat("}", 130)); ok {
		t.Fatal("accepted excessive nesting")
	}
}

func TestExternalWorkerRefusesHostExecution(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	for _, args := range [][]string{
		{"--source", "/missing", "--runtime", "/missing"},
		{"--allow-external", "--inside", "--source", "/missing", "--runtime", "/missing"},
	} {
		cmd := exec.Command(python, append([]string{"../examples/output/accuracy_external.py"}, args...)...)
		cmd.Stdin = strings.NewReader(`{"schema_version":1,"evaluator":"code-execution-v1","prediction":"untrusted","reference":"{}","dependency_versions":{"lighteval":"0.13.0"}}`)
		output, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(output), "accuracy worker:") {
			t.Fatalf("expected bounded refusal, got %v %s", err, output)
		}
	}
}

func TestExternalRequestValidation(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	valid := `{"schema_version":1,"evaluator":"math-symbolic-v1","prediction":"x","reference":"x","dependency_versions":{"sympy":"1.14.0","latex2sympy2-extended":"1.10.2"}}`
	for _, input := range []string{
		strings.Replace(valid, `"schema_version":1`, `"schema_version":true`, 1),
		strings.Replace(valid, `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1),
		strings.Replace(valid, `"evaluator":"math-symbolic-v1"`, `"evaluator":[]`, 1),
		strings.Replace(valid, `"sympy":"1.14.0",`, "", 1),
		strings.Repeat("x", (256<<10)+1),
	} {
		cmd := exec.Command(python, "-c", `import runpy; m=runpy.run_path("../examples/output/accuracy_external.py"); m["read_request"]()`)
		cmd.Stdin = strings.NewReader(input)
		if err := cmd.Run(); err == nil {
			t.Fatal("accepted invalid worker request")
		}
	}
	cmd := exec.Command(python, "-c", `import runpy; m=runpy.run_path("../examples/output/accuracy_external.py"); m["read_request"]()`)
	cmd.Stdin = strings.NewReader(valid)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("valid request failed: %v %s", err, out)
	}
}
