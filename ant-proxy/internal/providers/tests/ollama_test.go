package tests

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/tmc/misc/ant-proxy/internal/providers"
)

func TestOllamaProvider(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"response": "Hello, world!"}`))
    }))
    defer server.Close()

    provider := providers.NewOllamaProvider(server.URL)
    assert.Equal(t, "ollama", provider.Name())

    req := &providers.Request{
        Model: "llama2",
        Messages: []providers.Message{
            {Role: "user", Content: "Hello"},
        },
    }

    resp, err := provider.Generate(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, "Hello, world!", resp.Content)
}

