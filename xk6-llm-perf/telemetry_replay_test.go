package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func rawTelemetryFixture(counter bool) string {
	zero := 0.0
	complete := true
	m := TelemetryManifest{SchemaVersion: 1, RunID: "fixture", Collector: "injected@1", DefinitionVersion: "llm-telemetry-v1", ClockBasis: "same-host", ClockUncertainty: &zero, EndpointDevices: map[string][]string{"http://server/v1": {"GPU-1"}}, Devices: []TelemetryDevice{{UUID: "GPU-1", Scope: "whole-device", Model: "fixture", Driver: "fixture", Backend: "fixture"}}}
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.Encode(TelemetryRecord{Kind: "manifest", Manifest: &m})
	for i, at := range []float64{1000, 1500, 3000} {
		power := []float64{2, 4, 8}[i]
		energy := uint64(9007199254740993) + []uint64{0, 1000, 8000}[i]
		values := []TelemetryReading{{Name: "power", Unit: "watts", Evidence: "injected", Value: &power}}
		if counter {
			values = append(values, TelemetryReading{Name: "energy", Unit: "millijoules", Evidence: "injected", Counter: &energy})
		}
		e.Encode(TelemetryRecord{Kind: "sample", Sample: &TelemetrySample{UUID: "GPU-1", Monotonic: at - 1000, QueryStarted: at, QueryFinished: at + 1, SourceAt: &at, Readings: values}})
	}
	e.Encode(TelemetryRecord{Kind: "end", Complete: &complete})
	return b.String()
}

func ExampleReadTelemetryReplay() {
	raw, err := ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(true)))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(raw.Manifest.Collector, len(raw.Records))
	// Output: injected@1 5
}

func ExampleReplayTelemetry() {
	raw, _ := ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(true)))
	window, fields, err := ReplayTelemetry(raw, 1000, 3000, EnergyOptions{MaxGap: 1.5})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(*fields[0].Mean, window.Devices[0].Method)
	// Output: 5.25 cumulative-counter
}

func TestTelemetryReplayAnalytic(t *testing.T) {
	for _, counter := range []bool{false, true} {
		raw, err := ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(counter)))
		if err != nil {
			t.Fatal(err)
		}
		w, fields, err := ReplayTelemetry(raw, 1000, 3000, EnergyOptions{MaxGap: 1.5})
		if err != nil {
			t.Fatal(err)
		}
		if len(fields) == 0 || *fields[0].Mean != 5.25 || fields[0].CoveredDuration != 2 || fields[0].MissingDuration != 0 {
			t.Fatalf("fields=%+v", fields)
		}
		run, _ := energyFixture()
		summary, err := AnalyzeEnergy(run, w, EnergyOptions{MaxGap: 1.5})
		if err != nil {
			t.Fatal(err)
		}
		want := 10.5
		if counter {
			want = 8
		}
		if summary.Joules != want || summary.EstimatedEnergy == counter {
			t.Fatalf("summary=%+v", summary)
		}
		_, _, err = ReplayTelemetry(raw, 1100, 2900, EnergyOptions{MaxGap: 1.5})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestTelemetryReplayAvailabilityTransition(t *testing.T) {
	for _, missing := range []int{1, 2, 3} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			raw, err := ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(false)))
			if err != nil {
				t.Fatal(err)
			}
			raw.Records[missing].Sample.Readings[0] = TelemetryReading{Name: "power", Unit: "watts", Evidence: "unavailable", Unavailable: "query_failure"}
			var b bytes.Buffer
			for _, rec := range raw.Records {
				if err := json.NewEncoder(&b).Encode(rec); err != nil {
					t.Fatal(err)
				}
			}
			raw, err = ReadTelemetryReplay(&b)
			if err != nil {
				t.Fatal(err)
			}
			_, fields, err := ReplayTelemetry(raw, 1000, 3000, EnergyOptions{MaxGap: 1.5})
			if err != nil {
				t.Fatal(err)
			}
			wantCoverage := []float64{0, 1.5, 0, 0.5}[missing]
			if len(fields) != 1 || fields[0].CoveredDuration != wantCoverage || fields[0].MissingDuration != 2-wantCoverage || fields[0].Evidence != "injected" {
				t.Fatalf("fields=%+v, want coverage %v", fields, wantCoverage)
			}
		})
	}
}

func TestTelemetryReplayRefusals(t *testing.T) {
	base := rawTelemetryFixture(true)
	for _, tc := range []struct{ name, input string }{
		{"truncated", base[:len(base)-3]},
		{"unknown version", strings.Replace(base, `"schema_version":1`, `"schema_version":2`, 1)},
		{"identity", strings.Replace(base, `"uuid":"GPU-1","monotonic_ms":500`, `"uuid":"GPU-2","monotonic_ms":500`, 1)},
		{"loss", strings.Replace(base, `"complete":true`, `"complete":true,"lost_records":1`, 1)},
		{"incomplete", strings.Replace(base, `"complete":true`, `"complete":false`, 1)},
		{"unit", strings.Replace(base, `"unit":"watts"`, `"unit":"milliwatts"`, 1)},
		{"unknown field", strings.Replace(base, `"kind":"end"`, `"kind":"end","extra":1`, 1)},
		{"after end", base + `{"kind":"end","complete":true}`},
		{"unknown clock", strings.Replace(base, `"clock_uncertainty_ms":0`, `"clock_uncertainty_ms":null`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ReadTelemetryReplay(strings.NewReader(tc.input)); err == nil {
				t.Fatal("accepted invalid stream")
			}
		})
	}
	for _, name := range []string{"gap", "missing", "reset", "clock", "wider", "MIG", "partial device"} {
		t.Run(name, func(t *testing.T) {
			raw, err := ReadTelemetryReplay(strings.NewReader(base))
			if err != nil {
				t.Fatal(err)
			}
			gap := 1.5
			start, end := 1000.0, 3000.0
			switch name {
			case "gap":
				gap = 1
			case "missing":
				raw.Records[2].Sample.Readings[1].Counter = nil
				raw.Records[2].Sample.Readings[1].Unavailable = "permission-denied"
			case "reset":
				v := uint64(1)
				raw.Records[2].Sample.Readings[1].Counter = &v
			case "clock":
				v := 100.0
				raw.Manifest.ClockUncertainty = &v
				raw.Records[0].Manifest = &raw.Manifest
			case "wider":
				start, end = 1100, 2900
			case "MIG":
				raw.Manifest.Devices[0].ParentUUID = "parent"
				raw.Records[0].Manifest = &raw.Manifest
			case "partial device":
				raw.Manifest.Devices = append(raw.Manifest.Devices, TelemetryDevice{UUID: "GPU-2", Scope: "whole-device", Model: "test", Driver: "test", Backend: "test"})
				raw.Manifest.EndpointDevices["http://server/v1"] = append(raw.Manifest.EndpointDevices["http://server/v1"], "GPU-2")
				raw.Records[0].Manifest = &raw.Manifest
			}
			var reb bytes.Buffer
			if err := WriteTelemetryReplay(&reb, raw); err != nil {
				t.Fatal(err)
			}
			raw, err = ReadTelemetryReplay(&reb)
			if err != nil {
				t.Fatal(err)
			}
			w, _, err := ReplayTelemetry(raw, start, end, EnergyOptions{MaxGap: gap})
			if err != nil {
				return
			}
			run, _ := energyFixture()
			if _, err := AnalyzeEnergy(run, w, EnergyOptions{MaxGap: gap}); err == nil {
				t.Fatal("accepted unqualified energy")
			}
		})
	}
}

func ExampleWriteTelemetryReplay() {
	raw, _ := ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(true)))
	var b bytes.Buffer
	err := WriteTelemetryReplay(&b, raw)
	got, _ := ReadTelemetryReplay(&b)
	fmt.Println(err, len(got.Records))
	// Output: <nil> 5
}

func TestTelemetryWriterReplay(t *testing.T) {
	raw, _ := ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(true)))
	var b bytes.Buffer
	if err := WriteTelemetryReplay(&b, raw); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTelemetryReplay(&b)
	if err != nil {
		t.Fatal(err)
	}
	a, af, err := ReplayTelemetry(raw, 1000, 3000, EnergyOptions{MaxGap: 1.5})
	if err != nil {
		t.Fatal(err)
	}
	z, zf, err := ReplayTelemetry(got, 1000, 3000, EnergyOptions{MaxGap: 1.5})
	if err != nil {
		t.Fatal(err)
	}
	aj, _ := json.Marshal(a)
	zj, _ := json.Marshal(z)
	fj, _ := json.Marshal(af)
	gj, _ := json.Marshal(zf)
	if !bytes.Equal(aj, zj) || !bytes.Equal(fj, gj) {
		t.Fatal("replay changed summaries")
	}
	if err := WriteTelemetryReplay(shortTelemetryWriter{}, raw); err == nil {
		t.Fatal("accepted short writer")
	}
}

type shortTelemetryWriter struct{}

func (shortTelemetryWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestTelemetryReplayBoundsAndOrigin(t *testing.T) {
	if _, err := ReadTelemetryReplay(strings.NewReader(strings.Repeat(" ", 1024*1024+1))); err == nil {
		t.Fatal("accepted oversized line")
	}
	raw, _ := ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(true)))
	raw.SourceHash = "sha256:changed"
	if _, _, err := ReplayTelemetry(raw, 1000, 3000, EnergyOptions{MaxGap: 1.5}); err == nil {
		t.Fatal("accepted changed raw identity")
	}
	raw, _ = ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(true)))
	*raw.Records[1].Sample.Readings[0].Value = 100
	if _, _, err := ReplayTelemetry(raw, 1000, 3000, EnergyOptions{MaxGap: 1.5}); err == nil {
		t.Fatal("accepted mutated retained stream")
	}
}

func TestTelemetryUnavailableZeroAndBitmask(t *testing.T) {
	raw, _ := ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(false)))
	for i := 1; i <= 3; i++ {
		v := 0.0
		bits := uint64(4 - i)
		raw.Records[i].Sample.Readings = []TelemetryReading{{Name: "power", Unit: "watts", Evidence: "injected", Value: &v}, {Name: "throttle_reasons", Unit: "bitmask", Evidence: "injected", Counter: &bits}}
	}
	var b bytes.Buffer
	if err := WriteTelemetryReplay(&b, raw); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadTelemetryReplay(&b)
	if err != nil {
		t.Fatal(err)
	}
	w, fields, err := ReplayTelemetry(raw, 1000, 3000, EnergyOptions{MaxGap: 1.5})
	if err != nil {
		t.Fatal(err)
	}
	if *w.Devices[0].Joules != 0 || *fields[0].Mean != 0 || fields[1].Mean != nil {
		t.Fatalf("zero or bitmask interpretation wrong: %+v", fields)
	}
	run, _ := energyFixture()
	s, err := AnalyzeEnergy(run, w, EnergyOptions{MaxGap: 1.5})
	if err != nil {
		t.Fatal(err)
	}
	if s.OutputTokensPerJoule != nil {
		t.Fatal("zero energy yielded efficiency")
	}
}

func TestTelemetryReplayDuplicateKeys(t *testing.T) {
	for _, input := range []string{
		strings.Replace(rawTelemetryFixture(false), `"schema_version":1`, `"schema_version":999,"schema_version":1`, 1),
		strings.Replace(rawTelemetryFixture(false), `"complete":true`, `"complete":false,"complete":true`, 1),
	} {
		if _, err := ReadTelemetryReplay(strings.NewReader(input)); err == nil {
			t.Fatal("accepted replaced evidence")
		}
	}
}

func TestTelemetryReplayWindowLocalAge(t *testing.T) {
	// An old observation outside the requested window must not alter later coverage.
	base, _ := ReadTelemetryReplay(strings.NewReader(rawTelemetryFixture(true)))
	var records []TelemetryRecord
	records = append(records, base.Records[0])
	at := []float64{1000, 5000, 7000, 8500}
	for i, v := range at {
		power := 2.0
		counter := uint64(9007199254740993) + uint64(i)*1000
		finish := v + 1
		if i == 0 {
			finish = 3000
		}
		source := v
		records = append(records, TelemetryRecord{Kind: "sample", Sample: &TelemetrySample{UUID: "GPU-1", Monotonic: v - 1000, QueryStarted: v, QueryFinished: finish, SourceAt: &source, Readings: []TelemetryReading{{Name: "power", Unit: "watts", Evidence: "injected", Value: &power}, {Name: "energy", Unit: "millijoules", Evidence: "injected", Counter: &counter}}}})
	}
	records = append(records, base.Records[len(base.Records)-1])
	read := func(rs []TelemetryRecord) TelemetryReplay {
		t.Helper()
		var b bytes.Buffer
		for _, r := range rs {
			json.NewEncoder(&b).Encode(r)
		}
		v, err := ReadTelemetryReplay(&b)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	records[1].Sample.Readings[0].Evidence = "old-derived"
	outside := 25.0
	records[1].Sample.Readings = append(records[1].Sample.Readings, TelemetryReading{Name: "temperature", Unit: "celsius", Evidence: "injected", Value: &outside})
	full := read(records)
	short := read(append(append([]TelemetryRecord{records[0]}, records[3:5]...), records[5]))
	a, af, err := ReplayTelemetry(full, 7000, 8500, EnergyOptions{MaxGap: 1.5})
	if err != nil {
		t.Fatal(err)
	}
	b, bf, err := ReplayTelemetry(short, 7000, 8500, EnergyOptions{MaxGap: 1.5})
	if err != nil {
		t.Fatal(err)
	}
	x, _ := json.Marshal(a.Devices)
	y, _ := json.Marshal(b.Devices)
	if !bytes.Equal(x, y) {
		t.Fatalf("window changed: %s vs %s", x, y)
	}
	x, _ = json.Marshal(af)
	y, _ = json.Marshal(bf)
	if !bytes.Equal(x, y) {
		t.Fatalf("fields changed: %s vs %s", x, y)
	}
	if _, _, err := ReplayTelemetry(full, 1000, 5000, EnergyOptions{MaxGap: 1.5}); err == nil {
		t.Fatal("accepted stale relevant observation")
	}
	// A reset outside the window remains irrelevant; inside remains a refusal.
	records[1].Sample.Readings[1].Counter = new(uint64)
	*records[1].Sample.Readings[1].Counter = ^uint64(0)
	if _, _, err := ReplayTelemetry(read(records), 7000, 8500, EnergyOptions{MaxGap: 1.5}); err != nil {
		t.Fatal(err)
	}
	records[4].Sample.Readings[1].Counter = new(uint64)
	if _, _, err := ReplayTelemetry(read(records), 7000, 8500, EnergyOptions{MaxGap: 1.5}); err == nil {
		t.Fatal("accepted relevant reset")
	}
}

func TestOfflineTelemetryNullTiming(t *testing.T) {
	base := rawTelemetryFixture(true)
	// The first monotonic timestamp is genuinely zero. Null must not inherit it.
	for _, tc := range []struct{ name, old, new string }{
		{"monotonic", `"monotonic_ms":0`, `"monotonic_ms":null`},
		{"query start", `"query_started_at":1000`, `"query_started_at":null`},
		{"query end", `"query_finished_at":1001`, `"query_finished_at":null`},
		{"source", `"source_at":1000`, `"source_at":null`},
		{"end bound", `"kind":"end"`, `"kind":"end","at":null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := strings.Replace(base, tc.old, tc.new, 1)
			if input == base {
				t.Fatal("ineffective mutation")
			}
			if _, err := ReadTelemetryReplay(strings.NewReader(input)); err == nil {
				t.Fatal("null timing accepted")
			}
		})
	}
	// Numeric zero is valid at process and Unix epochs, including query bounds.
	replay, err := ReadTelemetryReplay(strings.NewReader(base))
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range replay.Records {
		if s := record.Sample; s != nil {
			s.QueryStarted -= 1000
			s.QueryFinished -= 1001
			*s.SourceAt -= 1000
		}
	}
	var buf bytes.Buffer
	for _, record := range replay.Records {
		if err := json.NewEncoder(&buf).Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	zero, err := ReadTelemetryReplay(&buf)
	if err != nil {
		t.Fatal(err)
	}
	window, _, err := ReplayTelemetry(zero, 0, 2000, EnergyOptions{MaxGap: 1.5})
	if err != nil {
		t.Fatal(err)
	}
	if window.Devices[0].Unavailable != "" || *window.Devices[0].EnergyEnd-*window.Devices[0].EnergyStart != 8000 {
		t.Fatalf("explicit zero changed admission: %+v", window)
	}
}
