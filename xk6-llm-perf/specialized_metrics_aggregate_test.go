package llmperf

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.k6.io/k6/v2/metrics"
)

func specializedAggregateRun(records ...Measurement) RunSummary {
	run := RunSummary{SchemaVersion: 1, RunID: "specialized-fixture", StartedAt: 1000, FinishedAt: 2000, Duration: 1, Attempts: len(records)}
	for i, record := range records {
		record.RequestID = fmt.Sprint(i)
		record.SchemaVersion = 1
		record.Attempted = true
		record.Recorded = true
		if record.Phase == "" {
			record.Phase = "measurement"
		}
		if record.Outcome == "" {
			record.Outcome = "success"
		}
		record.DispatchedAt = 1100
		record.StartedAt = 1100
		record.FinishedAt = 1200
		record.FullResponseLatency = 100
		if record.InputTokens.Value == nil {
			record.InputTokens.Source = "unavailable"
		}
		if record.OutputTokens.Value == nil {
			record.OutputTokens.Source = "unavailable"
		}
		if record.Outcome == "success" {
			run.Successes++
			if record.Good {
				run.GoodRequests++
			}
			if record.InputTokens.Value != nil {
				run.InputTokens += int64(*record.InputTokens.Value)
			} else {
				run.MissingInputCounts++
			}
			if record.OutputTokens.Value != nil {
				run.OutputTokens += int64(*record.OutputTokens.Value)
			} else {
				run.MissingOutputCounts++
			}
			if record.InputTokens.Source == "estimated" {
				run.EstimatedInputCounts++
			}
			if record.OutputTokens.Source == "estimated" {
				run.EstimatedOutputCounts++
			}
		} else {
			run.Failures++
		}
		run.Records = append(run.Records, record)
	}
	return run
}

func specializedAggregateRecord(reasoning, output *int) Measurement {
	m := Measurement{ReasoningTokens: reasoning, OutputTokens: TokenCount{Value: output, Source: "reported"}}
	if output == nil {
		m.OutputTokens.Source = "unavailable"
	}
	return m
}

func TestSpecializedIndependentSumPolicies(t *testing.T) {
	ptr := func(n int) *int { return &n }
	for _, tt := range []struct {
		name     string
		records  []Measurement
		want     *float64
		complete bool
	}{
		{"complete weighted ratio", []Measurement{specializedAggregateRecord(ptr(1), ptr(2)), specializedAggregateRecord(ptr(1), ptr(8))}, floatPtr(.2), true},
		{"missing reasoning independent denominator", []Measurement{specializedAggregateRecord(ptr(2), ptr(4)), specializedAggregateRecord(nil, ptr(6))}, floatPtr(.2), false},
		{"missing output independent numerator", []Measurement{specializedAggregateRecord(ptr(2), ptr(4)), specializedAggregateRecord(ptr(3), nil)}, floatPtr(1.25), false},
		{"zero reasoning", []Measurement{specializedAggregateRecord(ptr(0), ptr(4))}, floatPtr(0), true},
		{"zero denominator", []Measurement{specializedAggregateRecord(ptr(0), ptr(0))}, nil, true},
		{"all reasoning missing", []Measurement{specializedAggregateRecord(nil, ptr(4))}, nil, false},
		{"subset contradiction source formula", []Measurement{specializedAggregateRecord(ptr(8), ptr(4))}, floatPtr(2), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			run := specializedAggregateRun(tt.records...)
			got, err := AnalyzeSpecialized(run, SpecializedAggregateOptions{AIPerf: true})
			if err != nil {
				t.Fatal(err)
			}
			value := got.OverallThinkingEfficiency.Value
			if (value == nil) != (tt.want == nil) || value != nil && math.Abs(*value-*tt.want) > 1e-12 {
				t.Fatalf("got%+v want%v", got, tt.want)
			}
			strict, err := AnalyzeSpecialized(run, SpecializedAggregateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if !tt.complete && strict.OverallThinkingEfficiency.Value != nil {
				t.Fatal("strict policy promoted incomplete/inconsistent operands")
			}
		})
	}
}

func TestSpecializedSourceAndFailurePopulations(t *testing.T) {
	reasoning, output := 2, 4
	for _, source := range []string{"reported", "reported_derived", "client_declared", "estimated", "locally_tokenized"} {
		good := specializedAggregateRecord(&reasoning, &output)
		good.OutputTokens.Source = source
		failure := specializedAggregateRecord(&output, &output)
		failure.Outcome = "timeout"
		run := specializedAggregateRun(good, failure)
		got, err := AnalyzeSpecialized(run, SpecializedAggregateOptions{AIPerf: true})
		if err != nil {
			t.Fatal(err)
		}
		if got.Selected != 1 || got.ExcludedFailures != 1 || got.Attempts != 2 || got.ReasoningTokens.ObservedTotal != 2 || got.OutputTokens.ObservedTotal != 4 || got.OutputTokens.Sources[source] != 1 || *got.OverallThinkingEfficiency.Value != .5 {
			t.Fatal(got)
		}
		strict, err := AnalyzeSpecialized(run, SpecializedAggregateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if source != "reported" && strict.OverallThinkingEfficiency.Value != nil {
			t.Fatal("unreported strict denominator accepted")
		}
	}
}

func TestSpecializedCacheCoverageAndIntegerPrecision(t *testing.T) {
	zero, three, reasoning, output := 0, 3, 0, 4
	first := specializedAggregateRecord(&reasoning, &output)
	first.CachedTokens = &zero
	first.CacheWriteTokens = &three
	second := specializedAggregateRecord(nil, &output)
	run := specializedAggregateRun(first, second)
	got, err := AnalyzeSpecialized(run, SpecializedAggregateOptions{AIPerf: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.CacheReadTokens.Value == nil || *got.CacheReadTokens.Value != 0 || got.CacheReadTokens.Samples != 1 || got.CacheReadTokens.Missing != 1 || got.CacheReadTokens.Complete || *got.CacheWriteTokens.Value != 3 {
		t.Fatal(got)
	}
	strict, err := AnalyzeSpecialized(run, SpecializedAggregateOptions{})
	if err != nil || strict.CacheReadTokens.Value != nil || strict.CacheWriteTokens.Value != nil {
		t.Fatal(strict, err)
	}
	if strconv.IntSize < 64 {
		return
	}
	large := int(int64(1<<53) + 1)
	first.CachedTokens = &large
	got, err = AnalyzeSpecialized(specializedAggregateRun(first), SpecializedAggregateOptions{AIPerf: true})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(got)
	if err != nil || !strings.Contains(string(data), "9007199254740993") {
		t.Fatal(string(data), err)
	}
	overflow := int(int64(1<<63 - 1))
	first.CachedTokens = &overflow
	second.CachedTokens = &overflow
	if _, err := AnalyzeSpecialized(specializedAggregateRun(first, second), SpecializedAggregateOptions{AIPerf: true}); err == nil {
		t.Fatal("accepted cache integer overflow")
	}
}

func TestSpecializedCompleteRunAdmission(t *testing.T) {
	run := specializedAggregateRun()
	got, err := AnalyzeSpecialized(run, SpecializedAggregateOptions{AIPerf: true})
	if err != nil || got.Selected != 0 || got.OverallThinkingEfficiency.Value != nil || got.CacheReadTokens.Value != nil {
		t.Fatal(got, err)
	}
	reasoning, output := 1, 2
	run = specializedAggregateRun(specializedAggregateRecord(&reasoning, &output))
	for _, mutate := range []func(*RunSummary){func(r *RunSummary) { r.Records[0].Phase = "warmup" }, func(r *RunSummary) { r.DroppedRecords = 1; r.Records = nil }, func(r *RunSummary) { r.Records[0].Recorded = false }, func(r *RunSummary) { r.Records[0].CacheWriteTokens = intPtr(-1) }} {
		bad := run
		bad.Records = append([]Measurement(nil), run.Records...)
		mutate(&bad)
		if _, err := AnalyzeSpecialized(bad, SpecializedAggregateOptions{AIPerf: true}); err == nil {
			t.Fatal("accepted incomplete/invalid population")
		}
	}
}

func TestSpecializedActualUsageProducer(t *testing.T) {
	recorder := testMetrics(t, metrics.NewRegistry())
	samples := make(chan metrics.SampleContainer, 100)
	recorder.samples = samples
	var initial, empty, zero Usage
	for _, item := range []struct {
		text string
		dst  *Usage
	}{{`{"completion_tokens":4,"completion_tokens_details":{"reasoning_tokens":2},"prompt_tokens_details":{"cached_tokens":5},"cache_write_tokens":3}`, &initial}, {`{}`, &empty}, {`{"completion_tokens_details":{"reasoning_tokens":0},"prompt_tokens_details":{"cached_tokens":0},"cache_write_tokens":0}`, &zero}} {
		if err := json.Unmarshal([]byte(item.text), item.dst); err != nil {
			t.Fatal(err)
		}
	}
	usage, err := mergeUsage(initial, empty)
	if err != nil {
		t.Fatal(err)
	}
	usage, err = mergeUsage(usage, zero)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Unix(1, 0)
	record := Measurement{RequestID: "wire-fixture", Phase: "measurement", Outcome: "success", Recorded: true, Attempted: true, start: start, requestStart: start, responseEnd: start.Add(100 * time.Millisecond), reported: usage}
	(&Client{metrics: recorder, config: Config{StrictMeasurement: true}}).finishMeasurement(&record)
	if record.ReasoningTokens == nil || *record.ReasoningTokens != 0 || record.CachedTokens == nil || *record.CachedTokens != 0 || record.CacheWriteTokens == nil || *record.CacheWriteTokens != 0 || record.OutputTokens.Value == nil || *record.OutputTokens.Value != 4 || record.OutputTokens.Source != "reported" {
		t.Fatal(record)
	}
	values := metricValues(samples)
	for _, name := range []string{"llm_reasoning_tokens", "llm_cached_tokens", "llm_cache_write_tokens"} {
		if len(values[name]) != 1 || values[name][0] != 0 {
			t.Fatal(values)
		}
	}
	result, err := AnalyzeSpecialized(specializedAggregateRun(record), SpecializedAggregateOptions{AIPerf: true})
	if err != nil || result.OverallThinkingEfficiency.Value == nil || *result.OverallThinkingEfficiency.Value != 0 || result.CacheReadTokens.Value == nil || *result.CacheReadTokens.Value != 0 {
		t.Fatal(result, err)
	}
}
