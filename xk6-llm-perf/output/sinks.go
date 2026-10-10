package output

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	llm "github.com/tmc/misc/xk6-llm-perf"
	collector "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	metrics "go.opentelemetry.io/proto/otlp/metrics/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// OTLP encodes a standard OTLP/HTTP JSON metrics request. Milliseconds are
// converted to seconds. Each available observation is an individual gauge point;
// this post-run adapter does not claim live streaming or histogram aggregation.
func OTLP(run llm.RunSummary) ([]byte, error) {
	if _, err := Summarize(run); err != nil {
		return nil, err
	}
	if run.StartedAt < 0 || run.FinishedAt > float64(math.MaxInt64)/1e6 {
		return nil, fmt.Errorf("OTLP timestamp out of range")
	}
	attr := func(k, v string) *common.KeyValue {
		return &common.KeyValue{Key: k, Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: v}}}
	}
	group := map[string]*metrics.Metric{}
	add := func(name, unit string, v float64, r llm.Measurement, extra ...*common.KeyValue) {
		m := group[name]
		if m == nil {
			m = &metrics.Metric{Name: name, Unit: unit, Data: &metrics.Metric_Gauge{Gauge: &metrics.Gauge{}}}
			group[name] = m
		}
		a := []*common.KeyValue{attr("gen_ai.request.model", r.Model), attr("llmperf.outcome", r.Outcome), attr("llmperf.request.id", r.RequestID), attr("llmperf.phase", r.Phase), attr("llmperf.protocol", r.Protocol)}
		a = append(a, extra...)
		m.GetGauge().DataPoints = append(m.GetGauge().DataPoints, &metrics.NumberDataPoint{TimeUnixNano: uint64(r.FinishedAt * 1e6), Attributes: a, Value: &metrics.NumberDataPoint_AsDouble{AsDouble: v}})
	}
	for _, r := range run.Records {
		for _, v := range []*float64{&r.FullResponseLatency, r.TTFT, r.ITL} {
			if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0) {
				return nil, fmt.Errorf("invalid OTLP observation")
			}
		}
		add("gen_ai.client.operation.duration", "s", r.FullResponseLatency/1000, r)
		if r.TTFT != nil {
			add("gen_ai.client.operation.time_to_first_chunk", "s", *r.TTFT/1000, r)
		}
		if r.ITL != nil {
			add("llmperf.client.time_per_output_token", "s", *r.ITL/1000, r)
		}
		for _, c := range []struct {
			kind  string
			count llm.TokenCount
		}{{"input", r.InputTokens}, {"output", r.OutputTokens}} {
			if c.count.Value != nil {
				add("gen_ai.client.token.usage", "{token}", float64(*c.count.Value), r, attr("gen_ai.token.type", c.kind), attr("llmperf.token.source", c.count.Source))
			}
		}
	}
	var list []*metrics.Metric
	for _, name := range []string{"gen_ai.client.operation.duration", "gen_ai.client.operation.time_to_first_chunk", "llmperf.client.time_per_output_token", "gen_ai.client.token.usage"} {
		if m := group[name]; m != nil {
			list = append(list, m)
		}
	}
	req := &collector.ExportMetricsServiceRequest{ResourceMetrics: []*metrics.ResourceMetrics{{Resource: &resource.Resource{Attributes: []*common.KeyValue{attr("service.name", "xk6-llm-perf"), attr("llmperf.run.id", run.RunID)}}, ScopeMetrics: []*metrics.ScopeMetrics{{Scope: &common.InstrumentationScope{Name: "llmperf.postrun", Version: "1"}, Metrics: list}}}}}
	return protojson.Marshal(req)
}

// MLflowBatch encodes the documented log-batch request for an existing run.
// Missing metrics are absent and units are logged as separate immutable params.
func MLflowBatch(d Document, runID string, timestamp int64) ([]byte, error) {
	if runID == "" || len(runID) > 256 || timestamp < 0 {
		return nil, fmt.Errorf("invalid MLflow run identity or timestamp")
	}
	type metric struct {
		Key       string  `json:"key"`
		Value     float64 `json:"value"`
		Timestamp int64   `json:"timestamp"`
		Step      int64   `json:"step"`
	}
	type param struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	v := struct {
		RunID   string   `json:"run_id"`
		Metrics []metric `json:"metrics"`
		Params  []param  `json:"params"`
		Tags    []param  `json:"tags"`
	}{RunID: runID, Metrics: []metric{}, Params: []param{}, Tags: []param{{"llmperf.schema_version", "1"}}}
	for _, s := range d.Metrics {
		v.Params = append(v.Params, param{"unit." + s.Name, s.Unit})
		for _, p := range []struct {
			name string
			v    *float64
		}{{"mean", s.Mean}, {"min", s.Min}, {"max", s.Max}, {"p50", s.P50}, {"p90", s.P90}, {"p99", s.P99}, {"std", s.Std}} {
			if p.v != nil {
				v.Metrics = append(v.Metrics, metric{s.Name + "." + p.name, *p.v, timestamp, 0})
			}
		}
	}
	return json.Marshal(v)
}

// Send posts an explicitly prepared bounded JSON payload. Credentials are only
// HTTP headers; redirects are refused and response bodies are never echoed.
func Send(ctx context.Context, client *http.Client, endpoint, authorization string, data []byte) error {
	if len(data) > 32<<20 {
		return fmt.Errorf("sink payload exceeds 32 MiB")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("invalid sink endpoint")
	}
	if strings.ContainsAny(authorization, "\r\n") {
		return fmt.Errorf("invalid authorization header")
	}
	if client == nil {
		return fmt.Errorf("nil sink HTTP client")
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create sink request")
	}
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := copyClient.Do(req)
	if err != nil {
		return fmt.Errorf("sink transport failed")
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil || len(response) > 1<<20 {
		return fmt.Errorf("sink response exceeds budget or read failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sink returned HTTP %d", resp.StatusCode)
	}
	if strings.HasSuffix(u.Path, "/v1/metrics") {
		var result collector.ExportMetricsServiceResponse
		if err := protojson.Unmarshal(response, &result); err != nil {
			return fmt.Errorf("invalid OTLP response")
		}
		if result.PartialSuccess != nil && (result.PartialSuccess.RejectedDataPoints != 0 || result.PartialSuccess.ErrorMessage != "") {
			return fmt.Errorf("OTLP partial acceptance")
		}
	}
	return nil
}
