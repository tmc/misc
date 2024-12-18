package llmperf

import (
	"fmt"
	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/llm-perf", new(RootModule))
}

type (
	RootModule struct{}

	ModuleInstance struct {
		vu      modules.VU
		metrics *LLMPerfMetrics
		exports *sobek.Object
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	rt := vu.Runtime()

	mi := &ModuleInstance{
		vu:      vu,
		metrics: NewMetrics(vu.InitEnv().Registry),
		exports: rt.NewObject(),
	}

	// Create chat object
	chatObj := rt.NewObject()
	completionsObj := rt.NewObject()

	must := func(err error) {
		if err != nil {
			common.Throw(rt, err)
		}
	}

	// Set up completions.create method
	must(completionsObj.DefineDataProperty(
		"create",
		rt.ToValue(mi.createCompletion),
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

	// Set up constructor
	must(mi.exports.DefineDataProperty(
		"constructor",
		rt.ToValue(mi.constructor),
		sobek.FLAG_FALSE,
		sobek.FLAG_FALSE,
		sobek.FLAG_TRUE,
	))

	// Add chat property to instance
	must(mi.exports.DefineDataProperty(
		"chat",
		chatObj,
		sobek.FLAG_FALSE,
		sobek.FLAG_FALSE,
		sobek.FLAG_TRUE,
	))

	return mi
}

func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Default: mi.exports,
	}
}

// constructor implements the OpenAI constructor
func (mi *ModuleInstance) constructor(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()

	var config ClientConfig
	if err := rt.ExportTo(call.Argument(0), &config); err != nil {
		common.Throw(rt, fmt.Errorf("invalid client configuration: %w", err))
	}

	return mi.exports
}

// createCompletion implements chat.completions.create
func (mi *ModuleInstance) createCompletion(call sobek.FunctionCall) sobek.Value {
	rt := mi.vu.Runtime()

	var req CompletionRequest
	if err := rt.ExportTo(call.Argument(0), &req); err != nil {
		return rt.ToValue(fmt.Errorf("invalid completion request: %w", err))
	}

	client := &Client{
		config: ClientConfig{
			BaseURL: req.BaseURL,
		},
		metrics: mi.metrics,
		vu:      mi.vu,
	}

	resp, err := client.doComplete(call.Context(), &req)
	if err != nil {
		return rt.ToValue(err)
	}

	return rt.ToValue(resp)
}

