package llmperf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
		wantResp      *CompletionResponse
		wantErr       bool
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
			samples := make(chan metrics.SampleContainer, 20)
			client.metrics.samples = samples

			// Create runtime and function call
			rt := client.vu.Runtime()
			reqValue := rt.ToValue(tt.request)

			// Make request
			result := client.Complete(sobek.FunctionCall{
				Arguments: []sobek.Value{reqValue},
				This:      rt.ToValue(client),
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

			values := metricValues(samples)
			for _, name := range []string{
				"llm_request_latency",
				"llm_requests",
				"llm_prompt_tokens",
				"llm_completion_tokens",
				"llm_total_tokens",
				"llm_input_sequence_length",
				"llm_output_sequence_length",
				"llm_prompt_token_discrepancy",
				"llm_completion_token_discrepancy",
				"llm_usage_discrepancies",
			} {
				if _, ok := values[name]; !ok {
					t.Errorf("missing metric sample %q", name)
				}
			}
		})
	}
}

func TestClientStreamingCompletion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Accept"), "text/event-stream"; got != want {
			t.Errorf("Accept = %q; want %q", got, want)
		}
		var req CompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.StreamOptions == nil || !req.StreamOptions.IncludeUsage {
			t.Errorf("stream_options.include_usage not set")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" there\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"total_tokens\":12}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer ts.Close()

	client := setupTestClient(t, ts.URL)
	samples := make(chan metrics.SampleContainer, 100)
	client.metrics.samples = samples

	rt := client.vu.Runtime()
	result := client.Complete(sobek.FunctionCall{
		Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{
			Model:    "gpt-4",
			Messages: []Message{{Role: "user", Content: "Hello!"}},
			Stream:   true,
		})},
		This: rt.ToValue(client),
	})

	var got CompletionResponse
	if err := rt.ExportTo(result, &got); err != nil {
		t.Fatalf("export response: %v", err)
	}
	if got.Status != http.StatusOK {
		t.Fatalf("Status = %d; want %d", got.Status, http.StatusOK)
	}
	if got.Choices[0].Message.Content != "Hi there" {
		t.Fatalf("content = %q; want %q", got.Choices[0].Message.Content, "Hi there")
	}
	values := metricValues(samples)
	for _, name := range []string{
		"llm_ttft",
		"llm_ttfo",
		"llm_token_latency",
		"llm_inter_chunk_latency",
		"llm_request_latency",
		"llm_requests",
		"llm_good_request",
		"llm_completion_tokens",
		"llm_input_sequence_length",
		"llm_output_sequence_length",
		"llm_prompt_token_discrepancy",
		"llm_completion_token_discrepancy",
		"llm_usage_discrepancies",
	} {
		if _, ok := values[name]; !ok {
			t.Fatalf("missing metric sample %q", name)
		}
	}
	if got := values["llm_prompt_tokens"][0]; got != 10 {
		t.Fatalf("llm_prompt_tokens = %v; want 10", got)
	}
	if got := values["llm_completion_tokens"][0]; got != 2 {
		t.Fatalf("llm_completion_tokens = %v; want 2", got)
	}
}

func TestClientStreamingLongLine(t *testing.T) {
	longContent := strings.Repeat("x", 150*1024)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", longContent)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer ts.Close()

	client := setupTestClient(t, ts.URL)
	samples := make(chan metrics.SampleContainer, 100)
	client.metrics.samples = samples

	rt := client.vu.Runtime()
	result := client.Complete(sobek.FunctionCall{
		Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{
			Model:    "gpt-4",
			Messages: []Message{{Role: "user", Content: "Hello!"}},
			Stream:   true,
		})},
		This: rt.ToValue(client),
	})

	var got CompletionResponse
	if err := rt.ExportTo(result, &got); err != nil {
		t.Fatalf("export response: %v", err)
	}
	if got.Choices[0].Message.Content != longContent {
		t.Fatalf("content length = %d; want %d", len(got.Choices[0].Message.Content), len(longContent))
	}
}

func TestClientStreamingTruncated(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
	}))
	defer ts.Close()

	client := setupTestClient(t, ts.URL)
	samples := make(chan metrics.SampleContainer, 20)
	client.metrics.samples = samples

	rt := client.vu.Runtime()
	result := client.Complete(sobek.FunctionCall{
		Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{
			Model:    "gpt-4",
			Messages: []Message{{Role: "user", Content: "Hello!"}},
			Stream:   true,
		})},
		This: rt.ToValue(client),
	})

	err, ok := result.Export().(error)
	if !ok {
		t.Fatalf("Complete() returned %T; want error", result.Export())
	}
	if !strings.Contains(err.Error(), "stream ended before completion marker") {
		t.Fatalf("error = %q; want truncated stream error", err.Error())
	}
	if got := len(metricValues(samples)["llm_errors"]); got != 1 {
		t.Fatalf("llm_errors samples = %d; want 1", got)
	}
}

func TestClientErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer ts.Close()

	client := setupTestClient(t, ts.URL)
	rt := client.vu.Runtime()
	result := client.Complete(sobek.FunctionCall{
		Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{
			Model:    "gpt-4",
			Messages: []Message{{Role: "user", Content: "Hello!"}},
		})},
		This: rt.ToValue(client),
	})

	if _, ok := result.Export().(error); !ok {
		t.Fatalf("Complete() returned %T; want error", result.Export())
	}
}

func TestClientValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		request CompletionRequest
		want    string
	}{
		{
			name:    "missing base url",
			config:  Config{APIKey: "test-key", Model: "gpt-4"},
			request: CompletionRequest{Messages: []Message{{Role: "user", Content: "Hello!"}}},
			want:    "baseURL is required",
		},
		{
			name:    "bad base url scheme",
			config:  Config{BaseURL: "ftp://example.test", APIKey: "test-key", Model: "gpt-4"},
			request: CompletionRequest{Messages: []Message{{Role: "user", Content: "Hello!"}}},
			want:    "baseURL[0] must use http or https",
		},
		{
			name:    "missing model",
			config:  Config{BaseURL: "http://example.test", APIKey: "test-key"},
			request: CompletionRequest{Messages: []Message{{Role: "user", Content: "Hello!"}}},
			want:    "model is required",
		},
		{
			name:    "empty messages",
			config:  Config{BaseURL: "http://example.test", APIKey: "test-key", Model: "gpt-4"},
			request: CompletionRequest{},
			want:    "messages must not be empty",
		},
		{
			name:    "empty message content",
			config:  Config{BaseURL: "http://example.test", APIKey: "test-key", Model: "gpt-4"},
			request: CompletionRequest{Messages: []Message{{Role: "user"}}},
			want:    "messages[0].content is required",
		},
		{
			name:    "negative prefill concurrency",
			config:  Config{BaseURL: "http://example.test", APIKey: "test-key", Model: "gpt-4", PrefillConcurrency: -1},
			request: CompletionRequest{Messages: []Message{{Role: "user", Content: "Hello!"}}},
			want:    "prefillConcurrency must be non-negative",
		},
		{
			name:    "negative warmup count",
			config:  Config{BaseURL: "http://example.test", APIKey: "test-key", Model: "gpt-4", WarmupCount: -1},
			request: CompletionRequest{Messages: []Message{{Role: "user", Content: "Hello!"}}},
			want:    "warmupCount must be non-negative",
		},
		{
			name:    "negative network rtt",
			config:  Config{BaseURL: "http://example.test", APIKey: "test-key", Model: "gpt-4", NetworkRTT: -1},
			request: CompletionRequest{Messages: []Message{{Role: "user", Content: "Hello!"}}},
			want:    "networkRTT must be non-negative",
		},
		{
			name:    "negative token multiplier",
			config:  Config{BaseURL: "http://example.test", APIKey: "test-key", Model: "gpt-4", TokenMultiplier: -1},
			request: CompletionRequest{Messages: []Message{{Role: "user", Content: "Hello!"}}},
			want:    "tokenMultiplier must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := setupTestClient(t, "http://example.test")
			client.config = tt.config
			samples := make(chan metrics.SampleContainer, 4)
			client.metrics.samples = samples

			rt := client.vu.Runtime()
			result := client.Complete(sobek.FunctionCall{
				Arguments: []sobek.Value{rt.ToValue(&tt.request)},
				This:      rt.ToValue(client),
			})

			err, ok := result.Export().(error)
			if !ok {
				t.Fatalf("Complete() returned %T; want error", result.Export())
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q; want substring %q", err.Error(), tt.want)
			}
			values := metricValues(samples)
			if got := len(values["llm_errors"]); got != 1 {
				t.Fatalf("llm_errors samples = %d; want 1", got)
			}
		})
	}
}

func TestClientRoundRobinBaseURLs(t *testing.T) {
	var firstRequests int64
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&firstRequests, 1)
		writeCompletion(t, w)
	}))
	defer first.Close()

	var secondRequests int64
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&secondRequests, 1)
		writeCompletion(t, w)
	}))
	defer second.Close()

	client := setupTestClient(t, first.URL)
	client.config.BaseURL = ""
	client.config.BaseURLs = []string{first.URL, second.URL}
	client.config.Model = "gpt-4"
	samples := make(chan metrics.SampleContainer, 100)
	client.metrics.samples = samples

	rt := client.vu.Runtime()
	call := sobek.FunctionCall{
		Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{
			Messages: []Message{{Role: "user", Content: "Hello!"}},
		})},
		This: rt.ToValue(client),
	}
	for i := 0; i < 4; i++ {
		result := client.Complete(call)
		if _, ok := result.Export().(error); ok {
			t.Fatalf("Complete(%d) returned error: %v", i, result.Export())
		}
	}
	if got, want := atomic.LoadInt64(&firstRequests), int64(2); got != want {
		t.Fatalf("first requests = %d; want %d", got, want)
	}
	if got, want := atomic.LoadInt64(&secondRequests), int64(2); got != want {
		t.Fatalf("second requests = %d; want %d", got, want)
	}
}

func TestClientAdjustedLatencyMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		time.Sleep(3 * time.Millisecond)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
		time.Sleep(3 * time.Millisecond)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" there\"},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer ts.Close()

	client := setupTestClient(t, ts.URL)
	client.config.NetworkRTT = 1
	samples := make(chan metrics.SampleContainer, 20)
	client.metrics.samples = samples

	rt := client.vu.Runtime()
	result := client.Complete(sobek.FunctionCall{
		Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{
			Model:    "gpt-4",
			Messages: []Message{{Role: "user", Content: "Hello!"}},
			Stream:   true,
		})},
		This: rt.ToValue(client),
	})
	if _, ok := result.Export().(error); ok {
		t.Fatalf("Complete() returned error: %v", result.Export())
	}

	values := metricValues(samples)
	for _, name := range []string{
		"llm_ttft_adjusted",
		"llm_ttfo_adjusted",
		"llm_request_latency_adjusted",
	} {
		if _, ok := values[name]; !ok {
			t.Fatalf("missing metric sample %q", name)
		}
	}
}

func TestEstimateTokenCountUsesRobustFallback(t *testing.T) {
	if got, want := estimateTokenCount("hello world", 0), 3; got != want {
		t.Fatalf("estimateTokenCount words = %d; want %d", got, want)
	}
	if got, want := estimateTokenCount("abcdefghij", 0), 3; got != want {
		t.Fatalf("estimateTokenCount chars = %d; want %d", got, want)
	}
	if got, want := estimateTokenCount("hello world", 2), 4; got != want {
		t.Fatalf("estimateTokenCount multiplier = %d; want %d", got, want)
	}
}

func TestClientWarmupSuppressesMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CompletionResponse{
			Choices: []Choice{{Message: Message{Role: "assistant", Content: "ok"}}},
			Usage: Usage{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		})
	}))
	defer ts.Close()

	client := setupTestClient(t, ts.URL)
	client.config.WarmupCount = 1
	client.warmupRemaining = 1
	samples := make(chan metrics.SampleContainer, 20)
	client.metrics.samples = samples

	rt := client.vu.Runtime()
	call := sobek.FunctionCall{
		Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{
			Model:    "gpt-4",
			Messages: []Message{{Role: "user", Content: "Hello!"}},
		})},
		This: rt.ToValue(client),
	}
	for i := 0; i < 2; i++ {
		result := client.Complete(call)
		if _, ok := result.Export().(error); ok {
			t.Fatalf("Complete(%d) returned error: %v", i, result.Export())
		}
	}

	values := metricValues(samples)
	if got := len(values["llm_requests"]); got != 1 {
		t.Fatalf("llm_requests samples = %d; want 1", got)
	}
	if got := len(values["llm_completion_tokens"]); got != 1 {
		t.Fatalf("llm_completion_tokens samples = %d; want 1", got)
	}
}

func writeCompletion(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(CompletionResponse{
		Choices: []Choice{{Message: Message{Role: "assistant", Content: "ok"}}},
		Usage: Usage{
			PromptTokens:     1,
			CompletionTokens: 1,
			TotalTokens:      2,
		},
	}); err != nil {
		t.Fatalf("write completion: %v", err)
	}
}

func TestClientRecordsBuiltinHTTPMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CompletionResponse{
			Choices: []Choice{{Message: Message{Role: "assistant", Content: "ok"}}},
			Usage: Usage{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		})
	}))
	defer ts.Close()

	client := setupTestClient(t, ts.URL)
	samples := make(chan metrics.SampleContainer, 50)
	enableVUState(t, client, samples)

	rt := client.vu.Runtime()
	result := client.Complete(sobek.FunctionCall{
		Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{
			Model:    "gpt-4",
			Messages: []Message{{Role: "user", Content: "Hello!"}},
		})},
		This: rt.ToValue(client),
	})
	if _, ok := result.Export().(error); ok {
		t.Fatalf("Complete() returned error: %v", result.Export())
	}

	values := metricValues(samples)
	for _, name := range []string{
		"http_reqs",
		"http_req_duration",
		"http_req_waiting",
		"http_req_receiving",
		"http_req_failed",
		"llm_requests",
	} {
		if _, ok := values[name]; !ok {
			t.Fatalf("missing metric sample %q", name)
		}
	}
	if got := values["http_reqs"][0]; got != 1 {
		t.Fatalf("http_reqs = %v; want 1", got)
	}
	if got := values["http_req_failed"][0]; got != 0 {
		t.Fatalf("http_req_failed = %v; want 0", got)
	}
}

func TestClientWarmupSuppressesBuiltinHTTPMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CompletionResponse{
			Choices: []Choice{{Message: Message{Role: "assistant", Content: "ok"}}},
			Usage: Usage{
				PromptTokens:     1,
				CompletionTokens: 1,
				TotalTokens:      2,
			},
		})
	}))
	defer ts.Close()

	client := setupTestClient(t, ts.URL)
	client.config.WarmupCount = 1
	client.warmupRemaining = 1
	samples := make(chan metrics.SampleContainer, 50)
	enableVUState(t, client, samples)

	rt := client.vu.Runtime()
	call := sobek.FunctionCall{
		Arguments: []sobek.Value{rt.ToValue(&CompletionRequest{
			Model:    "gpt-4",
			Messages: []Message{{Role: "user", Content: "Hello!"}},
		})},
		This: rt.ToValue(client),
	}
	for i := 0; i < 2; i++ {
		result := client.Complete(call)
		if _, ok := result.Export().(error); ok {
			t.Fatalf("Complete(%d) returned error: %v", i, result.Export())
		}
	}

	values := metricValues(samples)
	if got := len(values["http_reqs"]); got != 1 {
		t.Fatalf("http_reqs samples = %d; want 1", got)
	}
	if got := len(values["llm_requests"]); got != 1 {
		t.Fatalf("llm_requests samples = %d; want 1", got)
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

func enableVUState(t *testing.T, client *Client, samples chan metrics.SampleContainer) {
	t.Helper()

	vu, ok := client.vu.(*modulestest.VU)
	if !ok {
		t.Fatalf("client VU = %T; want *modulestest.VU", client.vu)
	}
	registry := client.metrics.registry
	vu.StateField = &lib.State{
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Samples:        samples,
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
	}
	vu.InitEnvField = nil
	client.metrics.samples = samples
}

func metricValues(samples chan metrics.SampleContainer) map[string][]float64 {
	values := make(map[string][]float64)
	for _, sampleContainer := range metrics.GetBufferedSamples(samples) {
		for _, sample := range sampleContainer.GetSamples() {
			if sample.Metric != nil {
				values[sample.Metric.Name] = append(values[sample.Metric.Name], sample.Value)
			}
		}
	}
	return values
}
