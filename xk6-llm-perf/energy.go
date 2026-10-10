package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// DeviceEnergy is a producer's window summary. Cumulative energy stays an
// integer number of millijoules until a validated difference is computed.
// Power-integrated estimates supply Joules instead of cumulative boundaries.
type DeviceEnergy struct {
	UUID            string   `json:"uuid"`
	ParentUUID      string   `json:"parent_uuid,omitempty"`
	Scope           string   `json:"scope"`
	Backend         string   `json:"backend"`
	Evidence        string   `json:"evidence"`
	StartedAt       float64  `json:"started_at"`
	FinishedAt      float64  `json:"finished_at"`
	CoveredDuration float64  `json:"covered_duration"`
	MissingDuration float64  `json:"missing_duration"`
	MaxGap          float64  `json:"max_gap"`
	EnergyStart     *uint64  `json:"energy_start_millijoules"`
	EnergyEnd       *uint64  `json:"energy_end_millijoules"`
	Joules          *float64 `json:"joules"`
	Method          string   `json:"method"`
	Unavailable     string   `json:"unavailable,omitempty"`
}

// TelemetryWindow is the benchmark consumer interchange for producer summaries.
// It does not prescribe the native collector's wire format. Adapters must retain
// the original raw stream and manifest identified by SourceHash and Collector.
type TelemetryWindow struct {
	SchemaVersion    int                 `json:"schema_version"`
	RunID            string              `json:"run_id"`
	Collector        string              `json:"collector"`
	SourceHash       string              `json:"source_hash"`
	ClockBasis       string              `json:"clock_basis"`
	ClockUncertainty *float64            `json:"clock_uncertainty_ms"`
	EndpointDevices  map[string][]string `json:"endpoint_devices"`
	Devices          []DeviceEnergy      `json:"devices"`
}

// EnergyOptions specifies alignment and coverage admission bounds. Durations
// use seconds except clock uncertainty, which uses milliseconds.
type EnergyOptions struct {
	MaxClockUncertainty float64
	MaxGap              float64
}

// EnergySummary reports whole-device energy per successful output token.
// It does not attribute energy to a request, process, or MIG instance.
type EnergySummary struct {
	RunID                string         `json:"run_id"`
	StartedAt            float64        `json:"started_at"`
	FinishedAt           float64        `json:"finished_at"`
	Devices              []string       `json:"devices"`
	Joules               float64        `json:"joules"`
	OutputTokens         int64          `json:"output_tokens"`
	JoulesPerOutputToken *float64       `json:"joules_per_output_token"`
	OutputTokensPerJoule *float64       `json:"output_tokens_per_joule"`
	CountSources         map[string]int `json:"count_sources"`
	EstimatedEnergy      bool           `json:"estimated_energy"`
	Policy               string         `json:"policy"`
	SourceHash           string         `json:"source_hash"`
}

// ReadTelemetryWindow reads a bounded benchmark interchange. Unknown fields are
// rejected so schema changes require an explicit adapter version.
func ReadTelemetryWindow(r io.Reader) (TelemetryWindow, error) {
	const limit = 4 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return TelemetryWindow{}, fmt.Errorf("read telemetry: %w", err)
	}
	if len(data) > limit {
		return TelemetryWindow{}, fmt.Errorf("telemetry exceeds 4 MiB")
	}
	var w TelemetryWindow
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&w); err != nil {
		return w, fmt.Errorf("decode telemetry: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return w, fmt.Errorf("trailing telemetry data")
	}
	if w.SchemaVersion != 1 || len(w.Devices) == 0 || len(w.Devices) > 256 || w.RunID == "" || w.Collector == "" || w.SourceHash == "" {
		return w, fmt.Errorf("invalid telemetry schema or identity")
	}
	return w, nil
}

// AnalyzeEnergy joins an entire retained benchmark window with explicitly mapped
// device summaries. It refuses partial coverage, wider counter intervals, reset
// counters, unknown clocks, missing mappings, and incompatible device scopes.
func AnalyzeEnergy(run RunSummary, w TelemetryWindow, opts EnergyOptions) (EnergySummary, error) {
	s := EnergySummary{RunID: run.RunID, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, CountSources: map[string]int{}, SourceHash: w.SourceHash, Policy: "whole-device energy; wholly contained successful requests; no idle subtraction or per-request attribution"}
	if _, err := AnalyzeTimeline(run); err != nil {
		return s, err
	}
	if w.SchemaVersion != 1 || w.RunID != run.RunID || w.RunID == "" || w.Collector == "" || w.SourceHash == "" || len(w.Devices) == 0 || len(w.Devices) > 256 {
		return s, fmt.Errorf("telemetry identity or schema mismatch")
	}
	if opts.MaxClockUncertainty < 0 || !finite(opts.MaxClockUncertainty) || opts.MaxGap <= 0 || !finite(opts.MaxGap) {
		return s, fmt.Errorf("invalid energy admission bounds")
	}
	if w.ClockBasis == "" || w.ClockUncertainty == nil || !finite(*w.ClockUncertainty) || *w.ClockUncertainty < 0 || *w.ClockUncertainty > opts.MaxClockUncertainty {
		return s, fmt.Errorf("clock alignment unavailable or outside bound")
	}
	devices := map[string]DeviceEnergy{}
	for _, d := range w.Devices {
		if d.UUID == "" || d.Backend == "" || d.Evidence == "" || d.Scope != "whole-device" || d.ParentUUID != "" {
			return s, fmt.Errorf("invalid device identity or energy scope")
		}
		if _, ok := devices[d.UUID]; ok {
			return s, fmt.Errorf("duplicate device %q", d.UUID)
		}
		devices[d.UUID] = d
	}
	required := map[string]bool{}
	for _, m := range run.Records {
		ids := w.EndpointDevices[m.Endpoint]
		if len(ids) == 0 {
			return s, fmt.Errorf("endpoint %q has no device mapping", m.Endpoint)
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if _, ok := devices[id]; !ok || seen[id] {
				return s, fmt.Errorf("invalid mapped device %q", id)
			}
			seen[id], required[id] = true, true
		}
		if m.Outcome == "success" {
			if m.OutputTokens.Value == nil || *m.OutputTokens.Value < 0 {
				return s, fmt.Errorf("successful output count unavailable")
			}
			s.OutputTokens += int64(*m.OutputTokens.Value)
			s.CountSources[m.OutputTokens.Source]++
		}
	}
	if len(required) == 0 {
		return s, fmt.Errorf("no mapped benchmark device")
	}
	for _, d := range w.Devices {
		if !required[d.UUID] {
			continue
		}
		if d.Unavailable != "" {
			return s, fmt.Errorf("device %q energy unavailable: %s", d.UUID, d.Unavailable)
		}
		if d.StartedAt != run.StartedAt || d.FinishedAt != run.FinishedAt {
			return s, fmt.Errorf("device %q counter interval differs from benchmark window", d.UUID)
		}
		if !finite(d.CoveredDuration) || math.Abs(d.CoveredDuration-run.Duration) > 1e-6 || d.MissingDuration != 0 || !finite(d.MaxGap) || d.MaxGap < 0 || d.MaxGap > opts.MaxGap {
			return s, fmt.Errorf("device %q incomplete coverage or sample gap", d.UUID)
		}
		var joules float64
		switch d.Method {
		case "cumulative-counter":
			if d.EnergyStart == nil || d.EnergyEnd == nil || d.Joules != nil || *d.EnergyEnd < *d.EnergyStart {
				return s, fmt.Errorf("device %q missing or reset energy counter", d.UUID)
			}
			joules = float64(*d.EnergyEnd-*d.EnergyStart) / 1000
		case "power-integral":
			if d.Joules == nil || d.EnergyStart != nil || d.EnergyEnd != nil {
				return s, fmt.Errorf("device %q invalid power estimate", d.UUID)
			}
			joules = *d.Joules
			s.EstimatedEnergy = true
		default:
			return s, fmt.Errorf("device %q unknown energy method", d.UUID)
		}
		if !finite(joules) || joules < 0 {
			return s, fmt.Errorf("device %q invalid energy", d.UUID)
		}
		s.Joules += joules
		s.Devices = append(s.Devices, d.UUID)
	}
	if !finite(s.Joules) {
		return s, fmt.Errorf("energy sum overflow")
	}
	if s.OutputTokens > 0 {
		v := s.Joules / float64(s.OutputTokens)
		s.JoulesPerOutputToken = &v
	}
	if s.Joules > 0 && s.OutputTokens > 0 {
		v := float64(s.OutputTokens) / s.Joules
		s.OutputTokensPerJoule = &v
	}
	return s, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
