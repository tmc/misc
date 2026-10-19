package accuracy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func gradedFixture(id, phase string, passed bool) map[string]any {
	return map[string]any{"session_num": 0, "conversation_id": "same-problem", "x_request_id": id, "worker_id": "worker", "benchmark_phase": phase, "timestamp_ns": 0, "task": "math", "grader_name": "exact_match", "passed": passed, "unparsed": false, "confidence": 1, "expected": "secret-reference", "actual": "secret-answer", "explanation": "secret-explanation"}
}
func gradedText(records ...map[string]any) string {
	var out strings.Builder
	for _, r := range records {
		data, _ := json.Marshal(r)
		out.Write(data)
		out.WriteByte('\n')
	}
	return out.String()
}

func TestGradedPhaseAndSamplePopulation(t *testing.T) {
	warmup := gradedFixture("warmup", "warmup", false)
	first := gradedFixture("sample-1", "profiling", true)
	second := gradedFixture("errored-request-with-grade", "profiling", false)
	second["unparsed"] = true
	third := gradedFixture("sample-3", "profiling", true)
	third["task"] = nil
	text := gradedText(warmup, first, second, third)
	for _, tt := range []struct {
		phase                   string
		total, passed, unparsed int
	}{{"profiling", 3, 2, 1}, {"warmup", 1, 0, 0}, {"all", 4, 2, 1}} {
		t.Run(tt.phase, func(t *testing.T) {
			r, err := SummarizeGraded(strings.NewReader(text), tt.phase)
			if err != nil {
				t.Fatal(err)
			}
			if r.RecordCount != 4 || r.SelectedCount != tt.total || r.ExcludedByPhase != 4-tt.total || r.Summary.TotalEvaluated != tt.total || r.Summary.TotalPassed != tt.passed || r.Summary.OverallUnparsed != tt.unparsed || r.Summary.AccuracyRate != float64(tt.passed)/float64(tt.total) {
				t.Fatalf("%+v", r)
			}
			if r.FailedRequests != nil || r.UnavailableRequests != nil {
				t.Fatal("invented request population")
			}
			data, _ := json.Marshal(r)
			for _, secret := range []string{"secret-reference", "secret-answer", "secret-explanation"} {
				if strings.Contains(string(data), secret) {
					t.Fatal("answer leaked")
				}
			}
			hash := sha256.Sum256([]byte(text))
			if r.SourceSHA256 != hex.EncodeToString(hash[:]) {
				t.Fatal("source hash lost")
			}
		})
	}
	r, err := SummarizeGraded(strings.NewReader(text), "profiling")
	if err != nil {
		t.Fatal(err)
	}
	task := r.Summary.PerTask["math"]
	if task.Total != 2 || task.Passed != 1 || task.Unparsed != 1 || task.AccuracyRate != .5 || task.UnparsedRate != .5 {
		t.Fatal(task)
	}
	if len(r.Records) != 4 || r.Records[1].ID == r.Records[2].ID {
		t.Fatal("collapsed independent samples")
	}
}

func TestGradedMissingIdentityAndEmptyScope(t *testing.T) {
	record := gradedFixture("", "profiling", false)
	delete(record, "x_request_id")
	delete(record, "unparsed")
	text := gradedText(record, record)
	r, err := SummarizeGraded(strings.NewReader(text), "all")
	if err != nil {
		t.Fatal(err)
	}
	if r.Records[0].ID == r.Records[1].ID || r.Records[0].IdentityBasis != "source-line" || r.Records[1].SourceLine != 2 || r.Summary.TotalEvaluated != 2 {
		t.Fatal(r)
	}
	r, err = SummarizeGraded(strings.NewReader(text), "warmup")
	if err != nil || r.Summary != nil || r.SelectedCount != 0 || r.RecordCount != 2 {
		t.Fatal(r, err)
	}
	r, err = SummarizeGraded(strings.NewReader(""), "profiling")
	if err != nil || r.Summary != nil || r.RecordCount != 0 {
		t.Fatal(r, err)
	}
}

func TestGradedMalformedControls(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing phase", func(r map[string]any) { delete(r, "benchmark_phase") }},
		{"null phase", func(r map[string]any) { r["benchmark_phase"] = nil }},
		{"unknown phase", func(r map[string]any) { r["benchmark_phase"] = "measurement" }},
		{"numeric score", func(r map[string]any) { r["passed"] = 1 }},
		{"missing score", func(r map[string]any) { delete(r, "passed") }},
		{"null score", func(r map[string]any) { r["passed"] = nil }},
		{"null unparsed", func(r map[string]any) { r["unparsed"] = nil }},
		{"missing time", func(r map[string]any) { delete(r, "timestamp_ns") }},
		{"negative time", func(r map[string]any) { r["timestamp_ns"] = -1 }},
		{"fractional time", func(r map[string]any) { r["timestamp_ns"] = 1.5 }},
		{"negative session", func(r map[string]any) { r["session_num"] = -1 }},
		{"missing confidence", func(r map[string]any) { delete(r, "confidence") }},
		{"bad confidence", func(r map[string]any) { r["confidence"] = 2 }},
		{"missing answer", func(r map[string]any) { delete(r, "actual") }},
		{"null answer", func(r map[string]any) { r["actual"] = nil }},
		{"oversized answer", func(r map[string]any) { r["actual"] = strings.Repeat("x", 65537) }},
		{"unknown field", func(r map[string]any) { r["failed"] = true }},
		{"case alias", func(r map[string]any) { r["Passed"] = r["passed"]; delete(r, "passed") }},
		{"wrong record type", func(r map[string]any) { r["record_type"] = "metrics" }},
		{"empty record type", func(r map[string]any) { r["record_type"] = "" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := gradedFixture("id", "profiling", false)
			tt.mutate(r)
			if _, err := SummarizeGraded(strings.NewReader(gradedText(r)), "profiling"); err == nil {
				t.Fatal("accepted malformed record")
			}
		})
	}
	valid := gradedText(gradedFixture("id", "profiling", false))
	for _, text := range []string{valid + valid, strings.Replace(valid, `"passed":false`, `"passed":false,"passed":true`, 1), strings.Replace(valid, `"passed":false`, `"passed":false,"Passed":true`, 1), valid + "{}", valid + "not JSON", strings.Repeat("x", 1<<20+1), strings.Repeat("x", 16<<20+1), "\xff"} {
		if _, err := SummarizeGraded(strings.NewReader(text), "profiling"); err == nil {
			t.Fatal("accepted malformed stream")
		}
	}
	for _, phase := range []string{"", "measurement", "PROFILING"} {
		if _, err := SummarizeGraded(strings.NewReader(valid), phase); err == nil {
			t.Fatal("accepted invalid scope")
		}
	}
}

func ExampleSummarizeGraded() {
	text := `{"session_num":0,"worker_id":"worker","benchmark_phase":"profiling","timestamp_ns":0,"grader_name":"exact_match","passed":false,"confidence":1,"expected":"A","actual":"B","explanation":"different"}`
	report, err := SummarizeGraded(strings.NewReader(text), "profiling")
	fmt.Println(report.Summary.TotalEvaluated, report.Summary.TotalPassed, report.Summary.AccuracyRate, err)
	// Output: 1 0 0 <nil>
}

func TestGradedMixedLabelsAndCorrectUnparsed(t *testing.T) {
	first := gradedFixture("first", "profiling", true)
	first["grader_name"] = "b"
	first["unparsed"] = true
	second := gradedFixture("second", "profiling", true)
	second["grader_name"] = "a"
	report, err := SummarizeGraded(strings.NewReader(gradedText(first, second)), "profiling")
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.TotalPassed != 2 || report.Summary.OverallUnparsed != 1 || report.Summary.GraderName != "b" || strings.Join(report.GraderNames, ",") != "a,b" {
		t.Fatal(report)
	}
}

type gradedErrorReader struct{}

func (gradedErrorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestGradedReadError(t *testing.T) {
	_, err := SummarizeGraded(gradedErrorReader{}, "profiling")
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
}
