package campaign_test

import (
	"context"
	"fmt"
	"github.com/tmc/misc/xk6-llm-perf/campaign"
)

func ExampleConverged() {
	fmt.Println(campaign.Converged([]float64{10, 10, 10}, "cv", .05, 3))
	fmt.Println(campaign.Converged([]float64{5, 15}, "cv", .05, 2))
	// Output:
	// true
	// false
}
func ExampleConfig_Validate() {
	c := campaign.Config{Binary: "k6", Script: "load.js", Mode: "grid", Objective: campaign.Metric{Name: "llm_requests", Stat: "rate"}, MaxCandidates: 1, MaxAttempts: 3, MinRuns: 3, MaxRuns: 3, Convergence: "cv", Tolerance: .05, Timeout: "1m", MaxDuration: "5m"}
	fmt.Println(c.Validate())
	// Output: <nil>
}
func ExampleRun() {
	_, err := campaign.Run(context.Background(), campaign.Config{}, "unused")
	fmt.Println(err)
	// Output: binary and script are required
}
