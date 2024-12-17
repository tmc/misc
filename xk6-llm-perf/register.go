package llmperf

import (
    "go.k6.io/k6/js/modules"
)

func init() {
    modules.Register("k6/x/llm-perf", new(LLMPerf))
}

type LLMPerf struct{}

func (*LLMPerf) NewModuleInstance(vu modules.VU) interface{} {
    return &ModuleInstance{vu: vu}
}
