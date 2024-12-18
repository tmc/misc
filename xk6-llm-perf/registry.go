package llmperf

import (
    "fmt"
    "sync"
)

var (
    providersMu sync.RWMutex
    providers   = make(map[string]ProviderFactory)
)

type ProviderFactory func(config map[string]interface{}) (LLMProvider, error)

func RegisterProvider(name string, factory ProviderFactory) {
    providersMu.Lock()
    defer providersMu.Unlock()
    if factory == nil {
        panic("llmperf: RegisterProvider factory is nil")
    }
    if _, dup := providers[name]; dup {
        panic("llmperf: RegisterProvider called twice for provider " + name)
    }
    providers[name] = factory
}

func GetProvider(name string, config map[string]interface{}) (LLMProvider, error) {
    providersMu.RLock()
    factoryFunc, ok := providers[name]
    providersMu.RUnlock()
    if !ok {
        return nil, fmt.Errorf("llmperf: unknown provider %q (forgotten import?)", name)
    }
    return factoryFunc(config)
}

