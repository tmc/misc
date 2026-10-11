package serving

import "testing"

func validCapacity() (CapacityEvidence, CapacityBounds) {
	lag, cpu, rss, connections, limit, wait := 1., 20., 1024., 2., 100., 0.1
	return CapacityEvidence{SchemaVersion: 1, RunID: "cell", ServerKind: "fixed-response", Parameters: map[string]string{"executor": "constant-arrival-rate"}, Preconditions: []string{"dedicated client"}, DurationSeconds: 10, OfferedPerSecond: 10, Scheduled: 100, Attempts: 100, Successes: 100, PipelineAttempted: 100, PipelineDelivered: 100, ResourceCoverageComplete: true, MaxLagMilliseconds: &lag, PeakCPUPercent: &cpu, PeakRSSBytes: &rss, PeakConnections: &connections, ConnectionLimit: &limit, PipelineMaxWaitMilliseconds: &wait}, CapacityBounds{10, 80, 4096, .8, 1}
}

func TestCapacityControls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*CapacityEvidence)
		want   string
		bad    bool
	}{
		{"valid", func(*CapacityEvidence) {}, "DIAGNOSTIC", false},
		{"missing CPU", func(e *CapacityEvidence) { e.PeakCPUPercent = nil }, "UNQUALIFIED", false},
		{"saturated collector", func(e *CapacityEvidence) { v := 20.; e.PipelineMaxWaitMilliseconds = &v }, "UNQUALIFIED", false},
		{"drop", func(e *CapacityEvidence) { e.DroppedArrivals = 1; e.Attempts--; e.Successes-- }, "UNQUALIFIED", false},
		{"real server", func(e *CapacityEvidence) { e.ServerKind = "ollama" }, "DIAGNOSTIC", false},
		{"false population", func(e *CapacityEvidence) { e.Successes++ }, "", true},
		{"partial resources", func(e *CapacityEvidence) { e.ResourceCoverageComplete = false }, "UNQUALIFIED", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, b := validCapacity()
			tc.change(&e)
			r, err := QualifyCapacity(e, b)
			if (err != nil) != tc.bad || (!tc.bad && r.Status != tc.want) {
				t.Fatalf("result %+v err %v", r, err)
			}
		})
	}
}
