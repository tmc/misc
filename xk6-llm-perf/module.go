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
		run             runState
		limiters        prefillLimiters
		requestLimiters prefillLimiters
		transport       *http.Transport
		transportOnce   sync.Once
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
		client := &Client{
			config: config, metrics: mi.metrics, vu: mi.vu,
			transport: mi.root.sharedTransport(), limiters: &mi.root.limiters, requestLimiters: &mi.root.requestLimiters,
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
				ScheduledAt *float64       `json:"scheduledAt"`
				CancelAfter string         `json:"cancelAfter"`
				Path        string         `json:"path"`
				Protocol    string         `json:"protocol"`
				Body        map[string]any `json:"body"`
				Measurement *TokenCounts   `json:"measurement"`
			}
			if err := exportValue(call.Argument(0), &opts); err != nil {
				return rt.NewGoError(err)
			}
			value, err := jsonValue(rt, CompletionRequest{ScheduledAt: opts.ScheduledAt, CancelAfter: opts.CancelAfter, Raw: &RawRequest{Path: opts.Path, Protocol: opts.Protocol, Body: opts.Body}, Measurement: opts.Measurement})
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
		Named:   map[string]interface{}{"Client": mi.exports.Get("Client"), "beginMeasurement": mi.exports.Get("beginMeasurement"), "endMeasurement": mi.exports.Get("endMeasurement"), "measurementSummary": mi.exports.Get("measurementSummary")},
	}
}
