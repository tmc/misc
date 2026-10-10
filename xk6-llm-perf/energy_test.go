package llmperf

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func energyFixture() (RunSummary, TelemetryWindow) {
	start, end := uint64(9007199254740993), uint64(9007199254748993)
	zero := 0.0
	r := RunSummary{SchemaVersion: 1, RunID: "fixture", StartedAt: 1000, FinishedAt: 3000, Duration: 2, Attempts: 1, Successes: 1, OutputTokens: 4, Records: []Measurement{{SchemaVersion: 1, Phase: "measurement", Recorded: true, Attempted: true, Endpoint: "http://server/v1", DispatchedAt: 1000, FinishedAt: 2000, Outcome: "success", OutputTokens: count(4, "reported")}}}
	w := TelemetryWindow{SchemaVersion: 1, RunID: "fixture", Collector: "test@1", SourceHash: "sha256:fixture", ClockBasis: "same-host", ClockUncertainty: &zero, EndpointDevices: map[string][]string{"http://server/v1": {"GPU-1"}}, Devices: []DeviceEnergy{{UUID: "GPU-1", Scope: "whole-device", Backend: "fixture", Evidence: "injected", StartedAt: 1000, FinishedAt: 3000, CoveredDuration: 2, MaxGap: 1, EnergyStart: &start, EnergyEnd: &end, Method: "cumulative-counter"}}}
	return r, w
}

func ExampleAnalyzeEnergy() {
	r, w := energyFixture()
	s, err := AnalyzeEnergy(r, w, EnergyOptions{MaxGap: 1})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(s.Joules, *s.JoulesPerOutputToken, *s.OutputTokensPerJoule)
	// Output: 8 2 0.5
}

func ExampleReadTelemetryWindow() {
	_, w := energyFixture()
	data, _ := json.Marshal(w)
	got, err := ReadTelemetryWindow(strings.NewReader(string(data)))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(got.Devices[0].UUID, *got.Devices[0].EnergyStart)
	// Output: GPU-1 9007199254740993
}

func TestEnergyAdmission(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*RunSummary, *TelemetryWindow)
	}{
		{"counter reset", func(_ *RunSummary, w *TelemetryWindow) { v := uint64(0); w.Devices[0].EnergyEnd = &v }},
		{"gap", func(_ *RunSummary, w *TelemetryWindow) { w.Devices[0].MaxGap = 2 }},
		{"coverage", func(_ *RunSummary, w *TelemetryWindow) {
			w.Devices[0].CoveredDuration = 1
			w.Devices[0].MissingDuration = 1
		}},
		{"wider counter interval", func(_ *RunSummary, w *TelemetryWindow) { w.Devices[0].StartedAt = 999 }},
		{"unknown clock", func(_ *RunSummary, w *TelemetryWindow) { w.ClockUncertainty = nil }},
		{"misaligned clock", func(_ *RunSummary, w *TelemetryWindow) { v := 2.0; w.ClockUncertainty = &v }},
		{"identity", func(_ *RunSummary, w *TelemetryWindow) { w.RunID = "other" }},
		{"unmapped device", func(_ *RunSummary, w *TelemetryWindow) { w.EndpointDevices["http://server/v1"] = []string{"missing"} }},
		{"MIG", func(_ *RunSummary, w *TelemetryWindow) { w.Devices[0].ParentUUID = "parent" }},
		{"missing token count", func(r *RunSummary, _ *TelemetryWindow) { r.Records[0].OutputTokens.Value = nil }},
		{"duplicate", func(_ *RunSummary, w *TelemetryWindow) { w.Devices = append(w.Devices, w.Devices[0]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, w := energyFixture()
			tc.change(&r, &w)
			if _, err := AnalyzeEnergy(r, w, EnergyOptions{MaxGap: 1}); err == nil {
				t.Fatal("accepted invalid energy join")
			}
		})
	}
}

func TestEnergyReplayAndZero(t *testing.T) {
	r, w := energyFixture()
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadTelemetryWindow(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	s, err := AnalyzeEnergy(r, got, EnergyOptions{MaxGap: 1})
	if err != nil {
		t.Fatal(err)
	}
	if s.Joules != 8 || *s.JoulesPerOutputToken != 2 {
		t.Fatalf("summary=%+v", s)
	}
	v := 0
	r.Records[0].OutputTokens = count(v, "reported")
	s, err = AnalyzeEnergy(r, got, EnergyOptions{MaxGap: 1})
	if err != nil {
		t.Fatal(err)
	}
	if s.JoulesPerOutputToken != nil || s.OutputTokensPerJoule != nil {
		t.Fatal("zero tokens yielded efficiency")
	}
	for _, input := range []string{string(data) + "{}", strings.Replace(string(data), `"schema_version":1`, `"schema_version":2`, 1), string(data[:len(data)-1])} {
		if _, err := ReadTelemetryWindow(strings.NewReader(input)); err == nil {
			t.Fatal("accepted invalid export")
		}
	}
}
