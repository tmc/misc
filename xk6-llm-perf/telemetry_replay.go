package llmperf

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// TelemetryManifest identifies an offline adapter stream. Version 1 is a local
// consumer format, not a claim of compatibility with a delivered GPU producer.
// Wall times use Unix milliseconds; monotonic times use elapsed milliseconds.
type TelemetryManifest struct {
	SchemaVersion     int                 `json:"schema_version"`
	RunID             string              `json:"run_id"`
	Collector         string              `json:"collector"`
	DefinitionVersion string              `json:"definition_version"`
	ClockBasis        string              `json:"clock_basis"`
	ClockUncertainty  *float64            `json:"clock_uncertainty_ms"`
	EndpointDevices   map[string][]string `json:"endpoint_devices"`
	Devices           []TelemetryDevice   `json:"devices"`
}

// TelemetryDevice preserves device identity and discovery provenance.
type TelemetryDevice struct {
	UUID       string `json:"uuid"`
	ParentUUID string `json:"parent_uuid,omitempty"`
	Scope      string `json:"scope"`
	Model      string `json:"model"`
	Driver     string `json:"driver"`
	Backend    string `json:"backend"`
}

// TelemetryReading holds a canonical gauge or integer counter. An unavailable
// reason is mutually exclusive with values. Zero is an available value.
type TelemetryReading struct {
	Name        string   `json:"name"`
	Unit        string   `json:"unit"`
	Evidence    string   `json:"evidence"`
	Value       *float64 `json:"value,omitempty"`
	Counter     *uint64  `json:"counter,omitempty"`
	Unavailable string   `json:"unavailable,omitempty"`
}

// TelemetrySample preserves query and observation times separately. SourceAt
// must identify the sensor observation used for window alignment; query time
// alone cannot substitute for an unknown sensor timestamp.
type TelemetrySample struct {
	UUID          string             `json:"uuid"`
	Monotonic     float64            `json:"monotonic_ms"`
	QueryStarted  float64            `json:"query_started_at"`
	QueryFinished float64            `json:"query_finished_at"`
	SourceAt      *float64           `json:"source_at"`
	Readings      []TelemetryReading `json:"readings"`
}

// TelemetryRecord is one JSONL envelope. The stream starts with a manifest and
// ends with a complete terminal record. Phase records retain measurement markers.
type TelemetryRecord struct {
	Kind        string             `json:"kind"`
	Manifest    *TelemetryManifest `json:"manifest,omitempty"`
	Sample      *TelemetrySample   `json:"sample,omitempty"`
	Phase       string             `json:"phase,omitempty"`
	At          *float64           `json:"at,omitempty"`
	Complete    *bool              `json:"complete,omitempty"`
	LostRecords int                `json:"lost_records,omitempty"`
}

// TelemetryReplay retains validated raw records and their exact byte hash.
type TelemetryReplay struct {
	Manifest     TelemetryManifest `json:"manifest"`
	Records      []TelemetryRecord `json:"records"`
	SourceHash   string            `json:"source_hash"`
	fingerprint  [32]byte
	originalHash string
}

// ReadTelemetryReplay reads at most 64 MiB, 100000 records, and 1 MiB per line.
// It rejects truncated, incomplete, reordered, or identity-changing streams.
func ReadTelemetryReplay(r io.Reader) (TelemetryReplay, error) {
	var out TelemetryReplay
	data, err := io.ReadAll(io.LimitReader(r, 64*1024*1024+1))
	if err != nil {
		return out, fmt.Errorf("read telemetry replay: %w", err)
	}
	if len(data) > 64*1024*1024 {
		return out, fmt.Errorf("telemetry replay exceeds 64 MiB")
	}
	sum := sha256.Sum256(data)
	out.SourceHash = "sha256:" + hex.EncodeToString(sum[:])
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 4096), 1024*1024)
	ended := false
	devices := map[string]TelemetryDevice{}
	prev := map[string]TelemetrySample{}
	phase := 0
	var lastPhase float64
	for scan.Scan() {
		if len(out.Records) >= 100000 || ended {
			return out, fmt.Errorf("telemetry records exceed bound or follow terminal record")
		}
		if err := nativeTelemetryJSON(scan.Bytes()); err != nil {
			return out, fmt.Errorf("telemetry record %d: %w", len(out.Records)+1, err)
		}
		var rec TelemetryRecord
		dec := json.NewDecoder(bytes.NewReader(scan.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&rec); err != nil {
			return out, fmt.Errorf("telemetry record %d: %w", len(out.Records)+1, err)
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			return out, fmt.Errorf("trailing telemetry record data")
		}
		switch rec.Kind {
		case "manifest":
			if len(out.Records) != 0 || rec.Manifest == nil || rec.Sample != nil || rec.At != nil || rec.Complete != nil || rec.Phase != "" || rec.LostRecords != 0 {
				return out, fmt.Errorf("invalid manifest envelope")
			}
			m := *rec.Manifest
			if m.SchemaVersion != 1 || m.RunID == "" || m.Collector == "" || m.DefinitionVersion != "llm-telemetry-v1" || m.ClockBasis == "" || m.ClockUncertainty == nil || !finite(*m.ClockUncertainty) || *m.ClockUncertainty < 0 || len(m.Devices) == 0 || len(m.Devices) > 256 {
				return out, fmt.Errorf("invalid telemetry manifest")
			}
			for _, d := range m.Devices {
				if d.UUID == "" || d.Backend == "" || d.Model == "" || d.Driver == "" || d.Scope == "" {
					return out, fmt.Errorf("incomplete telemetry device")
				}
				if _, ok := devices[d.UUID]; ok {
					return out, fmt.Errorf("duplicate telemetry device")
				}
				devices[d.UUID] = d
			}
			for endpoint, ids := range m.EndpointDevices {
				if endpoint == "" || len(ids) == 0 {
					return out, fmt.Errorf("invalid endpoint mapping")
				}
				seen := map[string]bool{}
				for _, id := range ids {
					if _, ok := devices[id]; !ok || seen[id] {
						return out, fmt.Errorf("invalid device mapping")
					}
					seen[id] = true
				}
			}
			out.Manifest = m
		case "sample":
			if len(out.Records) == 0 || rec.Sample == nil || rec.Manifest != nil || rec.At != nil || rec.Complete != nil || rec.Phase != "" || rec.LostRecords != 0 {
				return out, fmt.Errorf("invalid sample envelope")
			}
			s := *rec.Sample
			if _, ok := devices[s.UUID]; !ok {
				return out, fmt.Errorf("unknown telemetry device")
			}
			if !finite(s.Monotonic) || s.Monotonic < 0 || !finite(s.QueryStarted) || !finite(s.QueryFinished) || s.QueryFinished < s.QueryStarted || s.SourceAt == nil || !finite(*s.SourceAt) || *s.SourceAt > s.QueryFinished || len(s.Readings) == 0 || len(s.Readings) > 64 {
				return out, fmt.Errorf("invalid sample timing or readings")
			}
			if p, ok := prev[s.UUID]; ok && (s.Monotonic <= p.Monotonic || *s.SourceAt <= *p.SourceAt || s.QueryStarted < p.QueryFinished) {
				return out, fmt.Errorf("nonmonotonic or overlapping device queries")
			}
			names := map[string]bool{}
			for _, v := range s.Readings {
				if err := validateTelemetryReading(v); err != nil {
					return out, err
				}
				if names[v.Name] {
					return out, fmt.Errorf("duplicate telemetry reading")
				}
				names[v.Name] = true
			}
			prev[s.UUID] = s
		case "phase":
			if len(out.Records) == 0 || rec.At == nil || !finite(*rec.At) || rec.Sample != nil || rec.Manifest != nil || rec.Complete != nil || rec.LostRecords != 0 {
				return out, fmt.Errorf("invalid phase envelope")
			}
			phases := []string{"warmup", "measurement", "cooldown"}
			if phase >= len(phases) || rec.Phase != phases[phase] {
				return out, fmt.Errorf("invalid phase order")
			}
			if phase > 0 && *rec.At < lastPhase {
				return out, fmt.Errorf("nonmonotonic phase marker")
			}
			lastPhase = *rec.At
			phase++
		case "end":
			if len(out.Records) == 0 || rec.Complete == nil || !*rec.Complete || rec.LostRecords != 0 || rec.Sample != nil || rec.Manifest != nil || rec.At != nil || rec.Phase != "" {
				return out, fmt.Errorf("incomplete telemetry stream")
			}
			ended = true
		default:
			return out, fmt.Errorf("unknown telemetry record kind")
		}
		out.Records = append(out.Records, rec)
	}
	if err := scan.Err(); err != nil {
		return out, fmt.Errorf("scan telemetry: %w", err)
	}
	if !ended {
		return out, fmt.Errorf("missing telemetry terminal record")
	}
	out.fingerprint = telemetryFingerprint(out.Records)
	out.originalHash = out.SourceHash
	return out, nil
}

func validateTelemetryReading(v TelemetryReading) error {
	units := map[string]string{"utilization": "ratio", "sm_activity": "ratio", "sm_occupancy": "ratio", "tensor_activity": "ratio", "dram_activity": "ratio", "memory_used": "bytes", "memory_total": "bytes", "power": "watts", "energy": "millijoules", "temperature": "celsius", "sm_clock": "hertz", "memory_clock": "hertz", "throttle_reasons": "bitmask", "pcie_tx": "bytes/second", "pcie_rx": "bytes/second", "nvlink_tx": "bytes/second", "nvlink_rx": "bytes/second"}
	unit, ok := units[v.Name]
	if !ok || unit != v.Unit || v.Evidence == "" {
		return fmt.Errorf("unknown metric, unit, or evidence")
	}
	values := 0
	if v.Value != nil {
		values++
	}
	if v.Counter != nil {
		values++
	}
	if v.Unavailable != "" {
		values++
	}
	if values != 1 {
		return fmt.Errorf("ambiguous telemetry availability")
	}
	if v.Value != nil && (!finite(*v.Value) || (v.Unit != "celsius" && *v.Value < 0) || (v.Unit == "ratio" && *v.Value > 1)) {
		return fmt.Errorf("invalid telemetry value")
	}
	if v.Unavailable == "" && ((v.Name == "energy" || v.Name == "throttle_reasons") != (v.Counter != nil)) {
		return fmt.Errorf("counter type mismatch")
	}
	return nil
}

// TelemetryFieldSummary uses linear interpolation between adjacent available
// samples. It never bridges missing readings or gaps exceeding MaxGap.
// Unavailable explains a field-local admission failure; such fields have no
// numeric summary and report the entire requested duration as missing.
type TelemetryFieldSummary struct {
	UUID            string   `json:"uuid"`
	Name            string   `json:"name"`
	Unit            string   `json:"unit"`
	Evidence        string   `json:"evidence"`
	Unavailable     string   `json:"unavailable,omitempty"`
	CoveredDuration float64  `json:"covered_duration"`
	MissingDuration float64  `json:"missing_duration"`
	MaxGap          float64  `json:"max_gap"`
	Mean            *float64 `json:"mean"`
	Min             *float64 `json:"min"`
	Max             *float64 `json:"max"`
}

// ReplayTelemetry adapts a validated stream into benchmark-window summaries.
// Counter energy requires samples exactly at both window boundaries. Power is
// a trapezoidal estimate; missing coverage remains explicit for AnalyzeEnergy.
func ReplayTelemetry(replay TelemetryReplay, start, end float64, opts EnergyOptions) (TelemetryWindow, []TelemetryFieldSummary, error) {
	m := replay.Manifest
	w := TelemetryWindow{SchemaVersion: 1, RunID: m.RunID, Collector: m.Collector, SourceHash: replay.SourceHash, ClockBasis: m.ClockBasis, ClockUncertainty: m.ClockUncertainty, EndpointDevices: m.EndpointDevices}
	if !finite(start) || !finite(end) || end <= start || opts.MaxGap <= 0 || !finite(opts.MaxGap) {
		return w, nil, fmt.Errorf("invalid replay bounds")
	}
	// Revalidate public structs through the same bounded parser.
	var buf bytes.Buffer
	for _, rec := range replay.Records {
		if err := json.NewEncoder(&buf).Encode(rec); err != nil {
			return w, nil, err
		}
	}
	valid, err := ReadTelemetryReplay(&buf)
	if err != nil {
		return w, nil, err
	}
	if !equalManifest(valid.Manifest, m) || replay.SourceHash == "" || replay.SourceHash != replay.originalHash || replay.fingerprint != telemetryFingerprint(replay.Records) {
		return w, nil, fmt.Errorf("replay manifest mismatch")
	}
	var fields []TelemetryFieldSummary
	duration := (end - start) / 1000
	for _, d := range m.Devices {
		var samples []TelemetrySample
		names := map[string]bool{}
		for _, rec := range replay.Records {
			if rec.Sample != nil && rec.Sample.UUID == d.UUID {
				samples = append(samples, *rec.Sample)
			}
		}
		summaries := map[string]TelemetryFieldSummary{}
		integrals := map[string]float64{}
		availableEvidence := map[string]string{}
		for i, s := range samples {
			relevant := *s.SourceAt >= start && *s.SourceAt <= end
			if i > 0 && *samples[i-1].SourceAt < end && *s.SourceAt > start {
				relevant = true
			}
			if i+1 < len(samples) && *s.SourceAt < end && *samples[i+1].SourceAt > start {
				relevant = true
			}
			if !relevant {
				continue
			}
			for _, v := range s.Readings {
				names[v.Name] = true
				if _, ok := summaries[v.Name]; !ok {
					summaries[v.Name] = TelemetryFieldSummary{UUID: d.UUID, Name: v.Name, Unit: v.Unit, Evidence: v.Evidence}
				}
				if v.Unavailable == "" {
					if previous := availableEvidence[v.Name]; previous != "" && previous != v.Evidence {
						return w, nil, fmt.Errorf("changing available metric evidence")
					}
					availableEvidence[v.Name] = v.Evidence
					f := summaries[v.Name]
					f.Evidence = v.Evidence
					summaries[v.Name] = f
				}
			}
		}
		for i := 1; i < len(samples); i++ {
			a, b := samples[i-1], samples[i]

			gap := (*b.SourceAt - *a.SourceAt) / 1000
			lo, hi := math.Max(start, *a.SourceAt), math.Min(end, *b.SourceAt)
			if hi <= lo {
				continue
			}
			// Observation age is a window-local admission check. Old samples
			// that cannot contribute to this window do not affect its coverage.
			if a.QueryFinished-*a.SourceAt > opts.MaxGap*1000 || b.QueryFinished-*b.SourceAt > opts.MaxGap*1000 {
				return w, nil, fmt.Errorf("stale sensor observation")
			}
			for name := range names {
				f := summaries[name]
				f.MaxGap = math.Max(f.MaxGap, gap)
				av, bv := reading(a, name), reading(b, name)
				if gap <= opts.MaxGap && av != nil && bv != nil && av.Unavailable == "" && bv.Unavailable == "" {
					if av.Counter != nil && bv.Counter != nil {
						if name == "energy" && *bv.Counter < *av.Counter {
							return w, nil, fmt.Errorf("device %q counter reset", d.UUID)
						}
						f.CoveredDuration += (hi - lo) / 1000
					} else if av.Value != nil && bv.Value != nil {
						x := *av.Value + (*bv.Value-*av.Value)*(lo-*a.SourceAt)/(*b.SourceAt-*a.SourceAt)
						y := *av.Value + (*bv.Value-*av.Value)*(hi-*a.SourceAt)/(*b.SourceAt-*a.SourceAt)
						dt := (hi - lo) / 1000
						f.CoveredDuration += dt
						integrals[name] += (x/2 + y/2) * dt
						if !finite(x) || !finite(y) || !finite(integrals[name]) {
							return w, nil, fmt.Errorf("telemetry integral overflow")
						}
						low, high := math.Min(x, y), math.Max(x, y)
						if f.Min == nil || low < *f.Min {
							v := low
							f.Min = &v
						}
						if f.Max == nil || high > *f.Max {
							v := high
							f.Max = &v
						}
					}
				}
				summaries[name] = f
			}
		}
		// Sort fields by the small fixed metric inventory for deterministic exports.
		ordered := []string{"utilization", "sm_activity", "sm_occupancy", "tensor_activity", "dram_activity", "memory_used", "memory_total", "power", "energy", "temperature", "sm_clock", "memory_clock", "throttle_reasons", "pcie_tx", "pcie_rx", "nvlink_tx", "nvlink_rx"}
		for _, name := range ordered {
			f, ok := summaries[name]
			if !ok {
				continue
			}
			f.MissingDuration = math.Max(0, duration-f.CoveredDuration)
			if f.MissingDuration < 1e-9 {
				f.MissingDuration = 0
			}
			if f.CoveredDuration > 0 && name != "energy" && name != "throttle_reasons" {
				v := integrals[name] / f.CoveredDuration
				f.Mean = &v
			}
			summaries[name] = f
			fields = append(fields, f)
		}
		de := DeviceEnergy{UUID: d.UUID, ParentUUID: d.ParentUUID, Scope: d.Scope, Backend: d.Backend, StartedAt: start, FinishedAt: end, Unavailable: "energy unavailable"}
		if f, ok := summaries["energy"]; ok {
			de.Evidence = f.Evidence
			de.Method = "cumulative-counter"
			de.CoveredDuration = f.CoveredDuration
			de.MissingDuration = f.MissingDuration
			de.MaxGap = f.MaxGap
			for _, s := range samples {
				v := reading(s, "energy")
				if v == nil || v.Counter == nil {
					continue
				}
				if *s.SourceAt == start {
					de.EnergyStart = v.Counter
				}
				if *s.SourceAt == end {
					de.EnergyEnd = v.Counter
				}
			}
			if de.EnergyStart != nil && de.EnergyEnd != nil {
				de.Unavailable = ""
			} else {
				de.Unavailable = "counter boundaries do not match benchmark window"
			}
		} else if f, ok := summaries["power"]; ok {
			de.Evidence = f.Evidence
			de.Method = "power-integral"
			de.CoveredDuration = f.CoveredDuration
			de.MissingDuration = f.MissingDuration
			de.MaxGap = f.MaxGap
			j := integrals["power"]
			de.Joules = &j
			de.Unavailable = ""
		}
		w.Devices = append(w.Devices, de)
	}
	return w, fields, nil
}

func reading(s TelemetrySample, name string) *TelemetryReading {
	for i := range s.Readings {
		if s.Readings[i].Name == name {
			return &s.Readings[i]
		}
	}
	return nil
}
func equalManifest(a, b TelemetryManifest) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

// WriteTelemetryReplay writes canonical JSONL after validating every record.
// It encodes a replay rather than recording live samples. The exported bytes
// acquire their own hash when read; serialization need not preserve original bytes.
func WriteTelemetryReplay(w io.Writer, replay TelemetryReplay) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, r := range replay.Records {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("encode telemetry replay: %w", err)
		}
	}
	got, err := ReadTelemetryReplay(bytes.NewReader(b.Bytes()))
	if err != nil {
		return err
	}
	if !equalManifest(got.Manifest, replay.Manifest) {
		return fmt.Errorf("replay manifest mismatch")
	}
	n, err := w.Write(b.Bytes())
	if err != nil {
		return fmt.Errorf("write telemetry replay: %w", err)
	}
	if n != b.Len() {
		return io.ErrShortWrite
	}
	return nil
}

func telemetryFingerprint(records []TelemetryRecord) [32]byte {
	b, _ := json.Marshal(records)
	return sha256.Sum256(b)
}
