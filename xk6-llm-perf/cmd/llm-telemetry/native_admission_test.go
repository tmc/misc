package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	llmperf "github.com/tmc/misc/xk6-llm-perf"
)

func TestNativeAdmissionCLI(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "cli")
	if b, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	rev := strings.Repeat("a", 40)
	digest := "sha256:" + strings.Repeat("b", 64)
	h := llmperf.NativeTelemetryManifest{SchemaVersion: 2, RunID: "capture", CollectorVersion: "synthetic", BuildRevision: rev, BinaryDigest: digest, ClockMonotonicDomain: "process_local_go_monotonic", ClockMonotonicEpochID: "epoch", ClockWallDomain: "unix_utc", StartTimeMonotonic: 1000000000, StartTimeUnix: 1000000000, Devices: []llmperf.NativeTelemetryDevice{{UUID: "gpu", Model: "synthetic", DriverVersion: "synthetic", MIGScope: "disabled"}}}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.Encode(llmperf.NativeTelemetryRecord{Manifest: &h})
	zero := int64(0)
	for i := 1; i <= 2; i++ {
		n := int64(i) * 1000000000
		enc.Encode(llmperf.NativeTelemetryRecord{Sample: &llmperf.NativeTelemetrySample{Seq: uint64(i), QueryStartNanos: n, QueryEndNanos: n, Readings: []llmperf.NativeTelemetryReading{{Name: "total_energy_millijoules", Unit: "millijoules", Backend: "synthetic", DeviceUUID: "gpu", Evidence: "measured", IsInt: true, IntValue: uint64(1<<54) + uint64(i*1000), TimestampUnix: n, TimestampMonotonic: n, HasSourceTimestamp: true, SourceTimestamp: n, SourceTimestampDomain: "unix_utc_ns", HasQueryBounds: true, QueryStartNanos: n, QueryEndNanos: n, HasWallClockUncertainty: true, WallClockUncertaintyNanos: &zero}}}})
	}
	h.EndTimeMonotonic = 2000000000
	h.EndTimeUnix = 2000000000
	h.IsComplete = true
	h.TotalSamples = 2
	enc.Encode(llmperf.NativeTelemetryRecord{Manifest: &h})
	raw, err := llmperf.ReadNativeTelemetry(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	capturePath := filepath.Join(dir, "capture.jsonl")
	proof := filepath.Join(dir, "mapping.json")
	if err = os.WriteFile(capturePath, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	m := llmperf.NativeTelemetryMapping{SchemaVersion: 1, SourceHash: raw.SourceHash, ProducerRevision: rev, ProducerDigest: digest, CaptureRunID: "capture", BenchmarkRunID: "benchmark", EpochID: "epoch", ValidUntil: 2000000000, Uncertainty: &zero, Evidence: "synthetic fixture", EndpointDevices: map[string][]string{"endpoint": {"gpu"}}}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(proof, b, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-input", capturePath, "-input-format", "nvtelemetry-v2", "-mapping", proof, "-start", "1000", "-end", "2000", "-max-gap", "1.1"}
	var stderr bytes.Buffer
	command := exec.Command(bin, args...)
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		t.Fatalf("admission: %v %s", err, stderr.Bytes())
	}
	var got struct {
		Window llmperf.TelemetryWindow `json:"window"`
	}
	if err = json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Window.RunID != "benchmark" || got.Window.SourceHash != raw.SourceHash || len(got.Window.Devices) != 1 || got.Window.Devices[0].EnergyEnd == nil || *got.Window.Devices[0].EnergyEnd != uint64(1<<54)+2000 {
		t.Fatalf("lost window identity or integer: %s", out)
	}

	t.Run("mixed unknown energy and query gauge", func(t *testing.T) {
		var mixed bytes.Buffer
		encoder := json.NewEncoder(&mixed)
		for _, r := range raw.Records {
			if r.Sample != nil {
				energy := &r.Sample.Readings[0]
				energy.HasSourceTimestamp = false
				energy.SourceTimestamp = 0
				energy.SourceTimestampDomain = ""
				gauge := *energy
				gauge.Name = "gpu_utilization_pct"
				gauge.Unit = "percent"
				gauge.IsInt = false
				gauge.IntValue = 0
				gauge.Value = 50
				r.Sample.Readings = append(r.Sample.Readings, gauge)
			}
			if err := encoder.Encode(r); err != nil {
				t.Fatal(err)
			}
		}
		capture, err := llmperf.ReadNativeTelemetry(bytes.NewReader(mixed.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		proofMapping := m
		proofMapping.SourceHash = capture.SourceHash
		proofMapping.AllowQueryEstimates = true
		b, err := json.Marshal(proofMapping)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(proof, b, 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(capturePath, mixed.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		command := exec.Command(bin, args...)
		command.Stderr = &stderr
		out, err := command.Output()
		if err != nil {
			t.Fatalf("mixed admission: %v %s", err, stderr.Bytes())
		}
		var got struct {
			Window llmperf.TelemetryWindow         `json:"window"`
			Fields []llmperf.TelemetryFieldSummary `json:"fields"`
		}
		if err = json.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}
		gaugeFound, energyFound := false, false
		for _, f := range got.Fields {
			if f.Name == "utilization" {
				gaugeFound = f.Mean != nil && *f.Mean == .5 && f.Evidence == "query-time-estimate"
			}
			if f.Name == "energy" {
				energyFound = f.Mean == nil && f.CoveredDuration == 0 && f.Unavailable != ""
			}
		}
		if !gaugeFound || !energyFound || got.Window.SourceHash != capture.SourceHash {
			t.Fatalf("mixed fields lost: %s", out)
		}
		for _, d := range got.Window.Devices {
			if d.EnergyStart != nil || d.EnergyEnd != nil || d.Joules != nil || d.Unavailable == "" {
				t.Fatalf("energy fabricated: %+v", d)
			}
		}
	})
	for _, invalid := range []string{`{"unknown":true}`, `{"schema_version":1,"uncertainty_nanos":null}`} {
		if err = os.WriteFile(proof, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		if b, err := exec.Command(bin, args...).CombinedOutput(); err == nil {
			t.Fatalf("invalid mapping accepted: %s", b)
		}
	}
}
