package llmperf

import (
	"encoding/json"
	"math"
	"testing"
)

func adjustedFixture() RunSummary {
	run := analysisFixture()
	for i := 0; i < 2; i++ {
		first, last := []float64{20, 50}[i], []float64{80, 250}[i]
		run.Records[i].Stream = true
		run.Records[i].TTFT = &first
		run.Records[i].LastContentLatency = &last
		decode := 3
		run.Records[i].DecodeTokens = &decode
		run.Records[i].FirstChunkSource = "assumed_one"
		itl, full := (last-first)/3, (run.Records[i].FullResponseLatency-first)/3
		run.Records[i].ITL = &itl
		run.Records[i].FullResponseITL = &full
	}
	return run
}

func TestAdjustedLatencies(t *testing.T) {
	report, err := AnalyzeAdjustedLatencies(adjustedFixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"request_latency", "time_to_first_token", "inter_token_latency", "full_response_inter_token_latency", "decode_duration", "full_decode_duration"} {
		metric := report.Metrics["adj_"+name]
		if !metric.Available || metric.Count != 3 || metric.SuccessfulObservations != 2 || metric.Failures != 1 || metric.MissingSuccessfulObservations != 0 || !metric.Mean.PositiveInfinity || !metric.Sum.PositiveInfinity || !metric.Max.PositiveInfinity || metric.StdDev != nil || !metric.Percentiles["p90"].PositiveInfinity {
			t.Fatalf("%s = %+v", name, metric)
		}
	}
	metric := report.Metrics["adj_request_latency"]
	if *metric.Min.Value != 80 || *metric.Percentiles["p50"].Value != 250 || *metric.Percentiles["p25"].Value != 80 {
		t.Fatalf("nearest round-even=%+v", metric)
	}
	if metric.Percentiles["p75"].Value != nil || !metric.Percentiles["p75"].PositiveInfinity {
		t.Fatal("infinity boundary lost")
	}
	if report.Metrics["adj_effective_latency"].Available || report.Metrics["adj_effective_latency"].Unavailable == "" {
		t.Fatal("invented credit anchor")
	}
	if _, err := json.Marshal(report); err != nil {
		t.Fatalf("non-finite JSON: %v", err)
	}
}

func TestAdjustedPopulations(t *testing.T) {
	t.Run("missing success", func(t *testing.T) {
		run := adjustedFixture()
		run.Records[1].TTFT = nil
		run.Records[1].ITL = nil
		run.Records[1].FullResponseITL = nil
		report, err := AnalyzeAdjustedLatencies(run)
		if err != nil {
			t.Fatal(err)
		}
		metric := report.Metrics["adj_time_to_first_token"]
		if metric.Count != 2 || metric.SuccessfulObservations != 1 || metric.MissingSuccessfulObservations != 1 || metric.Failures != 1 || *metric.Percentiles["p50"].Value != 20 {
			t.Fatalf("missing population: %+v", metric)
		}
	})
	t.Run("buffered flags", func(t *testing.T) {
		run := adjustedFixture()
		for i := range run.Records {
			run.Records[i].Stream = false
		}
		report, err := AnalyzeAdjustedLatencies(run)
		if err != nil {
			t.Fatal(err)
		}
		if !report.Metrics["adj_request_latency"].Available || report.Metrics["adj_time_to_first_token"].Available || report.Metrics["adj_time_to_first_token"].MissingSuccessfulObservations != 2 {
			t.Fatalf("flags: %+v", report)
		}
	})
	t.Run("all failure kinds", func(t *testing.T) {
		run := adjustedFixture()
		for _, outcome := range []string{"failure", "cancelled"} {
			record := run.Records[2]
			record.RequestID = outcome
			record.Outcome = outcome
			run.Records = append(run.Records, record)
			run.Attempts++
			run.Failures++
		}
		report, err := AnalyzeAdjustedLatencies(run)
		if err != nil {
			t.Fatal(err)
		}
		metric := report.Metrics["adj_request_latency"]
		if metric.Count != 5 || metric.Failures != 3 || !metric.Percentiles["p50"].PositiveInfinity {
			t.Fatalf("failed population %+v", metric)
		}
	})
	t.Run("dropped", func(t *testing.T) {
		run := adjustedFixture()
		run.Records = run.Records[:2]
		run.DroppedRecords = 1
		report, err := AnalyzeAdjustedLatencies(run)
		if err != nil {
			t.Fatal(err)
		}
		if report.Metrics["adj_request_latency"].Available || report.DroppedRecords != 1 || report.Failures != 1 {
			t.Fatal("dropped errors inferred")
		}
	})
	t.Run("no failures", func(t *testing.T) {
		run := adjustedFixture()
		run.Records = run.Records[:2]
		run.Attempts = 2
		run.Failures = 0
		report, err := AnalyzeAdjustedLatencies(run)
		if err != nil {
			t.Fatal(err)
		}
		if report.Metrics["adj_request_latency"].Available || report.Metrics["adj_request_latency"].Unavailable == "" {
			t.Fatal("invented adjusted companion")
		}
	})
	t.Run("no clean observations", func(t *testing.T) {
		report, err := AnalyzeAdjustedLatencies(analysisFixture())
		if err != nil {
			t.Fatal(err)
		}
		if report.Metrics["adj_request_latency"].Available || report.Metrics["adj_request_latency"].MissingSuccessfulObservations != 2 {
			t.Fatal("all-infinity distribution fabricated")
		}
	})
	t.Run("malformed", func(t *testing.T) {
		run := adjustedFixture()
		v := math.NaN()
		run.Records[0].TTFT = &v
		if _, err := AnalyzeAdjustedLatencies(run); err == nil {
			t.Fatal("NaN accepted")
		}
	})
}

func adjustedResponsesFixture() RunSummary {
	run := adjustedFixture()
	for i := 0; i < 2; i++ {
		run.Records[i].Protocol = "openai-responses"
	}
	record := &run.Records[0]
	first, last, viewFirst, viewLast := 5.0, 20.0, 20.0, 80.0
	events := 2
	record.TTFT = &first
	record.LastContentLatency = &last
	ordinary, full := (last-first)/3, (record.FullResponseLatency-first)/3
	record.ITL = &ordinary
	record.FullResponseITL = &full
	record.AIPerfContentPolicy = responsesContentPolicy
	record.AIPerfContentEvents = &events
	record.AIPerfTTFT = &viewFirst
	record.AIPerfLastContentLatency = &viewLast
	return run
}

func TestAdjustedResponsesPinnedView(t *testing.T) {
	report, err := AnalyzeAdjustedLatencies(adjustedResponsesFixture())
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]float64{"request_latency": 80, "time_to_first_token": 20, "decode_duration": 60, "full_decode_duration": 80, "inter_token_latency": 20, "full_response_inter_token_latency": 80.0 / 3} {
		metric := report.Metrics["adj_"+name]
		if !metric.Available || metric.SuccessfulObservations != 1 || metric.MissingSuccessfulObservations != 1 || metric.Count != 2 || metric.Min.Value == nil || math.Abs(*metric.Min.Value-want) > 1e-12 {
			t.Fatalf("%s: %+v want %v", name, metric, want)
		}
	}
	run := adjustedResponsesFixture()
	run.Records[0].AIPerfContentPolicy = ""
	run.Records[0].AIPerfContentEvents = nil
	run.Records[0].AIPerfTTFT = nil
	run.Records[0].AIPerfLastContentLatency = nil
	report, err = AnalyzeAdjustedLatencies(run)
	if err != nil {
		t.Fatal(err)
	}
	for name, metric := range report.Metrics {
		if metric.Available {
			t.Fatalf("old Responses record silently fell back: %s", name)
		}
	}
}

func TestAdjustedResponsesFirstChunkIdentity(t *testing.T) {
	for _, source := range []string{"reported", "client_declared"} {
		t.Run(source, func(t *testing.T) {
			run := adjustedResponsesFixture()
			record := &run.Records[0]
			firstChunk, decode := 2, 2
			record.FirstChunkSource = source
			record.FirstChunkOrigin = source
			record.FirstChunkTokens = &firstChunk
			record.DecodeTokens = &decode
			ordinary, full := (*record.LastContentLatency-*record.TTFT)/2, (record.FullResponseLatency-*record.TTFT)/2
			record.ITL = &ordinary
			record.FullResponseITL = &full
			report, err := AnalyzeAdjustedLatencies(run)
			if err != nil {
				t.Fatal(err)
			}
			if report.Metrics["adj_inter_token_latency"].Available || report.Metrics["adj_full_response_inter_token_latency"].Available {
				t.Fatal("first-chunk identity assumed across different anchors")
			}
			sameFirst := *record.AIPerfTTFT
			record.TTFT = &sameFirst
			ordinary = (*record.LastContentLatency - sameFirst) / 2
			full = (record.FullResponseLatency - sameFirst) / 2
			report, err = AnalyzeAdjustedLatencies(run)
			if err != nil {
				t.Fatal(err)
			}
			if value := report.Metrics["adj_inter_token_latency"].Min.Value; value == nil || *value != 30 {
				t.Fatalf("reported chunk view ITL: %+v", report)
			}
			if value := report.Metrics["adj_full_response_inter_token_latency"].Min.Value; value == nil || *value != 40 {
				t.Fatal("full reported view ITL")
			}
		})
	}
	t.Run("unknown policy", func(t *testing.T) {
		run := adjustedResponsesFixture()
		record := &run.Records[0]
		record.FirstChunkSource = "unavailable"
		record.DecodeTokens = nil
		record.ITL = nil
		record.FullResponseITL = nil
		report, err := AnalyzeAdjustedLatencies(run)
		if err != nil {
			t.Fatal(err)
		}
		if report.Metrics["adj_inter_token_latency"].Available {
			t.Fatal("unknown chunk policy replaced with assumed one")
		}
	})
	t.Run("invalid fallback", func(t *testing.T) {
		run := adjustedResponsesFixture()
		record := &run.Records[0]
		chunk := 0
		record.FirstChunkTokens = &chunk
		record.FirstChunkOrigin = "reported"
		record.FirstChunkUnavailable = "first chunk count outside output count"
		report, err := AnalyzeAdjustedLatencies(run)
		if err != nil {
			t.Fatal(err)
		}
		if value := report.Metrics["adj_inter_token_latency"].Min.Value; value == nil || *value != 20 {
			t.Fatal("qualified assumed-one fallback lost")
		}
	})
}
