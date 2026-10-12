package llmperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestBinaryAnthropicCacheBuckets(t *testing.T) {
	binary := os.Getenv("K6_SMOKE_BINARY")
	if binary == "" {
		t.Skip("set K6_SMOKE_BINARY")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":2,\"cache_read_input_tokens\":3}}}\n\n")
			fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n")
			fmt.Fprint(w, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5,\"cache_creation_input_tokens\":4}}\n\ndata: {\"type\":\"message_stop\"}\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":2,"output_tokens":5,"cache_read_input_tokens":3,"cache_creation_input_tokens":4}}`)
	}))
	defer server.Close()
	dir := t.TempDir()
	runPath := filepath.Join(dir, "run.json")
	script := fmt.Sprintf(`import llm from 'k6/x/llm-perf';import {check} from 'k6';
export const options={vus:1,iterations:1,thresholds:{checks:['rate==1'],llm_cache_write_tokens:['count==8'],llm_cached_tokens:['count==6']}};
const c=new llm.Client({baseURL:%q,strictMeasurement:true});
export default function(){llm.beginMeasurement({runId:'cache-buckets',maxRecords:10});
for(const stream of [false,true]){const r=c.request({protocol:'anthropic',path:stream?'/stream':'/sync',stream,body:{messages:[{role:'user',content:'hello'}]}});check(r,{'cache sum':r=>!(r instanceof Error)&&r.measurement.outcome==='success'&&r.measurement.input_tokens.value===9&&r.measurement.cached_tokens===3&&r.measurement.cache_write_tokens===4&&r.usage.cache_write_tokens===4});}llm.endMeasurement();}
export function handleSummary(){return {[%q]:JSON.stringify(llm.measurementSummary())};}`, server.URL, runPath)
	path := filepath.Join(dir, "cache.js")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--no-color", path)
	cmd.Env = append(os.Environ(), "K6_NO_USAGE_REPORT=true", "K6_AUTO_EXTENSION_RESOLUTION=false")
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(runPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	run, err := ReadRun(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Records) != 2 {
		t.Fatalf("records=%d", len(run.Records))
	}
	for _, m := range run.Records {
		if !sameCount(m.CacheWriteTokens, intPtr(4)) || m.InputTokens.Value == nil || *m.InputTokens.Value != 9 {
			t.Fatalf("receipt=%+v", m)
		}
	}
	// Replay the same records with the additive field omitted, as in old receipts.
	for i := range run.Records {
		run.Records[i].CacheWriteTokens = nil
	}
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]any
	if err := json.Unmarshal(data, &old); err != nil {
		t.Fatal(err)
	}
	for _, record := range old["records"].([]any) {
		delete(record.(map[string]any), "cache_write_tokens")
	}
	data, err = json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRun(bytes.NewReader(data)); err != nil {
		t.Fatalf("old receipt: %v", err)
	}
}
