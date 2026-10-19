package llmperf

import "fmt"

func ExampleAnalyzeSpecialized() {
	reasoning, output := 2, 4
	run := RunSummary{SchemaVersion: 1, StartedAt: 1000, FinishedAt: 2000, Duration: 1, Attempts: 1, Successes: 1, OutputTokens: 4, MissingInputCounts: 1, Records: []Measurement{{SchemaVersion: 1, Phase: "measurement", Recorded: true, Attempted: true, Outcome: "success", DispatchedAt: 1100, FinishedAt: 1200, FullResponseLatency: 100, InputTokens: TokenCount{Source: "unavailable"}, OutputTokens: TokenCount{Value: &output, Source: "reported"}, ReasoningTokens: &reasoning}}}
	result, err := AnalyzeSpecialized(run, SpecializedAggregateOptions{AIPerf: true})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(*result.OverallThinkingEfficiency.Value, result.ReasoningTokens.Samples, result.OutputTokens.Samples)
	// Output: 0.5 1 1
}
