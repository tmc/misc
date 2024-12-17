package llmperf

import (
    "time"

    "github.com/dop251/goja"
    "go.k6.io/k6/js/modules"
    "github.com/tmc/misc/xk6-llm-perf/llm"
)

type ModuleInstance struct {
    vu modules.VU
    metrics *llm.LLMMetrics
}

func New() modules.Module {
    return &ModuleInstance{}
}

func (mi *ModuleInstance) Exports() modules.Exports {
    return modules.Exports{
        Named: map[string]interface{}{
            "Client": mi.newClient,
        },
    }
}

// ClientOptions represents the JavaScript options object
type ClientOptions struct {
    BaseURL     string            `json:"baseURL"`
    Storage     llm.StorageConfig `json:"storage"`
    HTTPTimeout string            `json:"httpTimeout"`
    IsStreaming bool              `json:"isStreaming"`
}

func (mi *ModuleInstance) newClient(call goja.ConstructorCall) *goja.Object {
    rt := mi.vu.Runtime()

    // Register metrics if not already done
    if mi.metrics == nil {
        metrics, err := llm.registerMetrics(mi.vu)
        if err != nil {
            panic(rt.NewGoError(err))
        }
        mi.metrics = metrics
    }

    // Parse options
    var options ClientOptions
    if len(call.Arguments) > 0 {
        err := rt.ExportTo(call.Arguments[0], &options)
        if err != nil {
            panic(rt.NewGoError(err))
        }
    }

    // Parse timeout
    timeout := 30 * time.Second
    if options.HTTPTimeout != "" {
        var err error
        timeout, err = time.ParseDuration(options.HTTPTimeout)
        if err != nil {
            panic(rt.NewGoError(err))
        }
    }

    // Create client
    client, err := llm.NewClient(llm.ClientConfig{
        BaseURL:     options.BaseURL,
        Storage:     options.Storage,
        HTTPTimeout: timeout,
        IsStreaming: options.IsStreaming,
        Metrics:     mi.metrics,
    })
    if err != nil {
        panic(rt.NewGoError(err))
    }

    // Create wrapper object
    obj := rt.NewObject()
    must := func(err error) {
        if err != nil {
            panic(rt.NewGoError(err))
        }
    }

    // Add methods
    must(obj.Set("complete", func(call goja.FunctionCall) goja.Value {
        if len(call.Arguments) < 1 {
            panic(rt.NewGoError(errors.New("missing request parameters")))
        }

        var req llm.CompletionRequest
        err := rt.ExportTo(call.Arguments[0], &req)
        if err != nil {
            panic(rt.NewGoError(err))
        }

        resp, err := client.Complete(mi.vu.Context(), &req)
        if err != nil {
            panic(rt.NewGoError(err))
        }

        return rt.ToValue(resp)
    }))

    return obj
}
