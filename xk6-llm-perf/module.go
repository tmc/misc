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
		limiters      prefillLimiters
		transport     *http.Transport
		transportOnce sync.Once
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
			transport: mi.root.sharedTransport(), limiters: &mi.root.limiters,
			warmupRemaining: int64(config.WarmupCount),
		}
		completions := rt.NewObject()
		must(completions.Set("create", func(call sobek.FunctionCall) sobek.Value {
			result := client.Complete(call)
			if err, ok := result.Export().(error); ok {
				return rt.NewGoError(err)
			}
			return result
		}))
		chat := rt.NewObject()
		must(chat.Set("completions", completions))
		must(call.This.Set("chat", chat))
		return nil
	}
	must(mi.exports.Set("Client", constructor))

	return mi
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
		Named:   map[string]interface{}{"Client": mi.exports.Get("Client")},
	}
}
