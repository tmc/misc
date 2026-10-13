package llmperf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

type (
	rootModule struct {
		controls        controlGroups
		run             runState
		limiters        prefillLimiters
		requestLimiters prefillLimiters
		transport       *http.Transport
		transportOnce   sync.Once
		transportMu     sync.Mutex
		transports      map[int]*http.Transport
	}

	moduleInstance struct {
		root    *rootModule
		vu      modules.VU
		metrics *LLMPerfMetrics
		exports *sobek.Object
	}
)

var (
	_ modules.Module   = &rootModule{}
	_ modules.Instance = &moduleInstance{}
)

func (root *rootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	rt := vu.Runtime()
	metricSet, err := newMetrics(vu.InitEnv().Registry)
	if err != nil {
		common.Throw(rt, err)
	}

	mi := &moduleInstance{
		root:    root,
		vu:      vu,
		metrics: metricSet,
		exports: rt.NewObject(),
	}
	mi.metrics.vu = vu
	if state := vu.State(); state != nil {
		mi.metrics.samples = state.Samples
	}

	must := func(err error) {
		if err != nil {
			common.Throw(rt, err)
		}
	}
	constructor := func(call sobek.ConstructorCall) *sobek.Object {
		var config Config
		if err := exportValue(call.Argument(0), &config); err != nil {
			common.Throw(rt, fmt.Errorf("invalid config: %w", err))
		}
		if config.MaxConnsPerHost < 0 || config.MaxConnsPerHost > 65536 || config.UseK6Transport && config.MaxConnsPerHost != 0 {
			common.Throw(rt, fmt.Errorf("maxConnsPerHost must be 0..65536 and excludes useK6Transport"))
		}
		client := &Client{
			controls: &root.controls,
			config:   config, metrics: mi.metrics, vu: mi.vu,
			transport: mi.root.transportFor(config.MaxConnsPerHost), limiters: &mi.root.limiters, requestLimiters: &mi.root.requestLimiters,
			warmupRemaining: int64(config.WarmupCount),
			run:             &mi.root.run,
		}
		completions := rt.NewObject()
		must(completions.Set("create", func(call sobek.FunctionCall) sobek.Value {
			result := client.Complete(call)
			if err, ok := result.Export().(error); ok {
				value := rt.NewGoError(err)
				if client.measurement != nil {
					record, err := jsonValue(rt, client.measurement)
					must(err)
					must(value.Set("measurement", record))
				}
				return value
			}
			return result
		}))
		chat := rt.NewObject()
		must(chat.Set("completions", completions))
		must(call.This.Set("chat", chat))
		must(call.This.Set("request", func(call sobek.FunctionCall) sobek.Value {
			var opts struct {
				Multipart   *RawMultipart    `json:"multipart"`
				Poll        *VideoPolling    `json:"poll"`
				Stream      *bool            `json:"stream"`
				ScheduledAt *float64         `json:"scheduledAt"`
				CancelAfter string           `json:"cancelAfter"`
				Path        string           `json:"path"`
				Protocol    string           `json:"protocol"`
				Body        map[string]any   `json:"body"`
				Measurement *TokenCounts     `json:"measurement"`
				Work        *WorkExpectation `json:"work"`
			}
			if err := exportValue(call.Argument(0), &opts); err != nil {
				return rt.NewGoError(err)
			}
			value, err := jsonValue(rt, CompletionRequest{ScheduledAt: opts.ScheduledAt, CancelAfter: opts.CancelAfter, Raw: &RawRequest{Multipart: opts.Multipart, Poll: opts.Poll, Stream: opts.Stream, Path: opts.Path, Protocol: opts.Protocol, Body: opts.Body}, Measurement: opts.Measurement, Work: opts.Work})
			must(err)
			result := client.Complete(sobek.FunctionCall{Arguments: []sobek.Value{value}})
			if err, ok := result.Export().(error); ok {
				value := rt.NewGoError(err)
				if client.measurement != nil {
					record, err := jsonValue(rt, client.measurement)
					must(err)
					must(value.Set("measurement", record))
				}
				return value
			}
			return result
		}))
		return nil
	}
	must(mi.exports.Set("Client", constructor))
	must(mi.exportRuntimeTemplate())
	must(mi.exports.Set("setControlLimits", func(name string, v sobek.Value) {
		var limits ControlLimits
		must(exportValue(v, &limits))
		must(root.controls.set(name, limits))
	}))
	must(mi.exports.Set("controlSnapshot", func(name string) sobek.Value {
		snapshot, err := root.controls.snapshot(name)
		must(err)
		value, err := jsonValue(rt, snapshot)
		must(err)
		return value
	}))
	must(mi.exports.Set("renderRequestTemplate", func(v sobek.Value) sobek.Value {
		var spec RequestTemplate
		must(exportValue(v, &spec))
		rendered, err := RenderRequestTemplate(spec)
		must(err)
		value, err := jsonValue(rt, rendered)
		must(err)
		return value
	}))
	must(mi.exports.Set("calibrateHTTP", func(v sobek.Value) sobek.Value {
		must(root.run.calibrationAllowed())
		var opts CalibrationOptions
		must(exportValue(v, &opts))
		opts.TransportSource = "extension_shared_transport"
		report, err := CalibrateHTTP(vu.Context(), &http.Client{Transport: root.sharedTransport()}, opts)
		if err != nil {
			failure := rt.NewGoError(err)
			value, encodeErr := jsonValue(rt, report)
			must(encodeErr)
			must(failure.Set("calibration", value))
			panic(failure)
		}
		value, err := jsonValue(rt, report)
		must(err)
		return value
	}))
	must(mi.exports.Set("calibrateTCP", func(v sobek.Value) sobek.Value {
		must(root.run.calibrationAllowed())
		var opts CalibrationOptions
		must(exportValue(v, &opts))
		report, err := CalibrateTCP(vu.Context(), opts)
		if err != nil {
			failure := rt.NewGoError(err)
			value, encodeErr := jsonValue(rt, report)
			must(encodeErr)
			must(failure.Set("calibration", value))
			panic(failure)
		}
		value, err := jsonValue(rt, report)
		must(err)
		return value
	}))
	must(mi.exports.Set("pipelineSnapshot", func() sobek.Value {
		value, err := jsonValue(rt, mi.metrics.PipelineSnapshot())
		must(err)
		return value
	}))
	must(mi.exports.Set("resetPipeline", func() {
		must(mi.metrics.ResetPipeline())
	}))
	must(mi.exports.Set("beginMeasurement", func(v sobek.Value) {
		var opts RunOptions
		if err := exportValue(v, &opts); err != nil {
			common.Throw(rt, err)
		}
		if err := mi.root.run.begin(opts); err != nil {
			common.Throw(rt, err)
		}
	}))
	must(mi.exports.Set("endMeasurement", func() sobek.Value {
		s, err := mi.root.run.finish()
		if err != nil {
			common.Throw(rt, err)
		}
		if observed := s.Observation; observed != nil {
			mi.metrics.RecordMetric("llm_content_span_windows", 1)
			mi.metrics.RecordMetric("llm_content_span_goodput", observed.Goodput)
			mi.metrics.RecordMetric("llm_content_span_duration_ms", observed.Duration*1000)
			mi.metrics.RecordMetric("llm_content_span_good_requests", float64(observed.GoodRequests))
			mi.metrics.RecordMetric("llm_content_span_attempts", float64(observed.Attempts))
			if v := observed.InputThroughput; v != nil {
				mi.metrics.RecordMetric("llm_content_span_input_throughput", *v)
			}
			if v := observed.OutputThroughput; v != nil {
				mi.metrics.RecordMetric("llm_content_span_output_throughput", *v)
			}
		}
		value, err := jsonValue(rt, s)
		must(err)
		return value
	}))
	must(mi.exports.Set("measurementSummary", func() sobek.Value {
		s, err := mi.root.run.snapshot()
		must(err)
		value, err := jsonValue(rt, s)
		must(err)
		return value
	}))

	return mi
}

// jsonValue preserves JSON names and scalar nullability at the JS boundary.
func jsonValue(rt *sobek.Runtime, v any) (sobek.Value, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode result: %w", err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode result: %w", err)
	}
	return rt.ToValue(value), nil
}

func (root *rootModule) sharedTransport() *http.Transport {
	root.transportOnce.Do(func() {
		root.transport = newTransport()
	})
	return root.transport
}

// transportFor shares each explicitly bounded connection pool across VUs.
func (root *rootModule) transportFor(limit int) *http.Transport {
	if limit == 0 {
		return root.sharedTransport()
	}
	root.transportMu.Lock()
	defer root.transportMu.Unlock()
	if root.transports == nil {
		root.transports = make(map[int]*http.Transport)
	}
	if root.transports[limit] == nil {
		transport := newTransport()
		transport.MaxConnsPerHost = limit
		transport.MaxIdleConnsPerHost = min(limit, transport.MaxIdleConnsPerHost)
		root.transports[limit] = transport
	}
	return root.transports[limit]
}

func exportValue(v sobek.Value, dst interface{}) error {
	if v == nil || sobek.IsUndefined(v) || sobek.IsNull(v) {
		return nil
	}
	data, err := json.Marshal(v.Export())
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("unmarshal value: %w", err)
	}
	return nil
}

func (mi *moduleInstance) Exports() modules.Exports {
	return modules.Exports{
		Default: mi.exports,
		Named:   map[string]interface{}{"Client": mi.exports.Get("Client"), "setControlLimits": mi.exports.Get("setControlLimits"), "controlSnapshot": mi.exports.Get("controlSnapshot"), "renderRequestTemplate": mi.exports.Get("renderRequestTemplate"), "evaluateRuntimeTemplate": mi.exports.Get("evaluateRuntimeTemplate"), "calibrateHTTP": mi.exports.Get("calibrateHTTP"), "calibrateTCP": mi.exports.Get("calibrateTCP"), "beginMeasurement": mi.exports.Get("beginMeasurement"), "endMeasurement": mi.exports.Get("endMeasurement"), "measurementSummary": mi.exports.Get("measurementSummary"), "pipelineSnapshot": mi.exports.Get("pipelineSnapshot"), "resetPipeline": mi.exports.Get("resetPipeline")},
	}
}
