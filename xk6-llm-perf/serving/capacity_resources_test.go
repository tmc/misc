package serving

import (
	"math"
	"testing"
	"time"
)

func TestProcessCPUIdentity(t *testing.T) {
	first := ProcessResources{PID: 1, StartTicks: 4, ObservedAt: time.Unix(0, 0), UserSystemTicks: 100}
	last := first
	last.ObservedAt = first.ObservedAt.Add(time.Second)
	last.UserSystemTicks = 150
	if percent, err := ProcessCPUPercent(first, last, 100); err != nil || percent != 50 {
		t.Fatalf("CPU %v %v", percent, err)
	}
	last.StartTicks++
	if _, err := ProcessCPUPercent(first, last, 100); err == nil {
		t.Fatal("PID reuse accepted")
	}
	last.StartTicks = first.StartTicks
	if _, err := ProcessCPUPercent(first, last, math.Inf(1)); err == nil {
		t.Fatal("infinite clock frequency accepted")
	}
}
