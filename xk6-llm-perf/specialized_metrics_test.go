package llmperf

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func specializedFixture() Measurement {
	ttft, last := 250.0, 1000.0
	reasoning, cached, write := 2, 0, 3
	return Measurement{Phase: "measurement", Outcome: "success", Attempted: true, Recorded: true, Stream: true, RequestID: "r", TTFT: &ttft, LastContentLatency: &last, InputTokens: count(10, "reported"), OutputTokens: count(4, "reported"), ReasoningTokens: &reasoning, CachedTokens: &cached, CacheWriteTokens: &write}
}

func TestSpecializedMetrics(t *testing.T) {
	m := specializedFixture()
	got := m.SpecializedMetrics()
	if *got.ThinkingEfficiency.Value != .5 || *got.PrefillThroughputPerUser.Value != 40 || *got.CacheReadTokens.Value != 0 || *got.CacheWriteTokens.Value != 3 {
		t.Fatal(got)
	}
	for _, tt := range []struct {
		name   string
		mutate func(*Measurement)
	}{
		{"missing reasoning", func(m *Measurement) { m.ReasoningTokens = nil }},
		{"zero denominator", func(m *Measurement) { m.OutputTokens = count(0, "reported") }},
		{"estimated denominator", func(m *Measurement) { m.OutputTokens.Source = "estimated" }},
		{"negative reasoning", func(m *Measurement) { n := -1; m.ReasoningTokens = &n }},
		{"reasoning exceeds total", func(m *Measurement) { m.OutputTokens = count(1, "reported") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := specializedFixture()
			tt.mutate(&m)
			if m.SpecializedMetrics().ThinkingEfficiency.Value != nil {
				t.Fatal("unexpected value")
			}
		})
	}
	for _, value := range []float64{0, -1, math.Inf(1), math.NaN()} {
		m := specializedFixture()
		m.TTFT = &value
		if m.SpecializedMetrics().PrefillThroughputPerUser.Value != nil {
			t.Fatal(value)
		}
	}
	for _, source := range []string{"reported", "client_declared", "estimated"} {
		m := specializedFixture()
		m.InputTokens = count(0, source)
		got := m.SpecializedMetrics().PrefillThroughputPerUser
		if got.Value == nil || *got.Value != 0 || got.Source != source {
			t.Fatal(got)
		}
	}
}

func TestSpecializedPopulationAndJSON(t *testing.T) {
	for _, phase := range []string{"warmup", ""} {
		m := specializedFixture()
		m.Phase = phase
		got := m.SpecializedMetrics()
		if got.CacheReadTokens.Value != nil || got.ThinkingEfficiency.Value != nil {
			t.Fatal(got)
		}
	}
	m := specializedFixture()
	m.CachedTokens = nil
	data, err := json.Marshal(m.SpecializedMetrics())
	if err != nil || !strings.Contains(string(data), `"value":null`) {
		t.Fatalf("%s %v", data, err)
	}
}

func TestAudioMetrics(t *testing.T) {
	m := specializedFixture()
	m.Protocol = "audio-transcription"
	duration := 10.0
	input := AudioMetricInput{RequestID: "r", Duration: &duration, Unit: "s", Source: "wav header", AssetSHA256: strings.Repeat("a", 64)}
	got := m.AudioMetrics(input)
	if got.RTFX.Value == nil || *got.RTFX.Value != 10 {
		t.Fatal(got)
	}
	for _, tt := range []struct {
		name   string
		mutate func(*AudioMetricInput)
	}{
		{"wrong request", func(i *AudioMetricInput) { i.RequestID = "other" }},
		{"wrong unit", func(i *AudioMetricInput) { i.Unit = "ms" }},
		{"missing source", func(i *AudioMetricInput) { i.Source = "" }},
		{"invalid digest", func(i *AudioMetricInput) { i.AssetSHA256 = "x" }},
		{"missing duration", func(i *AudioMetricInput) { i.Duration = nil }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			i := input
			tt.mutate(&i)
			if m.AudioMetrics(i).RTFX.Value != nil {
				t.Fatal("unexpected value")
			}
		})
	}
	for _, v := range []float64{0, -1, math.Inf(1), math.NaN()} {
		i := input
		i.Duration = &v
		if m.AudioMetrics(i).RTFX.Value != nil {
			t.Fatal(v)
		}
	}
	m.LastContentLatency = nil
	if m.AudioMetrics(input).RTFX.Value != nil {
		t.Fatal("latency missing")
	}
}

func TestSpecializedInvalidPresence(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*Measurement)
	}{
		{"failed", func(m *Measurement) { m.Outcome = "failure" }},
		{"not attempted", func(m *Measurement) { m.Attempted = false }},
		{"not recorded", func(m *Measurement) { m.Recorded = false }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := specializedFixture()
			tt.mutate(&m)
			got := m.SpecializedMetrics()
			if got.ThinkingEfficiency.Value != nil || got.CacheReadTokens.Value != nil || got.PrefillThroughputPerUser.Value != nil {
				t.Fatal(got)
			}
		})
	}
	m := specializedFixture()
	m.Stream = false
	if m.SpecializedMetrics().PrefillThroughputPerUser.Value != nil {
		t.Fatal("unary prefill")
	}
	m = specializedFixture()
	m.InputTokens.Source = "unknown"
	if m.SpecializedMetrics().PrefillThroughputPerUser.Value != nil {
		t.Fatal("unknown source")
	}
	m = specializedFixture()
	n := -1
	m.CachedTokens = &n
	m.CacheWriteTokens = &n
	got := m.SpecializedMetrics()
	if got.CacheReadTokens.Value != nil || got.CacheWriteTokens.Value != nil {
		t.Fatal(got)
	}
	m = specializedFixture()
	zero := 0
	m.ReasoningTokens = &zero
	if value := m.SpecializedMetrics().ThinkingEfficiency.Value; value == nil || *value != 0 {
		t.Fatal("reported reasoning zero lost")
	}
}

func TestAudioInvalidOutputJSON(t *testing.T) {
	m := specializedFixture()
	m.Protocol = "audio-transcription"
	duration := math.NaN()
	input := AudioMetricInput{RequestID: m.RequestID, Duration: &duration, Unit: "s", Source: "asset metadata", AssetSHA256: strings.Repeat("b", 64)}
	got := m.AudioMetrics(input)
	if _, err := json.Marshal(got); err != nil {
		t.Fatal(err)
	}
	if got.RTFX.Value != nil || got.Input.Duration != nil {
		t.Fatal(got)
	}
	duration = 10
	for _, latency := range []float64{0, -1, math.Inf(1), math.NaN()} {
		m.LastContentLatency = &latency
		if m.AudioMetrics(input).RTFX.Value != nil {
			t.Fatal(latency)
		}
	}
}
