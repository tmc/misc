package llmperf

import "fmt"

func ExampleAnalyzeAdjustedLatencies() {
	report, err := AnalyzeAdjustedLatencies(adjustedFixture())
	if err != nil {
		fmt.Println(err)
		return
	}
	metric := report.Metrics["adj_request_latency"]
	fmt.Println(metric.Count, *metric.Percentiles["p50"].Value, metric.Percentiles["p90"].PositiveInfinity)
	// Output: 3 250 true
}

func ExampleAnalyzeAdjustedLatencies_responsesView() {
	report, err := AnalyzeAdjustedLatencies(adjustedResponsesFixture())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(*report.Metrics["adj_request_latency"].Min.Value, *report.Metrics["adj_time_to_first_token"].Min.Value)
	// Output: 80 20
}
