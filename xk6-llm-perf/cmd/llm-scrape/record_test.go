package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/tmc/misc/xk6-llm-perf/servermetrics"
)

func TestRecordCommand(t *testing.T) {
	var n atomic.Int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "# TYPE requests_total counter\nrequests_total %d\n", n.Add(1))
	}))
	defer s.Close()
	path := filepath.Join(t.TempDir(), "targets.json")
	data, _ := json.Marshal([]string{s.URL})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"-targets", path, "-duration", "60ms", "-interval", "10ms"}, &out); err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(&out)
	var first, last servermetrics.Observation
	count := 0
	for d.More() {
		var o servermetrics.Observation
		if err := d.Decode(&o); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			first = o
		}
		last = o
		count++
	}
	if count < 2 || first.Scrape == nil || last.Scrape == nil || len(last.Targets) != 1 {
		t.Fatalf("observations %d", count)
	}
}
