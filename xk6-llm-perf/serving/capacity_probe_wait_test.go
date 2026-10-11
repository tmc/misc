package serving

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This is a synthetic structural archive, not native capacity evidence. The
// source is the independent peer's hash-consistent two-arrival negative control.
func TestCapacityProbePipelineWaitEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		wait any
		omit bool
		want string
		bad  bool
	}{
		{name: "missing", omit: true, bad: true},
		{name: "null", wait: nil, bad: true},
		{name: "measured zero", wait: 0, want: "SAMPLED_CAPACITY"},
		{name: "over bound", wait: 1000000, want: "UNQUALIFIED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			entries, err := os.ReadDir("testdata/pipeline-wait")
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				data, err := os.ReadFile(filepath.Join("testdata/pipeline-wait", entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(directory, entry.Name()), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(filepath.Join(directory, "rows.json"))
			if err != nil {
				t.Fatal(err)
			}
			var rows []map[string]any
			if err = json.Unmarshal(data, &rows); err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				pipeline := row["pipeline"].(map[string]any)
				if tc.omit {
					delete(pipeline, "max_wait_nanoseconds")
				} else {
					pipeline["max_wait_nanoseconds"] = tc.wait
				}
			}
			data, err = json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(directory, "rows.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(data)
			receiptPath := filepath.Join(directory, "receipt.json")
			receiptBytes, err := os.ReadFile(receiptPath)
			if err != nil {
				t.Fatal(err)
			}
			var receipt map[string]any
			if err = json.Unmarshal(receiptBytes, &receipt); err != nil {
				t.Fatal(err)
			}
			receipt["rows_sha256"] = hex.EncodeToString(sum[:])
			receiptBytes, err = json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(receiptPath, receiptBytes, 0600); err != nil {
				t.Fatal(err)
			}
			result, err := EvaluateVerifiedProbe(directory, strings.Repeat("a", 64), "fixture", CapacityBounds{MaxCPUPercent: 100, MaxRSSBytes: 100, MaxConnectionFraction: .5, MaxLagMilliseconds: 10, MaxPipelineWaitMilliseconds: .001})
			if (err != nil) != tc.bad {
				t.Fatalf("result %+v error %v", result, err)
			}
			if tc.bad {
				if !strings.Contains(err.Error(), "pipeline maximum wait unavailable") {
					t.Fatalf("unexpected refusal %v", err)
				}
				return
			}
			if result.Status != tc.want || result.Evidence.PipelineMaxWaitMilliseconds == nil {
				t.Fatalf("result %+v", result)
			}
			if tc.name == "measured zero" && *result.Evidence.PipelineMaxWaitMilliseconds != 0 {
				t.Fatal("measured zero lost")
			}
		})
	}
}
