package llmperf

import (
    "fmt"
    "log"

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
            log.Printf("Creating new client")

            // Parse config from constructor argument
            var config Config
            if err := rt.ExportTo(call.Argument(0), &config); err != nil {
                common.Throw(rt, fmt.Errorf("invalid config: %w", err))
            }
            log.Printf("Client config: %+v", config)

            client := &Client{
                config:  config,
                metrics: mi.metrics,
                vu:      mi.vu,
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

func (mi *ModuleInstance) Exports() modules.Exports {
    return modules.Exports{
        Default: mi.exports,
    }
}
