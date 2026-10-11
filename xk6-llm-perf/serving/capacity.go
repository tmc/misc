package serving

import (
	"fmt"
	"math"
)

// CapacityEvidence records a null-server capacity cell at a declared open load.
// Resource peaks must cover the complete measured window. Nil means unavailable.
type CapacityEvidence struct {
	SchemaVersion               int               `json:"schema_version"`
	RunID                       string            `json:"run_id"`
	ServerKind                  string            `json:"server_kind"`
	Parameters                  map[string]string `json:"parameters"`
	Preconditions               []string          `json:"preconditions"`
	DurationSeconds             float64           `json:"duration_seconds"`
	OfferedPerSecond            float64           `json:"offered_per_second"`
	Scheduled                   int64             `json:"scheduled"`
	Attempts                    int64             `json:"attempts"`
	Successes                   int64             `json:"successes"`
	Failures                    int64             `json:"failures"`
	DroppedArrivals             int64             `json:"dropped_arrivals"`
	InflightAtEnd               int64             `json:"inflight_at_end"`
	MaxLagMilliseconds          *float64          `json:"max_lag_milliseconds"`
	PeakCPUPercent              *float64          `json:"peak_cpu_percent"`
	PeakRSSBytes                *float64          `json:"peak_rss_bytes"`
	PeakConnections             *float64          `json:"peak_connections"`
	ConnectionLimit             *float64          `json:"connection_limit"`
	PipelineAttempted           uint64            `json:"pipeline_attempted"`
	PipelineDelivered           uint64            `json:"pipeline_delivered"`
	PipelineCancelled           uint64            `json:"pipeline_cancelled"`
	PipelineInflight            int64             `json:"pipeline_inflight"`
	PipelineMaxWaitMilliseconds *float64          `json:"pipeline_max_wait_milliseconds"`
	ResourceCoverageComplete    bool              `json:"resource_coverage_complete"`
	ParsingNanoseconds          *float64          `json:"parsing_nanoseconds"`
	SerializationNanoseconds    *float64          `json:"serialization_nanoseconds"`
}

// CapacityBounds are declared before running the cell. CPU uses 100 percent per
// logical core; connection headroom is the largest permitted limit fraction.
type CapacityBounds struct {
	MaxLagMilliseconds          float64 `json:"max_lag_milliseconds"`
	MaxCPUPercent               float64 `json:"max_cpu_percent"`
	MaxRSSBytes                 float64 `json:"max_rss_bytes"`
	MaxConnectionFraction       float64 `json:"max_connection_fraction"`
	MaxPipelineWaitMilliseconds float64 `json:"max_pipeline_wait_milliseconds"`
}

// CapacityResult qualifies only this client's declared null-server cell. It
// cannot attribute a real server's latency to a client or server bottleneck.
type CapacityResult struct {
	Status   string           `json:"status"`
	Reasons  []string         `json:"reasons"`
	Evidence CapacityEvidence `json:"evidence"`
	Bounds   CapacityBounds   `json:"bounds"`
}

// QualifyCapacity checks accounting and independently observed resource bounds.
// Real-server cells remain diagnostic even when all client checks pass.
func QualifyCapacity(e CapacityEvidence, b CapacityBounds) (CapacityResult, error) {
	r := CapacityResult{Status: "QUALIFIED", Evidence: e, Bounds: b}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }
	for _, v := range []float64{e.DurationSeconds, e.OfferedPerSecond, b.MaxLagMilliseconds, b.MaxCPUPercent, b.MaxRSSBytes, b.MaxConnectionFraction, b.MaxPipelineWaitMilliseconds} {
		if !finite(v) {
			return r, fmt.Errorf("capacity: invalid numeric evidence or bound")
		}
	}
	if e.SchemaVersion != 1 || e.RunID == "" || e.ServerKind == "" || len(e.Parameters) == 0 || len(e.Preconditions) == 0 || e.DurationSeconds <= 0 || e.OfferedPerSecond <= 0 || b.MaxCPUPercent <= 0 || b.MaxRSSBytes <= 0 || b.MaxConnectionFraction <= 0 || b.MaxConnectionFraction >= 1 {
		return r, fmt.Errorf("capacity: incomplete declaration")
	}
	for _, n := range []int64{e.Scheduled, e.Attempts, e.Successes, e.Failures, e.DroppedArrivals, e.InflightAtEnd, e.PipelineInflight} {
		if n < 0 {
			return r, fmt.Errorf("capacity: negative count")
		}
	}
	if e.Successes > e.Attempts || e.Failures != e.Attempts-e.Successes || e.DroppedArrivals > e.Scheduled || e.Attempts != e.Scheduled-e.DroppedArrivals {
		return r, fmt.Errorf("capacity: contradictory attempt accounting")
	}
	if e.PipelineDelivered > e.PipelineAttempted || e.PipelineCancelled > e.PipelineAttempted-e.PipelineDelivered || uint64(e.PipelineInflight) != e.PipelineAttempted-e.PipelineDelivered-e.PipelineCancelled {
		return r, fmt.Errorf("capacity: contradictory pipeline accounting")
	}
	fail := func(s string) { r.Reasons = append(r.Reasons, s); r.Status = "UNQUALIFIED" }
	if e.Scheduled == 0 || math.Abs(float64(e.Scheduled)-e.DurationSeconds*e.OfferedPerSecond) > 1 {
		fail("scheduled arrivals do not match declared offer")
	}
	if e.Failures != 0 || e.DroppedArrivals != 0 || e.InflightAtEnd != 0 {
		fail("arrival or request population is incomplete")
	}
	if e.PipelineAttempted == 0 || e.PipelineCancelled != 0 || e.PipelineInflight != 0 {
		fail("sample delivery population is incomplete")
	}
	if !e.ResourceCoverageComplete {
		fail("resource observations do not cover the complete window")
	}
	check := func(name string, value *float64, limit float64) error {
		if value == nil {
			fail(name + " is unavailable")
			return nil
		}
		if !finite(*value) {
			return fmt.Errorf("capacity: invalid %s", name)
		}
		if *value > limit {
			fail(name + " exceeds declared bound")
		}
		return nil
	}
	for _, x := range []struct {
		name  string
		value *float64
		limit float64
	}{{"dispatch lag", e.MaxLagMilliseconds, b.MaxLagMilliseconds}, {"CPU", e.PeakCPUPercent, b.MaxCPUPercent}, {"RSS", e.PeakRSSBytes, b.MaxRSSBytes}, {"pipeline wait", e.PipelineMaxWaitMilliseconds, b.MaxPipelineWaitMilliseconds}} {
		if err := check(x.name, x.value, x.limit); err != nil {
			return r, err
		}
	}
	if e.ConnectionLimit == nil || e.PeakConnections == nil {
		fail("connection observations are unavailable")
	} else {
		if !finite(*e.ConnectionLimit) || *e.ConnectionLimit <= 0 {
			return r, fmt.Errorf("capacity: invalid connection limit")
		}
		if err := check("connections", e.PeakConnections, *e.ConnectionLimit*b.MaxConnectionFraction); err != nil {
			return r, err
		}
	}
	for _, x := range []*float64{e.ParsingNanoseconds, e.SerializationNanoseconds} {
		if x != nil && !finite(*x) {
			return r, fmt.Errorf("capacity: invalid overhead observation")
		}
	}
	if r.Status == "QUALIFIED" {
		r.Status = "DIAGNOSTIC"
		r.Reasons = append(r.Reasons, "submitted observations require a measured probe receipt before qualification")
	}
	if e.ServerKind != "null" && e.ServerKind != "fixed-response" {
		r.Status = "DIAGNOSTIC"
		r.Reasons = append(r.Reasons, "real-server evidence cannot isolate client capacity")
	}
	return r, nil
}
