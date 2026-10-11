package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nativeAdapterFixture(t *testing.T) (NativeTelemetryReplay, NativeTelemetryMapping) {
	t.Helper()
	h := NativeTelemetryManifest{RunID: "capture", SchemaVersion: 2, CollectorVersion: "test", BuildRevision: strings.Repeat("a", 40), BinaryDigest: "sha256:" + strings.Repeat("b", 64), ClockMonotonicDomain: "process_local_go_monotonic", ClockMonotonicEpochID: "epoch", ClockWallDomain: "unix_utc", StartTimeMonotonic: 1000000000, StartTimeUnix: 1000000000, Devices: []NativeTelemetryDevice{{UUID: "gpu", Model: "test", DriverVersion: "test", MIGScope: "disabled"}}}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.Encode(NativeTelemetryRecord{Manifest: &h})
	zero := int64(0)
	for i := 0; i < 3; i++ {
		n := int64(i+1) * 1000000000
		enc.Encode(NativeTelemetryRecord{Sample: &NativeTelemetrySample{Seq: uint64(i + 1), QueryStartNanos: n, QueryEndNanos: n + 1000, Readings: []NativeTelemetryReading{{Name: "total_energy_millijoules", Unit: "millijoules", Backend: "test", DeviceUUID: "gpu", Evidence: "measured", IsInt: true, IntValue: uint64(1<<54) + uint64(i*1000), TimestampMonotonic: n, TimestampUnix: n, HasSourceTimestamp: true, SourceTimestamp: n, SourceTimestampDomain: "unix_utc_ns", HasWallClockUncertainty: true, WallClockUncertaintyNanos: &zero, HasQueryBounds: true, QueryStartNanos: n, QueryEndNanos: n + 1000}}}})
	}
	h.EndTimeMonotonic = 3000001000
	h.EndTimeUnix = 3000001000
	h.IsComplete = true
	h.TotalSamples = 3
	enc.Encode(NativeTelemetryRecord{Manifest: &h})
	raw, err := ReadNativeTelemetry(&b)
	if err != nil {
		t.Fatal(err)
	}
	return raw, NativeTelemetryMapping{SchemaVersion: 1, SourceHash: raw.SourceHash, ProducerRevision: strings.Repeat("a", 40), ProducerDigest: "sha256:" + strings.Repeat("b", 64), CaptureRunID: "capture", BenchmarkRunID: "benchmark", EpochID: "epoch", ValidUntil: h.EndTimeMonotonic, Uncertainty: &zero, Evidence: "synthetic exact paired clocks", EndpointDevices: map[string][]string{"endpoint": {"gpu"}}}
}

func TestNativeAdapterAdmission(t *testing.T) {
	raw, m := nativeAdapterFixture(t)
	opts := EnergyOptions{MaxGap: 1.1}
	w, fs, err := ReplayNativeTelemetry(raw, m, 1000, 3000, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || fs[0].Mean != nil || len(w.Devices) != 1 || *w.Devices[0].EnergyEnd-*w.Devices[0].EnergyStart != 2000 || w.Devices[0].Unavailable != "" {
		t.Fatalf("lost integer or fabricated mean: %+v %+v", w, fs)
	}
	wider, _, err := ReplayNativeTelemetry(raw, m, 1500, 2500, opts)
	if err != nil {
		t.Fatal(err)
	}
	d := wider.Devices[0]
	if d.StartedAt != 1000 || d.FinishedAt != 3000 || d.Unavailable == "" {
		t.Fatalf("counter interval collapsed: %+v", d)
	}
	for _, tt := range []struct {
		name   string
		change func(*NativeTelemetryMapping)
	}{
		{"hash", func(m *NativeTelemetryMapping) { m.SourceHash = "wrong" }},
		{"epoch", func(m *NativeTelemetryMapping) { m.EpochID = "wrong" }},
		{"expired", func(m *NativeTelemetryMapping) { m.ValidUntil = 2000000000 }},
		{"unknown uncertainty", func(m *NativeTelemetryMapping) { m.Uncertainty = nil }},
		{"producer", func(m *NativeTelemetryMapping) { m.ProducerRevision = "other" }},
		{"mapping", func(m *NativeTelemetryMapping) { m.EndpointDevices = nil }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			copy := m
			tt.change(&copy)
			if _, _, err := ReplayNativeTelemetry(raw, copy, 1000, 3000, opts); err == nil {
				t.Fatal("accepted unqualified receipt")
			}
		})
	}
}

func TestNativeAdapterRawNegativeControls(t *testing.T) {
	raw, m := nativeAdapterFixture(t)
	for _, tt := range []struct{ name, old, new string }{
		{"unknown scope", `"mig_scope":"disabled"`, `"mig_scope":"unknown"`},
		{"clock step", `"seq":2`, `"seq":2,"clock_discontinuity":true`},
		{"unavailable", `"int_value":18014398509482984`, `"unavailable_reason":"permission_denied"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := strings.ReplaceAll(string(raw.Raw), tt.old, tt.new)
			if b == string(raw.Raw) {
				t.Fatal("ineffective mutation")
			}
			mut, err := ReadNativeTelemetry(strings.NewReader(b))
			if err != nil {
				return
			}
			proof := m
			proof.SourceHash = mut.SourceHash
			if _, _, err := ReplayNativeTelemetry(mut, proof, 1000, 3000, EnergyOptions{MaxGap: 1.1}); err == nil {
				t.Fatal("accepted unqualified native capture")
			}
		})
	}
}

func TestNativeMappingStrict(t *testing.T) {
	for _, s := range []string{`{"schema_version":1,"schema_version":2}`, `{"extra":true}`, `{} {}`} {
		if _, err := ReadNativeTelemetryMapping(strings.NewReader(s)); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}

func TestNativeAdapterTimestampPrecision(t *testing.T) {
	raw, m := nativeAdapterFixture(t)
	origin := int64(1800000000000000000)
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	for _, r := range raw.Records {
		if r.Manifest != nil {
			r.Manifest.StartTimeUnix += origin
			if r.Manifest.EndTimeUnix != 0 {
				r.Manifest.EndTimeUnix += origin
			}
		}
		if r.Sample != nil {
			for i := range r.Sample.Readings {
				r.Sample.Readings[i].TimestampUnix += origin
				r.Sample.Readings[i].SourceTimestamp += origin
			}
		}
		e.Encode(r)
	}
	shifted, err := ReadNativeTelemetry(&b)
	if err != nil {
		t.Fatal(err)
	}
	m.SourceHash = shifted.SourceHash
	m.UnixOrigin = origin
	if _, _, err = ReplayNativeTelemetry(shifted, m, float64(origin)/1e6+1000, float64(origin)/1e6+3000, EnergyOptions{MaxGap: 1.1}); err == nil {
		t.Fatal("zero precision bound accepted")
	}
	ns := int64(100)
	m.Uncertainty = &ns
	if _, _, err = ReplayNativeTelemetry(shifted, m, float64(origin)/1e6+1000, float64(origin)/1e6+3000, EnergyOptions{MaxGap: 1.1, MaxClockUncertainty: 1}); err == nil {
		t.Fatal("sub-rounding bound accepted")
	}
	ns = int64(1000000)
	m.Uncertainty = &ns
	if _, _, err = ReplayNativeTelemetry(shifted, m, float64(origin)/1e6+1000, float64(origin)/1e6+3000, EnergyOptions{MaxGap: 1.1, MaxClockUncertainty: 1}); err != nil {
		t.Fatal(err)
	}
}

func ExampleReadNativeTelemetryMapping() {
	m, err := ReadNativeTelemetryMapping(strings.NewReader(`{"schema_version":1,"uncertainty_nanos":0}`))
	fmt.Println(m.SchemaVersion, err)
	// Output: 1 <nil>
}

func ExampleReplayNativeTelemetry() {
	// A capture without raw evidence is deliberately refused.
	_, _, err := ReplayNativeTelemetry(NativeTelemetryReplay{}, NativeTelemetryMapping{}, 1000, 2000, EnergyOptions{MaxGap: 1})
	fmt.Println(err != nil)
	// Output: true
}

func TestNativeAdapterUnavailableAndEstimates(t *testing.T) {
	for _, mode := range []string{"unavailable", "query-estimate", "calibration"} {
		t.Run(mode, func(t *testing.T) {
			raw, m := nativeAdapterFixture(t)
			for _, r := range raw.Records {
				if mode == "calibration" && r.Manifest != nil {
					r.Manifest.HasClockCalibration = true
					r.Manifest.ClockMappingValidUntil = m.ValidUntil
					r.Manifest.ClockCalibrationBoundsNanos = 1
				}
				if r.Sample == nil {
					continue
				}
				for i := range r.Sample.Readings {
					v := &r.Sample.Readings[i]
					if mode == "unavailable" && r.Sample.Seq == 2 {
						v.Evidence = "unavailable"
						v.UnavailableReason = "permission_denied"
						v.IsInt = false
						v.IntValue = 0
					}
					if mode == "query-estimate" {
						v.Name = "gpu_utilization_pct"
						v.Unit = "percent"
						v.Value = 50
						v.IsInt = false
						v.IntValue = 0
						v.HasSourceTimestamp = false
						v.SourceTimestamp = 0
						v.SourceTimestampDomain = ""
					}
				}
			}
			var buf bytes.Buffer
			for _, r := range raw.Records {
				json.NewEncoder(&buf).Encode(r)
			}
			parsed, err := ReadNativeTelemetry(&buf)
			if err != nil {
				t.Fatal(err)
			}
			m.SourceHash = parsed.SourceHash
			m.AllowQueryEstimates = true
			w, fs, err := ReplayNativeTelemetry(parsed, m, 1000, 3000, EnergyOptions{MaxGap: 1.1})
			if mode == "calibration" {
				if err == nil {
					t.Fatal("contradictory calibration accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "unavailable" {
				if fs[0].MissingDuration != 2 || w.Devices[0].MissingDuration != 2 {
					t.Fatalf("availability bridged: %+v %+v", fs, w)
				}
			} else {
				if fs[0].Evidence != "query-time-estimate" || w.Devices[0].Unavailable == "" || w.Devices[0].Joules != nil {
					t.Fatalf("estimate promoted to energy: %+v %+v", fs, w)
				}
			}
		})
	}
}

func TestNativeAdapterIntervalsAndMIG(t *testing.T) {
	raw, m := nativeAdapterFixture(t)
	for _, r := range raw.Records {
		if r.Manifest != nil {
			d := &r.Manifest.Devices[0]
			d.MIGScope = "mig_instance"
			d.MIGParentUUID = "parent"
			d.HasMIGInstanceID = true
			d.HasMIGComputeInstanceID = true
		}
		if r.Sample != nil {
			v := &r.Sample.Readings[0]
			v.Name = "gpu_utilization_pct"
			v.Unit = "percent"
			v.IsInt = false
			v.IntValue = 0
			v.Value = float64(r.Sample.Seq) * 10
			v.IntegrationIntervalNanos = 1000000000
		}
	}
	var buf bytes.Buffer
	for _, r := range raw.Records {
		json.NewEncoder(&buf).Encode(r)
	}
	capture, err := ReadNativeTelemetry(&buf)
	if err != nil {
		t.Fatal(err)
	}
	m.SourceHash = capture.SourceHash
	deniedWindow, deniedFields, err := ReplayNativeTelemetry(capture, m, 1000, 3000, EnergyOptions{MaxGap: 1.1})
	if err != nil {
		t.Fatal(err)
	}
	if len(deniedFields) != 1 || deniedFields[0].Unavailable == "" || deniedFields[0].Mean != nil || deniedFields[0].CoveredDuration != 0 || deniedFields[0].MissingDuration != 2 || deniedWindow.Devices[0].Joules != nil {
		t.Fatal("interval without convention numerically admitted", deniedFields)
	}
	m.FieldTiming = map[string]NativeFieldTiming{"gpu_utilization_pct": {Anchor: "end", Evidence: "synthetic interval-average registry"}}
	w, fields, err := ReplayNativeTelemetry(capture, m, 1000, 3000, EnergyOptions{MaxGap: 1.1})
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].Mean == nil || *fields[0].Mean != .25 || fields[0].MissingDuration != 0 || fields[0].Evidence != "measured;sensor-interval-average" {
		t.Fatalf("wrong interval integration: %+v", fields)
	}
	if w.Devices[0].Scope != "mig-instance" || w.Devices[0].ParentUUID != "parent" || w.Devices[0].Unavailable == "" || w.Devices[0].Joules != nil {
		t.Fatalf("MIG attributed energy: %+v", w)
	}
}

func TestNativeIntervalEstimatePolicy(t *testing.T) {
	value := 100.0
	samples := []nativeIntervalObservation{{UUID: "gpu", Start: 1000, End: 2000, QueryEnd: 2000, Reading: TelemetryReading{Name: "power", Unit: "watts", Evidence: "measured", Value: &value}}}
	if _, err := nativeIntervalSummary(samples, 1250, 1750, 1.1, NativeFieldTiming{Anchor: "end", Evidence: "fixture"}); err == nil {
		t.Fatal("partial aggregate promoted to exact")
	}
	f, err := nativeIntervalSummary(samples, 1250, 1750, 1.1, NativeFieldTiming{Anchor: "end", Evidence: "fixture", Aggregation: "piecewise_constant_estimate"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Mean == nil || *f.Mean != 100 || f.CoveredDuration != .5 || !strings.Contains(f.Evidence, "estimate") {
		t.Fatalf("bad estimate: %+v", f)
	}
	samples[0].QueryEnd = 1999
	if _, err = nativeIntervalSummary(samples, 1000, 2000, 1.1, NativeFieldTiming{Anchor: "end"}); err == nil {
		t.Fatal("future sensor accepted")
	}
}

func TestNativeIntervalPowerEnergy(t *testing.T) {
	raw, m := nativeAdapterFixture(t)
	for _, r := range raw.Records {
		if r.Sample == nil {
			continue
		}
		v := &r.Sample.Readings[0]
		v.Name = "power_milliwatts"
		v.Unit = "milliwatts"
		v.IsInt = false
		v.IntValue = 0
		v.Value = 100000
		v.IntegrationIntervalNanos = 1000000000
	}
	var buf bytes.Buffer
	for _, r := range raw.Records {
		json.NewEncoder(&buf).Encode(r)
	}
	capture, err := ReadNativeTelemetry(&buf)
	if err != nil {
		t.Fatal(err)
	}
	m.SourceHash = capture.SourceHash
	m.FieldTiming = map[string]NativeFieldTiming{"power_milliwatts": {Anchor: "end", Evidence: "synthetic average"}}
	w, _, err := ReplayNativeTelemetry(capture, m, 1000, 3000, EnergyOptions{MaxGap: 1.1})
	if err != nil {
		t.Fatal(err)
	}
	d := w.Devices[0]
	if d.Method != "power-integral" || d.Joules == nil || *d.Joules != 200 || d.EnergyStart != nil || d.MissingDuration != 0 {
		t.Fatalf("bad integrated estimate: %+v", d)
	}
}

func ExampleNativeFieldTiming() {
	// Partial averages require an explicit estimate policy; no sensor convention
	// is inferred from an integration duration alone.
	policy := NativeFieldTiming{Anchor: "end", Evidence: "sensor integration registry receipt", Aggregation: "piecewise_constant_estimate"}
	fmt.Println(policy.Anchor, policy.Aggregation)
	// Output: end piecewise_constant_estimate
}

func TestNativeMIGIdentityNegative(t *testing.T) {
	for _, mode := range []string{"self-parent", "tuple-collision"} {
		t.Run(mode, func(t *testing.T) {
			raw, m := nativeAdapterFixture(t)
			for _, r := range raw.Records {
				if r.Manifest == nil {
					continue
				}
				d := &r.Manifest.Devices[0]
				d.MIGScope = "mig_instance"
				d.MIGParentUUID = "parent"
				d.HasMIGInstanceID = true
				d.HasMIGComputeInstanceID = true
				if mode == "self-parent" {
					d.MIGParentUUID = d.UUID
				} else {
					duplicate := *d
					duplicate.UUID = "other-instance"
					r.Manifest.Devices = append(r.Manifest.Devices, duplicate)
				}
			}
			var buf bytes.Buffer
			for _, r := range raw.Records {
				json.NewEncoder(&buf).Encode(r)
			}
			parsed, err := ReadNativeTelemetry(&buf)
			if err != nil {
				t.Fatal(err)
			}
			m.SourceHash = parsed.SourceHash
			if _, _, err := ReplayNativeTelemetry(parsed, m, 1000, 3000, EnergyOptions{MaxGap: 1.1}); err == nil {
				t.Fatal("impossible MIG identity accepted")
			}
		})
	}
}

func TestNativeUnsupportedScopeReceipt(t *testing.T) {
	for _, mode := range []string{"positive", "missing", "unknown", "parent", "ids", "hidden-id", "empty-proof"} {
		t.Run(mode, func(t *testing.T) {
			raw, m := nativeAdapterFixture(t)
			for _, r := range raw.Records {
				if r.Manifest != nil {
					d := &r.Manifest.Devices[0]
					d.UUID = "GPU-physical"
					d.MIGScope = "unsupported"
					switch mode {
					case "unknown":
						d.MIGScope = "unknown"
					case "parent":
						d.MIGParentUUID = "parent"
					case "ids":
						d.HasMIGInstanceID = true
					case "hidden-id":
						d.MIGInstanceID = 7
					}
				}
				if r.Sample != nil {
					r.Sample.Readings[0].DeviceUUID = "GPU-physical"
				}
			}
			var buf bytes.Buffer
			for _, r := range raw.Records {
				json.NewEncoder(&buf).Encode(r)
			}
			capture, err := ReadNativeTelemetry(&buf)
			if err != nil {
				t.Fatal(err)
			}
			m.SourceHash = capture.SourceHash
			m.EndpointDevices = map[string][]string{"endpoint": {"GPU-physical"}}
			m.EntityScopes = map[string]NativeEntityScope{"GPU-physical": {Scope: "whole-device", Evidence: "synthetic physical identity receipt"}}
			if mode == "missing" {
				m.EntityScopes = nil
			}
			if mode == "empty-proof" {
				m.EntityScopes["GPU-physical"] = NativeEntityScope{Scope: "whole-device"}
			}
			w, _, err := ReplayNativeTelemetry(capture, m, 1000, 3000, EnergyOptions{MaxGap: 1.1})
			if mode == "positive" {
				if err != nil {
					t.Fatal(err)
				}
				if w.Devices[0].Scope != "whole-device" {
					t.Fatal(w)
				}
			} else if err == nil {
				t.Fatal("unqualified physical override accepted")
			}
		})
	}
}

func ExampleNativeEntityScope() {
	proof := NativeEntityScope{Scope: "whole-device", Evidence: "physical UUID and unsupported-MIG hardware receipt"}
	fmt.Println(proof.Scope)
	// Output: whole-device
}

func TestNativeIntervalNeighborEvidence(t *testing.T) {
	value := 100.0
	for _, old := range []TelemetryReading{{Name: "power", Unit: "watts", Evidence: "derived", Value: &value}, {Name: "power", Unit: "watts", Evidence: "unavailable", Unavailable: "permission_denied"}} {
		samples := []nativeIntervalObservation{{UUID: "gpu", Start: 0, End: 1000, QueryEnd: 1000, Reading: old}, {UUID: "gpu", Start: 1000, End: 2000, QueryEnd: 2000, Reading: TelemetryReading{Name: "power", Unit: "watts", Evidence: "measured", Value: &value}}}
		f, err := nativeIntervalSummary(samples, 1000, 2000, 1.1, NativeFieldTiming{Anchor: "end"})
		if err != nil {
			t.Fatal(err)
		}
		if f.Evidence != "measured;sensor-interval-average" {
			t.Fatalf("irrelevant old evidence contaminated summary: %+v", f)
		}
	}
}

func TestNativeMixedFieldTimingAdmission(t *testing.T) {
	for _, gaugeClock := range []string{"known", "query-estimate"} {
		t.Run(gaugeClock, func(t *testing.T) {
			raw, m := nativeAdapterFixture(t)
			original := append([]byte(nil), raw.Raw...)
			for _, r := range raw.Records {
				if r.Sample == nil {
					continue
				}
				energy := &r.Sample.Readings[0]
				gauge := *energy
				gauge.Name = "gpu_utilization_pct"
				gauge.Unit = "percent"
				gauge.IntValue = 0
				gauge.IsInt = false
				gauge.Value = 50
				energy.HasSourceTimestamp = false
				energy.SourceTimestamp = 0
				energy.SourceTimestampDomain = ""
				if gaugeClock == "query-estimate" {
					gauge.HasSourceTimestamp = false
					gauge.SourceTimestamp = 0
					gauge.SourceTimestampDomain = ""
				}
				r.Sample.Readings = append(r.Sample.Readings, gauge)
			}
			var buf bytes.Buffer
			for _, r := range raw.Records {
				if err := json.NewEncoder(&buf).Encode(r); err != nil {
					t.Fatal(err)
				}
			}
			capture, err := ReadNativeTelemetry(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			m.SourceHash = capture.SourceHash
			m.AllowQueryEstimates = true
			w, fields, err := ReplayNativeTelemetry(capture, m, 1000, 3000, EnergyOptions{MaxGap: 1.1})
			if err != nil {
				t.Fatal(err)
			}
			var energy, gauge *TelemetryFieldSummary
			for i := range fields {
				switch fields[i].Name {
				case "energy":
					energy = &fields[i]
				case "utilization":
					gauge = &fields[i]
				}
			}
			if energy == nil || energy.Mean != nil || energy.CoveredDuration != 0 || energy.MissingDuration != 2 || energy.Unavailable != "sensor observation time unavailable" {
				t.Fatalf("unknown sensor energy promoted: %+v", fields)
			}
			if gauge == nil || gauge.Mean == nil || *gauge.Mean != .5 || gauge.CoveredDuration < 1.9 {
				t.Fatalf("independent gauge lost: %+v", fields)
			}
			if gaugeClock == "query-estimate" && gauge.Evidence != "query-time-estimate" {
				t.Fatalf("query timing unlabeled: %+v", gauge)
			}
			d := w.Devices[0]
			if d.EnergyStart != nil || d.EnergyEnd != nil || d.Joules != nil || d.Unavailable == "" {
				t.Fatalf("unqualified energy admitted: %+v", d)
			}
			run, _ := energyFixture()
			run.RunID = m.BenchmarkRunID
			run.Records[0].Endpoint = "endpoint"
			if _, err := AnalyzeEnergy(run, w, EnergyOptions{MaxGap: 1.1}); err == nil || !strings.Contains(err.Error(), "energy unavailable") {
				t.Fatalf("energy failure not demonstrated: %v", err)
			}
			if !bytes.Equal(capture.Raw, buf.Bytes()) || capture.Samples[1].Readings[0].IntValue != uint64(1<<54)+1000 || !bytes.Equal(raw.Raw, original) {
				t.Fatal("raw counter evidence changed")
			}
		})
	}
}

// TestNativePhysicalReferenceAdmission consumes the retained physical capture
// when explicitly requested. No sensor timestamp is added to this fixture.
func TestNativePhysicalReferenceAdmission(t *testing.T) {
	dir := os.Getenv("NATIVE_TELEMETRY_REFERENCE_DIR")
	if dir == "" {
		t.Skip("set NATIVE_TELEMETRY_REFERENCE_DIR to retained physical reference receipt")
	}
	data, err := os.ReadFile(filepath.Join(dir, "native.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ReadNativeTelemetry(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if raw.SourceHash != "sha256:efed9b68c50ecb7d45b9de4ffb41e9c83878ce73d58340aa98c59356323dc11a" {
		t.Fatal("physical fixture changed", raw.SourceHash)
	}
	mapping, err := os.Open(filepath.Join(dir, "mapping.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer mapping.Close()
	m, err := ReadNativeTelemetryMapping(mapping)
	if err != nil {
		t.Fatal(err)
	}
	wb, err := os.ReadFile(filepath.Join(dir, "window.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bounds struct {
		Start, End          float64
		MaxGap              float64 `json:"max_gap"`
		MaxClockUncertainty float64 `json:"max_clock_uncertainty"`
	}
	if err = json.Unmarshal(wb, &bounds); err != nil {
		t.Fatal(err)
	}
	w, fields, err := ReplayNativeTelemetry(raw, m, bounds.Start, bounds.End, EnergyOptions{MaxGap: bounds.MaxGap, MaxClockUncertainty: bounds.MaxClockUncertainty})
	if err != nil {
		t.Fatal(err)
	}
	if w.SourceHash != raw.SourceHash || w.RunID != m.BenchmarkRunID {
		t.Fatal("physical identity lost")
	}
	foundGauge, foundEnergy := false, false
	for _, f := range fields {
		if f.Name == "utilization" {
			foundGauge = f.Mean != nil && f.Evidence == "query-time-estimate"
		}
		if f.Name == "energy" {
			foundEnergy = f.Mean == nil && f.CoveredDuration == 0 && f.Unavailable != ""
		}
	}
	if !foundGauge || !foundEnergy {
		t.Fatalf("mixed physical evidence lost: %+v", fields)
	}
	for _, d := range w.Devices {
		if d.EnergyStart != nil || d.EnergyEnd != nil || d.Joules != nil || d.Unavailable == "" {
			t.Fatalf("physical sensor timing fabricated: %+v", d)
		}
	}
	if !bytes.Equal(raw.Raw, data) {
		t.Fatal("physical raw bytes changed")
	}
}

func TestNativeFieldQualificationLocal(t *testing.T) {
	for _, failure := range []string{"derived-energy", "unknown-source-domain", "cached-source"} {
		t.Run(failure, func(t *testing.T) {
			raw, m := nativeAdapterFixture(t)
			for _, r := range raw.Records {
				if r.Sample == nil {
					continue
				}
				gauge := r.Sample.Readings[0]
				gauge.Name = "gpu_utilization_pct"
				gauge.Unit = "percent"
				gauge.IsInt = false
				gauge.IntValue = 0
				gauge.Value = 50
				r.Sample.Readings = append(r.Sample.Readings, gauge)
				switch failure {
				case "derived-energy":
					r.Sample.Readings[0].Evidence = "derived"
				case "unknown-source-domain":
					r.Sample.Readings[0].SourceTimestampDomain = "unmapped_device_clock"
				case "cached-source":
					r.Sample.Readings[0].SourceTimestamp = 1000000000
				}
			}
			var buf bytes.Buffer
			for _, r := range raw.Records {
				if err := json.NewEncoder(&buf).Encode(r); err != nil {
					t.Fatal(err)
				}
			}
			capture, err := ReadNativeTelemetry(&buf)
			if err != nil {
				t.Fatal(err)
			}
			m.SourceHash = capture.SourceHash
			w, fs, err := ReplayNativeTelemetry(capture, m, 1000, 3000, EnergyOptions{MaxGap: 1.1})
			if err != nil {
				t.Fatal(err)
			}
			energyFound, gaugeFound := false, false
			for _, f := range fs {
				if f.Name == "energy" {
					energyFound = f.Unavailable != "" && f.Mean == nil && f.CoveredDuration == 0 && f.MissingDuration == 2
				}
				if f.Name == "utilization" {
					gaugeFound = f.Mean != nil && *f.Mean == .5 && f.CoveredDuration == 2
				}
			}
			if !energyFound || !gaugeFound || w.Devices[0].EnergyStart != nil || w.Devices[0].Joules != nil || w.Devices[0].Unavailable == "" {
				t.Fatal("local failure contaminated neighbor or admitted energy", fs, w)
			}
		})
	}
}
