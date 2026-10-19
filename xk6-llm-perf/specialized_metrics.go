package llmperf

import (
	"encoding/hex"
	"math"
)

// SpecializedMetric preserves a derived value's unit, source and absence.
// A nil Value is unavailable; a non-nil zero is an observation.
type SpecializedMetric struct {
	Value       *float64 `json:"value"`
	Unit        string   `json:"unit"`
	Source      string   `json:"source,omitempty"`
	Unavailable string   `json:"unavailable,omitempty"`
}

// SpecializedMetrics contains per-request descriptive metrics, not run totals.
type SpecializedMetrics struct {
	ThinkingEfficiency       SpecializedMetric `json:"thinking_efficiency"`
	PrefillThroughputPerUser SpecializedMetric `json:"prefill_throughput_per_user"`
	CacheReadTokens          SpecializedMetric `json:"usage_prompt_cache_read_tokens"`
	CacheWriteTokens         SpecializedMetric `json:"usage_prompt_cache_write_tokens"`
}

func specializedValue(unit, source string, value float64) SpecializedMetric {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return SpecializedMetric{Unit: unit, Unavailable: "invalid derived value"}
	}
	return SpecializedMetric{Value: &value, Unit: unit, Source: source}
}

func specializedPopulation(m Measurement) bool {
	return m.Phase == "measurement" && m.Outcome == "success" && m.Attempted && m.Recorded
}

func specializedCount(c TokenCount) bool {
	return c.Value != nil && *c.Value >= 0 && (c.Source == "reported" || c.Source == "client_declared" || c.Source == "estimated")
}

// SpecializedMetrics derives metrics for a recorded successful measurement.
// Reasoning and cache counts retain their reported meaning. Prefill throughput
// is an input-count/TTFT proxy, not an observation of server execution.
// The caller must validate the complete record, for example with ReadRun;
// this method checks its operands and population, not all record invariants.
func (m Measurement) SpecializedMetrics() SpecializedMetrics {
	result := SpecializedMetrics{
		ThinkingEfficiency:       SpecializedMetric{Unit: "ratio", Unavailable: "missing or invalid reported reasoning/output counts"},
		PrefillThroughputPerUser: SpecializedMetric{Unit: "tokens/s/user", Unavailable: "missing or invalid streaming input count or TTFT"},
		CacheReadTokens:          SpecializedMetric{Unit: "tokens", Unavailable: "missing or invalid reported cache-read count"},
		CacheWriteTokens:         SpecializedMetric{Unit: "tokens", Unavailable: "missing or invalid reported cache-write count"},
	}
	if !specializedPopulation(m) {
		result.ThinkingEfficiency.Unavailable = "outside successful recorded measurement population"
		result.PrefillThroughputPerUser.Unavailable = result.ThinkingEfficiency.Unavailable
		result.CacheReadTokens.Unavailable = result.ThinkingEfficiency.Unavailable
		result.CacheWriteTokens.Unavailable = result.ThinkingEfficiency.Unavailable
		return result
	}
	// Reported reasoning is a subset of reported output, not extra output.
	if m.ReasoningTokens != nil && *m.ReasoningTokens >= 0 && specializedCount(m.OutputTokens) && m.OutputTokens.Source == "reported" && *m.OutputTokens.Value > 0 && *m.ReasoningTokens <= *m.OutputTokens.Value {
		result.ThinkingEfficiency = specializedValue("ratio", "reported", float64(*m.ReasoningTokens)/float64(*m.OutputTokens.Value))
	}
	if m.Stream && specializedCount(m.InputTokens) && m.TTFT != nil && *m.TTFT > 0 && !math.IsInf(*m.TTFT, 0) && !math.IsNaN(*m.TTFT) {
		result.PrefillThroughputPerUser = specializedValue("tokens/s/user", m.InputTokens.Source, float64(*m.InputTokens.Value)/(*m.TTFT/1000))
	}
	if m.CachedTokens != nil && *m.CachedTokens >= 0 {
		result.CacheReadTokens = specializedValue("tokens", "reported", float64(*m.CachedTokens))
	}
	if m.CacheWriteTokens != nil && *m.CacheWriteTokens >= 0 {
		result.CacheWriteTokens = specializedValue("tokens", "reported", float64(*m.CacheWriteTokens))
	}
	return result
}

// AudioMetricInput declares input duration with request and source-asset lineage.
// Unit must be "s", Source must describe the duration producer, and AssetSHA256
// must be lowercase hex. These declarations do not verify the asset's contents.
type AudioMetricInput struct {
	RequestID   string   `json:"request_id"`
	Duration    *float64 `json:"duration"`
	Unit        string   `json:"unit"`
	Source      string   `json:"source"`
	AssetSHA256 string   `json:"asset_sha256"`
}

// AudioMetrics contains descriptive ASR duration and inverse real-time factor.
type AudioMetrics struct {
	Duration SpecializedMetric `json:"audio_duration"`
	RTFX     SpecializedMetric `json:"rtfx"`
	Input    AudioMetricInput  `json:"input"`
}

// AudioMetrics derives RTFx from an explicitly supplied input duration and
// last-content latency, excluding usage trailers. It requires an audio-
// transcription measurement and does not establish media or hardware parity.
// As with SpecializedMetrics, the caller validates the complete record first.
func (m Measurement) AudioMetrics(input AudioMetricInput) AudioMetrics {
	safeInput := input
	if input.Duration != nil && !math.IsNaN(*input.Duration) && !math.IsInf(*input.Duration, 0) {
		duration := *input.Duration
		safeInput.Duration = &duration
	} else {
		safeInput.Duration = nil
	}
	result := AudioMetrics{Input: safeInput, Duration: SpecializedMetric{Unit: "s", Unavailable: "invalid duration or source lineage"}, RTFX: SpecializedMetric{Unit: "ratio", Unavailable: "invalid duration, population or last-content latency"}}
	if len(input.AssetSHA256) != 64 {
		return result
	}
	digest, err := hex.DecodeString(input.AssetSHA256)
	if !specializedPopulation(m) || m.Protocol != "audio-transcription" || m.RequestID == "" || input.RequestID != m.RequestID || input.Unit != "s" || input.Source == "" || err != nil || len(digest) != 32 || hex.EncodeToString(digest) != input.AssetSHA256 || input.Duration == nil || *input.Duration <= 0 || math.IsNaN(*input.Duration) || math.IsInf(*input.Duration, 0) {
		return result
	}
	result.Duration = specializedValue("s", input.Source, *input.Duration)
	if m.LastContentLatency != nil && *m.LastContentLatency > 0 && !math.IsNaN(*m.LastContentLatency) && !math.IsInf(*m.LastContentLatency, 0) {
		result.RTFX = specializedValue("ratio", input.Source, *input.Duration/(*m.LastContentLatency/1000))
	}
	return result
}
