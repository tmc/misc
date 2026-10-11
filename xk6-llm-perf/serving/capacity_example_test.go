package serving_test

import (
	"context"
	"fmt"
	"github.com/tmc/misc/xk6-llm-perf/serving"
	"time"
)

func ExampleQualifyCapacity() {
	_, err := serving.QualifyCapacity(serving.CapacityEvidence{}, serving.CapacityBounds{})
	fmt.Println(err != nil)
	// Output: true
}
func ExampleProcessCPUPercent() {
	first := serving.ProcessResources{PID: 1, StartTicks: 10, ObservedAt: time.Unix(0, 0), UserSystemTicks: 100}
	last := first
	last.ObservedAt = first.ObservedAt.Add(time.Second)
	last.UserSystemTicks = 125
	percent, err := serving.ProcessCPUPercent(first, last, 100)
	fmt.Println(percent, err)
	// Output: 25 <nil>
}
func ExampleReadProcessResources() {
	_, err := serving.ReadProcessResources(-1)
	fmt.Println(err)
	// Output: process resources: invalid pid
}
func ExampleValidateCapacityJSON() {
	fmt.Println(serving.ValidateCapacityJSON([]byte(`{"a":1,"a":2}`)))
	// Output: capacity: duplicate JSON member
}

func ExampleSampleProcessResources() {
	_, err := serving.SampleProcessResources(context.Background(), 1, time.Second, 1)
	fmt.Println(err)
	// Output: process resources: invalid sampling bounds
}
func ExampleVerifyCapacityProbe() {
	_, err := serving.VerifyCapacityProbe("", "invalid")
	fmt.Println(err != nil)
	// Output: true
}

func ExampleProbeCapacityEvidence() {
	_, err := serving.ProbeCapacityEvidence("", "invalid")
	fmt.Println(err != nil)
	// Output: true
}
func ExampleEvaluateVerifiedProbe() {
	_, err := serving.EvaluateVerifiedProbe("", "invalid", "", serving.CapacityBounds{})
	fmt.Println(err != nil)
	// Output: true
}
func ExampleVerifyPinnedCapacityProbe() {
	_, err := serving.VerifyPinnedCapacityProbe("", "invalid", "invalid")
	fmt.Println(err != nil)
	// Output: true
}
