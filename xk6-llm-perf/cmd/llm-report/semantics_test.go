package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIPerfStatisticsFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.json")
	data := `{"schema_version":1,"started_at":1000,"finished_at":2000,"duration":1,"attempts":0,"successes":0,"failures":0,"good_requests":0,"records":[]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	for _, aiperf := range []bool{false, true} {
		args := []string{"-input", path}
		if aiperf {
			args = append(args, "-aiperf-statistics")
		}
		var out bytes.Buffer
		if err := run(args, &out); err != nil {
			t.Fatal(err)
		}
		want := "nearest rank"
		if aiperf {
			want = "linear interpolation at (n-1)*q"
		}
		if !strings.Contains(out.String(), want) {
			t.Fatal(out.String())
		}
		want = "completion in [start,end)"
		if aiperf {
			want = "dispatch start in [start,end)"
		}
		if !strings.Contains(out.String(), want) {
			t.Fatal(out.String())
		}
	}
}
