package llmperf_test

import (
	"fmt"
	llm "github.com/tmc/misc/xk6-llm-perf"
)

func ExampleLLMPerfMetrics_PipelineSnapshot() {
	var metrics llm.LLMPerfMetrics
	snapshot := metrics.PipelineSnapshot()
	fmt.Println(snapshot.Attempted, snapshot.Inflight)
	// Output: 0 0
}
func ExampleLLMPerfMetrics_ResetPipeline() {
	var metrics llm.LLMPerfMetrics
	fmt.Println(metrics.ResetPipeline(), metrics.PipelineSnapshot().Generation)
	// Output: <nil> 1
}
