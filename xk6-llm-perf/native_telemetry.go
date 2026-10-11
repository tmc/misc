package llmperf

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// NativeTelemetryReading retains nvtelemetry v1/v2 values without converting integer
// counters to floating point. When IsInt is true, IntValue is authoritative;
// Value may also contain the producer's rounded float copy and is retained.
// TimestampMonotonic is process-relative nanoseconds;
// TimestampUnix is Unix nanoseconds. Query times are not sensor observation bounds.
// V1 SourceTimestamp has no specified clock domain. V2 domains and presence
// flags are retained; a device clock does not share the producer process epoch.
type NativeTelemetryReading struct {
	// Source retains bounded managed/DCGM lineage for diagnostics only.
	Source                    json.RawMessage `json:"source,omitempty"`
	Name                      string          `json:"name"`
	Unit                      string          `json:"unit"`
	Backend                   string          `json:"backend"`
	DeviceUUID                string          `json:"device_uuid"`
	Evidence                  string          `json:"evidence"`
	Value                     float64         `json:"value,omitempty"`
	IntValue                  uint64          `json:"int_value,omitempty"`
	IsInt                     bool            `json:"is_int,omitempty"`
	UnavailableReason         string          `json:"unavailable_reason,omitempty"`
	TimestampMonotonic        int64           `json:"timestamp_monotonic"`
	TimestampUnix             int64           `json:"timestamp_unix"`
	HasSourceTimestamp        bool            `json:"has_source_timestamp,omitempty"`
	SourceTimestampDomain     string          `json:"source_timestamp_domain,omitempty"`
	HasWallClockUncertainty   bool            `json:"has_wall_clock_uncertainty,omitempty"`
	QueryStartNanos           int64           `json:"query_start_nanos,omitempty"`
	QueryEndNanos             int64           `json:"query_end_nanos,omitempty"`
	HasQueryBounds            bool            `json:"has_query_bounds,omitempty"`
	SourceTimestamp           int64           `json:"source_timestamp,omitempty"`
	WallClockUncertaintyNanos *int64          `json:"wall_clock_uncertainty_nanos,omitempty"`
	RefreshCadenceNanos       int64           `json:"refresh_cadence_nanos,omitempty"`
	IntegrationIntervalNanos  int64           `json:"integration_interval_nanos,omitempty"`
	IsWarmup                  bool            `json:"is_warmup,omitempty"`
}

// NativeTelemetryMetadataFailure preserves the producer's typed discovery failure.
type NativeTelemetryMetadataFailure struct {
	Property string `json:"property"`
	Reason   string `json:"reason"`
	Error    string `json:"error"`
}

// NativeTelemetryDevice preserves producer metadata. MIGEnabled=false and omitted
// instance IDs do not establish disabled MIG or absence of an instance in v1.
// FieldCapabilities records advertised candidates, not permanent measured support.
// FieldStatuses retains non-destructive discovery states; queryable does not
// establish continuing availability. Per-reading statuses remain authoritative.
type NativeTelemetryDevice struct {
	UUID                    string                           `json:"uuid"`
	Index                   int                              `json:"index"`
	Model                   string                           `json:"model"`
	Architecture            string                           `json:"architecture,omitempty"`
	DriverVersion           string                           `json:"driver_version,omitempty"`
	LibraryVersion          string                           `json:"library_version,omitempty"`
	PowerLimitMW            uint32                           `json:"power_limit_mw,omitempty"`
	MaxSMClockMHz           uint32                           `json:"max_sm_clock_mhz,omitempty"`
	MaxMemoryClockMHz       uint32                           `json:"max_memory_clock_mhz,omitempty"`
	MIGEnabled              bool                             `json:"mig_enabled,omitempty"`
	MIGParentUUID           string                           `json:"mig_parent_uuid,omitempty"`
	MIGInstanceID           int                              `json:"mig_instance_id,omitempty"`
	FieldCapabilities       []string                         `json:"field_capabilities,omitempty"`
	MIGScope                string                           `json:"mig_scope,omitempty"`
	HasMIGInstanceID        bool                             `json:"has_mig_instance_id,omitempty"`
	MIGComputeInstanceID    int                              `json:"mig_compute_instance_id,omitempty"`
	HasMIGComputeInstanceID bool                             `json:"has_mig_compute_instance_id,omitempty"`
	Status                  string                           `json:"status,omitempty"`
	Warnings                []string                         `json:"warnings,omitempty"`
	MetadataFailures        []NativeTelemetryMetadataFailure `json:"metadata_failures,omitempty"`
	FieldStatuses           map[string]string                `json:"field_statuses,omitempty"`
	ClockSettings           string                           `json:"clock_settings,omitempty"`
}

// NativeTelemetryConfig records producer collection budgets.
type NativeTelemetryConfig struct {
	IntervalNanos      int64  `json:"interval_nanos"`
	MaxDurationNanos   int64  `json:"max_duration_nanos"`
	MaxRecordBudget    int    `json:"max_record_budget"`
	MaxByteBudget      int    `json:"max_byte_budget"`
	MaxBufferedRecords int    `json:"max_buffered_records"`
	BudgetPolicy       string `json:"budget_policy,omitempty"`
	DropOnFull         bool   `json:"drop_on_full"`
}

// NativeTelemetryPhase uses half-open process-relative nanosecond boundaries.
type NativeTelemetryPhase struct {
	Name                string `json:"name"`
	StartMonotonicNanos int64  `json:"start_monotonic_nanos"`
	EndMonotonicNanos   int64  `json:"end_monotonic_nanos"`
}

// NativeTelemetryManifest retains the authoritative integer schema version.
// CollectorVersion is a producer label, not a source revision or binary digest.
// BuildModified=false or omitted does not prove a clean build. Optional mapping
// bounds are retained, without inferring measured clock pairs or a valid join.
type NativeTelemetryManifest struct {
	RunID                       string                  `json:"run_id"`
	SchemaVersion               int                     `json:"schema_version"`
	CollectorVersion            string                  `json:"collector_version"`
	BuildModified               bool                    `json:"build_modified,omitempty"`
	BuildRevision               string                  `json:"build_revision,omitempty"`
	BinaryDigest                string                  `json:"binary_digest,omitempty"`
	MetricRegistryVersion       string                  `json:"metric_registry_version,omitempty"`
	ClockMonotonicDomain        string                  `json:"clock_monotonic_domain,omitempty"`
	ClockMonotonicEpochID       string                  `json:"clock_monotonic_epoch_id,omitempty"`
	ClockWallDomain             string                  `json:"clock_wall_domain,omitempty"`
	ClockProvenance             string                  `json:"clock_provenance,omitempty"`
	ClockMappingValidFrom       int64                   `json:"clock_mapping_valid_from,omitempty"`
	ClockMappingValidUntil      int64                   `json:"clock_mapping_valid_until,omitempty"`
	ClockCalibrationBoundsNanos int64                   `json:"clock_calibration_bounds_nanos,omitempty"`
	HasClockCalibration         bool                    `json:"has_clock_calibration,omitempty"`
	Devices                     []NativeTelemetryDevice `json:"devices"`
	Config                      NativeTelemetryConfig   `json:"config"`
	StartTimeMonotonic          int64                   `json:"start_time_monotonic"`
	StartTimeUnix               int64                   `json:"start_time_unix"`
	EndTimeMonotonic            int64                   `json:"end_time_monotonic"`
	EndTimeUnix                 int64                   `json:"end_time_unix"`
	Phases                      []NativeTelemetryPhase  `json:"phases,omitempty"`
	TotalSamples                uint64                  `json:"total_samples"`
	MissedTicks                 uint64                  `json:"missed_ticks"`
	LostRecords                 uint64                  `json:"lost_records"`
	IsComplete                  bool                    `json:"is_complete"`
	IncompleteReason            string                  `json:"incomplete_reason,omitempty"`
}

// NativeTelemetrySample retains enclosing host query bounds. These bounds do not
// constrain cached sensor observation times. Sample/query fields share the
// producer process epoch; SourceTimestamp may use a separate device clock.
// Different captures/processes must not be joined without an explicit mapping.
type NativeTelemetrySample struct {
	Seq                      uint64                   `json:"seq"`
	QueryStartNanos          int64                    `json:"query_start_nanos"`
	QueryEndNanos            int64                    `json:"query_end_nanos"`
	LagNanos                 int64                    `json:"lag_nanos"`
	ClockDiscontinuity       bool                     `json:"clock_discontinuity,omitempty"`
	ClockDiscontinuityReason string                   `json:"clock_discontinuity_reason,omitempty"`
	Readings                 []NativeTelemetryReading `json:"readings"`
}

// NativeTelemetryRecord is the unmodified native envelope, with one payload.
type NativeTelemetryRecord struct {
	Manifest *NativeTelemetryManifest `json:"manifest,omitempty"`
	Sample   *NativeTelemetrySample   `json:"sample,omitempty"`
	Phase    *NativeTelemetryPhase    `json:"phase,omitempty"`
}

// NativeTelemetryReplay retains original bytes even when validation fails. Raw
// holds at most 64 MiB plus one budget-detection byte. Diagnostics describe valid
// but incomplete captures; completeness does not establish sensor timing,
// synchronized clocks, MIG scope, or admission to energy summaries.
type NativeTelemetryReplay struct {
	Raw         []byte
	SourceHash  string
	Records     []NativeTelemetryRecord
	Header      NativeTelemetryManifest
	Final       *NativeTelemetryManifest
	Samples     []NativeTelemetrySample
	Phases      []NativeTelemetryPhase
	Diagnostics []string
}

// ReadNativeTelemetry reads the nvtelemetry v1/v2 JSONL formats with 64 MiB total,
// 4 MiB line, 100000 records, 256 devices and 64 readings per sample limits. Unknown fields and unsupported versions
// are rejected. Missing final manifests and producer-reported loss remain
// inspectable diagnostics. It performs no benchmark conversion or aggregation.
// Legacy omitted and explicitly zero uncertainty are both unknown; retaining
// their distinction does not prove a zero uncertainty bound.
func ReadNativeTelemetry(r io.Reader) (NativeTelemetryReplay, error) {
	var out NativeTelemetryReplay
	data, err := io.ReadAll(io.LimitReader(r, (64<<20)+1))
	out.Raw = data
	sum := sha256.Sum256(data)
	out.SourceHash = "sha256:" + hex.EncodeToString(sum[:])
	if err != nil {
		return out, fmt.Errorf("read native telemetry: %w", err)
	}
	if len(data) > 64<<20 {
		return out, fmt.Errorf("native telemetry exceeds 64 MiB")
	}
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 4096), 4<<20)
	devices := map[string]bool{}
	counters := map[string]uint64{}
	diagnose := func(reason string) {
		for _, v := range out.Diagnostics {
			if v == reason {
				return
			}
		}
		out.Diagnostics = append(out.Diagnostics, reason)
	}
	var previous *NativeTelemetrySample
	var phaseEnd int64
	havePhase := false
	line := 0
	for scan.Scan() {
		line++
		if len(out.Records) >= 100000 {
			return out, fmt.Errorf("native telemetry exceeds record budget")
		}
		if err := nativeTelemetryJSON(scan.Bytes()); err != nil {
			return out, fmt.Errorf("native telemetry line %d: %w", line, err)
		}
		var rec NativeTelemetryRecord
		dec := json.NewDecoder(bytes.NewReader(scan.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&rec); err != nil {
			return out, fmt.Errorf("native telemetry line %d: %w", line, err)
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			return out, fmt.Errorf("native telemetry line %d: trailing JSON", line)
		}
		n := 0
		if rec.Manifest != nil {
			n++
		}
		if rec.Sample != nil {
			n++
		}
		if rec.Phase != nil {
			n++
		}
		if n != 1 || out.Final != nil || (len(out.Records) == 0 && rec.Manifest == nil) {
			return out, fmt.Errorf("native telemetry line %d: invalid envelope or ordering", line)
		}
		if m := rec.Manifest; m != nil {
			if m.SchemaVersion < 1 || m.SchemaVersion > 2 {
				return out, fmt.Errorf("native telemetry unsupported schema version %d", m.SchemaVersion)
			}
			if len(out.Records) == 0 {
				if m.RunID == "" {
					return out, fmt.Errorf("native telemetry missing run identity")
				}
				if len(m.Devices) > 256 {
					return out, fmt.Errorf("native telemetry exceeds device budget")
				}
				for _, d := range m.Devices {
					if d.UUID == "" || devices[d.UUID] || d.Status == "unidentifiable" {
						return out, fmt.Errorf("native telemetry missing or duplicate device UUID")
					}
					devices[d.UUID] = true
				}
				out.Header = *m
				if m.SchemaVersion == 1 {
					diagnose("legacy_sensor_timing_unknown")
					diagnose("legacy_clock_mapping_unknown")
					diagnose("legacy_mig_scope_unknown")
				} else {
					diagnose("clock_mapping_proof_incomplete")
					if m.ClockMonotonicDomain == "" || m.ClockMonotonicEpochID == "" {
						diagnose("unknown_monotonic_epoch")
					}
					for _, d := range m.Devices {
						if d.MIGScope == "" || d.MIGScope == "unknown" {
							diagnose("unknown_mig_scope")
						}
						if d.MIGScope == "permission_denied" {
							diagnose("permission_denied_mig_scope")
						}
						if d.MIGScope == "unsupported" {
							diagnose("unsupported_mig_scope")
						}
					}
					if m.HasClockCalibration && m.ClockMappingValidFrom > m.ClockMappingValidUntil {
						return out, fmt.Errorf("native telemetry reversed clock mapping validity")
					}
					if m.ClockCalibrationBoundsNanos < 0 {
						return out, fmt.Errorf("native telemetry negative calibration uncertainty")
					}
				}
			} else {
				h := out.Header
				if m.SchemaVersion != h.SchemaVersion || m.BuildRevision != h.BuildRevision || m.BuildModified != h.BuildModified || m.ClockMappingValidFrom != h.ClockMappingValidFrom || m.BinaryDigest != h.BinaryDigest || m.MetricRegistryVersion != h.MetricRegistryVersion || m.ClockMonotonicDomain != h.ClockMonotonicDomain || m.ClockMonotonicEpochID != h.ClockMonotonicEpochID || m.ClockWallDomain != h.ClockWallDomain || m.ClockProvenance != h.ClockProvenance || m.HasClockCalibration != h.HasClockCalibration || m.ClockCalibrationBoundsNanos != h.ClockCalibrationBoundsNanos || m.ClockMappingValidUntil != h.ClockMappingValidUntil || m.RunID != h.RunID || m.CollectorVersion != h.CollectorVersion || m.StartTimeMonotonic != h.StartTimeMonotonic || m.StartTimeUnix != h.StartTimeUnix || !reflect.DeepEqual(m.Devices, h.Devices) || m.Config != h.Config {
					return out, fmt.Errorf("native telemetry contradictory manifests")
				}
				if m.EndTimeMonotonic < m.StartTimeMonotonic || (previous != nil && m.EndTimeMonotonic < previous.QueryEndNanos) {
					return out, fmt.Errorf("native telemetry invalid final bounds")
				}
				if m.IsComplete && (m.LostRecords != 0 || m.IncompleteReason != "" || m.TotalSamples != uint64(len(out.Samples))) {
					return out, fmt.Errorf("native telemetry contradictory completion")
				}
				for _, p := range out.Phases {
					if p.EndMonotonicNanos > m.EndTimeMonotonic {
						return out, fmt.Errorf("native telemetry phase exceeds final bounds")
					}
				}
				if len(out.Phases) > 0 && !reflect.DeepEqual(out.Phases, m.Phases) {
					return out, fmt.Errorf("native telemetry contradictory phase markers")
				}
				out.Final = m
			}
			for i, p := range m.Phases {
				if i > 0 && p.StartMonotonicNanos < m.Phases[i-1].EndMonotonicNanos {
					return out, fmt.Errorf("native telemetry overlapping manifest phases")
				}
				if p.Name == "" || p.EndMonotonicNanos < p.StartMonotonicNanos || (out.Final != nil && (p.EndMonotonicNanos > m.EndTimeMonotonic)) {
					return out, fmt.Errorf("native telemetry invalid manifest phase")
				}
			}
		}
		if s := rec.Sample; s != nil {
			if (s.QueryStartNanos < out.Header.StartTimeMonotonic && !s.ClockDiscontinuity) || s.QueryEndNanos < s.QueryStartNanos || s.LagNanos < 0 || (previous != nil && (s.Seq <= previous.Seq || (s.QueryStartNanos < previous.QueryStartNanos && !s.ClockDiscontinuity))) {
				return out, fmt.Errorf("native telemetry invalid sample ordering or bounds")
			}
			if s.ClockDiscontinuity {
				diagnose("clock_discontinuity")
			}
			if out.Header.SchemaVersion == 2 && out.Header.HasClockCalibration && s.QueryStartNanos < out.Header.ClockMappingValidFrom {
				diagnose("clock_mapping_not_yet_valid")
			}
			if out.Header.SchemaVersion == 2 && out.Header.HasClockCalibration && s.QueryEndNanos > out.Header.ClockMappingValidUntil {
				diagnose("clock_mapping_expired")
			}
			if len(s.Readings) > 64 {
				return out, fmt.Errorf("native telemetry exceeds reading budget")
			}
			if previous != nil && s.Seq != previous.Seq+1 {
				diagnose("sequence_discontinuity")
			}
			if previous != nil && s.QueryStartNanos < previous.QueryEndNanos {
				diagnose("overlapping_query_passes")
			}
			identities := map[string]bool{}
			for _, v := range s.Readings {
				if len(v.Source) > 0 {
					if out.Header.SchemaVersion != 2 {
						return out, fmt.Errorf("managed source requires schema 2")
					}
					if err := nativeManagedSource(v.Source, v); err != nil {
						return out, err
					}
					diagnose("managed_lineage_unqualified")
				}
				key := v.DeviceUUID + "/" + v.Backend + "/" + v.Name
				if identities[key] {
					return out, fmt.Errorf("native telemetry duplicate reading identity")
				}
				identities[key] = true
				if !devices[v.DeviceUUID] || v.Name == "" || v.Unit == "" {
					return out, fmt.Errorf("native telemetry invalid reading identity")
				}
				if out.Header.SchemaVersion == 2 {
					if !v.HasSourceTimestamp {
						diagnose("unknown_sensor_timing")
					} else if v.SourceTimestampDomain == "" {
						diagnose("unknown_source_timestamp_domain")
					} else if v.SourceTimestampDomain != "unix_utc_ns" {
						diagnose("source_domain_mapping_unknown")
					}
					if !v.HasWallClockUncertainty {
						diagnose("unknown_wall_clock_uncertainty")
					}
					if !v.HasQueryBounds {
						diagnose("unknown_per_reading_query_bounds")
					} else if v.QueryStartNanos < s.QueryStartNanos || v.QueryEndNanos > s.QueryEndNanos || v.QueryEndNanos < v.QueryStartNanos {
						return out, fmt.Errorf("native telemetry invalid per-reading query bounds")
					}
				}
				if v.Backend == "" {
					diagnose("unknown_reading_backend")
				}
				if v.TimestampMonotonic == 0 && v.TimestampUnix == 0 {
					diagnose("unknown_reading_clock")
				}
				if unit, ok := nativeTelemetryUnits[v.Name]; ok && unit != v.Unit {
					return out, fmt.Errorf("native telemetry invalid unit for %s", v.Name)
				}
				if v.Evidence != "measured" && v.Evidence != "derived" && v.Evidence != "unavailable" {
					return out, fmt.Errorf("native telemetry invalid evidence")
				}
				if (v.Evidence == "unavailable") != (v.UnavailableReason != "") || (v.Evidence == "unavailable" && (v.Value != 0 || v.IntValue != 0 || v.IsInt)) {
					return out, fmt.Errorf("native telemetry contradictory availability")
				}
				if !v.IsInt && v.IntValue != 0 {
					return out, fmt.Errorf("native telemetry contradictory numeric representation")
				}
				if v.Evidence != "unavailable" && (v.Name == "total_energy_millijoules" || v.Name == "pcie_rx_bytes" || v.Name == "pcie_tx_bytes" || v.Unit == "bitmask" || v.Unit == "event") && !v.IsInt {
					return out, fmt.Errorf("native telemetry integer metric requires is_int")
				}
				if v.Evidence != "unavailable" && (v.Name == "total_energy_millijoules" || v.Name == "pcie_rx_bytes" || v.Name == "pcie_tx_bytes") {
					key := v.DeviceUUID + "/" + v.Backend + "/" + v.Name
					if old, ok := counters[key]; ok && v.IntValue < old {
						diagnose("counter_reset")
					}
					counters[key] = v.IntValue
				}
				if v.WallClockUncertaintyNanos != nil && *v.WallClockUncertaintyNanos < 0 || v.RefreshCadenceNanos < 0 || v.IntegrationIntervalNanos < 0 {
					return out, fmt.Errorf("native telemetry invalid timing metadata")
				}
			}
			out.Samples = append(out.Samples, *s)
			previous = s
		}
		if p := rec.Phase; p != nil {
			if p.Name == "" || p.EndMonotonicNanos < p.StartMonotonicNanos || (havePhase && p.StartMonotonicNanos < phaseEnd) {
				return out, fmt.Errorf("native telemetry invalid phase bounds")
			}
			if p.StartMonotonicNanos < out.Header.StartTimeMonotonic {
				diagnose("phase_precedes_collection")
			}
			out.Phases = append(out.Phases, *p)
			phaseEnd = p.EndMonotonicNanos
			havePhase = true
		}
		out.Records = append(out.Records, rec)
	}
	if err := scan.Err(); err != nil {
		return out, fmt.Errorf("scan native telemetry: %w", err)
	}
	if len(out.Records) == 0 {
		return out, fmt.Errorf("native telemetry missing manifest")
	}
	if out.Final == nil {
		out.Diagnostics = append(out.Diagnostics, "missing_completion_manifest")
	} else {
		if !out.Final.IsComplete {
			reason := out.Final.IncompleteReason
			if reason == "" {
				reason = "producer_marked_incomplete"
			}
			out.Diagnostics = append(out.Diagnostics, reason)
		}
		if out.Final.LostRecords != 0 {
			out.Diagnostics = append(out.Diagnostics, "lost_records")
		}
		if out.Final.MissedTicks != 0 {
			out.Diagnostics = append(out.Diagnostics, "missed_ticks")
		}
	}
	return out, nil
}

var nativeTelemetryUnits = map[string]string{
	"pcie_rx_bytes": "bytes", "pcie_tx_bytes": "bytes", "xid_event": "event", "gpu_utilization_pct": "percent", "memory_utilization_pct": "percent", "memory_used_bytes": "bytes", "memory_total_bytes": "bytes", "power_milliwatts": "milliwatts", "total_energy_millijoules": "millijoules", "temperature_celsius": "celsius", "sm_clock_mhz": "mhz", "memory_clock_mhz": "mhz", "clocks_event_reasons": "bitmask", "throttle_reasons": "bitmask", "sm_activity_pct": "percent", "tensor_activity_pct": "percent", "dram_activity_pct": "percent", "pcie_rx_bytes_per_sec": "bytes_per_sec", "pcie_tx_bytes_per_sec": "bytes_per_sec", "nvlink_rx_bytes_per_sec": "bytes_per_sec", "nvlink_tx_bytes_per_sec": "bytes_per_sec",
}

// Reject duplicate object keys rather than silently replacing source evidence.
func nativeTelemetryJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 128 {
			return fmt.Errorf("JSON nesting exceeds limit")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		if delim == '{' {
			keys := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				key, ok := k.(string)
				if !ok || keys[key] {
					return fmt.Errorf("duplicate or invalid JSON key")
				}
				keys[key] = true
				if nativeTelemetryNumericKeys[key] {
					var raw json.RawMessage
					if err := d.Decode(&raw); err != nil {
						return err
					}
					if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
						return fmt.Errorf("null numeric evidence")
					}
					// These scalar fields cannot hide nested duplicate keys.
					var number json.Number
					if err := json.Unmarshal(raw, &number); err != nil {
						return fmt.Errorf("invalid numeric evidence: %w", err)
					}
				} else if err := value(depth + 1); err != nil {
					return err
				}
			}
		} else if delim == '[' {
			for d.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		} else {
			return fmt.Errorf("unexpected JSON delimiter")
		}
		_, err = d.Token()
		return err
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

// Producer omitempty numbers may be absent, but explicit null is not evidence
// of a zero timestamp, uncertainty, instance ID or measured value.
var nativeTelemetryNumericKeys = map[string]bool{
	"value": true, "int_value": true, "wall_clock_uncertainty_nanos": true,
	"monotonic_ms": true, "query_started_at": true, "query_finished_at": true,
	"source_at": true, "at": true, "started_at": true, "finished_at": true,
	"covered_duration": true, "missing_duration": true, "max_gap": true,
	"unix_origin_nanos": true, "valid_from_nanos": true, "valid_until_nanos": true, "uncertainty_nanos": true,
	"timestamp_monotonic": true, "timestamp_unix": true, "query_start_nanos": true,
	"query_end_nanos": true, "source_timestamp": true, "refresh_cadence_nanos": true,
	"integration_interval_nanos": true, "index": true, "power_limit_mw": true,
	"max_sm_clock_mhz": true, "max_memory_clock_mhz": true, "mig_instance_id": true,
	"mig_compute_instance_id": true, "schema_version": true, "clock_mapping_valid_from": true,
	"clock_mapping_valid_until": true, "clock_calibration_bounds_nanos": true,
	"interval_nanos": true, "max_duration_nanos": true, "max_record_budget": true,
	"max_byte_budget": true, "max_buffered_records": true, "start_time_monotonic": true,
	"start_time_unix": true, "end_time_monotonic": true, "end_time_unix": true,
	"total_samples": true, "missed_ticks": true, "lost_records": true,
	"start_monotonic_nanos": true, "end_monotonic_nanos": true, "seq": true, "lag_nanos": true,
}
