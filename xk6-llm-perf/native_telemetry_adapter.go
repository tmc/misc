package llmperf

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

// NativeTelemetryMapping is an explicit, hash-bound admission receipt. UnixOrigin
// maps producer monotonic zero to Unix nanoseconds. Validity is in the producer
// process epoch. This receipt is an assertion supplied by the caller, not a
// calibration measurement performed by the consumer.
type NativeTelemetryMapping struct {
	SchemaVersion       int                          `json:"schema_version"`
	SourceHash          string                       `json:"source_hash"`
	ProducerRevision    string                       `json:"producer_revision"`
	ProducerDigest      string                       `json:"producer_digest"`
	CaptureRunID        string                       `json:"capture_run_id"`
	BenchmarkRunID      string                       `json:"benchmark_run_id"`
	EpochID             string                       `json:"epoch_id"`
	UnixOrigin          int64                        `json:"unix_origin_nanos"`
	ValidFrom           int64                        `json:"valid_from_nanos"`
	ValidUntil          int64                        `json:"valid_until_nanos"`
	Uncertainty         *int64                       `json:"uncertainty_nanos"`
	Evidence            string                       `json:"evidence"`
	EndpointDevices     map[string][]string          `json:"endpoint_devices"`
	EntityScopes        map[string]NativeEntityScope `json:"entity_scopes,omitempty"`
	FieldTiming         map[string]NativeFieldTiming `json:"field_timing,omitempty"`
	AllowQueryEstimates bool                         `json:"allow_query_estimates"`
}

// NativeEntityScope records caller-provided physical identity evidence for a
// device whose producer reports unsupported MIG discovery, not disabled MIG.
type NativeEntityScope struct {
	Scope    string `json:"scope"`
	Evidence string `json:"evidence"`
}

// NativeFieldTiming declares an externally evidenced sensor integration convention.
// End anchoring is supported only for average gauges, never integer counters.
type NativeFieldTiming struct {
	Anchor      string `json:"anchor"`
	Aggregation string `json:"aggregation,omitempty"`
	Evidence    string `json:"evidence"`
}

// ReadNativeTelemetryMapping decodes a bounded admission receipt without unknown
// fields, duplicate keys, null uncertainty or trailing input.
func ReadNativeTelemetryMapping(r io.Reader) (NativeTelemetryMapping, error) {
	var m NativeTelemetryMapping
	b, err := io.ReadAll(io.LimitReader(r, (1<<20)+1))
	if err != nil {
		return m, err
	}
	if len(b) > 1<<20 {
		return m, fmt.Errorf("native mapping exceeds 1 MiB")
	}
	if err = nativeTelemetryJSON(b); err != nil {
		return m, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&m); err != nil {
		return m, err
	}
	if m.SchemaVersion != 1 || m.Uncertainty == nil || *m.Uncertainty < 0 {
		return m, fmt.Errorf("native mapping schema or uncertainty unavailable")
	}
	return m, nil
}

// ReplayNativeTelemetry admits only complete schema2 captures bound to an
// explicit process-epoch receipt. Each metric retains its own sensor timestamp.
// Query-time estimates require opt-in and never supply energy. Interval averages
// require an explicit end-anchor policy. MIG gauges never supply energy. Wider cumulative
// counter boundaries are retained and remain inadmissible to AnalyzeEnergy.
func ReplayNativeTelemetry(raw NativeTelemetryReplay, m NativeTelemetryMapping, start, end float64, opts EnergyOptions) (TelemetryWindow, []TelemetryFieldSummary, error) {
	var w TelemetryWindow
	var fields []TelemetryFieldSummary
	// Re-read original bytes: public decoded fields cannot override raw evidence.
	parsed, err := ReadNativeTelemetry(bytes.NewReader(raw.Raw))
	if err != nil {
		return w, nil, err
	}
	raw = parsed
	h := raw.Header
	if h.SchemaVersion != 2 || raw.Final == nil || !raw.Final.IsComplete || raw.Final.LostRecords != 0 || raw.Final.MissedTicks != 0 {
		return w, nil, fmt.Errorf("native admission requires complete loss-free schema2")
	}
	if m.SchemaVersion != 1 || m.SourceHash != raw.SourceHash || m.CaptureRunID != h.RunID || m.BenchmarkRunID == "" || m.EpochID == "" || m.EpochID != h.ClockMonotonicEpochID || h.ClockMonotonicDomain != "process_local_go_monotonic" || h.ClockWallDomain != "unix_utc" || !nativeAdmissionRevision(m.ProducerRevision) || m.ProducerRevision != h.BuildRevision || !nativeAdmissionDigest(m.ProducerDigest) || m.ProducerDigest != h.BinaryDigest || h.BuildModified || m.Evidence == "" || m.Uncertainty == nil || *m.Uncertainty < 0 || m.ValidFrom > h.StartTimeMonotonic || m.ValidUntil < raw.Final.EndTimeMonotonic {
		return w, nil, fmt.Errorf("native mapping missing, expired or contradictory")
	}
	if h.HasClockCalibration && (m.ValidFrom < h.ClockMappingValidFrom || m.ValidUntil > h.ClockMappingValidUntil || *m.Uncertainty < h.ClockCalibrationBoundsNanos) {
		return w, nil, fmt.Errorf("native mapping contradicts producer calibration")
	}
	uncertainty := float64(*m.Uncertainty) / 1e6
	for _, n := range []int64{h.StartTimeUnix, raw.Final.EndTimeUnix, m.UnixOrigin} {
		if n > 1<<53 || n < -(1<<53) {
			x := float64(n) / 1e6
			rounding := 2 * (math.Nextafter(x, math.Inf(1)) - x)
			if uncertainty < rounding {
				return w, nil, fmt.Errorf("native timestamp precision exceeds uncertainty bound")
			}
		}
	}
	if !finite(start) || !finite(end) || end <= start || !finite(opts.MaxClockUncertainty) || opts.MaxClockUncertainty < uncertainty || !finite(opts.MaxGap) || opts.MaxGap <= 0 {
		return w, nil, fmt.Errorf("native alignment outside admission bounds")
	}
	wall := func(n int64) float64 { return float64(m.UnixOrigin)/1e6 + float64(n)/1e6 }
	if math.Abs(wall(h.StartTimeMonotonic)-float64(h.StartTimeUnix)/1e6) > uncertainty || math.Abs(wall(raw.Final.EndTimeMonotonic)-float64(raw.Final.EndTimeUnix)/1e6) > uncertainty || start < wall(h.StartTimeMonotonic) || end > wall(raw.Final.EndTimeMonotonic) {
		return w, nil, fmt.Errorf("native clock mapping contradicts capture or window")
	}
	manifest := TelemetryManifest{SchemaVersion: 1, RunID: m.BenchmarkRunID, Collector: h.CollectorVersion, DefinitionVersion: "llm-telemetry-v1", ClockBasis: "qualified-native-process-epoch", ClockUncertainty: &uncertainty, EndpointDevices: m.EndpointDevices}
	deviceScopes := map[string]string{}
	migIdentities := map[string]bool{}
	for _, d := range h.Devices {
		scope := "whole-device"
		proof := m.EntityScopes[d.UUID]
		unsupportedQualified := d.MIGScope == "unsupported" && strings.HasPrefix(d.UUID, "GPU-") && proof.Scope == "whole-device" && proof.Evidence != "" && !d.MIGEnabled && d.MIGParentUUID == "" && !d.HasMIGInstanceID && !d.HasMIGComputeInstanceID && d.MIGInstanceID == 0 && d.MIGComputeInstanceID == 0

		if d.MIGScope == "mig_instance" && d.MIGParentUUID != "" && d.HasMIGInstanceID && d.HasMIGComputeInstanceID && d.MIGInstanceID >= 0 && d.MIGComputeInstanceID >= 0 {
			scope = "mig-instance"
			if d.MIGParentUUID == d.UUID {
				return w, nil, fmt.Errorf("native MIG self parent")
			}
			identity := fmt.Sprintf("%s/%d/%d", d.MIGParentUUID, d.MIGInstanceID, d.MIGComputeInstanceID)
			if migIdentities[identity] {
				return w, nil, fmt.Errorf("native duplicate MIG entity identity")
			}
			migIdentities[identity] = true
		} else if !unsupportedQualified && (d.MIGScope != "disabled" || d.MIGEnabled || d.MIGParentUUID != "" || d.HasMIGInstanceID || d.HasMIGComputeInstanceID || d.MIGInstanceID != 0 || d.MIGComputeInstanceID != 0) {
			return w, nil, fmt.Errorf("native device or MIG scope unqualified")
		}
		if d.Model == "" || d.DriverVersion == "" {
			return w, nil, fmt.Errorf("native device metadata unavailable")
		}
		deviceScopes[d.UUID] = scope
		manifest.Devices = append(manifest.Devices, TelemetryDevice{UUID: d.UUID, ParentUUID: d.MIGParentUUID, Scope: scope, Model: d.Model, Driver: d.DriverVersion, Backend: "native"})
	}

	combinedUncertainty := uncertainty
	streams := map[string][]TelemetryRecord{}
	defs := map[string]TelemetryReading{}
	backends := map[string]string{}
	intervals := map[string][]nativeIntervalObservation{}
	intervalPolicies := map[string]NativeFieldTiming{}
	intervalEnergies := map[string]DeviceEnergy{}
	unqualified := map[string]TelemetryFieldSummary{}
	energyReasons := map[string]string{}
	qualifyFailure := func(uuid, name, unit, evidence, reason string) {
		key := uuid + "/" + name
		if _, exists := unqualified[key]; !exists {
			unqualified[key] = TelemetryFieldSummary{UUID: uuid, Name: name, Unit: unit, Evidence: evidence, Unavailable: reason, MissingDuration: (end - start) / 1000}
		}
		if name == "energy" {
			energyReasons[uuid] = reason
		}
	}
	for i, s := range raw.Samples {
		if s.ClockDiscontinuity || (i > 0 && s.Seq != raw.Samples[i-1].Seq+1) {
			return w, nil, fmt.Errorf("native clock step or sequence gap")
		}
	readings:
		for _, v := range s.Readings {
			if v.TimestampUnix != 0 && math.Abs(wall(v.TimestampMonotonic)-float64(v.TimestampUnix)/1e6) > uncertainty {
				return w, nil, fmt.Errorf("native per-reading wall clock step")
			}
			name, unit, factor := nativeAdapterMetric(v.Name)
			if name == "" || (name == "energy" && deviceScopes[v.DeviceUUID] != "whole-device") {
				continue
			}
			key := v.DeviceUUID + "/" + name
			if _, refused := unqualified[key]; refused {
				continue
			}
			refuse := func(reason string) { qualifyFailure(v.DeviceUUID, name, unit, v.Evidence, reason) }
			if len(v.Source) > 0 {
				refuse("managed lineage requires independent qualification")
				continue
			}
			if !v.HasQueryBounds || !v.HasWallClockUncertainty || (v.WallClockUncertaintyNanos != nil && (*v.WallClockUncertaintyNanos < 0 || uncertainty+float64(*v.WallClockUncertaintyNanos)/1e6 > opts.MaxClockUncertainty)) || v.Backend == "" {
				refuse("reading timing evidence unavailable")
				continue
			}
			if v.WallClockUncertaintyNanos != nil {
				combinedUncertainty = math.Max(combinedUncertainty, uncertainty+float64(*v.WallClockUncertaintyNanos)/1e6)
			}
			source := float64(0)
			estimated := false
			if v.HasSourceTimestamp {
				switch v.SourceTimestampDomain {
				case "unix_utc_ns":
					source = float64(v.SourceTimestamp) / 1e6
				case "process_local_go_monotonic":
					source = wall(v.SourceTimestamp)
				default:
					refuse("source clock domain unknown")
					continue readings
				}
			} else {
				if !m.AllowQueryEstimates || name == "energy" {
					refuse("sensor observation time unavailable")
					continue
				}
				source = wall(v.QueryEndNanos)
				estimated = true
			}
			if v.IntegrationIntervalNanos < 0 || v.RefreshCadenceNanos < 0 || float64(v.IntegrationIntervalNanos)/1e9 > opts.MaxGap || float64(v.RefreshCadenceNanos)/1e9 > opts.MaxGap {
				refuse("sensor cadence outside bound")
				continue
			}

			if name == "energy" && v.Evidence != "measured" && v.Evidence != "unavailable" {
				refuse("cumulative energy requires measured evidence")
				continue
			}
			reading := TelemetryReading{Name: name, Unit: unit, Evidence: v.Evidence, Unavailable: v.UnavailableReason}
			if v.Evidence != "unavailable" {
				if name == "energy" || name == "throttle_reasons" {
					if !v.IsInt {
						return w, nil, fmt.Errorf("native counter must retain integer")
					}
					value := v.IntValue
					reading.Counter = &value
				} else {
					value := v.Value * factor
					if v.IsInt {
						value = float64(v.IntValue) * factor
					}
					reading.Value = &value
				}
			}
			if estimated {
				reading.Evidence = "query-time-estimate"
			}
			if previous := backends[key]; previous != "" && previous != v.Backend {
				refuse("metric backend changed")
				continue
			}
			backends[key] = v.Backend
			defs[key] = reading
			if v.IntegrationIntervalNanos != 0 {
				policy, ok := m.FieldTiming[v.Name]
				if !ok || policy.Anchor != "end" || policy.Evidence == "" || estimated || reading.Counter != nil || name == "energy" || name == "throttle_reasons" {
					refuse("interval convention unqualified")
					continue
				}
				intervalPolicies[key] = policy
				intervals[key] = append(intervals[key], nativeIntervalObservation{UUID: v.DeviceUUID, Reading: reading, Start: source - float64(v.IntegrationIntervalNanos)/1e6, End: source, QueryEnd: wall(v.QueryEndNanos)})
				continue
			}
			if len(intervals[key]) > 0 {
				refuse("point and interval evidence changed")
				continue
			}
			streams[key] = append(streams[key], TelemetryRecord{Kind: "sample", Sample: &TelemetrySample{UUID: v.DeviceUUID, Monotonic: float64(v.QueryEndNanos) / 1e6, QueryStarted: wall(v.QueryStartNanos), QueryFinished: wall(v.QueryEndNanos), SourceAt: &source, Readings: []TelemetryReading{reading}}})
		}
	}
	w = TelemetryWindow{SchemaVersion: 1, RunID: m.BenchmarkRunID, Collector: h.CollectorVersion, SourceHash: raw.SourceHash, ClockBasis: manifest.ClockBasis, ClockUncertainty: &combinedUncertainty, EndpointDevices: m.EndpointDevices}
	intervalKeys := make([]string, 0, len(intervals))
	for key := range intervals {
		intervalKeys = append(intervalKeys, key)
	}
	sort.Strings(intervalKeys)
	for _, key := range intervalKeys {
		if _, refused := unqualified[key]; refused {
			continue
		}
		observations := intervals[key]
		if len(streams[key]) != 0 || len(observations) != len(raw.Samples) {
			v := defs[key]
			qualifyFailure(observations[0].UUID, v.Name, v.Unit, v.Evidence, "integration coverage incomplete")
			continue
		}
		f, e := nativeIntervalSummary(observations, start, end, opts.MaxGap, intervalPolicies[key])
		if e != nil {
			v := defs[key]
			qualifyFailure(observations[0].UUID, v.Name, v.Unit, v.Evidence, e.Error())
			continue
		}
		fields = append(fields, f)
		if f.Name == "power" && deviceScopes[f.UUID] == "whole-device" {
			d := DeviceEnergy{UUID: f.UUID, Scope: "whole-device", Backend: backends[key], Evidence: f.Evidence, StartedAt: start, FinishedAt: end, CoveredDuration: f.CoveredDuration, MissingDuration: f.MissingDuration, MaxGap: f.MaxGap, Method: "power-integral", Unavailable: "power integration coverage unavailable"}
			if f.Mean != nil {
				joules := *f.Mean * f.CoveredDuration
				d.Joules = &joules
				d.Unavailable = ""
			}
			intervalEnergies[f.UUID] = d
		}
	}
	keys := make([]string, 0, len(streams))
	for k := range streams {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	devices := intervalEnergies
	for _, k := range keys {
		if _, refused := unqualified[k]; refused {
			continue
		}
		records := streams[k]
		if len(records) != len(raw.Samples) {
			v := defs[k]
			qualifyFailure(records[0].Sample.UUID, v.Name, v.Unit, v.Evidence, "field absent from sample")
			continue
		}
		single := manifest
		single.Devices = nil
		for _, d := range manifest.Devices {
			if d.UUID == records[0].Sample.UUID {
				single.Devices = []TelemetryDevice{d}
			}
		}
		single.EndpointDevices = nil
		done := true
		all := append([]TelemetryRecord{{Kind: "manifest", Manifest: &single}}, records...)
		all = append(all, TelemetryRecord{Kind: "end", Complete: &done})
		var buf bytes.Buffer
		for _, r := range all {
			if err = json.NewEncoder(&buf).Encode(r); err != nil {
				return w, nil, err
			}
		}
		replay, e := ReadTelemetryReplay(&buf)
		if e != nil {
			v := defs[k]
			qualifyFailure(records[0].Sample.UUID, v.Name, v.Unit, v.Evidence, e.Error())
			continue
		}
		one, fs, e := ReplayTelemetry(replay, start, end, opts)
		if e != nil {
			v := defs[k]
			qualifyFailure(records[0].Sample.UUID, v.Name, v.Unit, v.Evidence, e.Error())
			continue
		}
		fields = append(fields, fs...)
		d := one.Devices[0]
		old, ok := devices[d.UUID]
		if defs[k].Name == "energy" {
			// Preserve the bracketing counter's actual interval, never interpolate it.
			var first, last *TelemetrySample
			for _, r := range records {
				s := r.Sample
				if s.Readings[0].Counter == nil {
					continue
				}
				if *s.SourceAt <= start {
					first = s
				}
				if last == nil && *s.SourceAt >= end {
					last = s
				}
			}
			if first != nil && last != nil {
				d.StartedAt = *first.SourceAt
				d.FinishedAt = *last.SourceAt
				d.EnergyStart = first.Readings[0].Counter
				d.EnergyEnd = last.Readings[0].Counter
				if d.StartedAt == start && d.FinishedAt == end {
					d.Unavailable = ""
				} else {
					d.Unavailable = "counter interval wider than benchmark window"
				}
			}
			devices[d.UUID] = d
		} else if defs[k].Name == "power" && deviceScopes[d.UUID] == "whole-device" && defs[k].Evidence != "query-time-estimate" && (!ok || old.Method != "cumulative-counter") {
			devices[d.UUID] = d
		}
	}
	refusedKeys := make([]string, 0, len(unqualified))
	for key := range unqualified {
		refusedKeys = append(refusedKeys, key)
	}
	sort.Strings(refusedKeys)
	for _, key := range refusedKeys {
		fields = append(fields, unqualified[key])
	}
	for _, d := range manifest.Devices {
		energy, ok := devices[d.UUID]
		if !ok {
			reason := energyReasons[d.UUID]
			if reason == "" {
				reason = "qualified energy unavailable"
			}
			energy = DeviceEnergy{UUID: d.UUID, Scope: d.Scope, ParentUUID: d.ParentUUID, Backend: d.Backend, StartedAt: start, FinishedAt: end, MissingDuration: (end - start) / 1000, Evidence: "unavailable", Unavailable: reason}
		}
		w.Devices = append(w.Devices, energy)
	}
	// Validate endpoint references using the same bounded interchange admission.
	for endpoint, ids := range m.EndpointDevices {
		if endpoint == "" || len(ids) == 0 {
			return w, nil, fmt.Errorf("native endpoint mapping unavailable")
		}
		seen := map[string]bool{}
		for _, id := range ids {
			found := false
			for _, d := range manifest.Devices {
				found = found || d.UUID == id
			}
			if !found || seen[id] {
				return w, nil, fmt.Errorf("native endpoint mapping invalid")
			}
			seen[id] = true
		}
	}
	if len(m.EndpointDevices) == 0 {
		return w, nil, fmt.Errorf("native endpoint mapping unavailable")
	}
	return w, fields, nil
}

func nativeAdapterMetric(name string) (string, string, float64) {
	switch name {
	case "total_energy_millijoules":
		return "energy", "millijoules", 1
	case "power_milliwatts":
		return "power", "watts", .001
	case "gpu_utilization_pct":
		return "utilization", "ratio", .01
	case "sm_activity_pct":
		return "sm_activity", "ratio", .01
	case "tensor_activity_pct":
		return "tensor_activity", "ratio", .01
	case "dram_activity_pct":
		return "dram_activity", "ratio", .01
	case "memory_used_bytes":
		return "memory_used", "bytes", 1
	case "memory_total_bytes":
		return "memory_total", "bytes", 1
	case "temperature_celsius":
		return "temperature", "celsius", 1
	case "sm_clock_mhz":
		return "sm_clock", "hertz", 1e6
	case "memory_clock_mhz":
		return "memory_clock", "hertz", 1e6
	case "clocks_event_reasons", "throttle_reasons":
		return "throttle_reasons", "bitmask", 1
	case "pcie_rx_bytes_per_sec":
		return "pcie_rx", "bytes/second", 1
	case "pcie_tx_bytes_per_sec":
		return "pcie_tx", "bytes/second", 1
	case "nvlink_rx_bytes_per_sec":
		return "nvlink_rx", "bytes/second", 1
	case "nvlink_tx_bytes_per_sec":
		return "nvlink_tx", "bytes/second", 1
	}
	return "", "", 0
}

func nativeAdmissionRevision(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}
func nativeAdmissionDigest(s string) bool {
	if !strings.HasPrefix(s, "sha256:") || len(s) != 71 {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil && strings.ToLower(s) == s
}

type nativeIntervalObservation struct {
	UUID                 string
	Reading              TelemetryReading
	Start, End, QueryEnd float64
}

func nativeIntervalSummary(samples []nativeIntervalObservation, start, end, maxGap float64, policy NativeFieldTiming) (TelemetryFieldSummary, error) {
	f := TelemetryFieldSummary{UUID: samples[0].UUID, Name: samples[0].Reading.Name, Unit: samples[0].Reading.Unit, Evidence: samples[0].Reading.Evidence + ";sensor-interval-average"}
	integral := float64(0)
	availableEvidence := ""
	partialEstimate := false
	for i, s := range samples {
		if s.End > s.QueryEnd {
			return f, fmt.Errorf("native integration observation after query")
		}
		if s.End <= s.Start || (i > 0 && s.Start < samples[i-1].End) {
			return f, fmt.Errorf("native overlapping or reversed sensor intervals")
		}
		if i > 0 {
			gapLo := math.Max(start, samples[i-1].End)
			gapHi := math.Min(end, s.Start)
			if gapHi > gapLo {
				f.MaxGap = math.Max(f.MaxGap, (gapHi-gapLo)/1000)
			}
		}
		lo, hi := math.Max(start, s.Start), math.Min(end, s.End)
		if hi <= lo {
			continue
		}
		if lo != s.Start || hi != s.End {
			if policy.Aggregation != "piecewise_constant_estimate" {
				return f, fmt.Errorf("native partial interval requires explicit piecewise constant estimate")
			}
		}
		if s.QueryEnd-s.End > maxGap*1000 {
			return f, fmt.Errorf("native stale integration interval")
		}
		span := (s.End - s.Start) / 1000
		f.MaxGap = math.Max(f.MaxGap, span)
		if span > maxGap || s.Reading.Unavailable != "" || s.Reading.Value == nil {
			continue
		}
		if err := validateTelemetryReading(s.Reading); err != nil {
			return f, err
		}
		if availableEvidence != "" && s.Reading.Evidence != availableEvidence {
			return f, fmt.Errorf("native interval evidence changed")
		}
		availableEvidence = s.Reading.Evidence
		if lo != s.Start || hi != s.End {
			partialEstimate = true
		}
		v := *s.Reading.Value
		dt := (hi - lo) / 1000
		integral += v * dt
		f.CoveredDuration += dt
		if !finite(v) || !finite(integral) {
			return f, fmt.Errorf("native interval gauge overflow")
		}
		if f.Min == nil || v < *f.Min {
			x := v
			f.Min = &x
		}
		if f.Max == nil || v > *f.Max {
			x := v
			f.Max = &x
		}
	}
	if availableEvidence != "" {
		f.Evidence = availableEvidence + ";sensor-interval-average"
		if partialEstimate {
			f.Evidence += ";piecewise-constant-estimate"
		}
	}
	f.MissingDuration = math.Max(0, (end-start)/1000-f.CoveredDuration)
	if f.CoveredDuration > 0 {
		v := integral / f.CoveredDuration
		f.Mean = &v
	}
	return f, nil
}
