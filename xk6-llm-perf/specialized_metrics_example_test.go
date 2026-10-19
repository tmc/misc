package llmperf

import (
	"fmt"
	"strings"
)

func ExampleSpecializedMetric() {
	metric := SpecializedMetric{Unit: "ratio", Unavailable: "missing count"}
	fmt.Println(metric.Value == nil, metric.Unit)
	// Output: true ratio
}

func ExampleSpecializedMetrics() {
	metrics := SpecializedMetrics{}
	fmt.Println(metrics.ThinkingEfficiency.Value == nil)
	// Output: true
}

func ExampleMeasurement_SpecializedMetrics() {
	input, output, reasoning, ttft := 10, 4, 2, 250.0
	m := Measurement{Phase: "measurement", Outcome: "success", Attempted: true, Recorded: true, Stream: true,
		InputTokens: TokenCount{Value: &input, Source: "reported"}, OutputTokens: TokenCount{Value: &output, Source: "reported"}, ReasoningTokens: &reasoning, TTFT: &ttft}
	metrics := m.SpecializedMetrics()
	fmt.Println(*metrics.ThinkingEfficiency.Value, *metrics.PrefillThroughputPerUser.Value)
	// Output: 0.5 40
}

func ExampleAudioMetricInput() {
	duration := 10.0
	input := AudioMetricInput{RequestID: "request-1", Duration: &duration, Unit: "s", Source: "WAV header", AssetSHA256: strings.Repeat("a", 64)}
	fmt.Println(*input.Duration, input.Unit)
	// Output: 10 s
}

func ExampleAudioMetrics() {
	metrics := AudioMetrics{}
	fmt.Println(metrics.RTFX.Value == nil)
	// Output: true
}

func ExampleMeasurement_AudioMetrics() {
	duration, latency := 10.0, 1000.0
	m := Measurement{RequestID: "request-1", Protocol: "audio-transcription", Phase: "measurement", Outcome: "success", Attempted: true, Recorded: true, LastContentLatency: &latency}
	input := AudioMetricInput{RequestID: m.RequestID, Duration: &duration, Unit: "s", Source: "WAV header", AssetSHA256: strings.Repeat("a", 64)}
	fmt.Println(*m.AudioMetrics(input).RTFX.Value)
	// Output: 10
}
