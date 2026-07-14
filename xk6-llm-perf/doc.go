// Package llmperf implements a k6 extension for benchmarking
// OpenAI-compatible chat completion APIs.
//
// The extension exposes k6/x/llm-perf to JavaScript tests. It records LLM
// serving metrics such as time to first token, time to first output token,
// inter-token latency, inter-chunk latency, token throughput, good-request SLO
// rate, provider usage discrepancies, and native k6 HTTP metrics.
//
// Build the extension with xk6:
//
//	xk6 build --with github.com/tmc/misc/xk6-llm-perf=.
package llmperf
