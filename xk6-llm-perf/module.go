package llmperf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/llm-perf", new(RootModule))
}

type (
	RootModule struct {
		limiters      prefillLimiters
		transport     *http.Transport
		transportOnce sync.Once
	}

	ModuleInstance struct {
		root    *RootModule
		vu      modules.VU
		metrics *LLMPerfMetrics
		exports *sobek.Object
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

func (root *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	rt := vu.Runtime()

	mi := &ModuleInstance{
		root:    root,
		vu:      vu,
		metrics: NewMetrics(vu.InitEnv().Registry),
		exports: rt.NewObject(),
	}
	if state := vu.State(); state != nil {
		mi.metrics.samples = state.Samples
	}

	// Create the Client class
	clientClass := rt.NewObject()
	must := func(err error) {
		if err != nil {
			common.Throw(rt, err)
		}
	}

	// Define constructor function
	must(clientClass.DefineDataProperty(
		"constructor",
		rt.ToValue(func(call sobek.ConstructorCall) *sobek.Object {
			// Parse config from constructor argument
			var config Config
			if err := exportValue(call.Argument(0), &config); err != nil {
				common.Throw(rt, fmt.Errorf("invalid config: %w", err))
			}

			client := &Client{
				config:          config,
				metrics:         mi.metrics,
				vu:              mi.vu,
				transport:       mi.root.sharedTransport(),
				limiters:        &mi.root.limiters,
				warmupRemaining: int32(config.WarmupCount),
			}

			// Create chat object
			chatObj := rt.NewObject()
			completionsObj := rt.NewObject()

			// Set up completions.create method
			must(completionsObj.DefineDataProperty(
				"create",
				rt.ToValue(client.Complete),
				sobek.FLAG_FALSE,
				sobek.FLAG_FALSE,
				sobek.FLAG_TRUE,
			))

			// Set up chat.completions
			must(chatObj.DefineDataProperty(
				"completions",
				completionsObj,
				sobek.FLAG_FALSE,
				sobek.FLAG_FALSE,
				sobek.FLAG_TRUE,
			))

			// Add chat property to instance
			must(call.This.DefineDataProperty(
				"chat",
				chatObj,
				sobek.FLAG_FALSE,
				sobek.FLAG_FALSE,
				sobek.FLAG_TRUE,
			))

			return call.This
		}),
		sobek.FLAG_FALSE,
		sobek.FLAG_FALSE,
		sobek.FLAG_TRUE,
	))

	// Add Client to exports
	must(mi.exports.DefineDataProperty(
		"Client",
		clientClass.Get("constructor"),
		sobek.FLAG_FALSE,
		sobek.FLAG_FALSE,
		sobek.FLAG_TRUE,
	))

	return mi
}

func (root *RootModule) sharedTransport() *http.Transport {
	root.transportOnce.Do(func() {
		root.transport = newTransport()
	})
	return root.transport
}

func exportValue(v sobek.Value, dst interface{}) error {
	data, err := json.Marshal(v.Export())
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("unmarshal value: %w", err)
	}
	return nil
}

func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Default: mi.exports,
	}
}
