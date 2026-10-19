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
