package accuracy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"unicode/utf8"
)

// GradedTaskSummary describes emitted grades, not a declared problem population.
type GradedTaskSummary struct {
	Total        int     `json:"total"`
	Passed       int     `json:"passed"`
	Unparsed     int     `json:"unparsed"`
	AccuracyRate float64 `json:"accuracy_rate"`
	UnparsedRate float64 `json:"unparsed_rate"`
}

// GradedPhaseSummary follows the pinned AIPerf accuracy accumulator's fields.
// GraderName is the first selected record's label, not proof of uniform graders.
type GradedPhaseSummary struct {
	TotalEvaluated  int                          `json:"total_evaluated"`
	TotalPassed     int                          `json:"total_passed"`
	AccuracyRate    float64                      `json:"accuracy_rate"`
	OverallUnparsed int                          `json:"overall_unparsed"`
	GraderName      string                       `json:"grader_name"`
	PerTask         map[string]GradedTaskSummary `json:"per_task"`
}

// GradedRecordReceipt retains sample identity and grading metadata without answers.
// Missing request IDs use source hash plus physical line ordinal, not problem ID.
type GradedRecordReceipt struct {
	ID            string  `json:"id"`
	IdentityBasis string  `json:"identity_basis"`
	SourceLine    int     `json:"source_line"`
	Phase         string  `json:"benchmark_phase"`
	Task          *string `json:"task"`
	GraderName    string  `json:"grader_name"`
	Passed        bool    `json:"passed"`
	Unparsed      bool    `json:"unparsed"`
	Selected      bool    `json:"selected"`
}

// GradedReport records an explicit graded-response population. Request failure
// and unavailable-request totals are unknown: graded exports cannot supply them.
// Summary is nil when no records match Phase, matching the upstream accumulator.
type GradedReport struct {
	SchemaVersion           int                   `json:"schema_version"`
	SourceFormat            string                `json:"source_format"`
	SourceSHA256            string                `json:"source_sha256"`
	Phase                   string                `json:"phase"`
	RecordCount             int                   `json:"record_count"`
	SelectedCount           int                   `json:"selected_count"`
	ExcludedByPhase         int                   `json:"excluded_by_phase"`
	FailedRequests          *int                  `json:"failed_requests"`
	UnavailableRequests     *int                  `json:"unavailable_requests"`
	RequestPopulationStatus string                `json:"request_population_status"`
	Policy                  string                `json:"policy"`
	GraderNames             []string              `json:"grader_names"`
	Summary                 *GradedPhaseSummary   `json:"summary"`
	Records                 []GradedRecordReceipt `json:"records"`
}

type gradedWireRecord struct {
	RecordType     string   `json:"record_type"`
	SessionNum     *int64   `json:"session_num"`
	ConversationID *string  `json:"conversation_id"`
	RequestID      *string  `json:"x_request_id"`
	WorkerID       string   `json:"worker_id"`
	Phase          string   `json:"benchmark_phase"`
	Timestamp      *int64   `json:"timestamp_ns"`
	Task           *string  `json:"task"`
	GraderName     string   `json:"grader_name"`
	Passed         *bool    `json:"passed"`
	Unparsed       *bool    `json:"unparsed"`
	Confidence     *float64 `json:"confidence"`
	Expected       *string  `json:"expected"`
	Actual         *string  `json:"actual"`
	Explanation    *string  `json:"explanation"`
	ModelOutput    *string  `json:"model_output"`
	ModelThinking  *string  `json:"model_thinking"`
}

// SummarizeGraded imports the pinned AIPerf graded-response JSONL format and
// selects explicit phase warmup, profiling or all. Every record must carry its
// own known phase. Repeated problems are independent samples; duplicate supplied
// request IDs refuse admission. Missing IDs use source-line identity.
//
// Limits are 16 MiB, 100000 records, 1 MiB per line and 64 KiB per answer field.
// No grader is executed. This does not infer grades for absent, failed or
// unavailable requests. A failed request with an emitted grade still counts.
func SummarizeGraded(reader io.Reader, phase string) (GradedReport, error) {
	if phase != "warmup" && phase != "profiling" && phase != "all" {
		return GradedReport{}, fmt.Errorf("accuracy phase must be warmup, profiling or all")
	}
	const maxBytes = 16 << 20
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return GradedReport{}, fmt.Errorf("read graded records: %w", err)
	}
	if len(data) > maxBytes || !utf8.Valid(data) {
		return GradedReport{}, fmt.Errorf("graded records exceed byte budget or invalid UTF-8")
	}
	hash := sha256.Sum256(data)
	report := GradedReport{SchemaVersion: 1, SourceFormat: "aiperf-accuracy-records-f10eb50a", SourceSHA256: hex.EncodeToString(hash[:]), Phase: phase, RequestPopulationStatus: "unknown: graded records omit request errors and requests without emitted grades", Policy: "one emitted grade per response in explicit phase; no problem deduplication; request failures and unavailable population unknown", GraderNames: []string{}, Records: []GradedRecordReceipt{}}
	seen := map[string]bool{}
	graders := map[string]bool{}
	for line, raw := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		if len(raw) > 1<<20 || report.RecordCount == 100000 {
			return GradedReport{}, fmt.Errorf("graded record size or count limit")
		}
		record, err := decodeGradedRecord(raw)
		if err != nil {
			return GradedReport{}, fmt.Errorf("graded record line %d: %w", line+1, err)
		}
		id, basis := report.SourceSHA256+":"+strconv.Itoa(line+1), "source-line"
		if record.RequestID != nil && *record.RequestID != "" {
			id, basis = *record.RequestID, "x_request_id"
		}
		if seen[id] {
			return GradedReport{}, fmt.Errorf("duplicate graded request identity")
		}
		seen[id] = true
		receipt := GradedRecordReceipt{ID: id, IdentityBasis: basis, SourceLine: line + 1, Phase: record.Phase, Task: record.Task, GraderName: record.GraderName, Passed: *record.Passed, Selected: phase == "all" || phase == record.Phase}
		if record.Unparsed != nil {
			receipt.Unparsed = *record.Unparsed
		}
		report.Records = append(report.Records, receipt)
		report.RecordCount++
		if !receipt.Selected {
			report.ExcludedByPhase++
			continue
		}
		report.SelectedCount++
		graders[record.GraderName] = true
		if report.Summary == nil {
			report.Summary = &GradedPhaseSummary{GraderName: record.GraderName, PerTask: map[string]GradedTaskSummary{}}
		}
		summary := report.Summary
		summary.TotalEvaluated++
		if receipt.Passed {
			summary.TotalPassed++
		}
		if receipt.Unparsed {
			summary.OverallUnparsed++
		}
		if record.Task != nil {
			task := summary.PerTask[*record.Task]
			task.Total++
			if receipt.Passed {
				task.Passed++
			}
			if receipt.Unparsed {
				task.Unparsed++
			}
			task.AccuracyRate = float64(task.Passed) / float64(task.Total)
			task.UnparsedRate = float64(task.Unparsed) / float64(task.Total)
			summary.PerTask[*record.Task] = task
		}
	}
	if report.Summary != nil {
		report.Summary.AccuracyRate = float64(report.Summary.TotalPassed) / float64(report.Summary.TotalEvaluated)
	}
	for name := range graders {
		report.GraderNames = append(report.GraderNames, name)
	}
	sort.Strings(report.GraderNames)
	return report, nil
}

func decodeGradedRecord(raw []byte) (gradedWireRecord, error) {
	var record gradedWireRecord
	if err := uniqueJSON(raw); err != nil {
		return record, err
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return record, err
	}
	for name, value := range members {
		switch name {
		case "record_type", "session_num", "conversation_id", "x_request_id", "worker_id", "benchmark_phase", "timestamp_ns", "task", "grader_name", "passed", "unparsed", "confidence", "expected", "actual", "explanation", "model_output", "model_thinking":
		default:
			return record, fmt.Errorf("unknown graded record field %q", name)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) && name != "conversation_id" && name != "x_request_id" && name != "task" && name != "model_thinking" {
			return record, fmt.Errorf("null graded field %s", name)
		}
	}
	if _, present := members["record_type"]; present && bytes.Equal(bytes.TrimSpace(members["record_type"]), []byte(`""`)) {
		return record, fmt.Errorf("invalid graded record type")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return record, err
	}
	if record.RecordType != "" && record.RecordType != "accuracy" || record.SessionNum == nil || *record.SessionNum < 0 || record.Timestamp == nil || *record.Timestamp < 0 || record.Phase != "warmup" && record.Phase != "profiling" || record.Passed == nil || record.Confidence == nil || math.IsNaN(*record.Confidence) || math.IsInf(*record.Confidence, 0) || *record.Confidence < 0 || *record.Confidence > 1 || record.Expected == nil || record.Actual == nil || record.Explanation == nil {
		return record, fmt.Errorf("missing or invalid graded evidence")
	}
	for _, identity := range []string{record.WorkerID, record.GraderName} {
		if identity == "" || len(identity) > 256 {
			return record, fmt.Errorf("invalid graded identity")
		}
	}
	for _, identity := range []*string{record.ConversationID, record.RequestID, record.Task} {
		if identity != nil && len(*identity) > 256 {
			return record, fmt.Errorf("oversized graded identity")
		}
	}
	for _, text := range []*string{record.Expected, record.Actual, record.Explanation, record.ModelOutput, record.ModelThinking} {
		if text != nil && len(*text) > 65536 {
			return record, fmt.Errorf("graded answer exceeds size budget")
		}
	}
	return record, nil
}
