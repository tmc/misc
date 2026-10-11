package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func testServingManifest() ServingManifest {
	digest := strings.Repeat("a", 64)
	m := ServingManifest{SchemaVersion: 1, ModelDigest: digest, TokenizerDigest: digest, TemplateDigest: digest, WorkloadDigest: digest, EnvironmentDigest: digest, Settings: map[string]ServingSetting{}}
	for _, name := range servingSettingNames {
		value := `{"enabled":false}`
		switch name {
		case "temperature", "top_p", "top_k", "seed", "max_output_tokens":
			value = `0`
		case "dtype", "device":
			value = `"test"`
		}
		m.Settings[name] = ServingSetting{Effective: json.RawMessage(value), EvidenceDigest: digest}
	}
	return m
}

func TestServingManifestHash(t *testing.T) {
	m := testServingManifest()
	a, err := ServingWorkHash(m)
	if err != nil {
		t.Fatal(err)
	}
	m.Arm = ServingArm{Backend: "different", BinaryDigest: strings.Repeat("b", 64)}
	s := m.Settings["temperature"]
	s.Configured = json.RawMessage(`1`)
	s.EvidenceDigest = strings.Repeat("b", 64)
	m.Settings["temperature"] = s
	b, err := ServingWorkHash(m)
	if err != nil || a != b {
		t.Fatalf("arm/configured/evidence changed work: %s %v", b, err)
	}
	s.Effective = json.RawMessage(`1`)
	m.Settings["temperature"] = s
	b, err = ServingWorkHash(m)
	if err != nil || a == b {
		t.Fatal("effective setting change not bound")
	}
	m.Confounds = []string{"different hardware"}
	if _, err := ServingWorkHash(m); err == nil {
		t.Fatal("confound qualified")
	}
}

func TestServingManifestUnknown(t *testing.T) {
	m := ServingManifest{SchemaVersion: 1}
	if err := ValidateServingManifest(m); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(m)
	if _, err := ReadServingManifest(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if _, err := ServingWorkHash(m); err == nil {
		t.Fatal("unknown qualified")
	}
}

func TestServingManifestStrict(t *testing.T) {
	for _, input := range []string{`null`, `{"schema_version":1,"schema_version":1}`, `{"schema_version":1,"secret":"token"}`, `{"schema_version":1,"settings":{"temperature":{"effective":null}}}`, `{"schema_version":1,"settings":{"temperature":{"effective":"zero"}}}`, `{"schema_version":1} {}`, strings.Repeat(" ", 65537)} {
		if _, err := ReadServingManifest(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	m := testServingManifest()
	data, _ := json.Marshal(m)
	decoded, err := ReadServingManifest(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := ServingWorkHash(m)
	b, _ := ServingWorkHash(*decoded)
	if a != b {
		t.Fatal("roundtrip hash changed")
	}
}

func TestRunManifestClone(t *testing.T) {
	m := testServingManifest()
	var run runState
	if err := run.begin(RunOptions{ServingManifest: &m}); err != nil {
		t.Fatal(err)
	}
	m.ModelDigest = "tampered"
	if run.summary.ServingManifest.ModelDigest == "tampered" {
		t.Fatal("caller modified bound manifest")
	}
}

func ExampleReadServingManifest() {
	m, err := ReadServingManifest(strings.NewReader(`{"schema_version":1}`))
	fmt.Println(m.SchemaVersion, err) // Output: 1 <nil>
}
func ExampleValidateServingManifest() {
	fmt.Println(ValidateServingManifest(ServingManifest{SchemaVersion: 1})) // Output: <nil>
}
func ExampleServingWorkHash() {
	_, err := ServingWorkHash(ServingManifest{SchemaVersion: 1})
	fmt.Println(err) // Output: serving work identity unavailable
}

func TestRunServingManifestReplayAndMerge(t *testing.T) {
	a, b := workerFixture("a", 0), workerFixture("b", 100)
	m := testServingManifest()
	a.Run.ServingManifest = &m
	b.Run.ServingManifest = &m
	merged, err := MergeRuns([]WorkerRun{a, b}, MergeOptions{ExpectedWorkers: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteRun(&buf, merged); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReadRun(&buf)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := ServingWorkHash(*replayed.ServingManifest)
	expected, _ := ServingWorkHash(m)
	if hash != expected {
		t.Fatal("manifest lost in run replay")
	}
	changed := testServingManifest()
	changed.Arm.Backend = "other"
	b.Run.ServingManifest = &changed
	if _, err := MergeRuns([]WorkerRun{a, b}, MergeOptions{ExpectedWorkers: []string{"a", "b"}}); err == nil {
		t.Fatal("mixed arms merged")
	}
	b.Run.ServingManifest = nil
	if _, err := MergeRuns([]WorkerRun{a, b}, MergeOptions{ExpectedWorkers: []string{"a", "b"}}); err == nil {
		t.Fatal("unknown manifest merged with known")
	}
	m.ModelDigest = "tampered"
	if err := WriteRun(&buf, a.Run); err == nil {
		t.Fatal("invalid direct manifest exported")
	}
}

func TestServingEndpointIdentity(t *testing.T) {
	for _, value := range []string{"https://user:secret@host/path?token=secret", "host/path", "token=secret", "line\nnext"} {
		m := ServingManifest{SchemaVersion: 1, Arm: ServingArm{EndpointIdentity: value}}
		if err := ValidateServingManifest(m); err == nil {
			t.Fatalf("accepted endpoint %q", value)
		}
	}
	if err := ValidateServingManifest(ServingManifest{SchemaVersion: 1, Arm: ServingArm{EndpointIdentity: "server-a.local_1"}}); err != nil {
		t.Fatal(err)
	}
}

func TestServingManifestExactNumbers(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		valid       bool
	}{
		{"seed", "1.0000000000000000000001", false},
		{"top_k", "1.0000000000000000000001", false},
		{"max_output_tokens", "1000000000.00000000001", false},
		{"top_p", "1.0000000000000000000001", false},
		{"temperature", "-1e-999", false},
		{"temperature", "-0.0000000000000000000001", false},
		{"seed", "9007199254740991.00000000001", false},
		{"seed", "-9007199254740991.00000000001", false},
		{"seed", "-9007199254740991", true},
		{"seed", "9007199254740991", true},
		{"seed", "1.0e2", true},
		{"seed", "1e-999", false},
		{"top_k", "1e9", true},
		{"max_output_tokens", "1000000000.0", true},
		{"top_p", "0.9999999999999999999999", true},
		{"top_p", "1e0", true},
		{"temperature", "1e-999", true},
		{"temperature", "1e309", false},
		{"temperature", "1e999999999999999999", false},
		{"temperature", "1e-999999999999999999", false},
		{"temperature", "0e999999999999999999", true},
		{"temperature", "-0e-999999999999999999", true},
		{"top_p", "\"0\"", false},
		{"top_k", "0.0", true},
	} {
		t.Run(tc.name+"/"+tc.value, func(t *testing.T) {
			m := testServingManifest()
			s := m.Settings[tc.name]
			s.Effective = json.RawMessage(tc.value)
			m.Settings[tc.name] = s
			_, err := ServingWorkHash(m)
			if (err == nil) != tc.valid {
				t.Fatalf("value %s error %v; valid=%v", tc.value, err, tc.valid)
			}
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ReadServingManifest(bytes.NewReader(data))
			if (err == nil) != tc.valid {
				t.Fatalf("reader error %v; valid=%v", err, tc.valid)
			}
		})
	}
}
