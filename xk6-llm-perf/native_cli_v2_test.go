package llmperf

import (
	"bytes"
	"os"
	"testing"
)

func TestNativeTelemetryFrozenCLIExport(t *testing.T) {
	data, err := os.ReadFile("testdata/nvtelemetry-v2/frozen_cli_mock.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	replay, err := ReadNativeTelemetry(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if replay.SourceHash != "sha256:1667a0f3ee86763d5f9e14b64f7a6728ed72122335af78a70f33ac8f2a7dda74" || len(replay.Samples) != 5 || replay.Final == nil || !replay.Final.IsComplete {
		t.Fatalf("frozen producer export changed: hash=%s samples=%d final=%+v", replay.SourceHash, len(replay.Samples), replay.Final)
	}
	if !bytes.Equal(data, replay.Raw) || replay.Header.SchemaVersion != 2 {
		t.Fatal("raw native v2 evidence changed")
	}
	found := false
	for _, d := range replay.Diagnostics {
		found = found || d == "clock_mapping_proof_incomplete"
	}
	if !found {
		t.Fatal("missing clock mapping admission limit")
	}
}
