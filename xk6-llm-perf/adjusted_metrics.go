package llmperf

import (
	"math"
	"sort"
)

// AdjustedValue represents a finite value or positive infinity without emitting
// invalid JSON numbers. A nil Value and false PositiveInfinity mean unavailable.
type AdjustedValue struct {
	Value            *float64 `json:"value"`
	PositiveInfinity bool     `json:"positive_infinity"`
}

// AdjustedLatencyMetric retains one available success observation per eligible
// record and one positive-infinity observation per failed record in the window.
// Missing successful observations do not become failures. StdDev is unavailable
// because variance of a distribution containing infinity is undefined.
type AdjustedLatencyMetric struct {
	Name                          string                   `json:"name"`
	Unit                          string                   `json:"unit"`
	Available                     bool                     `json:"available"`
	Unavailable                   string                   `json:"unavailable,omitempty"`
	SuccessfulObservations        int                      `json:"successful_observations"`
	MissingSuccessfulObservations int                      `json:"missing_successful_observations"`
	Failures                      int                      `json:"failures"`
	Count                         int                      `json:"count"`
	Sum                           AdjustedValue            `json:"sum"`
	Mean                          AdjustedValue            `json:"mean"`
	Min                           AdjustedValue            `json:"min"`
	Max                           AdjustedValue            `json:"max"`
	StdDev                        *float64                 `json:"std_dev"`
	Percentiles                   map[string]AdjustedValue `json:"percentiles"`
}

// AdjustedLatencyReport describes the failure-inflated client latency subset of
// pinned AIPerf. It does not change ordinary success-only run statistics.
type AdjustedLatencyReport struct {
	Reference                string                           `json:"reference"`
	PercentileMethod         string                           `json:"percentile_method"`
	WindowPolicy             string                           `json:"window_policy"`
	PopulationPolicy         string                           `json:"population_policy"`
	Attempts                 int                              `json:"attempts"`
	Successes                int                              `json:"successes"`
	Failures                 int                              `json:"failures"`
	DroppedRecords           int                              `json:"dropped_records"`
	ExcludedBoundaryRequests int                              `json:"excluded_boundary_requests"`
	InflightAtEnd            int                              `json:"inflight_at_end"`
	Limitations              []string                         `json:"limitations"`
	Metrics                  map[string]AdjustedLatencyMetric `json:"metrics"`
}

// AnalyzeAdjustedLatencies computes the six registry latency families flagged
// PERCENTILE_INCLUDES_FAILED_REQUESTS in AIPerf revision f10eb50a1776775d80d7f13396b0f668f14e2c03.
// It uses the existing retained run window, without choosing a new slice or
// discarding errors. Failure, timeout and cancellation each add one infinity to
// every otherwise observed metric. No adjustment is emitted without failures or
// available successful observations. Dropped records make every family unavailable.
//
// Values use milliseconds (ITL: milliseconds per token). Percentiles select the
// round-to-even nearest index (n-1)*q, matching NumPy method="nearest". This differs
// from both ordinary linear interpolation and nearest-rank percentiles.
// Stream-only families require persisted Stream=true; absent historical stream
// identity is not inferred from populated timing fields. Upstream credit-issued
// effective latency is unavailable because this run schema lacks that anchor.
func AnalyzeAdjustedLatencies(run RunSummary) (AdjustedLatencyReport, error) {
	if err := validateRun(run); err != nil {
		return AdjustedLatencyReport{}, err
	}
	report := AdjustedLatencyReport{
		Reference:        "f10eb50a1776775d80d7f13396b0f668f14e2c03",
		PercentileMethod: "numpy-nearest-round-even-v1",
		WindowPolicy:     run.WindowPolicy,
		PopulationPolicy: "available successful retained observations plus one positive infinity per retained failure/timeout/cancellation in the same run window",
		Attempts:         run.Attempts, Successes: run.Successes, Failures: run.Failures, DroppedRecords: run.DroppedRecords,
		ExcludedBoundaryRequests: run.ExcludedBoundaryRequests, InflightAtEnd: run.InflightAtEnd,
		Limitations: []string{"retained run containment is not automatically an upstream timestamp-mask window", "unattempted invalid local calls are not retained; match the upstream error window separately", "upstream endpoint PRODUCES_TOKENS_ONLY identity is not independently encoded; token-timing fields cover the client's supported text/token subset", "Responses streams require the validated pinned content view; old records without that view remain unavailable", "Responses reported/client-declared first-chunk counts require equal pinned and ordinary first-content anchors; otherwise token-normalized view latency is unavailable"},
		Metrics:     make(map[string]AdjustedLatencyMetric),
	}
	specs := []struct {
		name, unit string
		stream     bool
		value      func(Measurement) *float64
	}{
		{"request_latency", "ms", false, func(m Measurement) *float64 { _, last := adjustedContentTimes(m); return last }},
		{"time_to_first_token", "ms", true, func(m Measurement) *float64 { first, _ := adjustedContentTimes(m); return first }},
		{"inter_token_latency", "ms/token", true, func(m Measurement) *float64 { return adjustedITL(m, false) }},
		{"full_response_inter_token_latency", "ms/token", true, func(m Measurement) *float64 { return adjustedITL(m, true) }},
		{"decode_duration", "ms", true, func(m Measurement) *float64 {
			first, last := adjustedContentTimes(m)
			if last == nil || first == nil {
				return nil
			}
			v := *last - *first
			return &v
		}},
		{"full_decode_duration", "ms", true, func(m Measurement) *float64 {
			first, _ := adjustedContentTimes(m)
			if first == nil {
				return nil
			}
			v := m.FullResponseLatency - *first
			return &v
		}},
	}
	for _, spec := range specs {
		metric := AdjustedLatencyMetric{Name: "adj_" + spec.name, Unit: spec.unit, Failures: run.Failures}
		var values []float64
		for _, record := range run.Records {
			if record.Outcome != "success" {
				continue
			}
			value := spec.value(record)
			if value == nil || spec.stream && !record.Stream {
				metric.MissingSuccessfulObservations++
				continue
			}
			values = append(values, *value)
		}
		metric.SuccessfulObservations = len(values)
		switch {
		case run.DroppedRecords > 0:
			metric.Unavailable = "dropped records leave metric and error populations incomplete"
		case run.Failures == 0:
			metric.Unavailable = "no failed records; pinned upstream emits no adjusted companion"
		case len(values) == 0:
			metric.Unavailable = "no eligible successful observations; pinned upstream emits no adjusted companion"
		default:
			metric.Available = true
			metric.Count = len(values) + run.Failures
			sort.Float64s(values)
			metric.Sum = AdjustedValue{PositiveInfinity: true}
			metric.Mean = metric.Sum
			metric.Max = metric.Sum
			metric.Min = AdjustedValue{Value: &values[0]}
			metric.Percentiles = make(map[string]AdjustedValue)
			for _, p := range []struct {
				name string
				q    float64
			}{{"p1", .01}, {"p5", .05}, {"p10", .10}, {"p25", .25}, {"p50", .50}, {"p75", .75}, {"p90", .90}, {"p95", .95}, {"p99", .99}} {
				index := int(math.RoundToEven(float64(metric.Count-1) * p.q))
				value := AdjustedValue{PositiveInfinity: true}
				if index < len(values) {
					v := values[index]
					value = AdjustedValue{Value: &v}
				}
				metric.Percentiles[p.name] = value
			}
		}
		report.Metrics[metric.Name] = metric
	}
	report.Metrics["adj_effective_latency"] = AdjustedLatencyMetric{Name: "adj_effective_latency", Unit: "ms", Unavailable: "upstream credit_issued_ns anchor is unavailable; admission wait and schedule time are not substitutes"}
	return report, nil
}

// A Responses done event retains upstream timing even when its duplicate text
// is excluded from the ordinary recognized-content/token population.
func adjustedContentTimes(m Measurement) (first, last *float64) {
	if m.Protocol != "openai-responses" || !m.Stream {
		return m.TTFT, m.LastContentLatency
	}
	if m.AIPerfContentPolicy != responsesContentPolicy || m.AIPerfContentEvents == nil || *m.AIPerfContentEvents == 0 {
		return nil, nil
	}
	return m.AIPerfTTFT, m.AIPerfLastContentLatency
}

func adjustedITL(m Measurement, full bool) *float64 {
	if m.Protocol != "openai-responses" || !m.Stream {
		if full {
			return m.FullResponseITL
		}
		return m.ITL
	}
	first, last := adjustedContentTimes(m)
	if first == nil || last == nil || m.MultiChoice || m.OutputTokens.Value == nil || *m.OutputTokens.Value < 2 {
		return nil
	}
	output := *m.OutputTokens.Value
	decode := output - 1
	switch m.FirstChunkSource {
	case "assumed_one":
		if m.FirstChunkTokens != nil {
			if *m.FirstChunkTokens > 0 && *m.FirstChunkTokens < output || m.FirstChunkUnavailable != "first chunk count outside output count" || m.FirstChunkOrigin != "reported" && m.FirstChunkOrigin != "client_declared" {
				return nil
			}
		} else if m.FirstChunkUnavailable != "" || m.FirstChunkOrigin != "" {
			return nil
		}
	case "reported", "client_declared":
		if m.TTFT == nil || *m.TTFT != *first || m.FirstChunkTokens == nil || *m.FirstChunkTokens <= 0 || *m.FirstChunkTokens >= output || m.FirstChunkUnavailable != "" || m.FirstChunkOrigin != "" && m.FirstChunkOrigin != m.FirstChunkSource {
			return nil
		}
		decode = output - *m.FirstChunkTokens
	default:
		return nil
	}
	if decode <= 0 {
		return nil
	}
	end := *last
	if full {
		end = m.FullResponseLatency
	}
	if end < *first {
		return nil
	}
	value := (end - *first) / float64(decode)
	return &value
}
