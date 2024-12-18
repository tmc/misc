package llmperf

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "net/url"
    "testing"

    "github.com/grafana/sobek"
    "github.com/tmc/misc/xk6-llm-perf/testutils"
    "go.k6.io/k6/js/common"
    "go.k6.io/k6/js/modulestest"
    "go.k6.io/k6/lib"
    "go.k6.io/k6/metrics"
)

func TestClientAPI(t *testing.T) {
    tests := []struct {
        name          string
        serverHandler http.HandlerFunc
        request       *CompletionRequest
        wantResp     *CompletionResponse
        wantErr      bool
    }{
        {
            name: "basic completion",
            serverHandler: func(w http.ResponseWriter, r *http.Request) {
                if got, want := r.Method, "POST"; got != want {
                    t.Errorf("Method = %q; want %q", got, want)
                }
                if got, want := r.URL.Path, "/chat/completions"; got != want {
                    t.Errorf("Path = %q; want %q", got, want)
                }
                if got, want := r.Header.Get("Authorization"), "Bearer test-key"; got != want {
                    t.Errorf("Authorization = %q; want %q", got, want)
                }

                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusOK)
                json.NewEncoder(w).Encode(CompletionResponse{
                    ID:      "test-completion-id",
                    Object:  "chat.completion",
                    Created: 1703123456,
                    Model:   "gpt-4",
                    Choices: []Choice{
                        {
                            Message: Message{
                                Role:    "assistant",
                                Content: "Hi there!",
                            },
                            FinishReason: "stop",
                        },
                    },
                    Usage: Usage{
                        PromptTokens:     10,
                        CompletionTokens: 20,
                        TotalTokens:      30,
                    },
                })
            },
            request: &CompletionRequest{
                Messages: []Message{{Role: "user", Content: "Hello!"}},
                Model:    "gpt-4",
            },
            wantResp: &CompletionResponse{
                Status: 200,
                ID:     "test-completion-id",
                Object: "chat.completion",
                Model:  "gpt-4",
                Choices: []Choice{
                    {
                        Message: Message{
                            Role:    "assistant",
                            Content: "Hi there!",
                        },
                        FinishReason: "stop",
                    },
                },
                Usage: Usage{
                    PromptTokens:     10,
                    CompletionTokens: 20,
                    TotalTokens:      30,
                },
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup test server
            ts := httptest.NewServer(tt.serverHandler)
            defer ts.Close()

            // Setup test VU and client
            client := setupTestClient(t, ts.URL)

            // Create runtime and function call
            rt := client.vu.Runtime()
            reqValue := rt.ToValue(tt.request)

            // Make request
            result := client.Complete(sobek.FunctionCall{
                Arguments: []sobek.Value{reqValue},
                This:     rt.ToValue(client),
            })

            // Check for error
            if tt.wantErr {
                if _, ok := result.(error); !ok {
                    t.Errorf("Complete() expected error, got: %v", result)
                }
                return
            }

            // Parse response
            var got CompletionResponse
            if err := rt.ExportTo(result, &got); err != nil {
                t.Fatalf("Failed to export response: %v", err)
            }

            // Compare fields
            if got.Status != tt.wantResp.Status {
                t.Errorf("Status = %d; want %d", got.Status, tt.wantResp.Status)
            }
            if got.ID != tt.wantResp.ID {
                t.Errorf("ID = %q; want %q", got.ID, tt.wantResp.ID)
            }
            if got.Object != tt.wantResp.Object {
                t.Errorf("Object = %q; want %q", got.Object, tt.wantResp.Object)
            }
            if got.Model != tt.wantResp.Model {
                t.Errorf("Model = %q; want %q", got.Model, tt.wantResp.Model)
            }
            if len(got.Choices) != len(tt.wantResp.Choices) {
                t.Errorf("len(Choices) = %d; want %d", len(got.Choices), len(tt.wantResp.Choices))
            } else if len(got.Choices) > 0 {
                if got.Choices[0].Message.Role != tt.wantResp.Choices[0].Message.Role {
                    t.Errorf("Choices[0].Message.Role = %q; want %q", got.Choices[0].Message.Role, tt.wantResp.Choices[0].Message.Role)
                }
                if got.Choices[0].Message.Content != tt.wantResp.Choices[0].Message.Content {
                    t.Errorf("Choices[0].Message.Content = %q; want %q", got.Choices[0].Message.Content, tt.wantResp.Choices[0].Message.Content)
                }
            }
        })
    }
}

func setupTestClient(t *testing.T, serverURL string) *Client {
    t.Helper()

    registry := metrics.NewRegistry()
    logger := testutils.NewLogger(t)

    cwd, err := url.Parse("file://" + t.TempDir())
    if err != nil {
        t.Fatalf("Failed to create temp dir URL: %v", err)
    }

    rt := sobek.New()
    rt.SetFieldNameMapper(common.FieldNameMapper{})

    // Create a new VU
    vu := &modulestest.VU{
        RuntimeField: rt,
        InitEnvField: &common.InitEnvironment{
            TestPreInitState: &lib.TestPreInitState{
                Logger:   logger,
                Registry: registry,
            },
            CWD: cwd,
        },
        CtxField: context.Background(),
    }

    return &Client{
        config: Config{
            BaseURL: serverURL,
            APIKey:  "test-key",
        },
        metrics: NewMetrics(registry),
        vu:      vu,
    }
}
