package llmperf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func nativeFixture(t *testing.T) []byte {
	t.Helper()
	h := NativeTelemetryManifest{SchemaVersion: 1, RunID: "synthetic", CollectorVersion: "fixture", Devices: []NativeTelemetryDevice{{UUID: "GPU-1"}}, StartTimeMonotonic: 100, StartTimeUnix: 1000}
	zero := int64(0)
	a := NativeTelemetryReading{Name: "total_energy_millijoules", Unit: "millijoules", Backend: "nvml", DeviceUUID: "GPU-1", Evidence: "measured", IsInt: true, IntValue: 9007199254740993, Value: 9007199254740992, TimestampMonotonic: 150, TimestampUnix: 1050}
	b := a
	b.IntValue = 0
	b.Value = 0
	b.WallClockUncertaintyNanos = &zero
	c := a
	c.Evidence = "unavailable"
	c.UnavailableReason = "query_failure"
	c.IsInt = false
	c.IntValue = 0
	c.Value = 0
	records := []NativeTelemetryRecord{{Manifest: &h}, {Sample: &NativeTelemetrySample{Seq: 0, QueryStartNanos: 150, QueryEndNanos: 160, Readings: []NativeTelemetryReading{a}}}, {Sample: &NativeTelemetrySample{Seq: 1, QueryStartNanos: 170, QueryEndNanos: 180, Readings: []NativeTelemetryReading{c}}}, {Sample: &NativeTelemetrySample{Seq: 2, QueryStartNanos: 190, QueryEndNanos: 200, Readings: []NativeTelemetryReading{b}}}}
	f := h
	f.EndTimeMonotonic = 300
	f.EndTimeUnix = 1200
	f.TotalSamples = 3
	f.IsComplete = true
	records = append(records, NativeTelemetryRecord{Manifest: &f})
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	return out.Bytes()
}

func TestNativeTelemetryValuesAndRaw(t *testing.T) {
	data := nativeFixture(t)
	out, err := ReadNativeTelemetry(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if !bytes.Equal(out.Raw, data) || out.SourceHash != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatal("raw evidence changed")
	}
	if out.Final == nil || !out.Final.IsComplete {
		t.Fatalf("unexpected completion: %+v", out)
	}
	a := out.Samples[0].Readings[0]
	b := out.Samples[2].Readings[0]
	if a.Value != 9007199254740992 {
		t.Fatal("native rounded float copy lost")
	}
	if a.IntValue != 9007199254740993 || !b.IsInt || b.IntValue != 0 {
		t.Fatal("integer value corrupted")
	}
	if a.WallClockUncertaintyNanos != nil || b.WallClockUncertaintyNanos == nil || *b.WallClockUncertaintyNanos != 0 {
		t.Fatal("legacy presence corrupted")
	}
	if len(out.Diagnostics) != 4 {
		t.Fatalf("missing unknown/reset diagnostics: %v", out.Diagnostics)
	}
	if out.Samples[1].Readings[0].UnavailableReason != "query_failure" {
		t.Fatal("unavailable transition lost")
	}
}

func TestNativeTelemetryMalformed(t *testing.T) {
	base := string(nativeFixture(t))
	tests := []struct{ name, input string }{
		{"version", strings.Replace(base, `"schema_version":1`, `"schema_version":3`, 1)},
		{"unknown field", strings.Replace(base, `"schema_version":1`, `"schema_version":1,"invented":true`, 1)},
		{"truncation", base[:len(base)-4]},
		{"contradiction", strings.Replace(base, `"total_samples":3`, `"total_samples":4`, 1)},
		{"identity", strings.Replace(base, `"device_uuid":"GPU-1"`, `"device_uuid":"GPU-2"`, 1)},
		{"unit", strings.Replace(base, `"unit":"millijoules"`, `"unit":"watts"`, 1)},
		{"reordered", strings.Replace(base, `"seq":1`, `"seq":0`, 1)},
		{"negative uncertainty", strings.Replace(base, `"wall_clock_uncertainty_nanos":0`, `"wall_clock_uncertainty_nanos":-1`, 1)},
		{"changed backend identity", strings.Replace(base, `"collector_version":"fixture"`, `"collector_version":"other"`, 1)},
		{"trailing record", base + `{"phase":{"name":"late","start_monotonic_nanos":0,"end_monotonic_nanos":1}}`},
		{"float counter", strings.Replace(base, `"is_int":true,`, ``, 1)},
		{"duplicate key", strings.Replace(base, `"schema_version":1`, `"schema_version":2,"schema_version":1`, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := ReadNativeTelemetry(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("accepted malformed stream")
			}
			if string(out.Raw) != tt.input || out.SourceHash == "" {
				t.Fatal("lost raw evidence")
			}
		})
	}
}

func TestNativeTelemetryIncomplete(t *testing.T) {
	base := string(nativeFixture(t))
	pos := strings.LastIndex(base, `{"manifest":`)
	tests := []struct{ name, input, diagnostic string }{
		{"missing final", base[:pos], "missing_completion_manifest"},
		{"lost", strings.Replace(strings.Replace(base, `"lost_records":0,"is_complete":true`, `"lost_records":1,"is_complete":false`, 1), `"total_samples":3`, `"total_samples":4`, 1), "lost_records"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := ReadNativeTelemetry(strings.NewReader(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, d := range out.Diagnostics {
				found = found || d == tt.diagnostic
			}
			if !found {
				t.Fatalf("missing diagnostic: %v", out.Diagnostics)
			}
		})
	}
}

func TestNativeTelemetryCLIExport(t *testing.T) {
	data, err := os.ReadFile("testdata/nvtelemetry-v1/cli-mock.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	out, err := ReadNativeTelemetry(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if out.SourceHash != "sha256:49d9c9ba808a118cfdc6385e3c3061211b4de7b1e7dd92d3fdf1538633572c12" || len(out.Samples) != 6 || out.Final == nil || !out.Final.IsComplete {
		t.Fatal("producer export receipt changed")
	}
	for _, want := range []string{"unknown_reading_backend", "unknown_reading_clock", "phase_precedes_collection"} {
		found := false
		for _, d := range out.Diagnostics {
			found = found || d == want
		}
		if !found {
			t.Fatalf("missing %s", want)
		}
	}
}

func ExampleReadNativeTelemetry() {
	stream := `{"manifest":{"run_id":"example","schema_version":1,"collector_version":"fixture","devices":[],"config":{},"start_time_monotonic":100,"start_time_unix":1000,"end_time_monotonic":0,"end_time_unix":0,"total_samples":0,"missed_ticks":0,"lost_records":0,"is_complete":false}}` + "\n"
	replay, err := ReadNativeTelemetry(strings.NewReader(stream))
	fmt.Println(err, len(replay.Samples), replay.Diagnostics)
	// Output: <nil> 0 [legacy_sensor_timing_unknown legacy_clock_mapping_unknown legacy_mig_scope_unknown missing_completion_manifest]
}

func TestNativeTelemetryBudgets(t *testing.T) {
	for _, kind := range []string{"devices", "readings", "line"} {
		t.Run(kind, func(t *testing.T) {
			replay, err := ReadNativeTelemetry(bytes.NewReader(nativeFixture(t)))
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "devices":
				for i := 0; i < 257; i++ {
					replay.Records[0].Manifest.Devices = append(replay.Records[0].Manifest.Devices, NativeTelemetryDevice{UUID: fmt.Sprintf("extra-%d", i)})
				}
			case "readings":
				reading := replay.Records[1].Sample.Readings[0]
				for i := 0; i < 64; i++ {
					replay.Records[1].Sample.Readings = append(replay.Records[1].Sample.Readings, reading)
				}
			case "line":
				replay.Records[0].Manifest.RunID = strings.Repeat("x", 4<<20)
			}
			var data bytes.Buffer
			for _, r := range replay.Records {
				if err := json.NewEncoder(&data).Encode(r); err != nil {
					t.Fatal(err)
				}
			}
			out, err := ReadNativeTelemetry(bytes.NewReader(data.Bytes()))
			if err == nil {
				t.Fatal("budget admitted")
			}
			if !bytes.Equal(out.Raw, data.Bytes()) {
				t.Fatal("raw bounded evidence lost")
			}
		})
	}
}

func TestNativeTelemetryV2PairedFixtures(t *testing.T) {
	tests := []struct {
		name, hash string
		valid      bool
	}{
		{"negative_binary_digest_mismatch.jsonl", "7b943fcdf8aa786db0af769021dfefaa1ab03bbbdfeaee00d1ae429ceec692bd", false},
		{"negative_out_of_pass_bounds.jsonl", "a6410c35798f8726008c17a51aac839658786ac9c9049be3b7358da3d68bcb22", false},
		{"negative_provenance_mismatch.jsonl", "2739984a5678bd60fef4f8140da7f5c22e02fc6e293fb758bde7ec0b7921eba8", false},
		{"afb8_cli_mock.jsonl", "134350a6ab3a442f9c0577fb6e3c57bc4b424986ab661c54dbcfa979c9037722", true},
		{"b960_cli_mock.jsonl", "1e029f3fb94ac8d262c755542e1867b260c66f9c5edf5b7c338f7b3924db2e70", true},
		{"negative_epoch_mismatch.jsonl", "1da3cfd9544ff64bd1c72819ef3720d95dc944363d366d55c38684fce6b02064", false},
		{"negative_invalid_bounds.jsonl", "a7d24aa45a6fc30828a137e343bf4cce38799b40b0f0905f84923f69e81ccf83", false},
		{"negative_schema_downgrade.jsonl", "93d9dfb16c75d763ae96578ddc028d2dae4a2610e9a9443a3cbb05e9f7dcd6bb", false},
		{"remediation_cli_mock.jsonl", "c8a5e96ce2380ed971c2d3c1af9794ecae1404e3c31ad3aeb354d45d7acc8a8c", true},
		{"canonical_v1.jsonl", "8322d878a8afcfd98695af3f2db9e00b3d826c8049349a50e87b88db0b843986", true},
		{"canonical_v2.jsonl", "f5fe32b4c3273a71109583793e989b9c4cbbad4a0648313f6cbce1d5ee173b9b", true},
		{"negative_truncated.jsonl", "b547e4e7563a81385aac6b9f3849265fb2ed7f07f3e798e1e285155a111f4636", false},
		{"negative_contradictory_manifest.jsonl", "14905a5c3e746f43ed2fcfc640bdb6a0825820be278ba4fa95e08cda701dc6ba", false},
		{"negative_missing_uuid.jsonl", "f818364e8bd8407cf16447967719946580a3f9167b4257693dc98c5f3d45b617", false},
		// This fixture is valid raw bitmask evidence; only numeric aggregation is refused.
		{"negative_bitmask_averaging.jsonl", "7c43005da635cc27a1f89557109a8f26c4eb4beea6ff79007bf1eede261d15ab", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/nvtelemetry-v2/" + tt.name)
			if err != nil {
				t.Fatal(err)
			}
			out, err := ReadNativeTelemetry(bytes.NewReader(data))
			if (err == nil) != tt.valid {
				t.Fatalf("admission mismatch: %v", err)
			}
			if !bytes.Equal(out.Raw, data) || out.SourceHash != "sha256:"+tt.hash {
				t.Fatal("paired bytes/hash changed")
			}
			if tt.name == "canonical_v2.jsonl" {
				r := out.Samples[0].Readings[0]
				if !r.HasSourceTimestamp || r.SourceTimestampDomain != "nvml_device_monotonic_ns" || !r.HasQueryBounds || !r.HasWallClockUncertainty || r.QueryEndNanos != 1002000000 {
					t.Fatal("lost v2 metadata")
				}
				for _, want := range []string{"clock_mapping_proof_incomplete", "source_domain_mapping_unknown", "unknown_sensor_timing", "unknown_wall_clock_uncertainty"} {
					nativeWantDiagnostic(t, out, want)
				}
			}
			if tt.name == "negative_bitmask_averaging.jsonl" && out.Samples[1].Readings[0].IntValue != 0 {
				t.Fatal("lost zero bitmask evidence")
			}
		})
	}
}

func nativeWantDiagnostic(t *testing.T, out NativeTelemetryReplay, want string) {
	t.Helper()
	for _, d := range out.Diagnostics {
		if d == want {
			return
		}
	}
	t.Fatalf("missing %s: %v", want, out.Diagnostics)
}

func TestNativeTelemetryV2UnknownAndPresence(t *testing.T) {
	data, err := os.ReadFile("testdata/nvtelemetry-v2/canonical_v2.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name       string
		replace    [][2]string
		diagnostic string
		refuse     bool
	}{
		{name: "known zero", replace: [][2]string{{`"wall_clock_uncertainty_nanos":25000`, `"wall_clock_uncertainty_nanos":0`}}},
		{name: "known zero omitted scalar", replace: [][2]string{{`"wall_clock_uncertainty_nanos":25000,`, ``}}},
		{name: "expired", replace: [][2]string{{`"clock_mapping_valid_until":1800000000000000000`, `"clock_mapping_valid_until":1002000000`}}, diagnostic: "clock_mapping_expired"},
		{name: "unsupported mig", replace: [][2]string{{`"mig_scope":"disabled"`, `"mig_scope":"unsupported"`}}, diagnostic: "unsupported_mig_scope"},
		{name: "unknown mig", replace: [][2]string{{`"mig_scope":"disabled"`, `"mig_scope":"unknown"`}}, diagnostic: "unknown_mig_scope"},
		{name: "instance zero", replace: [][2]string{{`"mig_scope":"disabled"`, `"mig_scope":"mig_instance","has_mig_instance_id":true,"has_mig_compute_instance_id":true`}}},
		{name: "clock step", replace: [][2]string{{`"clock_discontinuity":false`, `"clock_discontinuity":true,"clock_discontinuity_reason":"wall_step"`}}, diagnostic: "clock_discontinuity"},
		{name: "bad query bounds", replace: [][2]string{{`"query_start_nanos":1000000000,"query_end_nanos":1002000000,"has_query_bounds":true`, `"query_start_nanos":900000000,"query_end_nanos":1002000000,"has_query_bounds":true`}}, refuse: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := string(data)
			for _, pair := range tt.replace {
				input = strings.ReplaceAll(input, pair[0], pair[1])
			}
			out, err := ReadNativeTelemetry(strings.NewReader(input))
			if tt.refuse {
				if err == nil {
					t.Fatal("invalid bounds admitted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			nativeWantDiagnostic(t, out, "clock_mapping_proof_incomplete")
			if tt.diagnostic != "" {
				nativeWantDiagnostic(t, out, tt.diagnostic)
			}
			if strings.HasPrefix(tt.name, "known zero") {
				if !out.Samples[0].Readings[0].HasWallClockUncertainty {
					t.Fatal("known-zero presence lost")
				}
			}
			if tt.name == "instance zero" {
				d := out.Header.Devices[0]
				if !d.HasMIGInstanceID || d.MIGInstanceID != 0 || !d.HasMIGComputeInstanceID || d.MIGComputeInstanceID != 0 {
					t.Fatal("zero identity lost")
				}
			}
		})
	}
}

func TestNativeTelemetryV2ManifestContradictions(t *testing.T) {
	data, err := os.ReadFile("testdata/nvtelemetry-v2/canonical_v2.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	base := string(data)
	for _, tt := range []struct {
		name, old, new string
		last           bool
	}{
		{"negative reading uncertainty", `"wall_clock_uncertainty_nanos":25000`, `"wall_clock_uncertainty_nanos":-1`, false},
		{"negative calibration", `"clock_calibration_bounds_nanos":50000`, `"clock_calibration_bounds_nanos":-1`, false},
		{"epoch change", `"clock_monotonic_epoch_id":"epoch-20261010-v2-001"`, `"clock_monotonic_epoch_id":"other"`, true},
		{"schema change", `"schema_version":2`, `"schema_version":1`, true},
		{"provenance change", `"clock_provenance":"system_kernel_ntp"`, `"clock_provenance":"other"`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := base
			if tt.last {
				pos := strings.LastIndex(input, tt.old)
				if pos < 0 {
					t.Fatal("missing fixture field")
				}
				input = input[:pos] + tt.new + input[pos+len(tt.old):]
			} else {
				input = strings.Replace(input, tt.old, tt.new, 1)
			}
			out, err := ReadNativeTelemetry(strings.NewReader(input))
			if err == nil {
				t.Fatal("accepted contradiction")
			}
			if string(out.Raw) != input {
				t.Fatal("lost refused evidence")
			}
		})
	}
}

func TestNativeTelemetryV2RemediationMetadata(t *testing.T) {
	data, err := os.ReadFile("testdata/nvtelemetry-v2/canonical_v2.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	base := string(data)
	identity := `"mig_scope":"permission_denied","metadata_failures":[{"property":"mig_mode","reason":"permission_denied","error":"fixture denied"}],"field_statuses":{"power_milliwatts":"queryable","sm_activity_pct":"unknown"}`
	input := strings.ReplaceAll(base, `"mig_scope":"disabled"`, identity)
	input = strings.ReplaceAll(input, `"collector_version":"nvtelemetry-2.0.0"`, `"collector_version":"nvtelemetry-2.0.0","build_revision":"fixture-revision","build_modified":true,"binary_digest":"fixture-digest"`)
	input = strings.ReplaceAll(input, `"clock_mapping_valid_until":1800000000000000000`, `"clock_mapping_valid_from":1000000000,"clock_mapping_valid_until":1800000000000000000`)
	out, err := ReadNativeTelemetry(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	d := out.Header.Devices[0]
	if d.MIGScope != "permission_denied" || len(d.MetadataFailures) != 1 || d.MetadataFailures[0].Property != "mig_mode" || d.MetadataFailures[0].Reason != "permission_denied" || d.MetadataFailures[0].Error != "fixture denied" || d.FieldStatuses["power_milliwatts"] != "queryable" || d.FieldStatuses["sm_activity_pct"] != "unknown" {
		t.Fatal("metadata altered")
	}
	if !out.Header.BuildModified || out.Header.BuildRevision != "fixture-revision" || out.Header.BinaryDigest != "fixture-digest" || out.Header.ClockMappingValidFrom != 1000000000 {
		t.Fatal("provenance altered")
	}
	nativeWantDiagnostic(t, out, "permission_denied_mig_scope")
	nativeWantDiagnostic(t, out, "clock_mapping_proof_incomplete")
	for _, tt := range []struct {
		name, old, new string
		last           bool
		diagnostic     string
	}{
		{"reversed mapping", `"clock_mapping_valid_from":1000000000`, `"clock_mapping_valid_from":1900000000000000000`, false, ""},
		{"changed mapping start", `"clock_mapping_valid_from":1000000000`, `"clock_mapping_valid_from":1000000001`, true, ""},
		{"changed build state", `"build_modified":true`, `"build_modified":false`, true, ""},
		{"changed registry", `"metric_registry_version":"2026.10-v2"`, `"metric_registry_version":"other"`, true, ""},
		{"changed domain", `"clock_monotonic_domain":"process_local_go_monotonic"`, `"clock_monotonic_domain":"other"`, true, ""},
		{"future mapping", `"clock_mapping_valid_from":1000000000`, `"clock_mapping_valid_from":1200000000`, false, "clock_mapping_not_yet_valid"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			changed := input
			if tt.last {
				pos := strings.LastIndex(changed, tt.old)
				if pos < 0 {
					t.Fatal("fixture key absent")
				}
				changed = changed[:pos] + tt.new + changed[pos+len(tt.old):]
			} else {
				changed = strings.ReplaceAll(changed, tt.old, tt.new)
			}
			replay, err := ReadNativeTelemetry(strings.NewReader(changed))
			if tt.diagnostic != "" {
				if err != nil {
					t.Fatal(err)
				}
				nativeWantDiagnostic(t, replay, tt.diagnostic)
			} else if err == nil {
				t.Fatal("contradiction accepted")
			}
		})
	}
}

func TestNativeTelemetryZeroPassBoundsAndBinaryIdentity(t *testing.T) {
	replay, err := ReadNativeTelemetry(bytes.NewReader(nativeFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	h, f := replay.Records[0].Manifest, replay.Records[len(replay.Records)-1].Manifest
	h.SchemaVersion = 2
	f.SchemaVersion = 2
	h.StartTimeMonotonic = 0
	f.StartTimeMonotonic = 0
	h.BinaryDigest = "fixture-digest"
	f.BinaryDigest = "fixture-digest"
	sample := replay.Records[1].Sample
	sample.QueryStartNanos = 0
	sample.QueryEndNanos = 10
	reading := &sample.Readings[0]
	reading.HasQueryBounds = true
	reading.QueryStartNanos = 0
	reading.QueryEndNanos = 10
	encode := func() []byte {
		t.Helper()
		var b bytes.Buffer
		for _, r := range replay.Records {
			if err := json.NewEncoder(&b).Encode(r); err != nil {
				t.Fatal(err)
			}
		}
		return b.Bytes()
	}
	valid := encode()
	out, err := ReadNativeTelemetry(bytes.NewReader(valid))
	if err != nil {
		t.Fatalf("zero start valid control refused: %v", err)
	}
	if out.Samples[0].QueryStartNanos != 0 || out.Samples[0].Readings[0].QueryStartNanos != 0 {
		t.Fatal("zero query start lost")
	}
	reading.QueryEndNanos = 11
	invalid := encode()
	out, err = ReadNativeTelemetry(bytes.NewReader(invalid))
	if err == nil || !strings.Contains(err.Error(), "invalid per-reading query bounds") {
		t.Fatalf("out-of-pass reading with zero sample start not refused: %v", err)
	}
	if !bytes.Equal(out.Raw, invalid) {
		t.Fatal("refused evidence changed")
	}
	reading.QueryEndNanos = 10
	f.BinaryDigest = "other-digest"
	invalid = encode()
	out, err = ReadNativeTelemetry(bytes.NewReader(invalid))
	if err == nil || !strings.Contains(err.Error(), "contradictory manifests") {
		t.Fatalf("terminal binary digest contradiction not refused: %v", err)
	}
	if !bytes.Equal(out.Raw, invalid) {
		t.Fatal("refused binary identity evidence changed")
	}
}

func TestNativeTelemetryStrictEvidence(t *testing.T) {
	base := string(nativeFixture(t))
	for _, tc := range []struct{ name, input string }{
		{"null float", strings.Replace(base, `"value":9007199254740992`, `"value":null`, 1)},
		{"null integer", strings.Replace(base, `"int_value":9007199254740993`, `"int_value":null`, 1)},
		{"null instance ID", strings.Replace(base, `"uuid":"GPU-1"`, `"uuid":"GPU-1","has_mig_instance_id":true,"mig_instance_id":null`, 1)},
		{"null query bound", strings.Replace(base, `"query_start_nanos":150`, `"query_start_nanos":null`, 1)},
		{"null uncertainty", strings.Replace(base, `"wall_clock_uncertainty_nanos":0`, `"wall_clock_uncertainty_nanos":null`, 1)},
		{"declared null uncertainty", strings.Replace(strings.ReplaceAll(base, `"schema_version":1`, `"schema_version":2`), `"evidence":"measured"`, `"has_wall_clock_uncertainty":true,"wall_clock_uncertainty_nanos":null,"evidence":"measured"`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ReadNativeTelemetry(strings.NewReader(tc.input)); err == nil {
				t.Fatal("accepted ambiguous evidence")
			}
		})
	}
	replay, err := ReadNativeTelemetry(strings.NewReader(base))
	if err != nil {
		t.Fatal(err)
	}
	replay.Records[1].Sample.Readings = append(replay.Records[1].Sample.Readings, replay.Records[1].Sample.Readings[0])
	var buf bytes.Buffer
	for _, r := range replay.Records {
		json.NewEncoder(&buf).Encode(r)
	}
	if _, err := ReadNativeTelemetry(&buf); err == nil {
		t.Fatal("accepted duplicate reading")
	}
	replay, _ = ReadNativeTelemetry(strings.NewReader(base))
	replay.Final.Phases = []NativeTelemetryPhase{{Name: "measurement", StartMonotonicNanos: 100, EndMonotonicNanos: 301}}
	buf.Reset()
	for _, r := range replay.Records {
		json.NewEncoder(&buf).Encode(r)
	}
	if _, err := ReadNativeTelemetry(&buf); err == nil {
		t.Fatal("accepted phase beyond capture")
	}
	// A sequence jump is diagnosed, without claiming the producer lost records.
	jumped := strings.Replace(base, `"seq":2`, `"seq":99`, 1)
	replay, err = ReadNativeTelemetry(strings.NewReader(jumped))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(replay.Diagnostics, ","), "sequence_discontinuity") {
		t.Fatal("missing sequence diagnostic")
	}
}

func TestNativeTelemetryPhaseConsistency(t *testing.T) {
	replay, err := ReadNativeTelemetry(bytes.NewReader(nativeFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	phase := NativeTelemetryPhase{Name: "measurement", StartMonotonicNanos: 100, EndMonotonicNanos: 300}
	replay.Final.Phases = []NativeTelemetryPhase{phase}
	records := append(append([]NativeTelemetryRecord{}, replay.Records[:len(replay.Records)-1]...), NativeTelemetryRecord{Phase: &phase}, replay.Records[len(replay.Records)-1])
	encode := func() []byte {
		var b bytes.Buffer
		for _, r := range records {
			json.NewEncoder(&b).Encode(r)
		}
		return b.Bytes()
	}
	if _, err := ReadNativeTelemetry(bytes.NewReader(encode())); err != nil {
		t.Fatal(err)
	}
	replay.Final.Phases[0].Name = "other"
	if _, err := ReadNativeTelemetry(bytes.NewReader(encode())); err == nil {
		t.Fatal("accepted contradictory phase marker")
	}
	replay.Final.Phases = []NativeTelemetryPhase{phase, {Name: "cooldown", StartMonotonicNanos: 299, EndMonotonicNanos: 300}}
	if _, err := ReadNativeTelemetry(bytes.NewReader(encode())); err == nil {
		t.Fatal("accepted overlapping phases")
	}
}
