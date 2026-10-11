package llmperf

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func managedSourceFixture(t *testing.T) json.RawMessage {
	t.Helper()
	m := nativeManagedMapping{"DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION", "total_energy_millijoules", "counter", 1, 1, true, "millijoules"}
	b, _ := json.Marshal(m)
	s := nativeManagedLineage{Mapping: m, MappingDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(b)), Schema: "nvtelemetry.managed.v1", SourceID: "exporter", Producer: "dcgm-exporter", ProducerVersion: "4.6.1", NativeOrigin: "dcgm", FieldID: m.Metric, RawValue: "9007199254740993", HasExpositionTimestamp: true, ExpositionTimestampUnix: 1000000000, RawTimestamp: "1000", TimestampUnit: "milliseconds"}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNativeManagedLineage(t *testing.T) {
	source := managedSourceFixture(t)
	base := string(nativeFixture(t))
	withSource := func(s string) string {
		return strings.Replace(strings.ReplaceAll(base, `"schema_version":1`, `"schema_version":2`), `"evidence":"measured"`, `"source":`+s+`,"evidence":"measured"`, 1)
	}
	replay, err := ReadNativeTelemetry(strings.NewReader(withSource(string(source))))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(replay.Samples[0].Readings[0].Source, source) || !strings.Contains(strings.Join(replay.Diagnostics, ","), "managed_lineage_unqualified") {
		t.Fatal("lineage lost or qualification diagnostic omitted")
	}
	for _, tc := range []struct{ name, source string }{
		{"null", "null"},
		{"schema", strings.Replace(string(source), "nvtelemetry.managed.v1", "nvtelemetry.managed.v2", 1)},
		{"digest", strings.Replace(string(source), `"mapping_digest":"sha256:`, `"mapping_digest":"bad:`, 1)},
		{"conversion", strings.Replace(string(source), `"Numerator":1`, `"Numerator":0`, 1)},
		{"null flag", strings.Replace(string(source), `"has_exposition_timestamp":true`, `"has_exposition_timestamp":null`, 1)},
		{"timestamp", strings.Replace(string(source), `"raw_timestamp":"1000"`, `"raw_timestamp":"1001"`, 1)},
		{"exponent", strings.Replace(string(source), `"raw_timestamp":"1000"`, `"raw_timestamp":"1e9999999"`, 1)},
		{"unknown", strings.Replace(string(source), `"schema":`, `"unknown":1,"schema":`, 1)},
		{"duplicate", strings.Replace(string(source), `"schema":`, `"schema":"other","schema":`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ReadNativeTelemetry(strings.NewReader(withSource(tc.source))); err == nil {
				t.Fatal("accepted invalid lineage")
			}
		})
	}
	if _, err := ReadNativeTelemetry(strings.NewReader(strings.Replace(base, `"evidence":"measured"`, `"source":`+string(source)+`,"evidence":"measured"`, 1))); err == nil {
		t.Fatal("accepted managed lineage in v1")
	}
}

func TestNativeManagedLineageCannotSupplyEnergy(t *testing.T) {
	raw, mapping := nativeAdapterFixture(t)
	var b bytes.Buffer
	for _, record := range raw.Records {
		if record.Sample != nil {
			record.Sample.Readings[0].Source = managedSourceFixture(t)
		}
		if err := json.NewEncoder(&b).Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	raw, err = ReadNativeTelemetry(&b)
	if err != nil {
		t.Fatal(err)
	}
	mapping.SourceHash = raw.SourceHash
	window, fields, err := ReplayNativeTelemetry(raw, mapping, 1000, 3000, EnergyOptions{MaxGap: 1.1})
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].Mean != nil || fields[0].Unavailable != "managed lineage requires independent qualification" || len(window.Devices) != 1 || window.Devices[0].Unavailable == "" {
		t.Fatalf("managed lineage promoted to energy: %+v %+v", fields, window)
	}
}

func TestNativeManagedNonFiniteLineage(t *testing.T) {
	for _, raw := range []string{"NaN", "+Inf", "-Inf"} {
		t.Run(raw, func(t *testing.T) {
			var source nativeManagedLineage
			if err := json.Unmarshal(managedSourceFixture(t), &source); err != nil {
				t.Fatal(err)
			}
			source.RawValue = raw
			encoded, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}
			reading := NativeTelemetryReading{Name: source.Mapping.Name, Evidence: "unavailable", UnavailableReason: "non_finite"}
			if err := nativeManagedSource(encoded, reading); err != nil {
				t.Fatal(err)
			}
			replay, err := ReadNativeTelemetry(bytes.NewReader(nativeFixture(t)))
			if err != nil {
				t.Fatal(err)
			}
			var wire bytes.Buffer
			for _, record := range replay.Records {
				if record.Manifest != nil {
					record.Manifest.SchemaVersion = 2
				}
				if record.Sample != nil && record.Sample.Seq == 0 {
					r := &record.Sample.Readings[0]
					r.Evidence, r.UnavailableReason = "unavailable", "non_finite"
					r.Value, r.IntValue, r.IsInt = 0, 0, false
					r.Source = encoded
				}
				if err := json.NewEncoder(&wire).Encode(record); err != nil {
					t.Fatal(err)
				}
			}
			imported, err := ReadNativeTelemetry(&wire)
			if err != nil {
				t.Fatal(err)
			}
			if len(imported.Samples) != len(replay.Samples) || !bytes.Equal(imported.Samples[0].Readings[0].Source, encoded) || !strings.Contains(strings.Join(imported.Diagnostics, ","), "managed_lineage_unqualified") {
				t.Fatal("unavailable lineage or peer samples lost")
			}
			for _, tc := range []struct {
				name   string
				mutate func(*NativeTelemetryReading)
			}{
				{"measured", func(r *NativeTelemetryReading) { r.Evidence = "measured"; r.UnavailableReason = "" }},
				{"derived", func(r *NativeTelemetryReading) { r.Evidence = "derived"; r.UnavailableReason = "" }},
				{"wrong reason", func(r *NativeTelemetryReading) { r.UnavailableReason = "missing" }},
				{"missing reason", func(r *NativeTelemetryReading) { r.UnavailableReason = "" }},
				{"canonical float", func(r *NativeTelemetryReading) { r.Value = 1 }},
				{"canonical integer", func(r *NativeTelemetryReading) { r.IntValue = 1 }},
				{"integer zero", func(r *NativeTelemetryReading) { r.IsInt = true }},
			} {
				t.Run(tc.name, func(t *testing.T) {
					r := reading
					tc.mutate(&r)
					if err := nativeManagedSource(encoded, r); err == nil {
						t.Fatal("accepted contradictory non-finite lineage")
					}
				})
			}
		})
	}
}

func TestNativeManagedFiniteUnavailableLineage(t *testing.T) {
	var source nativeManagedLineage
	if err := json.Unmarshal(managedSourceFixture(t), &source); err != nil {
		t.Fatal(err)
	}
	source.RawValue = "0"
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	r := NativeTelemetryReading{Name: source.Mapping.Name, Evidence: "unavailable", UnavailableReason: "missing"}
	if err := nativeManagedSource(encoded, r); err != nil {
		t.Fatal(err)
	}
	r.UnavailableReason = "non_finite"
	if err := nativeManagedSource(encoded, r); err == nil {
		t.Fatal("accepted finite raw with non_finite reason")
	}
}
